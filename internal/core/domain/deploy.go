package domain

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
)

// DeployUser is the system user the Deploy workflow logs in as over SSH.
const DeployUser = "vps-agent-deploy"

// SudoersRule lets DeployUser run exactly "vps-agent deploy" as root, keeping the SSH request.
const SudoersRule = `Defaults!/usr/local/bin/vps-agent env_keep += "SSH_ORIGINAL_COMMAND"
vps-agent-deploy ALL=(root) NOPASSWD: /usr/local/bin/vps-agent deploy
`

// ErrInvalidDeployKey marks a DEPLOY_SSH_PUBLIC_KEY that is not one ssh-ed25519 public key.
var ErrInvalidDeployKey = errors.New("DEPLOY_SSH_PUBLIC_KEY must be one line holding an ssh-ed25519 public key")

// ed25519Blob is the SSH wire prefix of an ed25519 public key: the string "ssh-ed25519" and the
// length of the 32-byte key, both length-prefixed. The whole blob is 51 bytes.
var ed25519Blob = []byte("\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20")

const ed25519BlobSize = 51

// AuthorizedKeyLine validates key ("ssh-ed25519 <base64> [comment]") and returns the
// authorized_keys line that forces "vps-agent deploy" and denies pty and forwarding. The
// supplied comment is replaced by DeployUser.
func AuthorizedKeyLine(key string) (string, error) {
	key = strings.TrimSpace(key)
	kind, rest, _ := strings.Cut(key, " ")
	encoded, _, _ := strings.Cut(rest, " ")
	blob, err := base64.StdEncoding.DecodeString(encoded)
	if strings.ContainsAny(key, "\r\n") || kind != "ssh-ed25519" || err != nil ||
		!bytes.HasPrefix(blob, ed25519Blob) || len(blob) != ed25519BlobSize {
		return "", ErrInvalidDeployKey
	}
	return `restrict,command="sudo /usr/local/bin/vps-agent deploy" ssh-ed25519 ` + encoded + " " + DeployUser, nil
}
