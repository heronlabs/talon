package entity

import (
	"time"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

// Report holds what talon reports about its host: the host name, when it was measured, the agent, the system and the projects.
type Report struct {
	Host       string    `json:"host"`
	MeasuredAt string    `json:"measuredAt"`
	Agent      Agent     `json:"agent"`
	System     System    `json:"system"`
	Projects   []Project `json:"projects"`
}

// NewReport returns the report of host, measured at measuredAt in RFC3339Nano UTC by version of talon, with the system and the projects of snapshot.
func NewReport(host string, measuredAt time.Time, version string, snapshot docker.Snapshot) *Report {
	return &Report{
		Host:       host,
		MeasuredAt: measuredAt.UTC().Format(time.RFC3339Nano),
		Agent:      Agent{Version: version},
		System:     NewSystem(snapshot.DiskUsage),
		Projects:   NewProjects(snapshot),
	}
}
