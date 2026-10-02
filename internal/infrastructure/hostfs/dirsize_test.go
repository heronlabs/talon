package hostfs_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.MustDo(t, os.MkdirAll(filepath.Join(root, "sub"), 0o750))
	testkit.MustDo(t, os.WriteFile(filepath.Join(root, "a"), make([]byte, 10), 0o600))
	testkit.MustDo(t, os.WriteFile(filepath.Join(root, "sub", "b"), make([]byte, 5), 0o600))
	testkit.MustDo(t, os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link")))
	return root
}

func TestDirSize_SumsRegularFilesWithoutFollowingLinks(t *testing.T) {
	root := tree(t)

	got, err := hostfs.DirSizer{}.DirSize(context.Background(), root)
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.Bytes(15), got)
}

func TestDirSize_MeasuresABindSourceThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yml")
	testkit.MustDo(t, os.WriteFile(file, make([]byte, 7), 0o600))

	got := testkit.Must(hostfs.DirSizer{}.DirSize(context.Background(), file))

	testkit.Equal(t, domain.Bytes(7), got)
}

func TestDirSize_FailsOnMissingDir(t *testing.T) {
	_, err := hostfs.DirSizer{}.DirSize(context.Background(), filepath.Join(t.TempDir(), "missing"))

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestDirSize_StopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := hostfs.DirSizer{}.DirSize(ctx, tree(t))

	testkit.ErrorIs(t, err, context.Canceled)
}
