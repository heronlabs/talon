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

// Calls records the order in which fakes sharing it were called.
type Calls struct{ Names []string }

func (c *Calls) add(name string) {
	if c != nil {
		c.Names = append(c.Names, name)
	}
}

// ReleaseSource answers Release with Rel or ReleaseErr and Download with Blobs[asset.ID] or
// DownloadErr. Requested records the versions asked for.
type ReleaseSource struct {
	Rel         domain.Release
	ReleaseErr  error
	Blobs       map[int64][]byte
	DownloadErr error
	Requested   []string
	Calls       *Calls
}

// Release implements port.ReleaseSource.
func (f *ReleaseSource) Release(_ context.Context, version string) (domain.Release, error) {
	f.Requested = append(f.Requested, version)
	f.Calls.add("release")
	return f.Rel, f.ReleaseErr
}

// Download implements port.ReleaseSource.
func (f *ReleaseSource) Download(_ context.Context, asset domain.Asset) ([]byte, error) {
	f.Calls.add("download " + asset.Name)
	return f.Blobs[asset.ID], f.DownloadErr
}

// BinaryStore records what it is asked to do and fails with the matching error.
type BinaryStore struct {
	Staged     []byte
	StageErr   error
	SwapErr    error
	RestoreErr error
	Calls      *Calls
}

// Stage implements port.BinaryStore.
func (f *BinaryStore) Stage(binary []byte) error {
	f.Staged = binary
	f.Calls.add("stage")
	return f.StageErr
}

// Swap implements port.BinaryStore.
func (f *BinaryStore) Swap() error {
	f.Calls.add("swap")
	return f.SwapErr
}

// Restore implements port.BinaryStore.
func (f *BinaryStore) Restore() error {
	f.Calls.add("restore")
	return f.RestoreErr
}

// HealthCheck returns Output and Err.
type HealthCheck struct {
	Output string
	Err    error
	Calls  *Calls
}

// Check implements port.HealthCheck.
func (f HealthCheck) Check(context.Context) (string, error) {
	f.Calls.add("check")
	return f.Output, f.Err
}

// UpdateLock fails with Err or hands out a release func that records the release.
type UpdateLock struct {
	Err   error
	Calls *Calls
}

// Acquire implements port.UpdateLock.
func (f UpdateLock) Acquire() (func(), error) {
	f.Calls.add("lock")
	if f.Err != nil {
		return nil, f.Err
	}
	return func() { f.Calls.add("unlock") }, nil
}

// ConfigStore holds Data, fails Read with ReadErr and Write with WriteErr, and records writes.
type ConfigStore struct {
	Data     []byte
	ReadErr  error
	WriteErr error
	Written  [][]byte
}

// Read implements port.ConfigStore.
func (f *ConfigStore) Read() ([]byte, error) { return f.Data, f.ReadErr }

// Write implements port.ConfigStore.
func (f *ConfigStore) Write(data []byte) error {
	f.Written = append(f.Written, data)
	return f.WriteErr
}

// ConfigSyntax returns Err.
type ConfigSyntax struct{ Err error }

// CheckSyntax implements port.ConfigSyntax.
func (f ConfigSyntax) CheckSyntax([]byte) error { return f.Err }

// SystemAccounts records its calls and fails the one named in Errs.
type SystemAccounts struct {
	Errs  map[string]error
	Lines []string
	Calls *Calls
}

// EnsureDeployUser implements port.SystemAccounts.
func (f *SystemAccounts) EnsureDeployUser(context.Context) error {
	f.Calls.add("user")
	return f.Errs["user"]
}

// WriteAuthorizedKey implements port.SystemAccounts.
func (f *SystemAccounts) WriteAuthorizedKey(line string) error {
	f.Lines = append(f.Lines, line)
	f.Calls.add("authorized-key")
	return f.Errs["authorized-key"]
}

// WriteSudoers implements port.SystemAccounts.
func (f *SystemAccounts) WriteSudoers(context.Context) error {
	f.Calls.add("sudoers")
	return f.Errs["sudoers"]
}

// Executable returns Data or Err.
type Executable struct {
	Data []byte
	Err  error
}

// Read implements port.Executable.
func (f Executable) Read() ([]byte, error) { return f.Data, f.Err }

// Updater records the versions it is asked to install and returns Err.
type Updater struct {
	Versions []domain.Version
	Err      error
}

// Run implements deploy.Updater.
func (f *Updater) Run(_ context.Context, v domain.Version) error {
	f.Versions = append(f.Versions, v)
	return f.Err
}
