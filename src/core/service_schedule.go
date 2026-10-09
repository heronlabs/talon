package core

import (
	"fmt"
	"time"
)

// intervalKey is the key of the configuration with the interval between the cycles of the daemon, and maxCycleTimeout the longest a cycle may run.
const (
	intervalKey     = "INTERVAL_REPORT"
	maxCycleTimeout = time.Minute
)

// ScheduleService reads how often the daemon runs a cycle and how long each cycle may run.
type ScheduleService struct {
	config ConfigGetter
}

// NewScheduleService returns a ScheduleService that reads the configuration through config.
func NewScheduleService(config ConfigGetter) *ScheduleService {
	return &ScheduleService{config: config}
}

// Read returns INTERVAL_REPORT as the interval between cycles and the timeout of each cycle, the interval or maxCycleTimeout, whichever is shorter; it fails when INTERVAL_REPORT cannot be read, is not a duration or is not positive.
func (s *ScheduleService) Read() (interval, timeout time.Duration, err error) {
	interval, err = s.interval()
	if err != nil {
		return 0, 0, fmt.Errorf("config: %w", err)
	}

	return interval, min(interval, maxCycleTimeout), nil
}

// interval returns INTERVAL_REPORT parsed as a duration, failing when it cannot be read, is not a duration or is not positive.
func (s *ScheduleService) interval() (time.Duration, error) {
	values, err := s.config.Get(intervalKey)
	if err != nil {
		return 0, err
	}

	interval, err := time.ParseDuration(values[intervalKey])
	if err != nil {
		return 0, fmt.Errorf("%s: %w", intervalKey, err)
	}

	if interval <= 0 {
		return 0, fmt.Errorf("%s: %s is not positive", intervalKey, interval)
	}

	return interval, nil
}
