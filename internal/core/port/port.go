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

// ReleaseSource reads the agent's published releases. Version "" means the latest release.
type ReleaseSource interface {
	Release(ctx context.Context, version string) (domain.Release, error)
	Download(ctx context.Context, asset domain.Asset) ([]byte, error)
}

// BinaryStore replaces the installed binary: Stage writes the new one next to it, Swap keeps the
// current one as the previous and puts the staged one in its place, Restore brings the previous back.
type BinaryStore interface {
	Stage(binary []byte) error
	Swap() error
	Restore() error
}

// HealthCheck runs the doctor of the installed binary and returns what it printed.
type HealthCheck interface {
	Check(ctx context.Context) (output string, err error)
}

// UpdateLock keeps two updates from running at once.
type UpdateLock interface {
	Acquire() (release func(), err error)
}

// ConfigStore reads and atomically writes the configuration file. Read of a missing file fails
// with an error wrapping fs.ErrNotExist.
type ConfigStore interface {
	Read() ([]byte, error)
	Write(data []byte) error
}

// ConfigSyntax checks that a configuration file would be accepted.
type ConfigSyntax interface {
	CheckSyntax(data []byte) error
}

// SystemAccounts prepares the deploy user the Deploy workflow logs in as.
type SystemAccounts interface {
	EnsureDeployUser(ctx context.Context) error
	WriteAuthorizedKey(line string) error
	WriteSudoers(ctx context.Context) error
}

// Executable reads the binary that is running.
type Executable interface {
	Read() ([]byte, error)
}
