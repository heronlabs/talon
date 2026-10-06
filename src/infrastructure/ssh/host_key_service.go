package ssh

import (
	"errors"
	"io/fs"
	"strings"
)

type HostKey interface {
	Read() (string, error)
}

type HostKeyService struct {
	fsys fs.FS
}

func NewHostKeyService(fsys fs.FS) *HostKeyService {
	return &HostKeyService{fsys: fsys}
}

// Read returns the type and the base64 of the key, without its comment, or ""
// when the host has no ed25519 key.
func (s *HostKeyService) Read() (string, error) {
	data, err := fs.ReadFile(s.fsys, "ssh_host_ed25519_key.pub")
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	fields := strings.Fields(string(data))

	return strings.Join(fields[:min(len(fields), 2)], " "), nil
}
