package docker

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

type reportEngine struct {
	diskUsage  http.HandlerFunc
	services   http.HandlerFunc
	tasks      http.HandlerFunc
	containers http.HandlerFunc
	stats      http.HandlerFunc
}

func reply(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}
}

func replyStatus(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}
}

func replyQuery(query *url.Values, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*query = r.URL.Query()
		_, _ = io.WriteString(w, body)
	}
}

func replyStats(stats map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, stats[r.PathValue("id")])
	}
}

func startReportEngine(t *testing.T, engine reportEngine) string {
	t.Helper()

	socketPath := newSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /system/df", engine.diskUsage)
	mux.HandleFunc("GET /services", engine.services)
	mux.HandleFunc("GET /tasks", engine.tasks)
	mux.HandleFunc("GET /containers/json", engine.containers)
	mux.HandleFunc("GET /containers/{id}/stats", engine.stats)

	server := httptest.NewUnstartedServer(mux)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}

func TestReportCollectorService(t *testing.T) {
	t.Run("Should decode the disk usage into the snapshot", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{"LayersSize":2500000000,"BuildCache":[{"Size":1000000000},{"Size":500000000}]}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := DiskUsage{LayersSize: 2500000000, BuildCache: []struct{ Size int64 }{{Size: 1000000000}, {Size: 500000000}}}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.DiskUsage, want) {
			t.Errorf("Collect() = %+v, %v; want disk usage %+v", snapshot, err, want)
		}
	})

	t.Run("Should decode the services into the snapshot", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[{"ID":"s1","Spec":{"Name":"shop_api"},"ServiceStatus":{"RunningTasks":2,"DesiredTasks":3}}]`),
			tasks:      reply(`[]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		api := SwarmService{ID: "s1"}
		api.Spec.Name = "shop_api"
		api.ServiceStatus.RunningTasks = 2
		api.ServiceStatus.DesiredTasks = 3
		want := []SwarmService{api}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Services, want) {
			t.Errorf("Collect() = %+v, %v; want services %+v", snapshot, err, want)
		}
	})

	t.Run("Should decode the tasks into the snapshot", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[{"ServiceID":"s1","Status":{"State":"failed"}}]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		failed := SwarmTask{ServiceID: "s1"}
		failed.Status.State = "failed"
		want := []SwarmTask{failed}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Tasks, want) {
			t.Errorf("Collect() = %+v, %v; want tasks %+v", snapshot, err, want)
		}
	})

	t.Run("Should decode the containers into the snapshot", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"}]}]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := []Container{{
			ID:     "a1",
			State:  "exited",
			Labels: map[string]string{"com.docker.swarm.service.name": "shop_api"},
			Mounts: []struct{ Type, Name string }{{Type: "volume", Name: "shop_data"}},
		}}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Containers, want) {
			t.Errorf("Collect() = %+v, %v; want containers %+v", snapshot, err, want)
		}
	})

	t.Run("Should request the containers with all=true and the filter of the service name label", func(t *testing.T) {
		var query url.Values
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: replyQuery(&query, `[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, _ = service.Collect()

		if query.Get("all") != "true" || query.Get("filters") != `{"label":["com.docker.swarm.service.name"]}` {
			t.Errorf("Collect() sent query %v, want all=true and filters %s", query, `{"label":["com.docker.swarm.service.name"]}`)
		}
	})

	t.Run("Should request the stats of the running containers only, by their ID", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage: reply(`{}`),
			services:  reply(`[]`),
			tasks:     reply(`[]`),
			containers: reply(`[
				{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_web"}},
				{"Id":"c3","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_api"}}
			]`),
			stats: replyStats(map[string]string{
				"a1": `{"memory_stats":{"usage":104857600}}`,
				"b2": `{"memory_stats":{"usage":52428800}}`,
			}),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		a1 := ContainerStats{}
		a1.MemoryStats.Usage = 104857600
		b2 := ContainerStats{}
		b2.MemoryStats.Usage = 52428800
		want := map[string]ContainerStats{"a1": a1, "b2": b2}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Stats, want) {
			t.Errorf("Collect() = %+v, %v; want stats %+v", snapshot, err, want)
		}
	})

	t.Run("Should fail when the disk usage answers other than 200", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  replyStatus(http.StatusInternalServerError),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect()

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the services answer other than 200", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   replyStatus(http.StatusServiceUnavailable),
			tasks:      reply(`[]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect()

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the tasks answer other than 200", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      replyStatus(http.StatusServiceUnavailable),
			containers: reply(`[]`),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect()

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the containers answer other than 200", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: replyStatus(http.StatusInternalServerError),
			stats:      replyStats(nil),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect()

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the stats of a running container answer other than 200", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[{"ID":"s1","Spec":{"Name":"shop_api"}}]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`),
			stats:      replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect()

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})
}
