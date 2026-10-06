package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

type ConfigWriter interface {
	Write(values map[string]string) error
}

type ConfigWriterService struct {
	root *os.Root
	keys []Key
}

func NewConfigWriterService(root *os.Root, keys []Key) *ConfigWriterService {
	return &ConfigWriterService{root: root, keys: keys}
}

func (s *ConfigWriterService) Write(values map[string]string) error {
	var content strings.Builder
	for _, key := range s.keys {
		fmt.Fprintf(&content, "%s=%s\n", key.Name, values[key.Name])
	}

	if err := s.root.Remove(".env.tmp"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := s.root.WriteFile(".env.tmp", []byte(content.String()), 0o600); err != nil {
		return err
	}

	return s.root.Rename(".env.tmp", ".env")
}
