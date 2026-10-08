package docker

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"net/url"
)

// ServiceNameLabel is the label with the name of the Swarm service of a container.
const ServiceNameLabel = "com.docker.swarm.service.name"

// The paths of the Engine API, the filter of the containers by a label, the state of a running container and the Swarm service of Easypanel.
const (
	diskUsagePath    = "/system/df"
	servicesPath     = "/services?status=true"
	tasksPath        = "/tasks"
	containersPath   = "/containers/json?all=true&filters="
	statsPath        = "/containers/%s/stats?stream=false"
	archivePath      = "/containers/%s/archive?path=/app/package.json"
	containersFilter = `{"label":["%s"]}`
	runningState     = "running"
	easypanelService = "easypanel"
)

// ReportCollectorService collects the snapshot of the host from the Engine API.
type ReportCollectorService struct {
	client *SocketClient
}

// NewReportCollectorService returns a ReportCollectorService that requests the Engine API through client.
func NewReportCollectorService(client *SocketClient) *ReportCollectorService {
	return &ReportCollectorService{client: client}
}

// Collect returns the snapshot of the host: the disk usage, the Swarm services and tasks, the containers of the Swarm services, running or not, the stats of the running ones and the package.json of the running container of Easypanel.
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

	easypanelPackage, err := s.easypanelPackage(snapshot.Containers)
	if err != nil {
		return nil, err
	}
	snapshot.Easypanel = easypanelPackage

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

// easypanelPackage returns the package.json of the first running container of containers of the Swarm service easypanel, nil when there is none.
func (s *ReportCollectorService) easypanelPackage(containers []Container) (*EasypanelPackage, error) {
	for _, container := range containers {
		if container.State == runningState && container.Labels[ServiceNameLabel] == easypanelService {
			return s.readPackage(container.ID)
		}
	}

	return nil, nil
}

// readPackage returns the package.json of the container id, from the archive of /app/package.json.
func (s *ReportCollectorService) readPackage(id string) (*EasypanelPackage, error) {
	archive, err := s.client.Open(fmt.Sprintf(archivePath, id))
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	return decodePackage(tar.NewReader(archive))
}

// decodePackage returns the package.json decoded from the first entry of reader, the tar of the archive; it fails when the tar has no entry or is invalid, or when the entry is not JSON.
func decodePackage(reader *tar.Reader) (*EasypanelPackage, error) {
	if _, err := reader.Next(); err != nil {
		return nil, err
	}

	var easypanelPackage EasypanelPackage
	if err := json.NewDecoder(reader).Decode(&easypanelPackage); err != nil {
		return nil, err
	}

	return &easypanelPackage, nil
}
