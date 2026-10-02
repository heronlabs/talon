package hostfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestExecutableRead_ReadsTheBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vps-agent")
	testkit.MustDo(t, os.WriteFile(path, []byte("binary"), 0o600))

	got := testkit.Must(hostfs.Executable{Path: path}.Read())

	testkit.Equal(t, "binary", string(got))
}

func TestExecutableRead_FailsWhenTheBinaryIsMissing(t *testing.T) {
	_, err := hostfs.Executable{Path: filepath.Join(t.TempDir(), "vps-agent")}.Read()

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}
