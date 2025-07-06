package term

import (
	"bufio"
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
		Errorln(fmt.Errorf("SCANNER failed to read from reader %s", err))
	}
	wg.Done()
}

func echo(on bool) {
	// Common settings and variables for both stty calls.
	attrs := syscall.ProcAttr{
		Dir:   "",
		Env:   []string{},
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Sys:   nil}
	var ws syscall.WaitStatus
	cmd := "echo"
	if on == false {
		cmd = "-echo"
	}

	// Enable/disable echoing.
	pid, err := syscall.ForkExec(
		"/bin/stty",
		[]string{"stty", cmd},
		&attrs)
	if err != nil {
		panic(err)
	}

	// Wait for the stty process to complete.
	_, err = syscall.Wait4(pid, &ws, 0, nil)
	if err != nil {
		panic(err)
	}
}

func PasswordPrompt(prompt string) string {
	fmt.Print(prompt)
	// Catch a ^C interrupt.
	// Make sure that we reset term echo before exiting.
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	go func() {
		for range signalChannel {
			fmt.Println("\n^C interrupt.")
			echo(true)
			os.Exit(1)
		}
	}()
	// Echo is disabled, now grab the data.
	echo(false) // disable terminal echo
	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')
	echo(true) // always re-enable terminal echo
	fmt.Println("")
	if err != nil {
		// The terminal has been reset, go ahead and exit.
		fmt.Println("ERROR:", err.Error())
		os.Exit(1)
	}
	return strings.TrimSpace(text)
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
