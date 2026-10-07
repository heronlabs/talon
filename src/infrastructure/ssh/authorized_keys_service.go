package ssh

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	keyType    = "ssh-ed25519"
	keyComment = "talon-deploy"
	command    = "export DEBIAN_FRONTEND=noninteractive; apt-get -qq update && apt-get -qq -y -o DPkg::Lock::Timeout=300 install --only-upgrade talon && talon version"
)

var ErrInvalidPublicKey = errors.New("DEPLOY_SSH_PUBLIC_KEY is not an ssh-ed25519 public key")

type AuthorizedKeys interface {
	Write(publicKey string) error
}

type AuthorizedKeysService struct {
	dir  string
	file string
	tmp  string
}

func NewAuthorizedKeysService(path string) *AuthorizedKeysService {
	file := filepath.Base(path)
	return &AuthorizedKeysService{dir: filepath.Dir(path), file: file, tmp: file + ".tmp"}
}

func (s *AuthorizedKeysService) Write(publicKey string) error {
	line, err := deployLine(publicKey)
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

	current, err := root.ReadFile(s.file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	others, found := otherLines(string(current))
	if line == "" && !found {
		return nil
	}

	if err := root.Remove(s.tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if err := root.WriteFile(s.tmp, []byte(others+line), 0o600); err != nil {
		return err
	}

	return root.Rename(s.tmp, s.file)
}

func deployLine(publicKey string) (string, error) {
	if publicKey == "" {
		return "", nil
	}

	fields := strings.Fields(publicKey)
	if strings.ContainsAny(publicKey, "\r\n") || len(fields) < 2 || fields[0] != keyType {
		return "", ErrInvalidPublicKey
	}

	if _, err := base64.StdEncoding.DecodeString(fields[1]); err != nil {
		return "", ErrInvalidPublicKey
	}

	return fmt.Sprintf("restrict,command=\"%s\" %s %s %s\n", command, keyType, fields[1], keyComment), nil
}

func otherLines(content string) (string, bool) {
	var others strings.Builder
	found := false
	for line := range strings.Lines(content) {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[len(fields)-1] == keyComment {
			found = true
			continue
		}
		others.WriteString(strings.TrimSuffix(line, "\n"))
		others.WriteByte('\n')
	}

	return others.String(), found
}
