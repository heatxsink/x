package progressbar

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultWidth   = 40
	redrawInterval = 100 * time.Millisecond
	fillChar       = "="
	headChar       = ">"
	emptyChar      = " "
)

type Bar struct {
	total     int64
	current   int64
	desc      string
	width     int
	output    io.Writer
	startTime time.Time
	lastDraw  time.Time
	mu        sync.Mutex
}

// DefaultBytes creates a progress bar for tracking byte transfers.
// The returned Bar implements io.Writer and renders to stderr.
func DefaultBytes(total int64, description string) *Bar {
	return &Bar{
		total:     total,
		desc:      description,
		width:     defaultWidth,
		output:    os.Stderr,
		startTime: time.Now(),
	}
}

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

func (b *Bar) Close() error {
	b.render()
	fmt.Fprintln(b.output)
	return nil
}

func (b *Bar) render() {
	b.mu.Lock()
	current := b.current
	total := b.total
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
			bar.WriteString(fillChar)
		} else if i == filled && filled < b.width {
			bar.WriteString(headChar)
		} else {
			bar.WriteString(emptyChar)
		}
	}

	fmt.Fprintf(b.output, "\r%s %3.0f%% |%s| %s / %s",
		b.desc,
		pct*100,
		bar.String(),
		formatBytes(current),
		formatBytes(total),
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
