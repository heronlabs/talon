package application

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type fakeSetup struct {
	err error
}

func (f fakeSetup) Configure() error {
	return f.err
}

func TestSetupCommand(t *testing.T) {
	t.Run("Should return 0 when the configuration is written", func(t *testing.T) {
		var output bytes.Buffer
		command := NewSetupCommand(fakeSetup{}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run()

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log status=ok when the configuration is written", func(t *testing.T) {
		var output bytes.Buffer
		command := NewSetupCommand(fakeSetup{}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run()

		if !strings.Contains(output.String(), "level=INFO msg=setup status=ok") {
			t.Errorf("Run() logged %q, want %q", output.String(), "level=INFO msg=setup status=ok")
		}
	})

	t.Run("Should return 1 when the configuration cannot be written", func(t *testing.T) {
		var output bytes.Buffer
		command := NewSetupCommand(fakeSetup{err: errors.New("configure failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run()

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the configuration cannot be written", func(t *testing.T) {
		var output bytes.Buffer
		command := NewSetupCommand(fakeSetup{err: errors.New("configure failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run()

		if !strings.Contains(output.String(), `level=ERROR msg=setup error="configure failed"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=setup error="configure failed"`)
		}
	})
}
