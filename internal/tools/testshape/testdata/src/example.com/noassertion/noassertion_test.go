package noassertion

import "testing"

func TestNone(t *testing.T) { // want `TestNone: 0 assertions, want exactly 1`
	_ = t
}
