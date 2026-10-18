package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeSchedule struct {
	interval time.Duration
	timeout  time.Duration
	err      error
}

func (f fakeSchedule) Read() (interval, timeout time.Duration, err error) {
	return f.interval, f.timeout, f.err
}

type fakeCycle struct {
	cancel   context.CancelFunc
	cancelOn int
	code     int
	runs     int
	ctxErr   error
	deadline time.Time
}

func (f *fakeCycle) Run(ctx context.Context) int {
	f.runs++
	if f.runs == f.cancelOn {
		f.cancel()
	}

	f.ctxErr = ctx.Err()
	f.deadline, _ = ctx.Deadline()

	return f.code
}

func TestDaemonCommand(t *testing.T) {
	t.Run("Should return 1 when the schedule cannot be read", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{err: errors.New("read failed")}, &fakeCycle{cancel: cancel, cancelOn: 1}, tick, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(ctx)

		if code != 1 {
			t.Errorf("Run() = %d, want %d", code, 1)
		}
	})

	t.Run("Should log the error when the schedule cannot be read", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{err: errors.New("read failed")}, &fakeCycle{cancel: cancel, cancelOn: 1}, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if !strings.Contains(output.String(), `level=ERROR msg=daemon error="read failed"`) {
			t.Errorf("Run() logged %q, want %q", output.String(), `level=ERROR msg=daemon error="read failed"`)
		}
	})

	t.Run("Should not run a cycle when the schedule cannot be read", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 1}
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{err: errors.New("read failed")}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.runs != 0 {
			t.Errorf("Run() ran %d cycles, want %d", cycle.runs, 0)
		}
	})

	t.Run("Should ask for the ticks of the interval", func(t *testing.T) {
		var output bytes.Buffer
		var asked time.Duration
		ctx, cancel := context.WithCancel(t.Context())
		tick := func(interval time.Duration) <-chan time.Time {
			asked = interval
			return nil
		}
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, &fakeCycle{cancel: cancel, cancelOn: 1}, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if asked != 10*time.Minute {
			t.Errorf("Run() asked for ticks of %v, want %v", asked, 10*time.Minute)
		}
	})

	t.Run("Should run a cycle at once", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 1}
		tick := func(time.Duration) <-chan time.Time { return make(chan time.Time) }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.runs != 1 {
			t.Errorf("Run() ran %d cycles, want %d", cycle.runs, 1)
		}
	})

	t.Run("Should run a cycle on each tick", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 2}
		ticks := make(chan time.Time, 1)
		ticks <- time.Now()
		tick := func(time.Duration) <-chan time.Time { return ticks }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.runs != 2 {
			t.Errorf("Run() ran %d cycles, want %d", cycle.runs, 2)
		}
	})

	t.Run("Should keep running after a cycle fails", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 2, code: 1}
		ticks := make(chan time.Time, 1)
		ticks <- time.Now()
		tick := func(time.Duration) <-chan time.Time { return ticks }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.runs != 2 {
			t.Errorf("Run() ran %d cycles, want %d", cycle.runs, 2)
		}
	})

	t.Run("Should run no other cycle when ctx is done during a cycle with a tick pending", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 1}
		ticks := make(chan time.Time, 1)
		ticks <- time.Now()
		tick := func(time.Duration) <-chan time.Time { return ticks }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.runs != 1 {
			t.Errorf("Run() ran %d cycles, want %d", cycle.runs, 1)
		}
	})

	t.Run("Should run the cycle on a context that ctx being done does not cancel", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 1}
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if cycle.ctxErr != nil {
			t.Errorf("Run() ran the cycle on a context done with %v, want nil", cycle.ctxErr)
		}
	})

	t.Run("Should run the cycle with the timeout as its deadline", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		cycle := &fakeCycle{cancel: cancel, cancelOn: 1}
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, cycle, tick, slog.New(slog.NewTextHandler(&output, nil)))
		before := time.Now()

		_ = command.Run(ctx)
		after := time.Now()

		if cycle.deadline.Before(before.Add(time.Minute)) || cycle.deadline.After(after.Add(time.Minute)) {
			t.Errorf("Run() ran the cycle with deadline %v, want between %v and %v", cycle.deadline, before.Add(time.Minute), after.Add(time.Minute))
		}
	})

	t.Run("Should return 0 when ctx is done", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, &fakeCycle{cancel: cancel, cancelOn: 1}, tick, slog.New(slog.NewTextHandler(&output, nil)))

		code := command.Run(ctx)

		if code != 0 {
			t.Errorf("Run() = %d, want %d", code, 0)
		}
	})

	t.Run("Should log status=stopped when ctx is done", func(t *testing.T) {
		var output bytes.Buffer
		ctx, cancel := context.WithCancel(t.Context())
		tick := func(time.Duration) <-chan time.Time { return nil }
		command := NewDaemonCommand(fakeSchedule{interval: 10 * time.Minute, timeout: time.Minute}, &fakeCycle{cancel: cancel, cancelOn: 1}, tick, slog.New(slog.NewTextHandler(&output, nil)))

		_ = command.Run(ctx)

		if !strings.Contains(output.String(), "level=INFO msg=daemon status=stopped") {
			t.Errorf("Run() logged %q, want %q", output.String(), "level=INFO msg=daemon status=stopped")
		}
	})
}
