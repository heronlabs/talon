// Package doctor runs the agent's health checks in order.
package doctor

import (
	"context"

	"github.com/heronlabs/vps-agent/internal/core/port"
)

// Check is one named health check.
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Result is the outcome of one check; Err is nil when it passed.
type Result struct {
	Check string
	Err   error
}

// Run runs checks in order and stops at the first failure.
func Run(ctx context.Context, checks ...Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		err := c.Run(ctx)
		results = append(results, Result{Check: c.Name, Err: err})
		if err != nil {
			break
		}
	}
	return results
}

// Healthy tells whether every result passed.
func Healthy(results []Result) bool {
	for _, r := range results {
		if r.Err != nil {
			return false
		}
	}
	return true
}

// ConfigCheck validates the configuration.
func ConfigCheck(v port.ConfigValidator) Check {
	return Check{Name: "config", Run: func(context.Context) error { return v.Validate() }}
}

// DockerCheck pings the Docker engine through its socket.
func DockerCheck(d port.Docker) Check {
	return Check{Name: "docker", Run: d.Ping}
}
