package hostfs

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrUpdateRunning means another update holds the lock.
var ErrUpdateRunning = errors.New("update already running")

// Lock is an exclusive flock(2) on Path.
type Lock struct{ Path string }

// Acquire implements port.UpdateLock without waiting: a held lock fails with ErrUpdateRunning.
// The lock is released when the returned func closes the file, or when the process exits.
func (l Lock) Acquire() (func(), error) {
	f, err := os.OpenFile(l.Path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, ErrUpdateRunning
	}
	return func() { _ = f.Close() }, nil
}
