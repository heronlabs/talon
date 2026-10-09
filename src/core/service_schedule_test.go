package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScheduleService(t *testing.T) {
	t.Run("Should read INTERVAL_REPORT", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "10m"}}
		service := NewScheduleService(config)
		want := [][]string{{"INTERVAL_REPORT"}}

		_, _, _ = service.Read()

		if !reflect.DeepEqual(config.requested, want) {
			t.Errorf("Read() requested %v, want %v", config.requested, want)
		}
	})

	t.Run("Should return INTERVAL_REPORT as the interval and maxCycleTimeout as the timeout when the interval is longer", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "10m"}}
		service := NewScheduleService(config)

		interval, timeout, err := service.Read()

		if interval != 10*time.Minute || timeout != time.Minute || err != nil {
			t.Errorf("Read() = %v, %v, %v; want %v, %v, nil", interval, timeout, err, 10*time.Minute, time.Minute)
		}
	})

	t.Run("Should return INTERVAL_REPORT as the interval and the timeout when it is shorter than maxCycleTimeout", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "1s"}}
		service := NewScheduleService(config)

		interval, timeout, err := service.Read()

		if interval != time.Second || timeout != time.Second || err != nil {
			t.Errorf("Read() = %v, %v, %v; want %v, %v, nil", interval, timeout, err, time.Second, time.Second)
		}
	})

	t.Run("Should fail when INTERVAL_REPORT is not a duration", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "abc"}}
		service := NewScheduleService(config)

		_, _, err := service.Read()

		if err == nil || err.Error() != `config: INTERVAL_REPORT: time: invalid duration "abc"` {
			t.Errorf("Read() = %v, want %q", err, `config: INTERVAL_REPORT: time: invalid duration "abc"`)
		}
	})

	t.Run("Should fail when INTERVAL_REPORT is zero", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "0s"}}
		service := NewScheduleService(config)

		_, _, err := service.Read()

		if err == nil || err.Error() != "config: INTERVAL_REPORT: 0s is not positive" {
			t.Errorf("Read() = %v, want %q", err, "config: INTERVAL_REPORT: 0s is not positive")
		}
	})

	t.Run("Should fail when INTERVAL_REPORT is negative", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"INTERVAL_REPORT": "-1s"}}
		service := NewScheduleService(config)

		_, _, err := service.Read()

		if err == nil || err.Error() != "config: INTERVAL_REPORT: -1s is not positive" {
			t.Errorf("Read() = %v, want %q", err, "config: INTERVAL_REPORT: -1s is not positive")
		}
	})

	t.Run("Should fail when the configuration cannot be read", func(t *testing.T) {
		errGet := errors.New("get failed")
		service := NewScheduleService(&fakeConfigGetter{err: errGet})

		_, _, err := service.Read()

		if !errors.Is(err, errGet) || !strings.HasPrefix(err.Error(), "config: ") {
			t.Errorf("Read() = %v, want %v with prefix %q", err, errGet, "config: ")
		}
	})
}
