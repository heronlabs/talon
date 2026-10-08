package docker

// SwarmTask is an item of the answer of the Engine API that lists the Swarm tasks: its service and its state.
type SwarmTask struct {
	ServiceID string
	Status    struct {
		State string
	}
}
