package clock_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/clock"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestNow_IsUTC(t *testing.T) {
	got := clock.System{}.Now()

	testkit.Equal(t, "UTC", got.Location().String())
}
