//go:build integration

// Package cli runs the talon binary built by `make test:integration`, so
// integration test packages can exercise the real CLI.
package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Run runs the binary at TALON_BIN with configDir as its configuration
// directory, from an empty working directory, and returns its combined output
// and exit code. TALON_AUTHORIZED_KEYS defaults to a file in an empty
// temporary directory, so a run never touches the host's; a test sets it with
// t.Setenv.
func Run(t *testing.T, configDir string, args ...string) (string, int) {
	t.Helper()
	binPath := os.Getenv("TALON_BIN")
	if binPath == "" {
		t.Fatal("TALON_BIN is not set: run `make test:integration`")
	}

	cmd := exec.Command(binPath, args...) //nolint:gosec // G204: test harness runs the binary built by `make test:integration`
	cmd.Dir = t.TempDir()
	// When a key repeats, the last value wins: the default goes before the environment.
	cmd.Env = append([]string{"TALON_AUTHORIZED_KEYS=" + filepath.Join(t.TempDir(), "authorized_keys")}, os.Environ()...)
	cmd.Env = append(cmd.Env, "TALON_CONFIG_DIR="+configDir)

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
