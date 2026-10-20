package docker

import (
	"encoding/json"
	"time"
)

// SwarmTask is an item of the answer of the Engine API that lists the Swarm tasks: its service, when it was created, its spec, kept raw, and its state with when it was reached.
type SwarmTask struct {
	ServiceID string
	CreatedAt time.Time
	Spec      json.RawMessage
	Status    struct {
		Timestamp time.Time
		State     string
	}
}
