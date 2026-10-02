package loop

import (
	"testing"

	"example.com/internal/testkit"
)

func TestLoop(t *testing.T) {
	n := 0
	for i := 0; i < 2; i++ { // want `TestLoop: loop in test; write one test per scenario`
		n += i
	}

	testkit.Equal(t, 1, n)
}
