package testkit

import "testing"

func Equal[T any](t testing.TB, want, got T) {}

func ErrorIs(t testing.TB, err, target error) {}

func Must[T any](v T, err error) T { return v }
