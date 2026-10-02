// Package release holds the public key heronlabs signs every release's checksums.txt with.
package release

import (
	"crypto/ed25519"
	"crypto/x509"
	_ "embed" // for the signing key
	"encoding/pem"
	"errors"
	"fmt"
)

// ErrInvalidKey marks a key that is not a PEM ed25519 public key.
var ErrInvalidKey = errors.New("invalid release signing key")

//go:embed release-signing.pub
var signingKey []byte

// PublicKey returns the embedded release signing key.
func PublicKey() (ed25519.PublicKey, error) {
	return ParsePublicKey(signingKey)
}

// ParsePublicKey reads a PEM SubjectPublicKeyInfo holding an ed25519 key, as
// `openssl pkey -pubout` writes it.
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%w: no PEM block", ErrInvalidKey)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidKey, err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: %T is not ed25519", ErrInvalidKey, key)
	}
	return pub, nil
}
