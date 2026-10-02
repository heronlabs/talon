package valid

import (
	"errors"
	"testing"

	"example.com/internal/testkit"
)

type suite struct{}

func (suite) TestMethod(xs []int) {
	for range xs {
	}
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

func TestValid(t *testing.T) {
	err := errors.New("x")
	_ = err.Error()
	_ = len("x")
	f := func() int { return 1 }
	_ = f()
	_ = string(rune(65))
	got := testkit.Must(sum([]int{1}), nil)

	testkit.Equal(t, 1, got)
}
