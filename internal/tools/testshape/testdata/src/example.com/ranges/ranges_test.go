package ranges

import (
	"testing"

	"example.com/internal/testkit"
)

func TestRange(t *testing.T) {
	n := 0
	for _, x := range []int{1} { // want `TestRange: loop in test; write one test per scenario`
		n += x
	}

	testkit.Equal(t, 1, n)
}
