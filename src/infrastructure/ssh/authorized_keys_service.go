package ssh

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
)

var (
	ErrInvalidPublicKey = errors.New("DEPLOY_SSH_PUBLIC_KEY is not an ssh-ed25519 public key")
	ErrHomeNotFound     = errors.New("not found, install the talon package")
)

type AuthorizedKeys interface {
	Write(publicKey string) error
}

type AuthorizedKeysService struct {
	home string
}

func NewAuthorizedKeysService(home string) *AuthorizedKeysService {
	return &AuthorizedKeysService{home: home}
}

func (s *AuthorizedKeysService) Write(publicKey string) error {
	if publicKey == "" {
		return s.remove()
	}

	line, err := authorizedKey(publicKey)
	if err != nil {
		return err
	}

	return s.write(line)
}

// authorizedKey builds the authorized_keys line from the type and the base64 of
// publicKey only, so a pasted comment, option or second line never reaches it.
func authorizedKey(publicKey string) (string, error) {
	if strings.ContainsAny(publicKey, "\r\n") {
		return "", ErrInvalidPublicKey
	}

	fields := strings.Fields(publicKey)
	if len(fields) < 2 || fields[0] != "ssh-ed25519" {
		return "", ErrInvalidPublicKey
	}

	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil || !bytes.HasPrefix(blob, []byte("\x00\x00\x00\x0bssh-ed25519")) {
		return "", ErrInvalidPublicKey
	}

	return fmt.Sprintf("restrict,command=\"sudo -n /usr/lib/talon/upgrade\" ssh-ed25519 %s talon-deploy\n", fields[1]), nil
}

func (s *AuthorizedKeysService) write(line string) error {
	info, err := os.Stat(s.home)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s %w", s.home, ErrHomeNotFound)
	}
	if err != nil {
		return err
	}

	root, err := os.OpenRoot(s.home)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.Mkdir(".ssh", 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	if err := root.Remove(".ssh/authorized_keys.tmp"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.WriteFile(".ssh/authorized_keys.tmp", []byte(line), 0o600); err != nil {
		return err
	}

	if err := root.Rename(".ssh/authorized_keys.tmp", ".ssh/authorized_keys"); err != nil {
		return err
	}

	// sshd reads the file as talon-deploy, the owner of the home.
	owner := info.Sys().(*syscall.Stat_t)

	return errors.Join(
		root.Chown(".ssh/authorized_keys", int(owner.Uid), int(owner.Gid)),
		root.Chmod(".ssh", 0o700),
		root.Chown(".ssh", int(owner.Uid), int(owner.Gid)),
	)
}

func (s *AuthorizedKeysService) remove() error {
	root, err := os.OpenRoot(s.home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()

	err = root.Remove(".ssh/authorized_keys")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	return err
}
