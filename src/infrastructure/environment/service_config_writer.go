package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ConfigWriterService writes the configuration to .env in its directory.
type ConfigWriterService struct {
	dir  string
	keys []Key
}

// NewConfigWriterService returns a ConfigWriterService that writes the keys, in order, to .env in dir.
func NewConfigWriterService(dir string, keys []Key) *ConfigWriterService {
	return &ConfigWriterService{dir: dir, keys: keys}
}

// Write replaces .env with the value of every key, in order, with mode 600.
func (s *ConfigWriterService) Write(values map[string]string) error {
	content := s.render(values)

	return s.replace(content)
}

// render returns the content of .env: one KEY=VALUE line per key, in order.
func (s *ConfigWriterService) render(values map[string]string) []byte {
	var content strings.Builder
	for _, key := range s.keys {
		fmt.Fprintf(&content, "%s=%s\n", key.Name, values[key.Name])
	}

	return []byte(content.String())
}

// replace writes content to .env.tmp with mode 600 inside the directory and renames it to .env.
func (s *ConfigWriterService) replace(content []byte) error {
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.Remove(".env.tmp"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.WriteFile(".env.tmp", content, 0o600); err != nil {
		return err
	}

	return root.Rename(".env.tmp", ".env")
}
