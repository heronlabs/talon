package application

import (
	"context"
	"log/slog"
	"time"
)

// Schedule reads how often the daemon runs a cycle and how long each cycle may run.
type Schedule interface {
	Read() (interval, timeout time.Duration, err error)
}

// Cycle runs one cycle of the daemon with ctx and returns its exit code.
type Cycle interface {
	Run(ctx context.Context) int
}

// DaemonCommand runs talon daemon.
type DaemonCommand struct {
	schedule Schedule
	cycle    Cycle
	tick     func(time.Duration) <-chan time.Time
	logger   *slog.Logger
}

// NewDaemonCommand returns a DaemonCommand that reads its schedule through schedule, runs cycle on the ticks that tick sends for the interval and logs through logger.
func NewDaemonCommand(schedule Schedule, cycle Cycle, tick func(time.Duration) <-chan time.Time, logger *slog.Logger) *DaemonCommand {
	return &DaemonCommand{schedule: schedule, cycle: cycle, tick: tick, logger: logger}
}

// Run reads the schedule and runs a cycle at once and then one on every tick of the interval until ctx is done, logs the result and returns 1 when the schedule cannot be read or 0 when ctx is done.
func (c *DaemonCommand) Run(ctx context.Context) int {
	interval, timeout, err := c.schedule.Read()
	if err != nil {
		c.logger.Error("daemon", "error", err)
		return 1
	}

	c.loop(ctx, c.tick(interval), timeout)

	c.logger.Info("daemon", "status", "stopped")

	return 0
}

// loop runs a cycle at once and then one on every tick of ticks, one at a time, until ctx is done.
func (c *DaemonCommand) loop(ctx context.Context, ticks <-chan time.Time, timeout time.Duration) {
	c.runCycle(ctx, timeout)
	for next(ctx, ticks) {
		c.runCycle(ctx, timeout)
	}
}

// runCycle runs one cycle under timeout, on a context that ctx being done does not cancel, so a cycle in flight always ends.
func (c *DaemonCommand) runCycle(ctx context.Context, timeout time.Duration) {
	cycleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	c.cycle.Run(cycleCtx)
}

// next waits for the next tick of ticks and reports whether ctx is still not done, so a tick pending when ctx is done starts no cycle.
func next(ctx context.Context, ticks <-chan time.Time) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ticks:
		return ctx.Err() == nil
	}
}
