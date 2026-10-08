package entity

import (
	"time"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
)

// Report holds what talon reports about its host: the host name, when it was measured, the agent, the system and the projects.
type Report struct {
	Host       string    `json:"host"`
	MeasuredAt string    `json:"measuredAt"`
	Agent      Agent     `json:"agent"`
	System     System    `json:"system"`
	Projects   []Project `json:"projects"`
}

// NewReport returns the report of hostname, measured at measuredAt in RFC3339Nano UTC, with the agent of version of talon and the Easypanel package of snapshot, the system of snapshot and hostSnapshot and the projects of snapshot and easypanelSnapshot.
func NewReport(hostname string, measuredAt time.Time, version string, snapshot docker.Snapshot, hostSnapshot host.Snapshot, easypanelSnapshot easypanel.Snapshot) *Report {
	return &Report{
		Host:       hostname,
		MeasuredAt: measuredAt.UTC().Format(time.RFC3339Nano),
		Agent:      NewAgent(version, snapshot.Easypanel),
		System:     NewSystem(snapshot.DiskUsage, hostSnapshot),
		Projects:   NewProjects(snapshot, easypanelSnapshot),
	}
}
