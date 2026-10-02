package twoassertions

import (
	"testing"

	"example.com/internal/testkit"
)

func TestTwo(t *testing.T) { // want `TestTwo: 2 assertions, want exactly 1`
	testkit.Equal(t, 1, 1)
	testkit.ErrorIs(t, nil, nil)
}
