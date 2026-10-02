package domain_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const binaryName = "vps-agent_0.2.0_linux_amd64"

var (
	releaseKey = ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	otherKey   = ed25519.NewKeyFromSeed([]byte("another-seed-of-32-bytes-length!"))
	binary     = []byte("#!/bin/sh\nexit 0\n")
)

func sumOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func checksumsFor(b []byte) []byte {
	return []byte(sumOf([]byte("arm")) + "  vps-agent_0.2.0_linux_arm64\n" + sumOf(b) + "  " + binaryName + "\n")
}

func publicKey(k ed25519.PrivateKey) ed25519.PublicKey {
	return k.Public().(ed25519.PublicKey)
}

type found struct {
	Asset domain.Asset
	OK    bool
}

func assetOf(r domain.Release, name string) found {
	a, ok := r.Asset(name)
	return found{a, ok}
}

func TestReleaseAsset_FindsTheAssetByName(t *testing.T) {
	r := domain.Release{Assets: []domain.Asset{{ID: 1, Name: "checksums.txt"}, {ID: 2, Name: binaryName}}}

	got := assetOf(r, binaryName)

	testkit.Equal(t, found{domain.Asset{ID: 2, Name: binaryName}, true}, got)
}

func TestReleaseAsset_FindsTheFirstAsset(t *testing.T) {
	r := domain.Release{Assets: []domain.Asset{{ID: 1, Name: "checksums.txt"}, {ID: 2, Name: binaryName}}}

	got := assetOf(r, "checksums.txt")

	testkit.Equal(t, found{domain.Asset{ID: 1, Name: "checksums.txt"}, true}, got)
}

func TestReleaseAsset_MissingAssetIsNotFound(t *testing.T) {
	r := domain.Release{Assets: []domain.Asset{{ID: 1, Name: "checksums.txt"}}}

	got := assetOf(r, binaryName)

	testkit.Equal(t, found{}, got)
}

func TestBinaryAssetName_NamesVersionAndArch(t *testing.T) {
	got := domain.BinaryAssetName(domain.Version{Minor: 2}, "arm64")

	testkit.Equal(t, "vps-agent_0.2.0_linux_arm64", got)
}

func TestIncompleteRelease_NamesTheTag(t *testing.T) {
	got := domain.IncompleteRelease("v0.2.0").Error()

	testkit.Equal(t, "release v0.2.0 is incomplete", got)
}

func TestIncompleteRelease_WrapsTheSentinel(t *testing.T) {
	err := domain.IncompleteRelease("v0.2.0")

	testkit.ErrorIs(t, err, domain.ErrIncompleteRelease)
}

func TestVerifyRelease_AcceptsSignedChecksumsAndMatchingBinary(t *testing.T) {
	checksums := checksumsFor(binary)

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(releaseKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, nil)
}

func TestVerifyRelease_AcceptsWindowsLineEndings(t *testing.T) {
	checksums := []byte(sumOf(binary) + "  " + binaryName + "\r\n")

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(releaseKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, nil)
}

func TestVerifyRelease_RejectsTamperedChecksums(t *testing.T) {
	signature := ed25519.Sign(releaseKey, checksumsFor(binary))
	tampered := checksumsFor([]byte("evil"))

	err := domain.VerifyRelease(publicKey(releaseKey), tampered, signature, binaryName, []byte("evil"))

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestVerifyRelease_RejectsSignatureFromOtherKey(t *testing.T) {
	checksums := checksumsFor(binary)

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(otherKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestVerifyRelease_RejectsShortSignature(t *testing.T) {
	checksums := checksumsFor(binary)

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, []byte("short"), binaryName, binary)

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestVerifyRelease_RejectsShortPublicKeyWithoutPanicking(t *testing.T) {
	checksums := checksumsFor(binary)

	err := domain.VerifyRelease(ed25519.PublicKey("short"), checksums, ed25519.Sign(releaseKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestVerifyRelease_RejectsChecksumsWithoutTheBinary(t *testing.T) {
	checksums := []byte(sumOf(binary) + "  vps-agent_0.2.0_linux_arm64\n")

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(releaseKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, domain.ErrChecksumMissing)
}

func TestVerifyRelease_RejectsChecksumThatIsNotHex(t *testing.T) {
	checksums := []byte("not-hex  " + binaryName + "\n")

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(releaseKey, checksums), binaryName, binary)

	testkit.ErrorIs(t, err, domain.ErrChecksumMissing)
}

func TestVerifyRelease_RejectsBinaryThatDoesNotMatch(t *testing.T) {
	checksums := checksumsFor(binary)

	err := domain.VerifyRelease(publicKey(releaseKey), checksums, ed25519.Sign(releaseKey, checksums), binaryName, []byte("evil"))

	testkit.ErrorIs(t, err, domain.ErrChecksumMismatch)
}
