package application

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type fakeReport struct {
	err error
}

func (f fakeReport) Send() error {
	return f.err
}

func TestReportCommand(t *testing.T) {
	t.Run("Should return 0 when the report is sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(fakeReport{}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run()

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log status=ok when the report is sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(fakeReport{}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run()

		if !strings.Contains(output.String(), "level=INFO msg=report status=ok") {
			t.Errorf("Run() logged %q, want %q", output.String(), "level=INFO msg=report status=ok")
		}
	})

	t.Run("Should return 1 when the report cannot be sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(fakeReport{err: errors.New("send failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run()

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the report cannot be sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(fakeReport{err: errors.New("send failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run()

		if !strings.Contains(output.String(), `level=ERROR msg=report error="send failed"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=report error="send failed"`)
		}
	})
}
