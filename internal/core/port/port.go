// Package port declares the interfaces the use cases need from the outside world.
package port

import (
	"context"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// HostMetrics reads memory, load, uptime and disk of the host.
type HostMetrics interface {
	Read(ctx context.Context) (domain.HostSample, error)
}

// DirSizer measures the size of a directory tree on the host.
type DirSizer interface {
	DirSize(ctx context.Context, path string) (domain.Bytes, error)
}

// Docker reads the Docker engine.
type Docker interface {
	Ping(ctx context.Context) error
	Snapshot(ctx context.Context) (domain.DockerSample, error)
}

// ReportSink receives a finished report.
type ReportSink interface {
	Publish(ctx context.Context, r domain.Report) error
}

// Clock tells the current time.
type Clock interface {
	Now() time.Time
}

// ConfigValidator checks the agent configuration.
type ConfigValidator interface {
	Validate() error
}
