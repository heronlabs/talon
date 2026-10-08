//go:build integration

package commands

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func failOn(pattern string, mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, matched := mux.Handler(r); matched == pattern {
			w.WriteHeader(http.StatusInternalServerError)
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

	return serveOnSocket(t, failOn(failing, mux))
}

func startRichReportEngine(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /system/df", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"LayersSize":1000000000,"BuildCache":[{"Size":500000000}],"Volumes":[{"Name":"shop_data","UsageData":{"Size":5000000000}},{"Name":"unused","UsageData":{"Size":-1}}]}`)
	})
	mux.HandleFunc("GET /services", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[`+
			`{"ID":"s1","Spec":{"Name":"shop_api","TaskTemplate":{"Resources":{"Limits":{"NanoCPUs":500000000,"MemoryBytes":268435456}}}},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":2}},`+
			`{"ID":"s2","Spec":{"Name":"shop_db"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s3","Spec":{"Name":"shop_worker"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s4","Spec":{"Name":"blog_web"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}},`+
			`{"ID":"s5","Spec":{"Name":"blog_cron"},"ServiceStatus":{"RunningTasks":0,"DesiredTasks":1}},`+
			`{"ID":"s6","Spec":{"Name":"traefik"},"ServiceStatus":{"RunningTasks":1,"DesiredTasks":1}}`+
			`]`)
	})
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"ServiceID":"s1","Status":{"State":"failed"}},{"ServiceID":"s1","Status":{"State":"rejected"}},{"ServiceID":"s1","Status":{"State":"running"}},{"ServiceID":"s2","Status":{"State":"shutdown"}}]`)
	})
	mux.HandleFunc("GET /containers/json", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[`+
			`{"Id":"c1","State":"running","Labels":{"com.docker.swarm.service.name":"shop_api"},"Mounts":[{"Type":"volume","Name":"shop_data"},{"Type":"bind","Name":""}]},`+
			`{"Id":"c2","State":"exited","Labels":{"com.docker.swarm.service.name":"shop_db"}},`+
			`{"Id":"c3","State":"removing","Labels":{"com.docker.swarm.service.name":"shop_db"}},`+
			`{"Id":"c4","State":"running","Labels":{"com.docker.swarm.service.name":"blog_web"}},`+
			`{"Id":"c5","State":"unknown","Labels":{"com.docker.swarm.service.name":"blog_cron"}}`+
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

func TestReportCommand(t *testing.T) {
	t.Run("Should post the report and log status=ok", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_TOKEN=token\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))

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
		wantAgent := `"agent":{"version":"1.2.3","easypanelVersion":null,"easypanelSupported":null,"lastReconcile":null}`
		wantProjects := `"projects":[{"name":"shop","reconcile":null,"services":[{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":1,"recentTaskFailures":0,"memoryUsedMiB":100,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]}]`

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

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="config: .env line 2: want KEY=VALUE"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of line 2", code, out)
		}
	})
	t.Run("Should post the state, limits, failures, disk and stats of each service of each project", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startRichReportEngine(t))
		wantSystem := `"dockerImagesGB":1,"dockerBuildCacheGB":0.5}`
		wantProjects := `"projects":[` +
			`{"name":"shop","reconcile":null,"services":[` +
			`{"name":"api","managed":false,"state":"running","replicasRunning":1,"replicasDesired":2,"recentTaskFailures":2,"memoryUsedMiB":100,"memoryLimitMiB":256,"cpuPercent":100,"cpuLimit":0.5,"diskGB":5,"netRxMB":2,"netTxMB":2,"blockReadMB":3,"blockWriteMB":4},` +
			`{"name":"db","managed":false,"state":"exited","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"worker","managed":false,"state":"missing","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]},` +
			`{"name":"blog","reconcile":null,"services":[` +
			`{"name":"web","managed":false,"state":"running","replicasRunning":1,"replicasDesired":1,"recentTaskFailures":0,"memoryUsedMiB":100,"memoryLimitMiB":0,"cpuPercent":50,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0},` +
			`{"name":"cron","managed":false,"state":"dead","replicasRunning":0,"replicasDesired":1,"recentTaskFailures":0,"memoryUsedMiB":0,"memoryLimitMiB":0,"cpuPercent":0,"cpuLimit":0,"diskGB":0,"netRxMB":0,"netTxMB":0,"blockReadMB":0,"blockWriteMB":0}]}]`

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

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="docker: unexpected status: 500"`) {
			t.Errorf("exit %d, output %q; want exit 1 with unexpected status: 500", code, out)
		}
	})
}

func closedWebhookURL(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()

	return server.URL
}
