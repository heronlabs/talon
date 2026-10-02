package hostfs_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestLockAcquire_TakesAFreeLock(t *testing.T) {
	l := hostfs.Lock{Path: filepath.Join(t.TempDir(), "update.lock")}

	release, err := l.Acquire()
	release()

	testkit.ErrorIs(t, err, nil)
}

func TestLockAcquire_FailsWhileTheLockIsHeld(t *testing.T) {
	l := hostfs.Lock{Path: filepath.Join(t.TempDir(), "update.lock")}
	release := testkit.Must(l.Acquire())
	t.Cleanup(release)

	_, err := l.Acquire()

	testkit.ErrorIs(t, err, hostfs.ErrUpdateRunning)
}

func TestLockAcquire_SaysAnUpdateIsRunning(t *testing.T) {
	l := hostfs.Lock{Path: filepath.Join(t.TempDir(), "update.lock")}
	release := testkit.Must(l.Acquire())
	t.Cleanup(release)

	_, err := l.Acquire()

	testkit.Equal(t, "update already running", err.Error())
}

func TestLockAcquire_TakesTheLockAgainAfterRelease(t *testing.T) {
	l := hostfs.Lock{Path: filepath.Join(t.TempDir(), "update.lock")}
	release := testkit.Must(l.Acquire())
	release()

	again, err := l.Acquire()
	again()

	testkit.ErrorIs(t, err, nil)
}

func TestLockAcquire_FailsWhenTheDirectoryIsMissing(t *testing.T) {
	l := hostfs.Lock{Path: filepath.Join(t.TempDir(), "missing", "update.lock")}

	_, err := l.Acquire()

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}
