package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type fakeDoctor struct {
	ctx context.Context
	err error
}

func (f *fakeDoctor) Check(ctx context.Context) error {
	f.ctx = ctx

	return f.err
}

func TestDoctorCommand(t *testing.T) {
	t.Run("Should return 0 when the check passes", func(t *testing.T) {
		var output bytes.Buffer
		command := NewDoctorCommand(&fakeDoctor{}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(t.Context())

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log status=ok when the check passes", func(t *testing.T) {
		var output bytes.Buffer
		command := NewDoctorCommand(&fakeDoctor{}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), "level=INFO msg=doctor status=ok") {
			t.Errorf("Run() logged %q, want %q", output.String(), "level=INFO msg=doctor status=ok")
		}
	})

	t.Run("Should check with the ctx it was given", func(t *testing.T) {
		var output bytes.Buffer
		ctx := t.Context()
		doctor := &fakeDoctor{}
		command := NewDoctorCommand(doctor, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if doctor.ctx != ctx {
			t.Errorf("Run() checked with %v, want %v", doctor.ctx, ctx)
		}
	})

	t.Run("Should return 1 when the check fails", func(t *testing.T) {
		var output bytes.Buffer
		command := NewDoctorCommand(&fakeDoctor{err: errors.New("check failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(t.Context())

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the check fails", func(t *testing.T) {
		var output bytes.Buffer
		command := NewDoctorCommand(&fakeDoctor{err: errors.New("check failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), `level=ERROR msg=doctor error="check failed"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=doctor error="check failed"`)
		}
	})
}
