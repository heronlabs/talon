// Package fake has hand-written fakes for every port.
package fake

import (
	"context"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// HostMetrics returns Sample or Err.
type HostMetrics struct {
	Sample domain.HostSample
	Err    error
}

// Read implements port.HostMetrics.
func (f HostMetrics) Read(context.Context) (domain.HostSample, error) { return f.Sample, f.Err }

// Docker returns Sample or SnapshotErr, and PingErr on Ping.
type Docker struct {
	Sample      domain.DockerSample
	SnapshotErr error
	PingErr     error
}

// Ping implements port.Docker.
func (f Docker) Ping(context.Context) error { return f.PingErr }

// Snapshot implements port.Docker.
func (f Docker) Snapshot(context.Context) (domain.DockerSample, error) {
	return f.Sample, f.SnapshotErr
}

// DirSizer answers from Sizes, or fails for paths in Errs.
type DirSizer struct {
	Sizes map[string]domain.Bytes
	Errs  map[string]error
}

// DirSize implements port.DirSizer.
func (f DirSizer) DirSize(_ context.Context, path string) (domain.Bytes, error) {
	return f.Sizes[path], f.Errs[path]
}

// Sink records published reports, or fails with Err.
type Sink struct {
	Published []domain.Report
	Err       error
}

// Publish implements port.ReportSink.
func (f *Sink) Publish(_ context.Context, r domain.Report) error {
	f.Published = append(f.Published, r)
	return f.Err
}

// Clock always returns T.
type Clock struct{ T time.Time }

// Now implements port.Clock.
func (f Clock) Now() time.Time { return f.T }

// ConfigValidator returns Err.
type ConfigValidator struct{ Err error }

// Validate implements port.ConfigValidator.
func (f ConfigValidator) Validate() error { return f.Err }
