//go:build integration

package commands

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heronlabs/talon/tests/cli"
)

func startReportEngine(t *testing.T) string {
	t.Helper()

	socketPath := newSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

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

	server := httptest.NewUnstartedServer(mux)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}

type webhookRequest struct {
	authorization string
	body          []byte
}

func startWebhook(t *testing.T, status int) (string, *webhookRequest) {
	t.Helper()

	received := &webhookRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.authorization = r.Header.Get("Authorization")
		received.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	t.Cleanup(server.Close)

	return server.URL, received
}

func TestReportCommand(t *testing.T) {
	t.Run("Should post the report and log status=ok", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_AGENT_TOKEN=token\n")
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

	t.Run("Should post with Authorization: Bearer and the WEBHOOK_AGENT_TOKEN", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, received := startWebhook(t, http.StatusOK)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_AGENT_TOKEN=token\n")
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

	t.Run("Should fail when the webhook answers 401", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, _ := startWebhook(t, http.StatusUnauthorized)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\nWEBHOOK_AGENT_TOKEN=wrong\n")
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
		writeEnvContent(t, dir, "WEBHOOK_AGENT_TOKEN=token\nWEBHOOK_URL\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))

		out, code := cli.Run(t, dir, "report")

		if code != 1 || !strings.Contains(out, `ERROR report error="config: .env line 2: want KEY=VALUE"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of line 2", code, out)
		}
	})
}
