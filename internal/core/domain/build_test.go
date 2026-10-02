package domain_test

import (
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var now = time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)

func input(d domain.DockerSample) domain.ReportInput {
	return domain.ReportInput{Host: "vps-heron-02", AgentVersion: "1.0.0", Now: now, DockerSample: d}
}

func service(id, name string) domain.SwarmService {
	return domain.SwarmService{ID: id, Name: name, ReplicasDesired: 1}
}

func onlyService(r domain.Report) domain.Service {
	return r.Projects[0].Services[0]
}

func TestBuildReport_EmptyDockerGivesHostOnlyReport(t *testing.T) {
	in := input(domain.DockerSample{})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Report{
		Host:         "vps-heron-02",
		MeasuredAt:   now,
		AgentVersion: "1.0.0",
		Projects:     []domain.Project{},
	}, got)
}

func TestBuildReport_DerivesSystemFromHostAndDocker(t *testing.T) {
	in := input(domain.DockerSample{ImagesSize: 6, BuildCacheSize: 2})
	in.HostSample = domain.HostSample{
		MemTotal: 100, MemAvailable: 40, SwapTotal: 10, SwapFree: 7,
		Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: time.Hour,
		DiskTotal: 1000, DiskUsed: 412,
	}

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.System{
		RAMTotal: 100, RAMUsed: 60, SwapTotal: 10, SwapUsed: 3,
		DiskTotal: 1000, DiskUsed: 412,
		Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: time.Hour,
		DockerImages: 6, DockerBuildCache: 2,
	}, got.System)
}

func TestBuildReport_GroupsServicesByProjectInNameOrder(t *testing.T) {
	in := input(domain.DockerSample{Services: []domain.SwarmService{
		service("3", "shop_web"), service("1", "cms-heronlabs_cms-database"), service("2", "cms-heronlabs_cms-api"),
	}})

	got := domain.BuildReport(in)

	testkit.Equal(t, []domain.Project{
		{Name: "cms-heronlabs", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "cms-api", State: domain.StateMissing, ReplicasDesired: 1},
			{Name: "cms-database", State: domain.StateMissing, ReplicasDesired: 1},
		}},
		{Name: "shop", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "web", State: domain.StateMissing, ReplicasDesired: 1},
		}},
	}, got.Projects)
}

func TestBuildReport_LeavesOutServicesWithoutProject(t *testing.T) {
	in := input(domain.DockerSample{Services: []domain.SwarmService{service("1", "easypanel")}})

	got := domain.BuildReport(in)

	testkit.Equal(t, []domain.Project{}, got.Projects)
}

func TestBuildReport_CopiesServiceLimits(t *testing.T) {
	svc := domain.SwarmService{ID: "1", Name: "p_api", ReplicasDesired: 2, MemoryLimit: 512, CPULimit: 0.5}
	in := input(domain.DockerSample{Services: []domain.SwarmService{svc}})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateMissing, ReplicasDesired: 2, MemoryLimit: 512, CPULimit: 0.5,
	}, onlyService(got))
}

func TestBuildReport_CountsRunningTasksAndRecentFailuresOfTheService(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Tasks: []domain.SwarmTask{
			{ServiceID: "1", State: domain.TaskRunning},
			{ServiceID: "1", State: domain.TaskFailed},
			{ServiceID: "1", State: domain.TaskRejected},
			{ServiceID: "1", State: "shutdown"},
			{ServiceID: "2", State: domain.TaskRunning},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateMissing, ReplicasRunning: 1, ReplicasDesired: 1, RecentTaskFailures: 2,
	}, onlyService(got))
}

func TestBuildReport_RunningContainerWinsOverLaterExitedOne(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning},
			{ID: "b", ServiceName: "p_api", State: domain.StateExited},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateRunning, onlyService(got).State)
}

func TestBuildReport_RunningContainerReplacesEarlierExitedOne(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "b", ServiceName: "p_api", State: domain.StateExited},
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateRunning, onlyService(got).State)
}

func TestBuildReport_UnknownContainerStateNeverWins(t *testing.T) {
	in := input(domain.DockerSample{
		Services:   []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_api", State: "unknown"}},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateMissing, onlyService(got).State)
}

func TestBuildReport_DeadContainerBeatsMissing(t *testing.T) {
	in := input(domain.DockerSample{
		Services:   []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_api", State: domain.StateDead}},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateDead, onlyService(got).State)
}

func TestBuildReport_SumsStatsOfTheServiceContainers(t *testing.T) {
	stats := &domain.ContainerStats{MemoryUsed: 100, CPUPercent: 1.5, NetRx: 10, NetTx: 20, BlockRead: 30, BlockWrite: 40}
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning, Stats: stats},
			{ID: "b", ServiceName: "p_api", State: domain.StateRunning, Stats: stats},
			{ID: "c", ServiceName: "p_api", State: domain.StateRunning},
			{ID: "d", ServiceName: "other_api", State: domain.StateRunning, Stats: stats},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateRunning, ReplicasDesired: 1,
		MemoryUsed: 200, CPUPercent: 3, NetRx: 20, NetTx: 40, BlockRead: 60, BlockWrite: 80,
	}, onlyService(got))
}

func TestBuildReport_DiskCountsEachVolumeOnceAndTheServiceBindDirs(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_db")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_db", State: domain.StateRunning, Volumes: []string{"p_db_data"},
				BindSources: []string{"/etc/easypanel/projects/p/db/conf", "/var/run/docker.sock"}},
			{ID: "b", ServiceName: "p_db", State: domain.StateExited, Volumes: []string{"p_db_data"}},
		},
		VolumeSizes: map[string]domain.Bytes{"p_db_data": 800, "other": 5},
	})
	in.DirSizes = map[string]domain.Bytes{"/etc/easypanel/projects/p/db/conf": 3, "/var/run/docker.sock": 9}

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Bytes(803), onlyService(got).Disk)
}
