//go:build mage

package main

import (
	"errors"
	"os"
	"os/exec"
)

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// Test runs the unit test suite.
func Test() error {
	return run("go", "test", "-race", "-count=1", "./...")
}

// Integration runs integration tests against real backends.
// Requires STORAGE_TEST_BUCKET and Application Default Credentials.
func Integration() error {
	if os.Getenv("STORAGE_TEST_BUCKET") == "" {
		return errors.New("STORAGE_TEST_BUCKET is required (e.g. export STORAGE_TEST_BUCKET=my-bucket)")
	}
	return run("go", "test", "-tags=integration", "-count=1", "-v",
		"-run=Integration", "./exp/storage/...")
}

// Lint runs golangci-lint across the module.
func Lint() error {
	return run("golangci-lint", "run", "--timeout=5m", "./...")
}

// Sec runs gosec across the module.
func Sec() error {
	return run("gosec", "./...")
}
