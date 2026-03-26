package progressbar

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultWidth   = 40
	redrawInterval = 100 * time.Millisecond
)

// Option configures a Bar.
type Option func(*Bar)

// WithBlockChars uses solid block characters for the progress bar fill.
func WithBlockChars() Option {
	return func(b *Bar) {
		b.fill = "\u2588"
		b.head = "\u2588"
		b.empty = " "
	}
}

// WithSpeed enables transfer speed display.
func WithSpeed() Option {
	return func(b *Bar) { b.showSpeed = true }
}

// WithETA enables elapsed and remaining time display.
func WithETA() Option {
	return func(b *Bar) { b.showETA = true }
}

// WithWidth sets the bar width in characters.
func WithWidth(n int) Option {
	return func(b *Bar) { b.width = n }
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Option {
	return func(b *Bar) { b.output = w }
}

type Bar struct {
	total       int64
	current     int64
	desc        string
	width       int
	output      io.Writer
	startTime   time.Time
	lastDraw    time.Time
	formatValue func(int64) string
	formatSpeed func(float64) string
	fill        string
	head        string
	empty       string
	showSpeed   bool
	showETA     bool
	mu          sync.Mutex
}

// DefaultBytes creates a progress bar for tracking byte transfers.
// The returned Bar implements io.Writer and renders to stderr.
func DefaultBytes(total int64, description string, opts ...Option) *Bar {
	b := &Bar{
		total:       total,
		desc:        description,
		width:       defaultWidth,
		output:      os.Stderr,
		startTime:   time.Now(),
		formatValue: formatBytes,
		formatSpeed: func(s float64) string { return formatBytes(int64(s)) + "/s" },
		fill:        "=",
		head:        ">",
		empty:       " ",
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// DefaultCount creates a progress bar for tracking counted items.
// Use Set or Add to update progress. Renders to stderr.
func DefaultCount(total int64, description string, opts ...Option) *Bar {
	b := &Bar{
		total:       total,
		desc:        description,
		width:       defaultWidth,
		output:      os.Stderr,
		startTime:   time.Now(),
		formatValue: formatCount,
		formatSpeed: func(s float64) string { return fmt.Sprintf("%.0f/s", s) },
		fill:        "=",
		head:        ">",
		empty:       " ",
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Set sets the current progress value.
func (b *Bar) Set(n int64) {
	b.mu.Lock()
	b.current = n
	now := time.Now()
	shouldDraw := now.Sub(b.lastDraw) >= redrawInterval
	b.mu.Unlock()
	if shouldDraw {
		b.render()
	}
}

// Add increments the current progress by n and returns the new value.
func (b *Bar) Add(n int64) int64 {
	b.mu.Lock()
	b.current += n
	current := b.current
	now := time.Now()
	shouldDraw := now.Sub(b.lastDraw) >= redrawInterval
	b.mu.Unlock()
	if shouldDraw {
		b.render()
	}
	return current
}

// Write implements io.Writer by adding len(p) to the current progress.
func (b *Bar) Write(p []byte) (int, error) {
	n := len(p)
	b.mu.Lock()
	b.current += int64(n)
	now := time.Now()
	shouldDraw := now.Sub(b.lastDraw) >= redrawInterval
	b.mu.Unlock()
	if shouldDraw {
		b.render()
	}
	return n, nil
}

// Close finalizes the progress bar with a final render and newline.
func (b *Bar) Close() error {
	b.render()
	fmt.Fprintln(b.output)
	return nil
}

func (b *Bar) render() {
	b.mu.Lock()
	current := b.current
	total := b.total
	start := b.startTime
	b.lastDraw = time.Now()
	b.mu.Unlock()

	pct := float64(0)
	if total > 0 {
		pct = float64(current) / float64(total)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	filled := int(pct * float64(b.width))
	var bar strings.Builder
	bar.Grow(b.width)
	for i := range b.width {
		if i < filled {
			bar.WriteString(b.fill)
		} else if i == filled && filled < b.width {
			bar.WriteString(b.head)
		} else {
			bar.WriteString(b.empty)
		}
	}

	values := b.formatValue(current) + " / " + b.formatValue(total)

	if b.showSpeed || b.showETA {
		elapsed := time.Since(start)
		var extra strings.Builder
		extra.WriteString("(")
		extra.WriteString(values)
		if b.showSpeed {
			secs := elapsed.Seconds()
			if secs >= 0.001 {
				speed := float64(current) / secs
				extra.WriteString(", ")
				extra.WriteString(b.formatSpeed(speed))
			}
		}
		extra.WriteString(")")
		if b.showETA && current > 0 && current < total {
			remaining := time.Duration(float64(elapsed) * float64(total-current) / float64(current))
			extra.WriteString(" [")
			extra.WriteString(formatDuration(elapsed))
			extra.WriteString(":")
			extra.WriteString(formatDuration(remaining))
			extra.WriteString("]")
		}
		values = extra.String()
	}

	fmt.Fprintf(b.output, "\r%s %3.0f%% |%s| %s",
		b.desc,
		pct*100,
		bar.String(),
		values,
	)
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatCount(n int64) string {
	return strconv.FormatInt(n, 10)
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s %= 60
	return fmt.Sprintf("%dm%ds", m, s)
}
