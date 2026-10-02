package hostfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/heronlabs/vps-agent/internal/infrastructure/atomicfile"
)

// Binaries keeps the installed binary Dir/Name, its staged replacement Dir/.Name.new and its
// previous version Dir/Name.previous. There is always a binary at Dir/Name once one was installed.
type Binaries struct {
	Dir  string
	Name string
}

func (b Binaries) current() string  { return filepath.Join(b.Dir, b.Name) }
func (b Binaries) staged() string   { return filepath.Join(b.Dir, "."+b.Name+".new") }
func (b Binaries) previous() string { return filepath.Join(b.Dir, b.Name+".previous") }

// Stage implements port.BinaryStore: it writes the new binary, 0755 and synced, next to the current one.
func (b Binaries) Stage(binary []byte) error {
	return atomicfile.Write(b.Dir, filepath.Base(b.staged()), binary, 0o755, atomicfile.NoCheck)
}

// Swap implements port.BinaryStore: the current binary becomes the previous one (a hard link)
// and the staged one is renamed over it. Without a current binary there is no previous one.
func (b Binaries) Swap() error {
	if err := os.Remove(b.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove previous binary: %w", err)
	}
	if err := os.Link(b.current(), b.previous()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("keep previous binary: %w", err)
	}
	if err := os.Rename(b.staged(), b.current()); err != nil {
		return fmt.Errorf("install staged binary: %w", err)
	}
	return nil
}

// Restore implements port.BinaryStore: the previous binary is renamed back over the current one.
func (b Binaries) Restore() error {
	if err := os.Rename(b.previous(), b.current()); err != nil {
		return fmt.Errorf("restore previous binary: %w", err)
	}
	return nil
}
