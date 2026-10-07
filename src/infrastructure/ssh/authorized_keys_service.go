package ssh

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// comment ends the deploy key's line, and marks it as talon's.
const comment = "talon-deploy"

// upgrade is the only command the deploy key runs.
const upgrade = "export DEBIAN_FRONTEND=noninteractive; apt-get -qq update && apt-get -qq -y -o DPkg::Lock::Timeout=300 install --only-upgrade talon && talon version"

var ErrInvalidPublicKey = errors.New("DEPLOY_SSH_PUBLIC_KEY is not an ssh-ed25519 public key")

type AuthorizedKeys interface {
	Write(publicKey string) error
}

type AuthorizedKeysService struct {
	dir  string
	name string
}

func NewAuthorizedKeysService(path string) *AuthorizedKeysService {
	return &AuthorizedKeysService{dir: filepath.Dir(path), name: filepath.Base(path)}
}

// Write puts the line of publicKey at the end of the file, in place of the one
// already there, and keeps every other line. An empty publicKey removes the
// line.
func (s *AuthorizedKeysService) Write(publicKey string) error {
	line, err := authorizedKey(publicKey)
	if err != nil {
		return err
	}

	if line != "" {
		if err := os.Mkdir(s.dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}

	root, err := os.OpenRoot(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()

	content, err := root.ReadFile(s.name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	others, found := withoutDeployKey(string(content))
	if line == "" && !found {
		return nil
	}

	if err := root.Remove(s.name + ".tmp"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.WriteFile(s.name+".tmp", []byte(others+line), 0o600); err != nil {
		return err
	}

	return root.Rename(s.name+".tmp", s.name)
}

// authorizedKey builds the authorized_keys line from the type and the base64 of
// publicKey only, so a pasted comment, option or second line never reaches it.
// An empty publicKey has no line.
func authorizedKey(publicKey string) (string, error) {
	if publicKey == "" {
		return "", nil
	}

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

	return fmt.Sprintf("restrict,command=\"%s\" ssh-ed25519 %s %s\n", upgrade, fields[1], comment), nil
}

// withoutDeployKey returns the lines of content, in order and each ending in a
// newline, except the deploy key's, and whether it had one.
func withoutDeployKey(content string) (string, bool) {
	var others strings.Builder
	found := false
	for line := range strings.Lines(content) {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[len(fields)-1] == comment {
			found = true
			continue
		}
		others.WriteString(strings.TrimSuffix(line, "\n") + "\n")
	}

	return others.String(), found
}
