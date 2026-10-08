package docker

import (
	"fmt"
	"net/url"
)

// ServiceNameLabel is the label with the name of the Swarm service of a container.
const ServiceNameLabel = "com.docker.swarm.service.name"

// The paths of the Engine API, the filter of the containers by a label, and the state of a running container.
const (
	diskUsagePath    = "/system/df"
	servicesPath     = "/services?status=true"
	tasksPath        = "/tasks"
	containersPath   = "/containers/json?all=true&filters="
	statsPath        = "/containers/%s/stats?stream=false"
	containersFilter = `{"label":["%s"]}`
	runningState     = "running"
)

// ReportCollectorService collects the snapshot of the host from the Engine API.
type ReportCollectorService struct {
	client *SocketClient
}

// NewReportCollectorService returns a ReportCollectorService that requests the Engine API through client.
func NewReportCollectorService(client *SocketClient) *ReportCollectorService {
	return &ReportCollectorService{client: client}
}

// Collect returns the snapshot of the host: the disk usage, the Swarm services and tasks, the containers of the Swarm services, running or not, and the stats of the running ones.
func (s *ReportCollectorService) Collect() (*Snapshot, error) {
	var snapshot Snapshot
	if err := s.client.Get(diskUsagePath, &snapshot.DiskUsage); err != nil {
		return nil, err
	}

	if err := s.client.Get(servicesPath, &snapshot.Services); err != nil {
		return nil, err
	}

	if err := s.client.Get(tasksPath, &snapshot.Tasks); err != nil {
		return nil, err
	}

	if err := s.client.Get(containersPath+url.QueryEscape(fmt.Sprintf(containersFilter, ServiceNameLabel)), &snapshot.Containers); err != nil {
		return nil, err
	}

	stats, err := s.stats(snapshot.Containers)
	if err != nil {
		return nil, err
	}
	snapshot.Stats = stats

	return &snapshot, nil
}

// stats returns the stats of each running container of containers, by its ID.
func (s *ReportCollectorService) stats(containers []Container) (map[string]ContainerStats, error) {
	stats := map[string]ContainerStats{}
	for _, container := range containers {
		if container.State != runningState {
			continue
		}

		var entry ContainerStats
		if err := s.client.Get(fmt.Sprintf(statsPath, container.ID), &entry); err != nil {
			return nil, err
		}

		stats[container.ID] = entry
	}

	return stats, nil
}
