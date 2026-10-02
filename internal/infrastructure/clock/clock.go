// Package clock is the real clock.
package clock

import "time"

// System reads the system clock, in UTC.
type System struct{}

// Now implements port.Clock.
func (System) Now() time.Time { return time.Now().UTC() }
