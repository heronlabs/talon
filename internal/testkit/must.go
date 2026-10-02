package testkit

import (
	"fmt"
	"testing"
)

// Must returns v and panics when err is not nil. Setup only: it is not an assertion.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("setup: %v", err))
	}
	return v
}

// MustDo fails the test when err is not nil. Setup only: it is not an assertion.
func MustDo(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
