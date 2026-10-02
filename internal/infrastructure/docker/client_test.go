package docker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const statsPath = "/containers/c1/stats?stream=false"

var baseRoutes = map[string]string{
	"/_ping": "OK",
	"/services": `[
		{"ID":"s1","Spec":{"Name":"p_api","Mode":{"Replicated":{"Replicas":2}},
			"TaskTemplate":{"Resources":{"Limits":{"NanoCPUs":500000000,"MemoryBytes":536870912}}}}},
		{"ID":"s2","Spec":{"Name":"p_cron","Mode":{"Global":{}}}},
		{"ID":"s3","Spec":{"Name":"p_job","Mode":{}}}]`,
	"/tasks": `[{"ServiceID":"s1","Status":{"State":"running"}}]`,
	"/containers/json?all=1": `[
		{"Id":"c1","State":"running","Labels":{"com.docker.swarm.service.name":"p_api"},"Mounts":[
			{"Type":"volume","Name":"p_api_data","Source":"/var/lib/docker/volumes/p_api_data/_data"},
			{"Type":"bind","Source":"/etc/easypanel/projects/p/api/code"},
			{"Type":"tmpfs"}]},
		{"Id":"c2","State":"exited","Labels":{}}]`,
	"/system/df": `{"LayersSize":6100000000,"BuildCache":[{"Size":2000000000},{"Size":400000000}],
		"Volumes":[{"Name":"p_api_data","UsageData":{"Size":800}},{"Name":"orphan","UsageData":{"Size":-1}}]}`,
	statsPath: `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000},
		"memory_stats":{"usage":1000,"stats":{"inactive_file":200}},
		"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20},"eth1":{"rx_bytes":1,"tx_bytes":2}},
		"blkio_stats":{"io_service_bytes_recursive":[{"op":"read","value":30},{"op":"Write","value":40},{"op":"discard","value":9}]}}`,
}

func routes(overrides map[string]string) map[string]string {
	r := maps.Clone(baseRoutes)
	for path, body := range overrides {
		if body == "" {
			delete(r, path)
			continue
		}
		r[path] = body
	}
	return r
}

func client(t *testing.T, r map[string]string, statsTimeout time.Duration) *docker.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, ok := r[req.URL.RequestURI()]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return docker.New(srv.Client(), srv.URL, statsTimeout)
}

func snapshot(t *testing.T, overrides map[string]string) (domain.DockerSample, error) {
	t.Helper()
	return client(t, routes(overrides), time.Second).Snapshot(context.Background())
}

func statsOf(t *testing.T, statsBody string) *domain.ContainerStats {
	t.Helper()
	s := testkit.Must(snapshot(t, map[string]string{statsPath: statsBody}))
	return s.Containers[0].Stats
}

func TestPing_SucceedsWhenEngineAnswersOK(t *testing.T) {
	err := client(t, baseRoutes, time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, nil)
}

func TestPing_FailsOnNonOKStatus(t *testing.T) {
	err := client(t, routes(map[string]string{"/_ping": ""}), time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestPing_FailsWhenEngineIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := docker.New(srv.Client(), srv.URL, time.Second)
	srv.Close()

	err := c.Ping(context.Background())

	testkit.ErrorIs(t, err, syscall.ECONNREFUSED)
}

func TestPing_FailsOnInvalidBaseURL(t *testing.T) {
	var urlErr *url.Error

	err := docker.New(http.DefaultClient, "http://bad host", time.Second).Ping(context.Background())

	testkit.Equal(t, true, errors.As(err, &urlErr))
}

func TestNewUnix_TalksThroughTheSocket(t *testing.T) {
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	ln := testkit.Must(net.Listen("unix", socket))
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "OK")
	}), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	err := docker.NewUnix(socket, time.Second, time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, nil)
}

func TestSnapshot_MapsTheEngineState(t *testing.T) {
	got := testkit.Must(snapshot(t, nil))

	testkit.Equal(t, domain.DockerSample{
		Services: []domain.SwarmService{
			{ID: "s1", Name: "p_api", ReplicasDesired: 2, MemoryLimit: 536870912, CPULimit: 0.5},
			{ID: "s2", Name: "p_cron", ReplicasDesired: 1},
			{ID: "s3", Name: "p_job"},
		},
		Tasks: []domain.SwarmTask{{ServiceID: "s1", State: domain.TaskRunning}},
		Containers: []domain.Container{
			{ID: "c1", ServiceName: "p_api", State: domain.StateRunning,
				Volumes:     []string{"p_api_data"},
				BindSources: []string{"/etc/easypanel/projects/p/api/code"},
				Stats: &domain.ContainerStats{
					MemoryUsed: 800, CPUPercent: 4, NetRx: 11, NetTx: 22, BlockRead: 30, BlockWrite: 40,
				}},
			{ID: "c2", State: domain.StateExited},
		},
		VolumeSizes:    map[string]domain.Bytes{"p_api_data": 800, "orphan": 0},
		ImagesSize:     6100000000,
		BuildCacheSize: 2400000000,
	}, got)
}

func TestSnapshot_FailsWhenServicesFail(t *testing.T) {
	_, err := snapshot(t, map[string]string{"/services": ""})

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsWhenNodeIsNotASwarmManager(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"This node is not a swarm manager."}`, http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	_, err := docker.New(srv.Client(), srv.URL, time.Second).Snapshot(context.Background())

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsWhenDiskUsageFails(t *testing.T) {
	_, err := snapshot(t, map[string]string{"/system/df": ""})

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsOnInvalidJSON(t *testing.T) {
	var syntaxErr *json.SyntaxError

	_, err := snapshot(t, map[string]string{"/tasks": "{"})

	testkit.Equal(t, true, errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF))
}

func TestSnapshot_FailedStatsLeaveStatsEmpty(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{statsPath: ""}))

	testkit.Equal(t, (*domain.ContainerStats)(nil), got.Containers[0].Stats)
}

func TestSnapshot_SlowStatsLeaveStatsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.RequestURI() == statsPath {
			<-req.Context().Done()
			return
		}
		_, _ = io.WriteString(w, baseRoutes[req.URL.RequestURI()])
	}))
	t.Cleanup(srv.Close)
	c := docker.New(srv.Client(), srv.URL, 20*time.Millisecond)

	got := testkit.Must(c.Snapshot(context.Background()))

	testkit.Equal(t, (*domain.ContainerStats)(nil), got.Containers[0].Stats)
}

func TestSnapshot_MemoryUsesCgroupV1InactiveFile(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":1000,"stats":{"total_inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(700), got.MemoryUsed)
}

func TestSnapshot_MemoryNeverGoesBelowZero(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":100,"stats":{"inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(100), got.MemoryUsed)
}

func TestSnapshot_CPUIsZeroWithoutSystemDelta(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":100000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_CPUIsZeroWithoutUsageDelta(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_CPUIsZeroWhenUsageCounterGoesBack(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_MemoryKeepsUsageWhenInactiveCacheEqualsIt(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":300,"stats":{"inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(300), got.MemoryUsed)
}

func TestSnapshot_MemoryPrefersTheCgroupV1Key(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":1000,"stats":{"total_inactive_file":300,"inactive_file":100}}}`)

	testkit.Equal(t, domain.Bytes(700), got.MemoryUsed)
}

func TestSnapshot_RemovingContainerReadsAsExited(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{"/containers/json?all=1": `[{"Id":"c9","State":"removing"}]`}))

	testkit.Equal(t, domain.StateExited, got.Containers[0].State)
}

func TestSnapshot_UnknownContainerStateReadsAsDead(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{"/containers/json?all=1": `[{"Id":"c9","State":"zombie"}]`}))

	testkit.Equal(t, domain.StateDead, got.Containers[0].State)
}

func TestSnapshot_CPUAssumesOneCoreWhenOnlineCPUsIsMissing(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":200000},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 2.0, got.CPUPercent)
}
