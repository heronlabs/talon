//go:build integration

// Package cli runs the talon binary built by `make test:integration`, so
// integration test packages can exercise the real CLI.
package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// exitTimeout is how long Wait waits for a started process to exit before it
// fails the test.
const exitTimeout = 30 * time.Second

// Process is a binary started by Start, running in the background until it
// exits or the test ends.
type Process struct {
	cmd    *exec.Cmd
	output bytes.Buffer
	exited chan struct{}
	err    error
}

// Run runs the binary at TALON_BIN with configDir as its configuration
// directory, from an empty working directory and with no input, and returns
// its combined output and exit code.
func Run(t *testing.T, configDir string, args ...string) (string, int) {
	t.Helper()
	return RunWithInput(t, configDir, "", args...)
}

// RunWithInput runs the binary like Run, with input as its standard input.
func RunWithInput(t *testing.T, configDir, input string, args ...string) (string, int) {
	t.Helper()
	cmd := command(t, configDir, input, args...)

	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return string(out), 0
}

// Start starts the binary like Run, without waiting for it, and returns the
// running process; a process still running when the test ends is killed.
func Start(t *testing.T, configDir string, args ...string) *Process {
	t.Helper()
	process := &Process{cmd: command(t, configDir, "", args...), exited: make(chan struct{})}
	process.cmd.Stdout = &process.output
	process.cmd.Stderr = &process.output

	if err := process.cmd.Start(); err != nil {
		t.Fatalf("start %v: %v", args, err)
	}

	go func() {
		process.err = process.cmd.Wait()
		close(process.exited)
	}()
	t.Cleanup(func() {
		_ = process.cmd.Process.Kill()
		<-process.exited
	})

	return process
}

// Signal sends signal to the process.
func (p *Process) Signal(t *testing.T, signal os.Signal) {
	t.Helper()
	if err := p.cmd.Process.Signal(signal); err != nil {
		t.Fatalf("signal %v: %v", signal, err)
	}
}

// Wait waits for the process to exit, failing the test after exitTimeout, and
// returns its combined output and exit code.
func (p *Process) Wait(t *testing.T) (string, int) {
	t.Helper()
	select {
	case <-p.exited:
	case <-time.After(exitTimeout):
		t.Fatalf("the process did not exit within %v", exitTimeout)
	}

	var exitErr *exec.ExitError
	if errors.As(p.err, &exitErr) {
		return p.output.String(), exitErr.ExitCode()
	}
	if p.err != nil {
		t.Fatalf("wait: %v", p.err)
	}
	return p.output.String(), 0
}

// command returns the command that runs the binary at TALON_BIN with args,
// configDir as its configuration directory, an empty working directory and
// input as its standard input.
func command(t *testing.T, configDir, input string, args ...string) *exec.Cmd {
	t.Helper()
	binPath := os.Getenv("TALON_BIN")
	if binPath == "" {
		t.Fatal("TALON_BIN is not set: run `make test:integration`")
	}

	cmd := exec.Command(binPath, args...) //nolint:gosec // G204: test harness runs the binary built by `make test:integration`
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "TALON_CONFIG_DIR="+configDir)
	cmd.Stdin = strings.NewReader(input)

	return cmd
}
