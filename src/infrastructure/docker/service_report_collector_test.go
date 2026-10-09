package docker

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type reportEngine struct {
	diskUsage  http.HandlerFunc
	services   http.HandlerFunc
	tasks      http.HandlerFunc
	containers http.HandlerFunc
	stats      http.HandlerFunc
	archive    http.HandlerFunc
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

func replyStatsOr(status int, stats map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, found := stats[r.PathValue("id")]
		if !found {
			w.WriteHeader(status)
		}
		_, _ = io.WriteString(w, body)
	}
}

func replyStatsAtOnce(inFlight int, peak *int) http.HandlerFunc {
	var mutex sync.Mutex
	var once sync.Once
	current := 0
	full := make(chan struct{})

	return func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		current++
		*peak = max(*peak, current)
		if current == inFlight {
			once.Do(func() { close(full) })
		}
		mutex.Unlock()

		select {
		case <-full:
		case <-r.Context().Done():
		}

		mutex.Lock()
		current--
		mutex.Unlock()
		_, _ = io.WriteString(w, `{}`)
	}
}

func replyArchives(archives map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, archives[r.PathValue("id")])
	}
}

func tarOf(t *testing.T, content string) string {
	t.Helper()

	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	if err := writer.WriteHeader(&tar.Header{Name: "package.json", Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}

	if _, err := io.WriteString(writer, content); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.String()
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
	mux.HandleFunc("GET /containers/{id}/archive", engine.archive)

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := DiskUsage{LayersSize: 2500000000, BuildCache: []struct{ Size int64 }{{Size: 1000000000}, {Size: 500000000}}}

		snapshot, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		api := SwarmService{ID: "s1"}
		api.Spec.Name = "shop_api"
		api.ServiceStatus.RunningTasks = 2
		api.ServiceStatus.DesiredTasks = 3
		want := []SwarmService{api}

		snapshot, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		failed := SwarmTask{ServiceID: "s1"}
		failed.Status.State = "failed"
		want := []SwarmTask{failed}

		snapshot, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := []Container{{
			ID:     "a1",
			State:  "exited",
			Labels: map[string]string{"com.docker.swarm.service.name": "shop_api"},
			Mounts: []struct{ Type, Name string }{{Type: "volume", Name: "shop_data"}},
		}}

		snapshot, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, _ = service.Collect(t.Context())

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
			archive: replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		a1 := ContainerStats{}
		a1.MemoryStats.Usage = 104857600
		b2 := ContainerStats{}
		b2.MemoryStats.Usage = 52428800
		want := map[string]ContainerStats{"a1": a1, "b2": b2}

		snapshot, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

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
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail with ErrNotFound when the services answer 404", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   replyStatus(http.StatusNotFound),
			tasks:      reply(`[]`),
			containers: reply(`[]`),
			stats:      replyStats(nil),
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Collect() = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("Should fail when the stats of a running container answer other than 200 and 404", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage: reply(`{}`),
			services:  reply(`[{"ID":"s1","Spec":{"Name":"shop_api"}}]`),
			tasks:     reply(`[]`),
			containers: reply(`[
				{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_web"}}
			]`),
			stats:   replyStatsOr(http.StatusInternalServerError, map[string]string{"b2": `{}`}),
			archive: replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should leave out the stats of a running container that answer 404", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage: reply(`{}`),
			services:  reply(`[]`),
			tasks:     reply(`[]`),
			containers: reply(`[
				{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_web"}}
			]`),
			stats:   replyStatsOr(http.StatusNotFound, map[string]string{"b2": `{"memory_stats":{"usage":52428800}}`}),
			archive: replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		b2 := ContainerStats{}
		b2.MemoryStats.Usage = 52428800
		want := map[string]ContainerStats{"b2": b2}

		snapshot, err := service.Collect(t.Context())

		if err != nil || !reflect.DeepEqual(snapshot.Stats, want) {
			t.Errorf("Collect() = %+v, %v; want stats %+v", snapshot, err, want)
		}
	})

	t.Run("Should request the stats of maxParallelStats running containers at once", func(t *testing.T) {
		peak := 0
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply("[" + strings.TrimSuffix(strings.Repeat(`{"Id":"a1","State":"running"},`, 2*maxParallelStats), ",") + "]"),
			stats:      replyStatsAtOnce(maxParallelStats, &peak),
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if err != nil || peak != maxParallelStats {
			t.Errorf("Collect() = %v with %d stats at once, want nil with %d", err, peak, maxParallelStats)
		}
	})

	t.Run("Should fail when the ID of a running container is not a valid path", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"%zz","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`),
			stats:      replyStats(nil),
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		var urlError *url.Error

		_, err := service.Collect(t.Context())

		if !errors.As(err, &urlError) || urlError.Op != "parse" {
			t.Errorf("Collect() = %v, want a %T from parse", err, urlError)
		}
	})

	t.Run("Should read the package.json of the running easypanel container into the snapshot", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    reply(tarOf(t, `{"name":"easypanel","version":"2.36.0"}`)),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := &EasypanelPackage{Version: "2.36.0"}

		snapshot, err := service.Collect(t.Context())

		if err != nil || !reflect.DeepEqual(snapshot.Easypanel, want) {
			t.Errorf("Collect() = %+v, %v; want Easypanel %+v", snapshot, err, want)
		}
	})

	t.Run("Should request the archive of /app/package.json", func(t *testing.T) {
		var query url.Values
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    replyQuery(&query, tarOf(t, `{"version":"2.36.0"}`)),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, _ = service.Collect(t.Context())

		if query.Get("path") != "/app/package.json" {
			t.Errorf("Collect() sent query %v, want path %s", query, "/app/package.json")
		}
	})

	t.Run("Should request the archive of the running easypanel container by its ID", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage: reply(`{}`),
			services:  reply(`[]`),
			tasks:     reply(`[]`),
			containers: reply(`[
				{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"easypanel"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}},
				{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}
			]`),
			stats:   replyStats(map[string]string{"b2": `{}`, "e1": `{}`}),
			archive: replyArchives(map[string]string{"e1": tarOf(t, `{"version":"2.36.0"}`)}),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		want := &EasypanelPackage{Version: "2.36.0"}

		snapshot, err := service.Collect(t.Context())

		if err != nil || !reflect.DeepEqual(snapshot.Easypanel, want) {
			t.Errorf("Collect() = %+v, %v; want Easypanel %+v", snapshot, err, want)
		}
	})

	t.Run("Should leave the Easypanel package nil without a running easypanel container", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage: reply(`{}`),
			services:  reply(`[]`),
			tasks:     reply(`[]`),
			containers: reply(`[
				{"Id":"a1","State":"exited","Labels":{"com.docker.swarm.service.name":"easypanel"}},
				{"Id":"b2","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}
			]`),
			stats:   replyStats(map[string]string{"b2": `{}`}),
			archive: replyStatus(http.StatusInternalServerError),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		snapshot, err := service.Collect(t.Context())

		if err != nil || snapshot.Easypanel != nil {
			t.Errorf("Collect() = %+v, %v; want Easypanel nil", snapshot, err)
		}
	})

	t.Run("Should leave the Easypanel package nil when the archive of the easypanel container answers 404", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    replyStatus(http.StatusNotFound),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		snapshot, err := service.Collect(t.Context())

		if err != nil || snapshot.Easypanel != nil {
			t.Errorf("Collect() = %+v, %v; want Easypanel nil", snapshot, err)
		}
	})

	t.Run("Should fail when the archive of the easypanel container answers other than 200 and 404", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    replyStatus(http.StatusInternalServerError),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Collect() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail when the archive of the easypanel container is not a tar", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    reply(strings.Repeat("x", 1024)),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, tar.ErrHeader) {
			t.Errorf("Collect() = %v, want %v", err, tar.ErrHeader)
		}
	})

	t.Run("Should fail when the archive of the easypanel container has no entry", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    reply(strings.Repeat("\x00", 1024)),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))

		_, err := service.Collect(t.Context())

		if !errors.Is(err, io.EOF) {
			t.Errorf("Collect() = %v, want %v", err, io.EOF)
		}
	})

	t.Run("Should fail when the package.json of the easypanel container is not JSON", func(t *testing.T) {
		socketPath := startReportEngine(t, reportEngine{
			diskUsage:  reply(`{}`),
			services:   reply(`[]`),
			tasks:      reply(`[]`),
			containers: reply(`[{"Id":"e1","State":"running","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`),
			stats:      replyStats(map[string]string{"e1": `{}`}),
			archive:    reply(tarOf(t, "not json")),
		})
		service := NewReportCollectorService(NewSocketClient(socketPath, time.Second))
		var syntaxErr *json.SyntaxError

		_, err := service.Collect(t.Context())

		if !errors.As(err, &syntaxErr) {
			t.Errorf("Collect() = %v, want a JSON syntax error", err)
		}
	})
}
