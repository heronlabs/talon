package hostfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/heronlabs/vps-agent/internal/infrastructure/atomicfile"
)

// ConfigFile is the configuration file Dir/Name, written 0600 inside a 0700 Dir.
type ConfigFile struct {
	Dir  string
	Name string
}

// Read implements port.ConfigStore.
func (c ConfigFile) Read() ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(c.Dir, c.Name))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", c.Name, err)
	}
	return data, nil
}

// Write implements port.ConfigStore: Dir is created 0700 when missing and the file is replaced
// atomically, 0600.
func (c ConfigFile) Write(data []byte) error {
	if err := os.Mkdir(c.Dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create %s: %w", c.Dir, err)
	}
	return atomicfile.Write(c.Dir, c.Name, data, 0o600, atomicfile.NoCheck)
}
