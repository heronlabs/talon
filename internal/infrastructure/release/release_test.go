package release_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/release"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

// installScriptKey returns what scripts/install.sh assigns to RELEASE_PUBLIC_KEY, between the
// opening "RELEASE_PUBLIC_KEY='" and the next "'".
func installScriptKey(t *testing.T) []byte {
	t.Helper()
	script := testkit.Must(os.ReadFile("../../../scripts/install.sh"))
	_, rest, found := bytes.Cut(script, []byte("RELEASE_PUBLIC_KEY='"))
	if !found {
		testkit.MustDo(t, errors.New("scripts/install.sh has no RELEASE_PUBLIC_KEY='...' literal"))
	}
	key, _, _ := bytes.Cut(rest, []byte("'"))
	return key
}

func pemOf(blockType string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}

// fileKey reads release-signing.pub without x509: an ed25519 SubjectPublicKeyInfo ends with the
// 32 key bytes.
func fileKey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	block, _ := pem.Decode(testkit.Must(os.ReadFile("release-signing.pub")))
	if block == nil {
		testkit.MustDo(t, errors.New("release-signing.pub is not PEM"))
	}
	return ed25519.PublicKey(block.Bytes[len(block.Bytes)-ed25519.PublicKeySize:])
}

func TestPublicKey_IsTheKeyInReleaseSigningPub(t *testing.T) {
	want := fileKey(t)

	got := testkit.Must(release.PublicKey())

	testkit.Equal(t, want, got)
}

func TestInstallScript_CarriesTheEmbeddedKey(t *testing.T) {
	embedded := testkit.Must(os.ReadFile("release-signing.pub"))

	got := installScriptKey(t)

	testkit.Equal(t, string(embedded), string(got))
}

func TestParsePublicKey_ReadsAnEd25519Key(t *testing.T) {
	pub := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	data := pemOf("PUBLIC KEY", testkit.Must(x509.MarshalPKIXPublicKey(pub)))

	got := testkit.Must(release.ParsePublicKey(data))

	testkit.Equal(t, pub, got)
}

func TestParsePublicKey_RejectsTextThatIsNotPEM(t *testing.T) {
	_, err := release.ParsePublicKey([]byte("not a key"))

	testkit.ErrorIs(t, err, release.ErrInvalidKey)
}

func TestParsePublicKey_RejectsAPEMThatIsNotAPublicKey(t *testing.T) {
	_, err := release.ParsePublicKey(pemOf("PUBLIC KEY", []byte("garbage")))

	testkit.ErrorIs(t, err, release.ErrInvalidKey)
}

func TestParsePublicKey_RejectsAnotherAlgorithm(t *testing.T) {
	key := testkit.Must(ecdsa.GenerateKey(elliptic.P256(), rand.Reader))
	data := pemOf("PUBLIC KEY", testkit.Must(x509.MarshalPKIXPublicKey(&key.PublicKey)))

	_, err := release.ParsePublicKey(data)

	testkit.ErrorIs(t, err, release.ErrInvalidKey)
}
