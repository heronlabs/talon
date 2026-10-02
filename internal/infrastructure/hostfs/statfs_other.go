//go:build !linux

package hostfs

import (
	"errors"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrUnsupportedPlatform is returned off Linux, where the agent does not run.
var ErrUnsupportedPlatform = errors.New("statfs is only supported on linux")

// StatFS measures a filesystem with statfs(2); off Linux it always fails.
type StatFS struct{}

// Stat implements DiskStater.
func (StatFS) Stat(string) (total, used domain.Bytes, err error) {
	return 0, 0, ErrUnsupportedPlatform
}
