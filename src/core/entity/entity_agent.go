package entity

// Agent holds the version of talon, the Easypanel version and whether talon supports it, and the last reconcile.
type Agent struct {
	Version            string         `json:"version"`
	EasypanelVersion   *string        `json:"easypanelVersion"`
	EasypanelSupported *bool          `json:"easypanelSupported"`
	LastReconcile      *LastReconcile `json:"lastReconcile"`
}
