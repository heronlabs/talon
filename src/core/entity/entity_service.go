package entity

import (
	"slices"
	"strings"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

// The container states, removing, the state of a container being removed, and missing, the state of a service without containers.
const (
	stateRunning      = "running"
	stateRestarting   = "restarting"
	statePaused       = "paused"
	stateCreated      = "created"
	stateExited       = "exited"
	stateDead         = "dead"
	containerRemoving = "removing"
	stateMissing      = "missing"
)

// The states of a task that count as failures, the mount type of a named volume, the inactive file of the memory stats in cgroup v1 and v2, and the block I/O operations that count as read and write.
const (
	taskFailed           = "failed"
	taskRejected         = "rejected"
	volumeMountType      = "volume"
	cgroupV1InactiveFile = "total_inactive_file"
	cgroupV2InactiveFile = "inactive_file"
	blkioRead            = "read"
	blkioWrite           = "write"
)

// The units of the report and the scale of a percent.
const (
	bytesPerGB     = 1e9
	bytesPerMB     = 1e6
	bytesPerMiB    = 1 << 20
	nanoCPUsPerCPU = 1e9
	percent        = 100
)

// statePrecedence lists the container states from the most alive to the least.
var statePrecedence = []string{stateRunning, stateRestarting, statePaused, stateCreated, stateExited, stateDead}

// Service holds a service of a project: its state, replicas, failed tasks, limits and the resources it uses.
type Service struct {
	Name               string  `json:"name"`
	Managed            bool    `json:"managed"`
	State              string  `json:"state"`
	ReplicasRunning    int64   `json:"replicasRunning"`
	ReplicasDesired    int64   `json:"replicasDesired"`
	RecentTaskFailures int64   `json:"recentTaskFailures"`
	MemoryUsedMiB      float64 `json:"memoryUsedMiB"`
	MemoryLimitMiB     float64 `json:"memoryLimitMiB"`
	CPUPercent         float64 `json:"cpuPercent"`
	CPULimit           float64 `json:"cpuLimit"`
	DiskGB             float64 `json:"diskGB"`
	NetRxMB            float64 `json:"netRxMB"`
	NetTxMB            float64 `json:"netTxMB"`
	BlockReadMB        float64 `json:"blockReadMB"`
	BlockWriteMB       float64 `json:"blockWriteMB"`
}

// NewService returns the service named name for the Swarm service: the state of its containers in snapshot, its replicas, its failed tasks, its limits, the size of its named volumes and the stats of its containers.
func NewService(name string, service docker.SwarmService, snapshot docker.Snapshot) Service {
	containers := serviceContainers(snapshot.Containers, service.Spec.Name)
	limits := service.Spec.TaskTemplate.Resources.Limits
	entry := Service{
		Name:               name,
		State:              serviceState(containers),
		ReplicasRunning:    service.ServiceStatus.RunningTasks,
		ReplicasDesired:    service.ServiceStatus.DesiredTasks,
		RecentTaskFailures: taskFailures(snapshot.Tasks, service.ID),
		MemoryLimitMiB:     float64(limits.MemoryBytes) / bytesPerMiB,
		CPULimit:           float64(limits.NanoCPUs) / nanoCPUsPerCPU,
		DiskGB:             float64(volumesSize(containers, volumeSizes(snapshot.DiskUsage))) / bytesPerGB,
	}
	entry.addContainersStats(containers, snapshot.Stats)

	return entry
}

// addContainersStats adds the stats of each of containers that has an entry in stats.
func (s *Service) addContainersStats(containers []docker.Container, stats map[string]docker.ContainerStats) {
	for _, container := range containers {
		if containerStats, ok := stats[container.ID]; ok {
			s.addStats(containerStats)
		}
	}
}

// addStats adds the memory used, the CPU percent, the network received and sent and the block read and written of stats.
func (s *Service) addStats(stats docker.ContainerStats) {
	rx, tx := network(stats)
	read, write := blockIO(stats)
	s.MemoryUsedMiB += memoryUsed(stats) / bytesPerMiB
	s.CPUPercent += cpuPercent(stats)
	s.NetRxMB += rx / bytesPerMB
	s.NetTxMB += tx / bytesPerMB
	s.BlockReadMB += read / bytesPerMB
	s.BlockWriteMB += write / bytesPerMB
}

// serviceContainers returns the containers whose service name label is name.
func serviceContainers(containers []docker.Container, name string) []docker.Container {
	var matched []docker.Container
	for _, container := range containers {
		if container.Labels[docker.ServiceNameLabel] == name {
			matched = append(matched, container)
		}
	}

	return matched
}

// serviceState returns the most alive state of containers, missing when there is none and dead when no state is known.
func serviceState(containers []docker.Container) string {
	if len(containers) == 0 {
		return stateMissing
	}

	states := containerStates(containers)
	for _, state := range statePrecedence {
		if slices.Contains(states, state) {
			return state
		}
	}

	return stateDead
}

// containerStates returns the state of each of containers; a container being removed counts as exited.
func containerStates(containers []docker.Container) []string {
	states := make([]string, 0, len(containers))
	for _, container := range containers {
		state := container.State
		if state == containerRemoving {
			state = stateExited
		}

		states = append(states, state)
	}

	return states
}

// taskFailures returns how many tasks of the service serviceID failed or were rejected.
func taskFailures(tasks []docker.SwarmTask, serviceID string) int64 {
	var failures int64
	for _, task := range tasks {
		if task.ServiceID == serviceID && (task.Status.State == taskFailed || task.Status.State == taskRejected) {
			failures++
		}
	}

	return failures
}

// volumeSizes returns the bytes of each volume of usage by its name; a negative size counts as 0.
func volumeSizes(usage docker.DiskUsage) map[string]int64 {
	sizes := map[string]int64{}
	for _, volume := range usage.Volumes {
		sizes[volume.Name] = max(volume.UsageData.Size, 0)
	}

	return sizes
}

// volumesSize returns the bytes of the named volumes that containers mount, each volume counted once, from their sizes.
func volumesSize(containers []docker.Container, sizes map[string]int64) int64 {
	volumes := map[string]int64{}
	for _, container := range containers {
		for _, mount := range container.Mounts {
			if mount.Type == volumeMountType {
				volumes[mount.Name] = sizes[mount.Name]
			}
		}
	}

	var size int64
	for _, volume := range volumes {
		size += volume
	}

	return size
}

// memoryUsed returns the bytes of memory of stats without the inactive file cache, by the docker formula: the total_inactive_file of cgroup v1 has priority over the inactive_file of cgroup v2, each subtracted only when below the usage.
func memoryUsed(stats docker.ContainerStats) float64 {
	usage := stats.MemoryStats.Usage
	if inactive, ok := stats.MemoryStats.Stats[cgroupV1InactiveFile]; ok && inactive < usage {
		return float64(usage - inactive)
	}

	if inactive := stats.MemoryStats.Stats[cgroupV2InactiveFile]; inactive < usage {
		return float64(usage - inactive)
	}

	return float64(usage)
}

// cpuPercent returns the CPU of stats used between its two samples, where 100 is one core, or 0 when the usage of the container or of the system did not grow.
func cpuPercent(stats docker.ContainerStats) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage) - float64(stats.PreCPUStats.SystemCPUUsage)
	if systemDelta <= 0 {
		return 0
	}

	return max(cpuDelta, 0) / systemDelta * onlineCPUs(stats.CPUStats) * percent
}

// onlineCPUs returns the online CPUs of sample or, when it has none, the number of CPUs in its per-CPU usage.
func onlineCPUs(sample docker.CPUStats) float64 {
	if sample.OnlineCPUs == 0 {
		return float64(len(sample.CPUUsage.PercpuUsage))
	}

	return float64(sample.OnlineCPUs)
}

// network returns the bytes received and sent over every network of stats.
func network(stats docker.ContainerStats) (rx, tx float64) {
	for _, traffic := range stats.Networks {
		rx += float64(traffic.RxBytes)
		tx += float64(traffic.TxBytes)
	}

	return rx, tx
}

// blockIO returns the bytes read from and written to the block devices of stats, matching the operation case-insensitively.
func blockIO(stats docker.ContainerStats) (read, write float64) {
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch {
		case strings.EqualFold(entry.Op, blkioRead):
			read += float64(entry.Value)
		case strings.EqualFold(entry.Op, blkioWrite):
			write += float64(entry.Value)
		}
	}

	return read, write
}
