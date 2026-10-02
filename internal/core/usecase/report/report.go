// Package report collects one measurement of the host and hands it to a sink.
package report

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/port"
)

// Deps are the collaborators of the report use case.
type Deps struct {
	Host         string
	AgentVersion string
	Clock        port.Clock
	Metrics      port.HostMetrics
	Docker       port.Docker
	Dirs         port.DirSizer
	Sink         port.ReportSink
	Logger       *slog.Logger
}

// UseCase collects and publishes reports.
type UseCase struct{ deps Deps }

// New builds the report use case.
func New(deps Deps) *UseCase { return &UseCase{deps: deps} }

// Run collects one report and publishes it.
func (u *UseCase) Run(ctx context.Context) error {
	host, err := u.deps.Metrics.Read(ctx)
	if err != nil {
		return fmt.Errorf("read host metrics: %w", err)
	}
	docker, err := u.deps.Docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read docker: %w", err)
	}
	r := domain.BuildReport(domain.ReportInput{
		Host:         u.deps.Host,
		AgentVersion: u.deps.AgentVersion,
		Now:          u.deps.Clock.Now(),
		HostSample:   host,
		DockerSample: docker,
		DirSizes:     u.dirSizes(ctx, domain.AllBindMountPaths(docker)),
	})
	if err := u.deps.Sink.Publish(ctx, r); err != nil {
		return fmt.Errorf("publish report: %w", err)
	}
	return nil
}

// dirSizes measures each path; a path that fails is logged and counts as zero.
func (u *UseCase) dirSizes(ctx context.Context, paths []string) map[string]domain.Bytes {
	sizes := make(map[string]domain.Bytes, len(paths))
	for _, p := range paths {
		size, err := u.deps.Dirs.DirSize(ctx, p)
		if err != nil {
			u.deps.Logger.WarnContext(ctx, "dir size failed",
				slog.String("logId", "report.dir-size-failed"),
				slog.String("path", p),
				slog.String("errorName", fmt.Sprintf("%T", err)),
				slog.String("errorMessage", err.Error()))
			continue
		}
		sizes[p] = size
	}
	return sizes
}
