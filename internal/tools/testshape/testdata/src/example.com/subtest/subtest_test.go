package subtest

import (
	"testing"

	"example.com/internal/testkit"
)

func TestSubtest(t *testing.T) {
	t.Run("case", func(t *testing.T) {}) // want `TestSubtest: t.Run in test; write one test per scenario`

	testkit.Equal(t, 1, 1)
}
