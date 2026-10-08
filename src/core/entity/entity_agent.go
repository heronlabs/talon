package entity

import (
	"slices"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

// supportedEasypanelVersions are the versions of Easypanel that talon supports.
var supportedEasypanelVersions = []string{"2.36.0"}

// Agent holds the version of talon, the Easypanel version and whether talon supports it, and the last reconcile.
type Agent struct {
	Version            string         `json:"version"`
	EasypanelVersion   *string        `json:"easypanelVersion"`
	EasypanelSupported *bool          `json:"easypanelSupported"`
	LastReconcile      *LastReconcile `json:"lastReconcile"`
}

// NewAgent returns the agent with version of talon and, when easypanelPackage is not nil, its Easypanel version and whether that version is in supportedEasypanelVersions.
func NewAgent(version string, easypanelPackage *docker.EasypanelPackage) Agent {
	if easypanelPackage == nil {
		return Agent{Version: version}
	}

	supported := slices.Contains(supportedEasypanelVersions, easypanelPackage.Version)

	return Agent{Version: version, EasypanelVersion: &easypanelPackage.Version, EasypanelSupported: &supported}
}
