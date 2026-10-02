package domain

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Release asset names besides the binary.
const (
	ChecksumsAsset = "checksums.txt"
	SignatureAsset = "checksums.txt.sig"
)

// Errors of release verification.
var (
	ErrIncompleteRelease = errors.New("is incomplete")
	ErrBadSignature      = errors.New("checksums.txt is not signed by the release key")
	ErrChecksumMissing   = errors.New("checksums.txt has no line for the binary")
	ErrChecksumMismatch  = errors.New("binary does not match its sha256 in checksums.txt")
)

// Asset is one file attached to a release.
type Asset struct {
	ID   int64
	Name string
}

// Release is a published release of the agent.
type Release struct {
	Tag    string
	Assets []Asset
}

// Asset finds the asset called name.
func (r Release) Asset(name string) (Asset, bool) {
	i := slices.IndexFunc(r.Assets, func(a Asset) bool { return a.Name == name })
	if i < 0 {
		return Asset{}, false
	}
	return r.Assets[i], true
}

// BinaryAssetName is the release asset holding the Linux binary of v for arch (amd64, arm64).
func BinaryAssetName(v Version, arch string) string {
	return fmt.Sprintf("vps-agent_%s_linux_%s", v, arch)
}

// IncompleteRelease is the error for a release that lacks an asset: the CD publishes the
// release before it uploads the assets, so trying again later is enough.
func IncompleteRelease(tag string) error {
	return fmt.Errorf("release %s %w", tag, ErrIncompleteRelease)
}

// VerifyRelease checks that signature is pub's ed25519 signature of checksums and that binary's
// sha256 is the one checksums.txt lists for asset ("<sha256>  <asset>", as sha256sum writes it).
func VerifyRelease(pub ed25519.PublicKey, checksums, signature []byte, asset string, binary []byte) error {
	if len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, checksums, signature) {
		return ErrBadSignature
	}
	want, ok := checksumOf(checksums, asset)
	if !ok {
		return fmt.Errorf("%w: %s", ErrChecksumMissing, asset)
	}
	got := sha256.Sum256(binary)
	if !bytes.Equal(want, got[:]) {
		return fmt.Errorf("%w: %s", ErrChecksumMismatch, asset)
	}
	return nil
}

func checksumOf(checksums []byte, asset string) ([]byte, bool) {
	for line := range strings.Lines(string(checksums)) {
		sum, name, _ := strings.Cut(strings.TrimRight(line, "\r\n"), "  ")
		if name == asset {
			want, err := hex.DecodeString(sum)
			return want, err == nil
		}
	}
	return nil, false
}
