// Package testkit holds the only assertions tests may use, plus setup helpers.
package testkit

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Equal fails the test with a diff when want and got differ.
func Equal[T any](t testing.TB, want, got T, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

// ErrorIs fails the test when err does not wrap target.
func ErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want %v", err, target)
	}
}
