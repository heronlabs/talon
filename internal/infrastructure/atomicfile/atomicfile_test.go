package atomicfile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/atomicfile"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var errBoom = errors.New("boom")

func reject(string) error { return errBoom }

func entries(t *testing.T, dir string) []string {
	t.Helper()
	names := testkit.Must(filepath.Glob(filepath.Join(dir, "*")))
	hidden := testkit.Must(filepath.Glob(filepath.Join(dir, ".*")))
	return append(names, hidden...)
}

func TestWrite_WritesTheContents(t *testing.T) {
	dir := t.TempDir()

	testkit.MustDo(t, atomicfile.Write(dir, "f", []byte("data"), 0o600, atomicfile.NoCheck))

	testkit.Equal(t, "data", string(testkit.Must(os.ReadFile(filepath.Clean(filepath.Join(dir, "f"))))))
}

func TestWrite_ReplacesAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "f"), []byte("old contents"), 0o600))

	testkit.MustDo(t, atomicfile.Write(dir, "f", []byte("new"), 0o600, atomicfile.NoCheck))

	testkit.Equal(t, "new", string(testkit.Must(os.ReadFile(filepath.Clean(filepath.Join(dir, "f"))))))
}

func TestWrite_SetsThePermissions(t *testing.T) {
	dir := t.TempDir()

	testkit.MustDo(t, atomicfile.Write(dir, "f", []byte("data"), 0o440, atomicfile.NoCheck))

	testkit.Equal(t, fs.FileMode(0o440), testkit.Must(os.Stat(filepath.Join(dir, "f"))).Mode().Perm())
}

func TestWrite_LeavesOnlyTheTarget(t *testing.T) {
	dir := t.TempDir()

	testkit.MustDo(t, atomicfile.Write(dir, "f", []byte("data"), 0o600, atomicfile.NoCheck))

	testkit.Equal(t, []string{filepath.Join(dir, "f")}, entries(t, dir))
}

func TestWrite_ChecksTheTemporaryFileWithItsContents(t *testing.T) {
	dir := t.TempDir()
	var seen string

	testkit.MustDo(t, atomicfile.Write(dir, "f", []byte("data"), 0o600, func(tmp string) error {
		seen = string(testkit.Must(os.ReadFile(filepath.Clean(tmp))))
		return nil
	}))

	testkit.Equal(t, "data", seen)
}

func TestWrite_FailsWhenTheCheckFails(t *testing.T) {
	err := atomicfile.Write(t.TempDir(), "f", []byte("data"), 0o600, reject)

	testkit.ErrorIs(t, err, errBoom)
}

func TestWrite_KeepsTheTargetWhenTheCheckFails(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "f"), []byte("old"), 0o600))

	_ = atomicfile.Write(dir, "f", []byte("new"), 0o600, reject)

	testkit.Equal(t, []string{filepath.Join(dir, "f")}, entries(t, dir))
}

func TestWrite_KeepsTheOldContentsWhenTheCheckFails(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "f"), []byte("old"), 0o600))

	_ = atomicfile.Write(dir, "f", []byte("new"), 0o600, reject)

	testkit.Equal(t, "old", string(testkit.Must(os.ReadFile(filepath.Clean(filepath.Join(dir, "f"))))))
}

func TestWrite_FailsWhenTheDirectoryIsMissing(t *testing.T) {
	err := atomicfile.Write(filepath.Join(t.TempDir(), "missing"), "f", []byte("data"), 0o600, atomicfile.NoCheck)

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestWrite_FailsWhenTheTargetIsADirectory(t *testing.T) {
	dir := t.TempDir()
	testkit.MustDo(t, os.MkdirAll(filepath.Join(dir, "f", "child"), 0o700))

	_ = atomicfile.Write(dir, "f", []byte("data"), 0o600, atomicfile.NoCheck)

	testkit.Equal(t, []string{filepath.Join(dir, "f")}, entries(t, dir))
}
