//go:build integration

package commands

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heronlabs/talon/tests/cli"
)

func serveOnSocket(t *testing.T, handler http.Handler) string {
	t.Helper()

	socketPath := newSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}

func replyOn(pattern string, status int, body string, mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, matched := mux.Handler(r); matched == pattern {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

func startReportEngine(t *testing.T) string {
	t.Helper()

	return startFailingReportEngine(t, "")
}

func startFailingReportEngine(t *testing.T, failing string) string {
	t.Helper()

	return startReportEngineReplying(t, failing, http.StatusInternalServerError, "")
}

func startReportEngineReplying(t *testing.T, pattern string, status int, body string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /system/df", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"LayersSize":1000000000}`)
	})
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"ID":"s1","Spec":{"Name":"shop_api"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}]`)
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"Id":"a1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`)
	})
	mux.HandleFunc("GET /containers/{id}/stats", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"memory_stats":{"usage":104857600}}`)
	})

	return serveOnSocket(t, replyOn(pattern, status, body, mux))
}

func startRichReportEngine(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /system/df", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"LayersSize":1000000000,"BuildCache":[{"Size":500000000}],"Volumes":[{"Name":"shop_data","UsageData":{"Size":5000000000}},{"Name":"unused","UsageData":{"Size":-1}}]}`)
	})
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[`+
			`{"ID":"s1","Spec":{"Name":"shop_api","TaskTemplate":{"Resources":{"Limits":{"NanoCPUs":500000000,"MemoryBytes":268435456}},"Runtime":"container"}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":2}},`+
			`{"ID":"s2","Spec":{"Name":"shop_db"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s3","Spec":{"Name":"shop_worker"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s4","Spec":{"Name":"blog_web"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},`+
			`{"ID":"s5","Spec":{"Name":"blog_cron"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s7","Spec":{"Name":"blog_backup"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":0}},`+
			`{"ID":"s6","Spec":{"Name":"traefik"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}`+
			`]`)
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, _ *http.Request) {
		recently := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
		daysAgo := time.Now().Add(-96 * time.Hour).UTC().Format(time.RFC3339Nano)
		_, _ = io.WriteString(w, `[`+
			`{"ServiceID":"s1","Status":{"Timestamp":"`+recently+`","State":"failed"}},`+
			`{"ServiceID":"s1","Status":{"Timestamp":"`+recently+`","State":"rejected"}},`+
			`{"ServiceID":"s1","Status":{"Timestamp":"`+daysAgo+`","State":"failed"}},`+
			`{"ServiceID":"s1","CreatedAt":"2026-10-18T12:10:00-03:00","Spec":{"Resources":{"Limits":{"MemoryBytes":268435456,"NanoCPUs":500000000}}},"Status":{"State":"running"}},`+
			`{"ServiceID":"s2","Status":{"State":"shutdown"}}`+
			`]`)
	})
	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[`+
			`{"Id":"c1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"bind","Name":""}]},`+
			`{"Id":"c2","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_db"}},`+
			`{"Id":"c3","State":"removing","Labels":{"com.docker.swarm.service.name":"shop_db"}},`+
			`{"Id":"c4","State":"running","Labels":{"com.docker.swarm.service.name":"blog_web"}},`+
			`{"Id":"c5","State":"unknown","Labels":{"com.docker.swarm.service.name":"blog_cron"}},`+
			`{"Id":"c6","State":"exited","Labels":{"com.docker.swarm.service.name":"blog_backup"}}`+
			`]`)
	})
	mux.HandleFunc("GET /containers/c1/stats", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{`+
			`"memory_stats":{"usage":209715200,"stats":{"total_inactive_file":104857600}},`+
			`"cpu_stats":{"cpu_usage":{"total_usage":1000000000,"percpu_usage":[1,1]},"system_cpu_usage":3000000000},`+
			`"precpu_stats":{"cpu_usage":{"total_usage":500000000},"system_cpu_usage":2000000000},`+
			`"networks":{"eth0":{"rx_bytes":1000000,"tx_bytes":2000000},"eth1":{"rx_bytes":1000000}},`+
			`"blkio_stats":{"io_service_bytes_recursive":[{"op":"Read","value":3000000},{"op":"write","value":4000000},{"op":"Total","value":7000000}]}`+
			`}`)
	})
	mux.HandleFunc("GET /containers/c4/stats", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{`+
			`"memory_stats":{"usage":104857600,"stats":{"inactive_file":209715200}},`+
			`"cpu_stats":{"cpu_usage":{"total_usage":125000000},"system_cpu_usage":2000000000,"online_cpus":4},`+
			`"precpu_stats":{"system_cpu_usage":1000000000}`+
			`}`)
	})

	return serveOnSocket(t, mux)
}

func startEasypanelReportEngine(t *testing.T, state string, archiveStatus int, archive string) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /system/df", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"ID":"s1","Spec":{"Name":"easypanel"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}]`)
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	})
	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"Id":"e1","State":"`+state+`","Labels":{"com.docker.swarm.service.name":"easypanel"}}]`)
	})
	mux.HandleFunc("GET /containers/e1/stats", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})
	mux.HandleFunc("GET /containers/e1/archive", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(archiveStatus)
		_, _ = io.WriteString(w, archive)
	})

	return serveOnSocket(t, mux)
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

func writeHostFile(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func removeHostFile(t *testing.T, root, name string) {
	t.Helper()

	if err := os.Remove(filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
}

func chmodHostDir(t *testing.T, root, name string, mode os.FileMode) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restores the temporary directory so it can be removed
}

func newHostRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeHostFile(t, root, "proc/meminfo", "MemTotal:        8388608 kB\nMemFree:         1048576 kB\nMemAvailable:    2097152 kB\nSwapTotal:       1048576 kB\nSwapFree:         524288 kB\nHugePages_Total:       0\n")
	writeHostFile(t, root, "proc/loadavg", "0.52 0.58 0.59 1/389 12345\n")
	writeHostFile(t, root, "proc/uptime", "12345.67 54321.00\n")

	return root
}

type webhookRequest struct {
	authorization string
	body          []byte
}

func startWebhook(t *testing.T, status int) (string, *webhookRequest) {
	t.Helper()

	return startWebhookReplying(t, status, `{"data":{}}`)
}

func startWebhookReplying(t *testing.T, status int, reply string) (string, *webhookRequest) {
	t.Helper()

	received := &webhookRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.authorization = r.Header.Get("Authorization")
		received.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

type manifestRequest struct {
	requested     bool
	authorization string
}

func startManifest(t *testing.T, status int, body string) (string, *manifestRequest) {
	t.Helper()

	received := &manifestRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.requested = true
		received.authorization = r.Header.Get("Authorization")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()

	content, err := fs.ReadFile(os.DirFS("testdata"), name)
	if err != nil {
		t.Fatal(err)
	}

	return string(content)
}

func startEasypanel(t *testing.T) string {
	t.Helper()

	return startEasypanelReplying(t, "", http.StatusNotFound, "")
}

func startEasypanelReplying(t *testing.T, pattern string, status int, body string) string {
	t.Helper()

	replay := replayEasypanel(os.DirFS(filepath.Join("testdata", "easypanel")), "easypanel-token")
	mux := http.NewServeMux()
	mux.Handle("GET /api/listProjectsAndServices", replay)
	mux.Handle("GET /api/inspectAppService", replay)
	mux.Handle("GET /api/inspectPostgresService", replay)
	mux.Handle("GET /api/inspectMySQLService", replay)
	mux.Handle("GET /api/listDomains", replay)
	mux.Handle("GET /api/getUser", replay)

	server := httptest.NewServer(replyOn(pattern, status, body, mux))
	t.Cleanup(server.Close)

	return server.URL
}

func replayEasypanel(recorded fs.FS, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operation := strings.TrimPrefix(r.URL.Path, "/api/")
		if r.Header.Get("Authorization") != "Bearer "+token {
			refuseEasypanel(w, operation)
			return
		}

		answer, err := fs.ReadFile(recorded, path.Join(operation, r.URL.Query().Get("projectName"), r.URL.Query().Get("serviceName"))+".json")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"defined":false,"code":"NOT_FOUND","status":404,"message":"Service not found."}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(answer) //nolint:gosec // G705: the fake replays the JSON answers recorded in testdata
	})
}

func refuseEasypanel(w http.ResponseWriter, operation string) {
	if operation == "getUser" {
		_, _ = io.WriteString(w, "null")
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
	_, _ = io.WriteString(w, `{"defined":false,"code":"UNAUTHORIZED","status":401,"message":"Unauthorized"}`)
}

func TestReportCommand(t *testing.T) {
	t.Run("Should post the report and log status=ok", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_TOKEN=token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")
		var body map[string]any
		_ = json.Unmarshal(received.body, &body)

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || body["host"] == nil || body["host"] == "" {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with status=ok and a host", code, out, received.body)
		}
	})

	t.Run("Should post the services of each project and the version of talon", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":null,"easypanelSupported":null,"lastReconcile":null}`
		wantProjects := `"projects":[{"name":"shop","reconcile":null,"services":[{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":100,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]}]`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAgent) || !strings.Contains(string(received.body), wantProjects) {
			t.Errorf("exit %d, body %s; want exit 0 with %s and %s", code, received.body, wantAgent, wantProjects)
		}
	})

	t.Run("Should post with Authorization: Bearer and the WEBHOOK_TOKEN", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_TOKEN=token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		_, code := cli.Run(t, dir, "report")

		if code != 0 || received.authorization != "Bearer token" {
			t.Errorf("exit %d, Authorization %q; want exit 0 with %q", code, received.authorization, "Bearer token")
		}
	})

	t.Run("Should log status=ok without posting when WEBHOOK_URL is empty", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL=\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, "INFO report status=ok") {
			t.Errorf("exit %d, output %q; want exit 0 with status=ok", code, out)
		}
	})

	t.Run("Should log the report as JSON in payload without posting when WEBHOOK_URL is empty", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL=\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProjects := `\"projects\":[{\"name\":\"shop\",`

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, `INFO report status=ok payload="{\"host\":\"`) || !strings.Contains(out, wantProjects) {
			t.Errorf("exit %d, output %q; want exit 0 with the report in payload and %s", code, out, wantProjects)
		}
	})

	t.Run("Should fail when the webhook answers 401", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, _ := startWebhook(t, http.StatusUnauthorized)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_TOKEN=wrong\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="webhook: unexpected status: 401"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 401", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine does not answer", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL=\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", newSocketPath(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: `) {
			t.Errorf("exit %d, output %q; want exit 1 with docker error", code, out)
		}
	})

	t.Run("Should fail when .env cannot be parsed", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_TOKEN=token\nWEBHOOK_URL\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="config: .env line 2: want KEY=VALUE"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of line 2", code, out)
		}
	})
	t.Run("Should post the state, last deploy, limits, recent failures, disk and stats of each service of each project", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startRichReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantSystem := `"dockerImagesGB":1,"dockerBuildCacheGB":0.5}`
		wantProjects := `"projects":[` +
			`{"name":"shop","reconcile":null,"services":[` +
			`{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":2,"recentTaskFailures":2,"lastDeployAt":"2026-10-18T15:10:00Z","lastDeployState":null,"memoryUsedMiB":100,"memoryLimitMiB":256,"cpuPercent":100,"cpuLimit":0.5,"diskGB":5,"netRxMB":2,"netTxMB":2,"blockReadMB":3,"blockWriteMB":4},` +
			`{"name":"db","managed":false,"state":"exited","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"worker","managed":false,"state":"missing","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]},` +
			`{"name":"blog","reconcile":null,"services":[` +
			`{"name":"web","managed":false,"state":"running","replicasRunning":1,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":100,"memoryLimitMiB":0,"cpuPercent":50,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"cron","managed":false,"state":"dead","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"backup","managed":false,"state":"stopped","replicasRunning":0,"replicasDesired":0,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]}]`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantSystem) || !strings.Contains(string(received.body), wantProjects) {
			t.Errorf("exit %d, body %s; want exit 0 with %s and %s", code, received.body, wantSystem, wantProjects)
		}
	})

	t.Run("Should post to WEBHOOK_URL from the environment variable", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("WEBHOOK_URL", webhookURL)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), `"host":`) {
			t.Errorf("exit %d, body %s; want exit 0 with the report posted", code, received.body)
		}
	})

	t.Run("Should fail when WEBHOOK_URL is not a URL", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL=://webhook.example.com\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="webhook: parse `) || !strings.Contains(out, "missing protocol scheme") {
			t.Errorf("exit %d, output %q; want exit 1 with missing protocol scheme", code, out)
		}
	})

	t.Run("Should fail when the webhook cannot be reached", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL="+closedWebhookURL(t)+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="webhook: Post `) || !strings.Contains(out, "connection refused") {
			t.Errorf("exit %d, output %q; want exit 1 with connection refused", code, out)
		}
	})

	t.Run("Should fail when the webhook replies with something other than JSON", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, _ := startWebhookReplying(t, http.StatusOK, "ok")
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="webhook: invalid character 'o' looking for beginning of value"`) {
			t.Errorf("exit %d, output %q; want exit 1 with invalid character", code, out)
		}
	})

	t.Run("Should fail when .env cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
			t.Fatal(err)
		}
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="config: read `) || !strings.Contains(out, "is a directory") {
			t.Errorf("exit %d, output %q; want exit 1 with is a directory", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine fails to list the services", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startFailingReportEngine(t, "GET /services"))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine fails to list the tasks", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startFailingReportEngine(t, "GET /tasks"))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine fails to list the containers", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startFailingReportEngine(t, "GET /containers/json"))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine fails to give the stats of a container", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startFailingReportEngine(t, "GET /containers/{id}/stats"))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})

	t.Run("Should fail when the Docker Engine answers 404 to the list of the services", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngineReplying(t, "GET /services", http.StatusNotFound, ""))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: not found"`) {
			t.Errorf("exit %d, output %q; want exit 1 with not found", code, out)
		}
	})

	t.Run("Should post the report without the stats of a container whose stats answer 404", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngineReplying(t, "GET /containers/{id}/stats", http.StatusNotFound, ""))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAPI := `{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,`

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(string(received.body), wantAPI) {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with status=ok and %s", code, out, received.body, wantAPI)
		}
	})

	t.Run("Should fail when the ID of a running container is not a valid path", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngineReplying(t, "GET /containers/json", http.StatusOK, `[{"Id":"%zz","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"}}]`))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: parse `) || !strings.Contains(out, "invalid URL escape") {
			t.Errorf("exit %d, output %q; want exit 1 with invalid URL escape", code, out)
		}
	})

	t.Run("Should post the memory, swap, disk, load and uptime of the host", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantMemory := `"system":{"ramTotalGiB":8,"ramUsedGiB":6,"swapTotalGiB":1,"swapUsedGiB":0.5,"diskTotalGB":`
		wantLoad := `"load1":0.52,"load5":0.58,"load15":0.59,"uptimeSeconds":12345,"dockerImagesGB":1,"dockerBuildCacheGB":0}`
		var body struct {
			System struct {
				DiskTotalGB float64 `json:"diskTotalGB"`
				DiskUsedGB  float64 `json:"diskUsedGB"`
			} `json:"system"`
		}

		_, code := cli.Run(t, dir, "report")
		_ = json.Unmarshal(received.body, &body)

		if code != 0 || !strings.Contains(string(received.body), wantMemory) || !strings.Contains(string(received.body), wantLoad) || body.System.DiskTotalGB <= 0 || body.System.DiskUsedGB <= 0 {
			t.Errorf("exit %d, body %s; want exit 0 with %s, %s and a disk total and used above 0", code, received.body, wantMemory, wantLoad)
		}
	})

	t.Run("Should fail when the root of the host does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", filepath.Join(t.TempDir(), "missing"))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: statfs `) || !strings.Contains(out, "missing: no such file or directory") {
			t.Errorf("exit %d, output %q; want exit 1 with statfs no such file or directory", code, out)
		}
	})

	t.Run("Should fail when proc/meminfo does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		removeHostFile(t, root, "proc/meminfo")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: open proc/meminfo: no such file or directory"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open proc/meminfo: no such file or directory", code, out)
		}
	})

	t.Run("Should fail when a field of proc/meminfo is not a number of kB", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "proc/meminfo", "MemTotal: lots kB\nMemAvailable: 2097152 kB\nSwapTotal: 1048576 kB\nSwapFree: 524288 kB\n")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: proc/meminfo MemTotal: strconv.ParseInt: parsing \"lots\": invalid syntax"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of MemTotal", code, out)
		}
	})

	t.Run("Should fail when proc/loadavg does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		removeHostFile(t, root, "proc/loadavg")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: open proc/loadavg: no such file or directory"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open proc/loadavg: no such file or directory", code, out)
		}
	})

	t.Run("Should fail when proc/loadavg has fewer than three loads", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "proc/loadavg", "0.52 0.58\n")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: proc/loadavg: EOF"`) {
			t.Errorf("exit %d, output %q; want exit 1 with proc/loadavg: EOF", code, out)
		}
	})

	t.Run("Should fail when proc/uptime does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		removeHostFile(t, root, "proc/uptime")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: open proc/uptime: no such file or directory"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open proc/uptime: no such file or directory", code, out)
		}
	})

	t.Run("Should fail when proc/uptime is not a number of seconds", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "proc/uptime", "up\n")
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="system: proc/uptime: strconv.ParseFloat: parsing \"\": invalid syntax"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of proc/uptime", code, out)
		}
	})

	t.Run("Should post the diskGB of each service as its named volumes plus the files of its directory in etc/easypanel/projects", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startRichReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/projects/notes.txt", "notes")
		writeHostFile(t, root, "etc/easypanel/projects/shop/README", "readme")
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/config.json", strings.Repeat("c", 400000))
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/uploads/image.png", strings.Repeat("i", 600000))
		writeHostFile(t, root, "etc/easypanel/projects/shop/db/data/base", strings.Repeat("d", 2000000))
		t.Setenv("TALON_HOST_ROOT", root)
		wantAPI := `{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":2,"recentTaskFailures":2,"lastDeployAt":"2026-10-18T15:10:00Z","lastDeployState":null,"memoryUsedMiB":100,"memoryLimitMiB":256,"cpuPercent":100,"cpuLimit":0.5,"diskGB":5.001,`
		wantDB := `{"name":"db","managed":false,"state":"exited","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0.002,`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAPI) || !strings.Contains(string(received.body), wantDB) {
			t.Errorf("exit %d, body %s; want exit 0 with %s and %s", code, received.body, wantAPI, wantDB)
		}
	})

	t.Run("Should post the diskGB of the named volumes alone when etc/easypanel/projects does not exist", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startRichReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/config.json", "{}")
		t.Setenv("TALON_HOST_ROOT", root)
		wantAPI := `{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":2,"recentTaskFailures":2,"lastDeployAt":"2026-10-18T15:10:00Z","lastDeployState":null,"memoryUsedMiB":100,"memoryLimitMiB":256,"cpuPercent":100,"cpuLimit":0.5,"diskGB":5,`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAPI) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantAPI)
		}
	})

	t.Run("Should fail when etc/easypanel/projects cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/config.json", "{}")
		chmodHostDir(t, root, "etc/easypanel/projects", 0o000)
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="easypanel: open etc/easypanel/projects: permission denied"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open etc/easypanel/projects: permission denied", code, out)
		}
	})

	t.Run("Should fail when the directory of a project cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/config.json", "{}")
		chmodHostDir(t, root, "etc/easypanel/projects/shop", 0o000)
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="easypanel: open etc/easypanel/projects/shop: permission denied"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open etc/easypanel/projects/shop: permission denied", code, out)
		}
	})

	t.Run("Should fail when the directory of a service cannot be walked", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/config.json", "{}")
		chmodHostDir(t, root, "etc/easypanel/projects/shop/api", 0o000)
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="easypanel: open etc/easypanel/projects/shop/api: permission denied"`) {
			t.Errorf("exit %d, output %q; want exit 1 with open etc/easypanel/projects/shop/api: permission denied", code, out)
		}
	})

	t.Run("Should fail when the info of a file of a service cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		root := newHostRoot(t)
		writeHostFile(t, root, "etc/easypanel/projects/shop/api/config.json", "{}")
		chmodHostDir(t, root, "etc/easypanel/projects/shop/api", 0o444)
		t.Setenv("TALON_HOST_ROOT", root)

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="easypanel: lstat `) || !strings.Contains(out, `etc/easypanel/projects/shop/api/config.json: permission denied"`) {
			t.Errorf("exit %d, output %q; want exit 1 with lstat of config.json: permission denied", code, out)
		}
	})

	t.Run("Should post the Easypanel version of the running easypanel container as supported", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusOK, tarOf(t, `{"name":"easypanel","version":"2.36.0"}`)))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":"2.36.0","easypanelSupported":true,"lastReconcile":null}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAgent) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantAgent)
		}
	})

	t.Run("Should post the Easypanel version of the running easypanel container as not supported", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusOK, tarOf(t, `{"name":"easypanel","version":"2.35.1"}`)))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":"2.35.1","easypanelSupported":false,"lastReconcile":null}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAgent) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantAgent)
		}
	})

	t.Run("Should post the Easypanel version and whether it is supported as null when the easypanel container is not running", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "exited", http.StatusInternalServerError, ""))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":null,"easypanelSupported":null,"lastReconcile":null}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantAgent) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantAgent)
		}
	})

	t.Run("Should fail when the archive of the easypanel container answers 500", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusInternalServerError, ""))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})

	t.Run("Should post the Easypanel version and whether it is supported as null when the archive of the easypanel container answers 404", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusNotFound, ""))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":null,"easypanelSupported":null,"lastReconcile":null}`

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(string(received.body), wantAgent) {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with status=ok and %s", code, out, received.body, wantAgent)
		}
	})

	t.Run("Should fail when the archive of the easypanel container is not a tar", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusOK, strings.Repeat("x", 1024)))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: archive/tar: invalid tar header"`) {
			t.Errorf("exit %d, output %q; want exit 1 with invalid tar header", code, out)
		}
	})

	t.Run("Should fail when the archive of the easypanel container has no entry", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusOK, strings.Repeat("\x00", 1024)))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: EOF"`) {
			t.Errorf("exit %d, output %q; want exit 1 with EOF", code, out)
		}
	})

	t.Run("Should fail when the package.json of the easypanel container is not JSON", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startEasypanelReportEngine(t, "running", http.StatusOK, tarOf(t, "{version}")))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: invalid character 'v' looking for beginning of object key string"`) {
			t.Errorf("exit %d, output %q; want exit 1 with invalid character 'v'", code, out)
		}
	})
	t.Run("Should post the reconcile of a project as converged when Easypanel holds the schema of its client", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngineReplying(t, "GET /services", http.StatusOK, `[{"ID":"s1","Spec":{"Name":"cms-heronlabs_cms-api"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},{"ID":"s2","Spec":{"Name":"cms-heronlabs_cms-database"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}]`))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":{"state":"converged","missingSecrets":[],"error":null},"services":[{"name":"cms-api","managed":true,"state":"missing","replicasRunning":1,`
		wantDatabase := `{"name":"cms-database","managed":true,"state":"missing","replicasRunning":1,`
		wantLastReconcile := `"outcome":"ok","error":null}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) || !strings.Contains(string(received.body), wantDatabase) || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s, %s and %s", code, received.body, wantProject, wantDatabase, wantLastReconcile)
		}
	})

	t.Run("Should post the reconcile of a project as pending when Easypanel differs from the schema of its client", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/pending.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":{"state":"pending","missingSecrets":[],"error":null},`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should post the reconcile of a project as pending when its client declares a build that Easypanel does not hold", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, `{"data":{"host":"vps-01","clients":[{"slug":"cms-heronlabs","lifecycle":"active","secretsRoleArn":null,"schema":{"services":[{"type":"app","data":{"projectName":"cms-heronlabs","serviceName":"cms-api","build":{"type":"dockerfile","file":"apps/server-cms/Dockerfile.production"}}}]}}]}}`)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":{"state":"pending","missingSecrets":[],"error":null},`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should post the reconcile of a project as pending when Easypanel serves only one of the declared domains", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, `{"data":{"host":"vps-01","clients":[{"slug":"cms-heronlabs","lifecycle":"active","secretsRoleArn":null,"schema":{"services":[{"type":"app","data":{"projectName":"cms-heronlabs","serviceName":"cms-api","domains":[{"host":"cms-heronlabs-cms-api.example.easypanel.host","port":80},{"host":"cms.example.com","port":80}]}}]}}]}}`)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/listDomains", http.StatusOK, `[`+
			`{"certificateResolver":"","destinationType":"service","host":"cms-heronlabs-cms-api.example.easypanel.host","https":true,"id":"d1","middlewares":[],"path":"/","serviceDestination":{"path":"/","port":80,"projectName":"cms-heronlabs","protocol":"http","serviceName":"cms-api"},"wildcard":false},`+
			`{"certificateResolver":"","destinationType":"service","host":"www.example.com","https":true,"id":"d2","middlewares":[],"path":"/","serviceDestination":{"path":"/","port":80,"projectName":"cms-heronlabs","protocol":"http","serviceName":"cms-api"},"wildcard":false}`+
			`]`)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":{"state":"pending","missingSecrets":[],"error":null},`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should post a project that is not in the manifest as unmanaged with its services not managed", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"shop","reconcile":{"state":"unmanaged","missingSecrets":[],"error":null},"services":[{"name":"api","managed":false,`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should post each client without a Swarm service after the Swarm projects, with its declared services managed and missing", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `]},{"name":"cms-heronlabs","reconcile":{"state":"converged","missingSecrets":[],"error":null},"services":[` +
			`{"name":"cms-api","managed":true,"state":"missing","replicasRunning":0,"replicasDesired":0,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"cms-database","managed":true,"state":"missing","replicasRunning":0,"replicasDesired":0,"recentTaskFailures":0,"lastDeployAt":null,"lastDeployState":null,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]}]`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should send MANIFEST_TOKEN to the manifest as Authorization: Bearer", func(t *testing.T) {
		dir := t.TempDir()
		manifestURL, manifest := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "MANIFEST_URL="+manifestURL+"\nMANIFEST_TOKEN=manifest-token\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		_, code := cli.Run(t, dir, "report")

		if code != 0 || manifest.authorization != "Bearer manifest-token" {
			t.Errorf("exit %d, Authorization %q; want exit 0 with %q", code, manifest.authorization, "Bearer manifest-token")
		}
	})

	t.Run("Should not read the manifest and post every reconcile and lastReconcile null without MANIFEST_URL", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		_, manifest := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProjects := `"projects":[{"name":"shop","reconcile":null,"services":[{"name":"api","managed":false,`
		wantLastReconcile := `"lastReconcile":null`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || manifest.requested || !strings.Contains(string(received.body), wantProjects) || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, manifest requested %v, body %s; want exit 0 without the manifest and with %s and %s", code, manifest.requested, received.body, wantProjects, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed and every reconcile null when the manifest answers 500", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusInternalServerError, `{"data":null,"error":{"status":500,"message":"infra-manifest-failed"}}`)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProjects := `"projects":[{"name":"shop","reconcile":null,"services":[{"name":"api","managed":false,`
		wantLastReconcile := `"outcome":"failed","error":"manifest: unexpected status: 500"}`

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(string(received.body), wantProjects) || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with status=ok, %s and %s", code, out, received.body, wantProjects, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when MANIFEST_URL is not a URL", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL=://cms.example.com\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"manifest: parse \"://cms.example.com\": missing protocol scheme"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when the manifest cannot be reached", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+closedWebhookURL(t)+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), `"outcome":"failed","error":"manifest: Get `) || !strings.Contains(string(received.body), "connection refused") {
			t.Errorf("exit %d, body %s; want exit 0 with lastReconcile failed on connection refused", code, received.body)
		}
	})

	t.Run("Should post the report with lastReconcile failed when the manifest is not JSON", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, "manifest")
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"manifest: invalid character 'm' looking for beginning of value"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the clients of the manifest with every reconcile null and lastReconcile failed when Easypanel refuses EASYPANEL_TOKEN", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanel(t)+"\nEASYPANEL_TOKEN=wrong\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":null,"services":[{"name":"cms-api","managed":true,"state":"missing",`
		wantLastReconcile := `"outcome":"failed","error":"easypanel: listProjectsAndServices: unexpected status: 401"}`

		out, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(string(received.body), wantProject) || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with status=ok, %s and %s", code, out, received.body, wantProject, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when EASYPANEL_URL is not a URL", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL=://easypanel.example.com\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: listProjectsAndServices: parse \"://easypanel.example.com\": missing protocol scheme"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel cannot be reached", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+closedWebhookURL(t)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), `"outcome":"failed","error":"easypanel: listProjectsAndServices: Get `) || !strings.Contains(string(received.body), "connection refused") {
			t.Errorf("exit %d, body %s; want exit 0 with lastReconcile failed on connection refused", code, received.body)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel answers something other than JSON", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/listProjectsAndServices", http.StatusOK, "projects")+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: listProjectsAndServices: invalid character 'p' looking for beginning of value"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel fails to inspect an app", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/inspectAppService", http.StatusInternalServerError, "")+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: inspectAppService: unexpected status: 500"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel fails to list the domains of an app", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/listDomains", http.StatusInternalServerError, "")+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: listDomains: unexpected status: 500"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel fails to inspect a postgres service", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/inspectPostgresService", http.StatusInternalServerError, "")+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: inspectPostgresService: unexpected status: 500"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})

	t.Run("Should post the reconcile of a mysql service read with inspectMySQLService, leaving out the services of other types", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, `{"data":{"host":"vps-01","clients":[{"slug":"cms-heronlabs","lifecycle":"active","secretsRoleArn":null,"schema":{"services":[{"type":"mysql","data":{"projectName":"cms-heronlabs","serviceName":"cms-mysql","image":"mysql:9","databaseName":"cms-heronlabs","user":"mysql","exposedPort":0,"resources":{"cpuLimit":0.5,"cpuReservation":0.1,"memoryLimit":512,"memoryReservation":128}}}]}}]}}`)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/listProjectsAndServices", http.StatusOK, `{"projects":[{"name":"cms-heronlabs"}],"services":[{"name":"cms-mysql","projectName":"cms-heronlabs","type":"mysql"},{"name":"cms-cache","projectName":"cms-heronlabs","type":"redis"}]}`)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantProject := `{"name":"cms-heronlabs","reconcile":{"state":"converged","missingSecrets":[],"error":null},"services":[{"name":"cms-mysql","managed":true,"state":"missing",`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantProject) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantProject)
		}
	})

	t.Run("Should post the report with lastReconcile failed when Easypanel fails to inspect a mysql service", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		manifestURL, _ := startManifest(t, http.StatusOK, readTestdata(t, "manifest/converged.json"))
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nMANIFEST_URL="+manifestURL+"\nEASYPANEL_URL="+startEasypanelReplying(t, "GET /api/listProjectsAndServices", http.StatusOK, `{"services":[{"name":"cms-replica","projectName":"cms-heronlabs","type":"mysql"}]}`)+"\nEASYPANEL_TOKEN=easypanel-token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		wantLastReconcile := `"outcome":"failed","error":"easypanel: inspectMySQLService: unexpected status: 404"}`

		_, code := cli.Run(t, dir, "report")

		if code != 0 || !strings.Contains(string(received.body), wantLastReconcile) {
			t.Errorf("exit %d, body %s; want exit 0 with %s", code, received.body, wantLastReconcile)
		}
	})
}

func closedWebhookURL(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()

	return server.URL
}
