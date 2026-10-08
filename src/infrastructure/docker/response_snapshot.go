package docker

// Snapshot holds the answers of the Engine API about the host, taken one after the other.
type Snapshot struct {
	// DiskUsage is the disk usage of the images, the build cache and the volumes.
	DiskUsage DiskUsage
	// Services are the Swarm services, with the tasks running and desired.
	Services []SwarmService
	// Tasks are the Swarm tasks of every service.
	Tasks []SwarmTask
	// Containers are the containers of the Swarm services, running or not.
	Containers []Container
	// Stats are the stats of the running containers, by container ID.
	Stats map[string]ContainerStats
}
