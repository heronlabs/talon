package application

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"

	"github.com/heronlabs/talon/src/core/entity"
)

type fakeReport struct {
	ctx    context.Context
	report *entity.Report
	err    error
}

func (f *fakeReport) Send(ctx context.Context) (*entity.Report, error) {
	f.ctx = ctx

	return f.report, f.err
}

func TestReportCommand(t *testing.T) {
	t.Run("Should return 0 when the report is sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(t.Context())

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log status=ok when the report is sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), "level=INFO msg=report status=ok") {
			t.Errorf("Run() logged %q, want %q", output.String(), "level=INFO msg=report status=ok")
		}
	})

	t.Run("Should log the report as JSON in payload when it is sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{report: &entity.Report{Host: "vps-01"}}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), `status=ok payload="{\"host\":\"vps-01\",`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `status=ok payload="{\"host\":\"vps-01\",`)
		}
	})

	t.Run("Should send with the ctx it was given", func(t *testing.T) {
		var output bytes.Buffer
		ctx := t.Context()
		report := &fakeReport{}
		command := NewReportCommand(report, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if report.ctx != ctx {
			t.Errorf("Run() sent with %v, want %v", report.ctx, ctx)
		}
	})

	t.Run("Should return 1 when the report cannot be sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{err: errors.New("send failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(t.Context())

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the report cannot be sent", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{err: errors.New("send failed")}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), `level=ERROR msg=report error="send failed"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=report error="send failed"`)
		}
	})

	t.Run("Should return 1 when the report cannot be encoded as JSON", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{report: &entity.Report{System: entity.System{Load1: math.NaN()}}}, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(t.Context())

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the report cannot be encoded as JSON", func(t *testing.T) {
		var output bytes.Buffer
		command := NewReportCommand(&fakeReport{report: &entity.Report{System: entity.System{Load1: math.NaN()}}}, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(t.Context())

		if !strings.Contains(output.String(), `level=ERROR msg=report error="json: unsupported value: NaN"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=report error="json: unsupported value: NaN"`)
		}
	})
}
