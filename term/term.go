//go:build !windows

package term

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// if ENV: NO_COLOR is not empty, will disable color render.
	NoColorFlag = os.Getenv("NO_COLOR") == ""
)

const (
	MaxScannerBufferCapacity = 256 * 1024
	Esc                      = "\u001B["
	Osc                      = "\u001B]"
	Bel                      = "\u0007"
	Rst                      = Esc + "0m"
)

type TermColor string

const (
	// Foreground Colors
	FgBlack     TermColor = Esc + "30m"
	FgRed       TermColor = Esc + "31m"
	FgGreen     TermColor = Esc + "32m"
	FgYellow    TermColor = Esc + "33m"
	FgBlue      TermColor = Esc + "34m"
	FgMagenta   TermColor = Esc + "35m"
	FgCyan      TermColor = Esc + "36m"
	FgWhite     TermColor = Esc + "37m"
	FgHiBlack   TermColor = Esc + "30;1m"
	FgHiRed     TermColor = Esc + "31;1m"
	FgHiGreen   TermColor = Esc + "32;1m"
	FgHiYellow  TermColor = Esc + "33;1m"
	FgHiBlue    TermColor = Esc + "34;1m"
	FgHiMagenta TermColor = Esc + "35;1m"
	FgHiCyan    TermColor = Esc + "36;1m"
	FgHiWhite   TermColor = Esc + "37;1m"
	FgDefault   TermColor = Esc + "39;1m"

	// Background Colors
	BgBlack     TermColor = Esc + "40m"
	BgRed       TermColor = Esc + "41m"
	BgGreen     TermColor = Esc + "42m"
	BgYellow    TermColor = Esc + "43m"
	BgBlue      TermColor = Esc + "44m"
	BgMagenta   TermColor = Esc + "45m"
	BgCyan      TermColor = Esc + "46m"
	BgWhite     TermColor = Esc + "47m"
	BgHiBlack   TermColor = Esc + "40;1m"
	BgHiRed     TermColor = Esc + "41;1m"
	BgHiGreen   TermColor = Esc + "42;1m"
	BgHiYellow  TermColor = Esc + "43;1m"
	BgHiBlue    TermColor = Esc + "44;1m"
	BgHiMagenta TermColor = Esc + "45;1m"
	BgHiCyan    TermColor = Esc + "46;1m"
	BgHiWhite   TermColor = Esc + "47;1m"
	BgDefault   TermColor = Esc + "49;1m"
)

func renderCode(color string, args ...interface{}) string {
	var message string
	if ln := len(args); ln == 0 {
		return ""
	}
	message = fmt.Sprint(args...)
	if len(color) == 0 {
		return message
	}
	if NoColorFlag {
		return fmt.Sprintf("%s%s%s", color, message, Rst)
	}
	return message
}

func (tc TermColor) String() string {
	return string(tc)
}

func (tc TermColor) Printf(format string, args ...interface{}) {
	fmt.Println(renderCode(tc.String(), fmt.Sprintf(format, args...)))
}

func (tc TermColor) Println(args ...interface{}) {
	fmt.Println(renderCode(tc.String(), args...))
}

func (tc TermColor) Render(args ...interface{}) string {
	return renderCode(tc.String(), args...)
}

func Errorln(err error) {
	FgHiRed.Printf("~~~ %v\n", err)
}

func Warnln(line string) {
	FgRed.Printf("%%%%%% %v\n", line)
}

func Infoln(line string) {
	FgGreen.Println(line)
}

func Startln(command string) {
	yellow := FgYellow.Render
	fmt.Printf("=== Executing: '%s'\n", yellow(command))
}

func StartlnWithTime(command string, args ...string) time.Time {
	now := time.Now()
	yellow := FgYellow.Render
	if len(args) > 0 {
		fmt.Printf("=== Executing: '%s' at %s\n", yellow(command, args), yellow(now.Format("15:04:05")))
		return now
	}
	fmt.Printf("=== Executing: '%s' at %s\n", yellow(command), yellow(now.Format("15:04:05")))
	return now
}

func passedFailStatus(passedFlag bool) string {
	red := FgHiRed.Render
	green := FgHiGreen.Render
	status := red("✗")
	if passedFlag {
		status = green("√")
	}
	return status
}

func EndlnWithTime(duration time.Duration, passedFlag bool) {
	yellow := FgYellow.Render
	status := passedFailStatus(passedFlag)
	fmt.Printf("=== %s End: %s, Total: %s\n", status, yellow(time.Now().Format("15:04:05")), yellow(duration.String()))
}

func Endln(passedFlag bool) {
	status := passedFailStatus(passedFlag)
	fmt.Printf("=== %s Done.\n", status)
}

func DisplayLn(reader io.Reader, wg *sync.WaitGroup, displayFn func(string)) {
	r := bufio.NewReader(reader)
	scanner := bufio.NewScanner(r)
	buf := make([]byte, MaxScannerBufferCapacity)
	scanner.Buffer(buf, MaxScannerBufferCapacity)
	for scanner.Scan() {
		displayFn(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		Errorln(fmt.Errorf("SCANNER failed to read from reader %w", err))
	}
	wg.Done()
}

func echo(on bool) error {
	// Common settings and variables for both stty calls.
	attrs := syscall.ProcAttr{
		Dir:   "",
		Env:   []string{},
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Sys:   nil}
	var ws syscall.WaitStatus
	cmd := "echo"
	if !on {
		cmd = "-echo"
	}

	// Enable/disable echoing.
	pid, err := syscall.ForkExec(
		"/bin/stty",
		[]string{"stty", cmd},
		&attrs)
	if err != nil {
		return fmt.Errorf("stty fork: %w", err)
	}

	// Wait for the stty process to complete.
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return fmt.Errorf("stty wait: %w", err)
	}
	return nil
}

// PasswordPromptContext writes prompt to stdout, disables terminal echo, and
// reads a line from stdin. Echo is restored before returning. If ctx is
// canceled while waiting for input the function returns ctx.Err(); the caller
// is responsible for any signal-to-cancel wiring (e.g. signal.NotifyContext
// with os.Interrupt).
func PasswordPromptContext(ctx context.Context, prompt string) (string, error) {
	fmt.Print(prompt)
	if err := echo(false); err != nil {
		return "", fmt.Errorf("disable echo: %w", err)
	}
	defer func() {
		// Best-effort echo restore; the caller already has a value (or error),
		// and there is no useful action to take if restore fails.
		_ = echo(true)
		fmt.Println("")
	}()

	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := bufio.NewReader(os.Stdin).ReadString('\n')
		done <- result{text: text, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-done:
		if r.err != nil {
			return "", fmt.Errorf("read password: %w", r.err)
		}
		return strings.TrimSpace(r.text), nil
	}
}

// PasswordPrompt is a context-less wrapper around PasswordPromptContext that
// installs an os.Interrupt handler and exits the process on ^C or read error.
//
// Deprecated: use PasswordPromptContext, which returns errors instead of
// terminating the process and lets the caller wire signal handling.
func PasswordPrompt(prompt string) string {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	text, err := PasswordPromptContext(ctx, prompt)
	cancel()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\n^C interrupt.")
		} else {
			fmt.Println("ERROR:", err.Error())
		}
		os.Exit(1)
	}
	return text
}

func YesNoPrompt(label string, def bool) bool {
	choices := "Y/n"
	if !def {
		choices = "y/N"
	}
	r := bufio.NewReader(os.Stdin)
	var s string
	for {
		fmt.Fprintf(os.Stderr, "%s (%s) ", label, choices)
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		s = strings.ToLower(s)
		if s == "y" || s == "yes" {
			return true
		}
		if s == "n" || s == "no" {
			return false
		}
	}
}
