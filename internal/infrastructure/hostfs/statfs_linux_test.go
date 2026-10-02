//go:build linux

package hostfs_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type usage struct{ Total, Used domain.Bytes }

// statfsOracle reads the same filesystem straight from the syscall, to check the adapter's arithmetic.
func statfsOracle(t *testing.T, path string) usage {
	t.Helper()
	var st syscall.Statfs_t
	testkit.MustDo(t, syscall.Statfs(path, &st))
	return usage{
		Total: domain.Bytes(st.Blocks) * domain.BytesOf(st.Bsize),
		Used:  domain.Bytes(st.Blocks-st.Bfree) * domain.BytesOf(st.Bsize),
	}
}

func TestStatFS_MatchesTheSyscall(t *testing.T) {
	dir := t.TempDir()
	total, used, err := hostfs.StatFS{}.Stat(dir)
	testkit.MustDo(t, err)

	testkit.Equal(t, statfsOracle(t, dir), usage{total, used})
}

func TestStatFS_FailsOnMissingPath(t *testing.T) {
	_, _, err := hostfs.StatFS{}.Stat(filepath.Join(t.TempDir(), "missing"))

	testkit.ErrorIs(t, err, syscall.ENOENT)
}
