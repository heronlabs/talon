package hostfs_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func binDir(t *testing.T, current string) hostfs.Binaries {
	t.Helper()
	dir := t.TempDir()
	if current != "" {
		testkit.MustDo(t, os.WriteFile(filepath.Join(dir, "vps-agent"), []byte(current), 0o600))
	}
	return hostfs.Binaries{Dir: dir, Name: "vps-agent"}
}

func contentsOf(t *testing.T, path string) string {
	t.Helper()
	return string(testkit.Must(os.ReadFile(filepath.Clean(path))))
}

func TestBinariesStage_WritesTheNewBinaryNextToTheCurrent(t *testing.T) {
	b := binDir(t, "old")

	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.Equal(t, "new", contentsOf(t, filepath.Join(b.Dir, ".vps-agent.new")))
}

func TestBinariesStage_MakesTheBinaryExecutable(t *testing.T) {
	b := binDir(t, "old")

	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.Equal(t, fs.FileMode(0o755), testkit.Must(os.Stat(filepath.Join(b.Dir, ".vps-agent.new"))).Mode().Perm())
}

func TestBinariesStage_LeavesTheCurrentBinaryAlone(t *testing.T) {
	b := binDir(t, "old")

	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.Equal(t, "old", contentsOf(t, filepath.Join(b.Dir, "vps-agent")))
}

func TestBinariesStage_FailsWhenTheDirectoryIsMissing(t *testing.T) {
	b := hostfs.Binaries{Dir: filepath.Join(t.TempDir(), "missing"), Name: "vps-agent"}

	err := b.Stage([]byte("new"))

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestBinariesSwap_InstallsTheStagedBinary(t *testing.T) {
	b := binDir(t, "old")
	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.MustDo(t, b.Swap())

	testkit.Equal(t, "new", contentsOf(t, filepath.Join(b.Dir, "vps-agent")))
}

func TestBinariesSwap_KeepsTheCurrentBinaryAsThePrevious(t *testing.T) {
	b := binDir(t, "old")
	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.MustDo(t, b.Swap())

	testkit.Equal(t, "old", contentsOf(t, filepath.Join(b.Dir, "vps-agent.previous")))
}

func TestBinariesSwap_ReplacesAnOlderPrevious(t *testing.T) {
	b := binDir(t, "old")
	testkit.MustDo(t, os.WriteFile(filepath.Join(b.Dir, "vps-agent.previous"), []byte("older"), 0o600))
	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.MustDo(t, b.Swap())

	testkit.Equal(t, "old", contentsOf(t, filepath.Join(b.Dir, "vps-agent.previous")))
}

func TestBinariesSwap_InstallsTheFirstBinary(t *testing.T) {
	b := binDir(t, "")
	testkit.MustDo(t, b.Stage([]byte("new")))

	testkit.MustDo(t, b.Swap())

	testkit.Equal(t, "new", contentsOf(t, filepath.Join(b.Dir, "vps-agent")))
}

func TestBinariesSwap_KeepsTheCurrentWhenThePreviousCannotBeRemoved(t *testing.T) {
	b := binDir(t, "old")
	testkit.MustDo(t, os.MkdirAll(filepath.Join(b.Dir, "vps-agent.previous", "child"), 0o700))
	testkit.MustDo(t, b.Stage([]byte("new")))

	_ = b.Swap()

	testkit.Equal(t, "old", contentsOf(t, filepath.Join(b.Dir, "vps-agent")))
}

func TestBinariesSwap_KeepsTheStagedBinaryWhenTheCurrentCannotBeLinked(t *testing.T) {
	b := binDir(t, "")
	testkit.MustDo(t, os.Mkdir(filepath.Join(b.Dir, "vps-agent"), 0o700))
	testkit.MustDo(t, b.Stage([]byte("new")))

	_ = b.Swap()

	testkit.Equal(t, "new", contentsOf(t, filepath.Join(b.Dir, ".vps-agent.new")))
}

func TestBinariesSwap_FailsWithoutAStagedBinary(t *testing.T) {
	b := binDir(t, "old")

	err := b.Swap()

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestBinariesRestore_BringsThePreviousBack(t *testing.T) {
	b := binDir(t, "old")
	testkit.MustDo(t, b.Stage([]byte("new")))
	testkit.MustDo(t, b.Swap())

	testkit.MustDo(t, b.Restore())

	testkit.Equal(t, "old", contentsOf(t, filepath.Join(b.Dir, "vps-agent")))
}

func TestBinariesRestore_FailsWithoutAPrevious(t *testing.T) {
	b := binDir(t, "old")

	err := b.Restore()

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}
