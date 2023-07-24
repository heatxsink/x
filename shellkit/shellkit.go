package shellkit

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/heatxsink/experimental/termkit"
)

func ExecuteWith(env map[string]string, cmd string, args ...string) {
	execute(env, cmd, args...)
}

func Execute(cmd string, args ...string) error {
	var env map[string]string
	return execute(env, cmd, args...)
}

func execute(env map[string]string, command string, args ...string) error {
	start := termkit.StartlnWithTime(command, args...)
	c := exec.Command(command, args...)
	c.Env = os.Environ()
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}
	stdout, err := c.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout. %v", err)
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr: %v", err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go termkit.DisplayLn(stdout, &wg, func(line string) {
		termkit.Infoln(line)
	})
	wg.Add(1)
	go termkit.DisplayLn(stderr, &wg, func(line string) {
		termkit.Warnln(line)
	})
	err = c.Run()
	wg.Wait()
	if err != nil {
		termkit.Errorln(err)
		termkit.EndlnWithTime(time.Since(start), false)
		return err
	}
	termkit.EndlnWithTime(time.Since(start), true)
	return nil
}
