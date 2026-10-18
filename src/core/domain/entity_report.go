package domain

import (
	"time"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
)

// Report holds what talon reports about its host: the host name, when it was measured, the agent, the system and the projects.
type Report struct {
	Host       string    `json:"host"`
	MeasuredAt string    `json:"measuredAt"`
	Agent      Agent     `json:"agent"`
	System     System    `json:"system"`
	Projects   []Project `json:"projects"`
}

// NewReport returns the report of hostname, measured at measuredAt in RFC3339Nano UTC, with the agent of version of talon, the Easypanel package of snapshot and the last reconcile at measuredAt, the system of snapshot and hostSnapshot and the projects of snapshot and easypanelSnapshot against the observation: declared, the manifest, and configured, the configuration of Easypanel, each nil when it was not read, and observeErr, the error of the observation.
func NewReport(hostname string, measuredAt time.Time, version string, snapshot docker.Snapshot, hostSnapshot host.Snapshot, easypanelSnapshot easypanel.Snapshot, declared *manifest.Manifest, configured *easypanel.ConfigSnapshot, observeErr error) *Report {
	at := measuredAt.UTC().Format(time.RFC3339Nano)

	return &Report{
		Host:       hostname,
		MeasuredAt: at,
		Agent:      NewAgent(version, snapshot.Easypanel, NewLastReconcile(at, declared, observeErr)),
		System:     NewSystem(snapshot.DiskUsage, hostSnapshot),
		Projects:   NewProjects(snapshot, easypanelSnapshot, declared, configured),
	}
}
