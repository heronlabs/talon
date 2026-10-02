package docker

import (
	"strings"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

const swarmServiceLabel = "com.docker.swarm.service.name"

type serviceDTO struct {
	ID   string
	Spec struct {
		Name string
		Mode struct {
			Replicated *struct{ Replicas int }
			Global     *struct{}
		}
		TaskTemplate struct {
			Resources struct {
				Limits struct {
					NanoCPUs    int64
					MemoryBytes int64
				}
			}
		}
	}
}

type taskDTO struct {
	ServiceID string
	Status    struct{ State string }
}

type containerDTO struct {
	ID     string `json:"Id"`
	State  string
	Labels map[string]string
	Mounts []struct {
		Type   string
		Name   string
		Source string
	}
}

type diskUsageDTO struct {
	LayersSize int64
	BuildCache []struct{ Size int64 }
	Volumes    []struct {
		Name      string
		UsageData struct{ Size int64 }
	}
}

type cpuDTO struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint64 `json:"online_cpus"`
}

type statsDTO struct {
	CPU         cpuDTO `json:"cpu_stats"`
	PreCPU      cpuDTO `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IOServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

func toSample(services []serviceDTO, tasks []taskDTO, containers []containerDTO, df diskUsageDTO) domain.DockerSample {
	s := domain.DockerSample{VolumeSizes: map[string]domain.Bytes{}, ImagesSize: domain.BytesOf(df.LayersSize)}
	for _, svc := range services {
		s.Services = append(s.Services, svc.toDomain())
	}
	for _, t := range tasks {
		s.Tasks = append(s.Tasks, domain.SwarmTask{ServiceID: t.ServiceID, State: domain.TaskState(t.Status.State)})
	}
	for _, c := range containers {
		s.Containers = append(s.Containers, c.toDomain())
	}
	for _, b := range df.BuildCache {
		s.BuildCacheSize += domain.BytesOf(b.Size)
	}
	for _, v := range df.Volumes {
		s.VolumeSizes[v.Name] = domain.BytesOf(v.UsageData.Size)
	}
	return s
}

func (s serviceDTO) toDomain() domain.SwarmService {
	desired := 0
	if s.Spec.Mode.Global != nil {
		desired = 1
	}
	if r := s.Spec.Mode.Replicated; r != nil {
		desired = r.Replicas
	}
	limits := s.Spec.TaskTemplate.Resources.Limits
	return domain.SwarmService{
		ID:              s.ID,
		Name:            s.Spec.Name,
		ReplicasDesired: desired,
		MemoryLimit:     domain.BytesOf(limits.MemoryBytes),
		CPULimit:        float64(limits.NanoCPUs) / 1e9,
	}
}

func (c containerDTO) toDomain() domain.Container {
	out := domain.Container{ID: c.ID, ServiceName: c.Labels[swarmServiceLabel], State: containerState(c.State)}
	for _, m := range c.Mounts {
		switch m.Type {
		case "volume":
			out.Volumes = append(out.Volumes, m.Name)
		case "bind":
			out.BindSources = append(out.BindSources, m.Source)
		}
	}
	return out
}

func (s statsDTO) toDomain() domain.ContainerStats {
	out := domain.ContainerStats{MemoryUsed: domain.Bytes(memoryUsed(s.MemoryStats.Usage, s.MemoryStats.Stats))}
	out.CPUPercent = cpuPercent(s.CPU, s.PreCPU)
	for _, n := range s.Networks {
		out.NetRx += domain.Bytes(n.RxBytes)
		out.NetTx += domain.Bytes(n.TxBytes)
	}
	for _, io := range s.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(io.Op) {
		case "read":
			out.BlockRead += domain.Bytes(io.Value)
		case "write":
			out.BlockWrite += domain.Bytes(io.Value)
		}
	}
	return out
}

// containerState maps the engine's container states to the contract; "removing" reads as exited
// and anything unknown as dead.
func containerState(s string) domain.ServiceState {
	switch s {
	case "running", "restarting", "paused", "created", "exited", "dead":
		return domain.ServiceState(s)
	case "removing":
		return domain.StateExited
	}
	return domain.StateDead
}

// memoryUsed follows the docker CLI: usage minus the inactive page cache (cgroup v1 key first,
// then v2), unless the cache is not below the usage.
func memoryUsed(usage uint64, stats map[string]uint64) uint64 {
	inactive, ok := stats["total_inactive_file"]
	if !ok {
		inactive = stats["inactive_file"]
	}
	if inactive < usage {
		return usage - inactive
	}
	return usage
}

// cpuPercent follows the docker CLI: 100 is one full core.
func cpuPercent(cur, pre cpuDTO) float64 {
	if cur.SystemUsage <= pre.SystemUsage {
		return 0
	}
	cpuDelta := max(float64(cur.CPUUsage.TotalUsage)-float64(pre.CPUUsage.TotalUsage), 0)
	systemDelta := float64(cur.SystemUsage - pre.SystemUsage)
	return cpuDelta / systemDelta * float64(max(cur.OnlineCPUs, 1)) * 100
}
