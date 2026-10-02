//go:build !linux

package hostfs_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestStatFS_IsUnsupportedOffLinux(t *testing.T) {
	_, _, err := hostfs.StatFS{}.Stat("/")

	testkit.ErrorIs(t, err, hostfs.ErrUnsupportedPlatform)
}
