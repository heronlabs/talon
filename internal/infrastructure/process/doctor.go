// Package process runs other programs: the installed binary's doctor and the system's account tools.
package process

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Doctor runs "<Binary> doctor --config <ConfigPath>" and gives it Timeout to finish.
type Doctor struct {
	Binary     string
	ConfigPath string
	Timeout    time.Duration
}

// Check implements port.HealthCheck; the output holds stdout and stderr.
func (d Doctor) Check(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()
	//nolint:gosec // Binary is the fixed install path set in main, not user input
	out, err := exec.CommandContext(ctx, d.Binary, "doctor", "--config", d.ConfigPath).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("doctor: %w", err)
	}
	return string(out), nil
}

// Exec runs name with args and returns an error holding its output when it fails.
func Exec(ctx context.Context, name string, args ...string) error {
	//nolint:gosec // callers pass fixed program names and arguments
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, bytes.TrimSpace(out))
	}
	return nil
}
