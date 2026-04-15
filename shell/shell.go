package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/heatxsink/x/term"
)

// ExecuteWithContext runs cmd with the given env overlay and the supplied
// context, so the caller can cancel or set a deadline on the child process.
func ExecuteWithContext(ctx context.Context, env map[string]string, cmd string, args ...string) error {
	return execute(ctx, env, cmd, args...)
}

// ExecuteContext runs cmd under the supplied context.
func ExecuteContext(ctx context.Context, cmd string, args ...string) error {
	return execute(ctx, nil, cmd, args...)
}

// ExecuteWith is a context-less wrapper around ExecuteWithContext.
//
// Deprecated: use ExecuteWithContext.
func ExecuteWith(env map[string]string, cmd string, args ...string) error {
	return execute(context.Background(), env, cmd, args...)
}

// Execute is a context-less wrapper around ExecuteContext.
//
// Deprecated: use ExecuteContext.
func Execute(cmd string, args ...string) error {
	return execute(context.Background(), nil, cmd, args...)
}

func execute(ctx context.Context, env map[string]string, command string, args ...string) error {
	start := term.StartlnWithTime(command, args...)
	c := exec.CommandContext(ctx, command, args...)
	c.Env = os.Environ()
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout. %w", err)
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr: %w", err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go term.DisplayLn(stdout, &wg, func(line string) {
		term.Infoln(line)
	})
	wg.Add(1)
	go term.DisplayLn(stderr, &wg, func(line string) {
		term.Warnln(line)
	})
	err = c.Run()
	wg.Wait()
	if err != nil {
		term.Errorln(err)
		term.EndlnWithTime(time.Since(start), false)
		return err
	}
	term.EndlnWithTime(time.Since(start), true)
	return nil
}
