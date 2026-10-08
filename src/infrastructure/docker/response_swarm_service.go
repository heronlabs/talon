package docker

// SwarmService is an item of the answer of the Engine API that lists the Swarm services: its ID, name, limits and the tasks running and desired.
type SwarmService struct {
	ID   string
	Spec struct {
		Name         string
		TaskTemplate struct {
			Resources struct {
				Limits struct {
					NanoCPUs    int64
					MemoryBytes int64
				}
			}
		}
	}
	ServiceStatus struct {
		RunningTasks int64
		DesiredTasks int64
	}
}
