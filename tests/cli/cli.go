//go:build integration

// Package cli runs the vps-agent binary built by `make test:integration`, so
// integration test packages can exercise the real CLI.
package cli

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// Run runs the binary at VPS_AGENT_BIN inside dir and returns its combined
// output and exit code.
func Run(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	binPath := os.Getenv("VPS_AGENT_BIN")
	if binPath == "" {
		t.Fatal("VPS_AGENT_BIN is not set: run `make test:integration`")
	}

	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir

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
