package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
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

// maxParallelStats is the most requests for the stats of containers that run at once.
const maxParallelStats = 8

// ReportCollectorService collects the snapshot of the host from the Engine API.
type ReportCollectorService struct {
	client *SocketClient
}

// NewReportCollectorService returns a ReportCollectorService that requests the Engine API through client.
func NewReportCollectorService(client *SocketClient) *ReportCollectorService {
	return &ReportCollectorService{client: client}
}

// Collect returns the snapshot of the host, requested with ctx: the disk usage, the Swarm services and tasks, the containers of the Swarm services, running or not, the stats of the running ones and the package.json of the running container of Easypanel.
func (s *ReportCollectorService) Collect(ctx context.Context) (*Snapshot, error) {
	var snapshot Snapshot
	if err := s.client.Get(ctx, diskUsagePath, &snapshot.DiskUsage); err != nil {
		return nil, err
	}

	if err := s.client.Get(ctx, servicesPath, &snapshot.Services); err != nil {
		return nil, err
	}

	if err := s.client.Get(ctx, tasksPath, &snapshot.Tasks); err != nil {
		return nil, err
	}

	if err := s.client.Get(ctx, containersPath+url.QueryEscape(fmt.Sprintf(containersFilter, ServiceNameLabel)), &snapshot.Containers); err != nil {
		return nil, err
	}

	stats, err := s.stats(ctx, snapshot.Containers)
	if err != nil {
		return nil, err
	}
	snapshot.Stats = stats

	easypanelPackage, err := s.easypanelPackage(ctx, snapshot.Containers)
	if err != nil {
		return nil, err
	}
	snapshot.Easypanel = easypanelPackage

	return &snapshot, nil
}

// stats returns the stats of each running container of containers, by its ID, leaving out a container whose stats answer 404 and failing with the first other error.
func (s *ReportCollectorService) stats(ctx context.Context, containers []Container) (map[string]ContainerStats, error) {
	ids := runningIDs(containers)
	entries := make([]ContainerStats, len(ids))
	errs := make([]error, len(ids))

	s.requestStats(ctx, ids, entries, errs)

	return statsByID(ids, entries, errs)
}

// runningIDs returns the IDs of the running containers of containers.
func runningIDs(containers []Container) []string {
	ids := []string{}
	for _, container := range containers {
		if container.State == runningState {
			ids = append(ids, container.ID)
		}
	}

	return ids
}

// requestStats requests the stats of each container of ids with ctx, at most maxParallelStats at once, into the entry and the error at its index, and returns when every request is done.
func (s *ReportCollectorService) requestStats(ctx context.Context, ids []string, entries []ContainerStats, errs []error) {
	var group sync.WaitGroup
	slots := make(chan struct{}, maxParallelStats)
	for index, id := range ids {
		slots <- struct{}{}
		group.Go(func() {
			defer func() { <-slots }()
			errs[index] = s.client.Get(ctx, fmt.Sprintf(statsPath, id), &entries[index])
		})
	}

	group.Wait()
}

// statsByID returns each entry by the ID at its index, leaving out those whose error is ErrNotFound, and fails with the first other error.
func statsByID(ids []string, entries []ContainerStats, errs []error) (map[string]ContainerStats, error) {
	stats := map[string]ContainerStats{}
	for index, id := range ids {
		if errors.Is(errs[index], ErrNotFound) {
			continue
		}

		if errs[index] != nil {
			return nil, errs[index]
		}

		stats[id] = entries[index]
	}

	return stats, nil
}

// easypanelPackage returns the package.json of the first running container of containers of the Swarm service easypanel, requested with ctx, nil when there is none.
func (s *ReportCollectorService) easypanelPackage(ctx context.Context, containers []Container) (*EasypanelPackage, error) {
	for _, container := range containers {
		if container.State == runningState && container.Labels[ServiceNameLabel] == easypanelService {
			return s.readPackage(ctx, container.ID)
		}
	}

	return nil, nil
}

// readPackage returns the package.json of the container id, from the archive of /app/package.json requested with ctx, nil when the archive answers 404.
func (s *ReportCollectorService) readPackage(ctx context.Context, id string) (*EasypanelPackage, error) {
	archive, err := s.client.Open(ctx, fmt.Sprintf(archivePath, id))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
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
