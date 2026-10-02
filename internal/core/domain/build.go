package domain

import (
	"slices"
	"strings"
	"time"
)

// ReportInput is everything BuildReport needs.
type ReportInput struct {
	Host         string
	AgentVersion string
	Now          time.Time
	HostSample   HostSample
	DockerSample DockerSample
	DirSizes     map[string]Bytes
}

// BuildReport turns one host reading and one Docker reading into a Report.
// Swarm services named without "<project>_<service>" (Easypanel's own) are left out.
func BuildReport(in ReportInput) Report {
	return Report{
		Host:         in.Host,
		MeasuredAt:   in.Now,
		AgentVersion: in.AgentVersion,
		System:       buildSystem(in.HostSample, in.DockerSample),
		Projects:     buildProjects(in.DockerSample, in.DirSizes),
	}
}

func buildSystem(h HostSample, d DockerSample) System {
	return System{
		RAMTotal:         h.MemTotal,
		RAMUsed:          h.MemTotal - h.MemAvailable,
		SwapTotal:        h.SwapTotal,
		SwapUsed:         h.SwapTotal - h.SwapFree,
		DiskTotal:        h.DiskTotal,
		DiskUsed:         h.DiskUsed,
		Load1:            h.Load1,
		Load5:            h.Load5,
		Load15:           h.Load15,
		Uptime:           h.Uptime,
		DockerImages:     d.ImagesSize,
		DockerBuildCache: d.BuildCacheSize,
	}
}

func buildProjects(d DockerSample, dirSizes map[string]Bytes) []Project {
	byName := map[string]*Project{}
	for _, svc := range d.Services {
		project, name, ok := SplitServiceName(svc.Name)
		if !ok {
			continue
		}
		p, found := byName[project]
		if !found {
			p = &Project{Name: project, Reconcile: StateUnmanaged}
			byName[project] = p
		}
		p.Services = append(p.Services, buildService(name, svc, d, dirSizes))
	}
	projects := make([]Project, 0, len(byName))
	for _, p := range byName {
		slices.SortFunc(p.Services, func(a, b Service) int { return strings.Compare(a.Name, b.Name) })
		projects = append(projects, *p)
	}
	slices.SortFunc(projects, func(a, b Project) int { return strings.Compare(a.Name, b.Name) })
	return projects
}

// SplitServiceName splits a Swarm service name "<project>_<service>" at the first underscore.
func SplitServiceName(full string) (project, service string, ok bool) {
	project, service, ok = strings.Cut(full, "_")
	return project, service, ok && project != "" && service != ""
}

func buildService(name string, svc SwarmService, d DockerSample, dirSizes map[string]Bytes) Service {
	out := Service{
		Name:            name,
		State:           StateMissing,
		ReplicasDesired: svc.ReplicasDesired,
		MemoryLimit:     svc.MemoryLimit,
		CPULimit:        svc.CPULimit,
	}
	for _, task := range d.Tasks {
		if task.ServiceID != svc.ID {
			continue
		}
		switch task.State {
		case TaskRunning:
			out.ReplicasRunning++
		case TaskFailed, TaskRejected:
			out.RecentTaskFailures++
		}
	}
	volumes := map[string]bool{}
	for _, c := range d.Containers {
		if c.ServiceName != svc.Name {
			continue
		}
		out.State = higherState(out.State, c.State)
		for _, v := range c.Volumes {
			volumes[v] = true
		}
		if c.Stats != nil {
			out.MemoryUsed += c.Stats.MemoryUsed
			out.CPUPercent += c.Stats.CPUPercent
			out.NetRx += c.Stats.NetRx
			out.NetTx += c.Stats.NetTx
			out.BlockRead += c.Stats.BlockRead
			out.BlockWrite += c.Stats.BlockWrite
		}
	}
	for v := range volumes {
		out.Disk += d.VolumeSizes[v]
	}
	for _, path := range BindMountPaths(d, svc.Name) {
		out.Disk += dirSizes[path]
	}
	return out
}

// stateOrder ranks service states from least to most alive.
var stateOrder = []ServiceState{
	StateMissing, StateDead, StateExited, StateCreated, StatePaused, StateRestarting, StateRunning,
}

// higherState keeps the most alive of two states; a state outside stateOrder never wins.
func higherState(current, candidate ServiceState) ServiceState {
	return stateOrder[max(slices.Index(stateOrder, current), slices.Index(stateOrder, candidate))]
}
