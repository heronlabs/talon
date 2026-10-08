package application

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	t.Run("Should return 0", func(t *testing.T) {
		var output bytes.Buffer
		command := NewVersionCommand("1.0.0", slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run()

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log the version it was built with", func(t *testing.T) {
		var output bytes.Buffer
		command := NewVersionCommand("1.0.0", slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run()

		if !strings.Contains(output.String(), "version=1.0.0") {
			t.Errorf("Run() logged %q, want %q", output.String(), "version=1.0.0")
		}
	})
}
