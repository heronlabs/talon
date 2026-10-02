package hostfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestConfigFileRead_ReadsTheFile(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "config.env"), []byte("A=1\n"), 0o600))

	got := testkit.Must(hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Read())

	testkit.Equal(t, "A=1\n", string(got))
}

func TestConfigFileRead_FailsWhenTheFileIsMissing(t *testing.T) {
	_, err := hostfs.ConfigFile{Dir: t.TempDir(), Name: "config.env"}.Read()

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestConfigFileWrite_CreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vps-agent")

	testkit.MustDo(t, hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Write([]byte("A=1\n")))

	testkit.Equal(t, "A=1\n", contentsOf(t, filepath.Join(dir, "config.env")))
}

func TestConfigFileWrite_CreatesAPrivateDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "vps-agent")

	testkit.MustDo(t, hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Write([]byte("A=1\n")))

	testkit.Equal(t, fs.FileMode(0o700), testkit.Must(os.Stat(dir)).Mode().Perm())
}

func TestConfigFileWrite_WritesAPrivateFile(t *testing.T) {
	dir := t.TempDir()

	testkit.MustDo(t, hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Write([]byte("A=1\n")))

	testkit.Equal(t, fs.FileMode(0o600), testkit.Must(os.Stat(filepath.Join(dir, "config.env"))).Mode().Perm())
}

func TestConfigFileWrite_ReplacesTheFileInAnExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "config.env"), []byte("A=1\n"), 0o600))

	testkit.MustDo(t, hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Write([]byte("A=1\nB=2\n")))

	testkit.Equal(t, "A=1\nB=2\n", contentsOf(t, filepath.Join(dir, "config.env")))
}

func TestConfigFileWrite_FailsWhenTheParentIsMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing", "vps-agent")

	err := hostfs.ConfigFile{Dir: dir, Name: "config.env"}.Write([]byte("A=1\n"))

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}
