package hostfs

import (
	"fmt"
	"os"
)

// Executable is the running binary, at Path (os.Executable in production).
type Executable struct{ Path string }

// Read implements port.Executable.
func (e Executable) Read() ([]byte, error) {
	data, err := os.ReadFile(e.Path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", e.Path, err)
	}
	return data, nil
}
