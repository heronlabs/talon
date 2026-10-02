package domain_test

import (
	"encoding/base64"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const (
	deployKeyBase64 = "AAAAC3NzaC1lZDI1NTE5AAAAIMJO6hd9vIfa3qb8Vilon7lkFnaxu6wTEdEZpWfxv+vn"
	deployLine      = `restrict,command="sudo /usr/local/bin/vps-agent deploy" ssh-ed25519 ` + deployKeyBase64 + " vps-agent-deploy"
)

func TestAuthorizedKeyLine_ForcesTheDeployCommand(t *testing.T) {
	got := testkit.Must(domain.AuthorizedKeyLine("ssh-ed25519 " + deployKeyBase64 + " someone@laptop"))

	testkit.Equal(t, deployLine, got)
}

func TestAuthorizedKeyLine_AcceptsAKeyWithoutComment(t *testing.T) {
	got := testkit.Must(domain.AuthorizedKeyLine("ssh-ed25519 " + deployKeyBase64))

	testkit.Equal(t, deployLine, got)
}

func TestAuthorizedKeyLine_DropsSurroundingWhitespace(t *testing.T) {
	got := testkit.Must(domain.AuthorizedKeyLine("  ssh-ed25519 " + deployKeyBase64 + " vps-agent-deploy\n"))

	testkit.Equal(t, deployLine, got)
}

func TestAuthorizedKeyLine_RejectsEmpty(t *testing.T) {
	_, err := domain.AuthorizedKeyLine("")

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsOtherKeyType(t *testing.T) {
	_, err := domain.AuthorizedKeyLine("ssh-rsa " + deployKeyBase64)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsOptionsBeforeTheKey(t *testing.T) {
	_, err := domain.AuthorizedKeyLine(`command="sh" ssh-ed25519 ` + deployKeyBase64)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsASecondLine(t *testing.T) {
	_, err := domain.AuthorizedKeyLine("ssh-ed25519 " + deployKeyBase64 + "\nssh-ed25519 " + deployKeyBase64)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsACarriageReturn(t *testing.T) {
	_, err := domain.AuthorizedKeyLine("ssh-ed25519 " + deployKeyBase64 + "\rx")

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsInvalidBase64(t *testing.T) {
	_, err := domain.AuthorizedKeyLine("ssh-ed25519 not*base64")

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsABlobOfAnotherKeyType(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte("\x00\x00\x00\x07ssh-rsa\x00\x00\x00\x20" + string(make([]byte, 36))))

	_, err := domain.AuthorizedKeyLine("ssh-ed25519 " + blob)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsATruncatedKey(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte("\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20" + string(make([]byte, 31))))

	_, err := domain.AuthorizedKeyLine("ssh-ed25519 " + blob)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestAuthorizedKeyLine_RejectsAKeyWithExtraBytes(t *testing.T) {
	blob := base64.StdEncoding.EncodeToString([]byte("\x00\x00\x00\x0bssh-ed25519\x00\x00\x00\x20" + string(make([]byte, 33))))

	_, err := domain.AuthorizedKeyLine("ssh-ed25519 " + blob)

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}
