// Package domain holds the agent's entities and pure rules. It does no I/O.
package domain

import "time"

// Bytes is a size in bytes.
type Bytes uint64

// BytesOf converts a signed size, as the engine and the filesystem report it, treating negatives as zero.
func BytesOf(v int64) Bytes {
	return Bytes(max(v, 0))
}

// HostSample is one reading of the host, already parsed from /proc and statfs.
type HostSample struct {
	MemTotal     Bytes
	MemAvailable Bytes
	SwapTotal    Bytes
	SwapFree     Bytes
	Load1        float64
	Load5        float64
	Load15       float64
	Uptime       time.Duration
	DiskTotal    Bytes
	DiskUsed     Bytes
}

// DockerSample is one reading of the Docker engine.
type DockerSample struct {
	Services       []SwarmService
	Tasks          []SwarmTask
	Containers     []Container
	VolumeSizes    map[string]Bytes
	ImagesSize     Bytes
	BuildCacheSize Bytes
}

// SwarmService is a Swarm service as the engine declares it.
type SwarmService struct {
	ID              string
	Name            string
	ReplicasDesired int
	MemoryLimit     Bytes
	CPULimit        float64
}

// TaskState is the state of a Swarm task.
type TaskState string

// Task states the report counts.
const (
	TaskRunning  TaskState = "running"
	TaskFailed   TaskState = "failed"
	TaskRejected TaskState = "rejected"
)

// SwarmTask is one task of a Swarm service.
type SwarmTask struct {
	ServiceID string
	State     TaskState
}

// Container is a container with the Swarm service it belongs to, if any.
type Container struct {
	ID          string
	ServiceName string
	State       ServiceState
	Volumes     []string
	BindSources []string
	Stats       *ContainerStats
}

// ContainerStats is one stats reading of a running container.
type ContainerStats struct {
	MemoryUsed Bytes
	CPUPercent float64
	NetRx      Bytes
	NetTx      Bytes
	BlockRead  Bytes
	BlockWrite Bytes
}
