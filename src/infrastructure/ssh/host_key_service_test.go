package ssh

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestHostKeyService(t *testing.T) {
	t.Run("Should read the type and the base64 of the host key without its comment", func(t *testing.T) {
		fsys := fstest.MapFS{"ssh_host_ed25519_key.pub": {Data: []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1 root@vps-01\n")}}
		service := NewHostKeyService(fsys)

		hostKey, _ := service.Read()

		if hostKey != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1" {
			t.Errorf("Read() = %q, want %q", hostKey, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1")
		}
	})

	t.Run("Should read an empty host key when the host has none", func(t *testing.T) {
		service := NewHostKeyService(fstest.MapFS{})

		hostKey, err := service.Read()

		if hostKey != "" || err != nil {
			t.Errorf("Read() = %q, %v; want \"\", nil", hostKey, err)
		}
	})

	t.Run("Should read an empty host key when the file is empty", func(t *testing.T) {
		service := NewHostKeyService(fstest.MapFS{"ssh_host_ed25519_key.pub": {Data: []byte("\n")}})

		hostKey, err := service.Read()

		if hostKey != "" || err != nil {
			t.Errorf("Read() = %q, %v; want \"\", nil", hostKey, err)
		}
	})

	t.Run("Should fail when the host key cannot be read", func(t *testing.T) {
		service := NewHostKeyService(fstest.MapFS{"ssh_host_ed25519_key.pub": {Mode: fs.ModeDir}})

		_, err := service.Read()

		if !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("Read() = %v, want %v", err, fs.ErrInvalid)
		}
	})
}
