package docker

import "encoding/json"

// SwarmService is an item of the answer of the Engine API that lists the Swarm services: its ID, name, task template, kept raw, the state of its last update, nil without one, and the tasks running and desired.
type SwarmService struct {
	ID   string
	Spec struct {
		Name         string
		TaskTemplate json.RawMessage
	}
	UpdateStatus *struct {
		State string
	}
	ServiceStatus struct {
		RunningTasks int64
		DesiredTasks int64
	}
}
