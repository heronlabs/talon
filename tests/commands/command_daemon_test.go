//go:build integration

package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/heronlabs/talon/tests/cli"
)

func startHoldingWebhook(t *testing.T, release <-chan struct{}) (string, <-chan []byte) {
	t.Helper()

	requests := make(chan []byte, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- body

		select {
		case <-release:
		case <-r.Context().Done():
		}

		_, _ = io.WriteString(w, `{"data":{}}`)
	}))
	t.Cleanup(server.Close)

	return server.URL, requests
}

func released() <-chan struct{} {
	release := make(chan struct{})
	close(release)

	return release
}

func receiveRequest(t *testing.T, requests <-chan []byte) []byte {
	t.Helper()

	select {
	case body := <-requests:
		return body
	case <-time.After(30 * time.Second):
		t.Fatal("the webhook received no request within 30s")
		return nil
	}
}

func TestDaemonCommand(t *testing.T) {
	t.Run("Should post the first report at once and exit 0 on SIGTERM", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, requests := startHoldingWebhook(t, released())
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		process := cli.Start(t, dir, "daemon")

		body := receiveRequest(t, requests)
		process.Signal(t, syscall.SIGTERM)
		out, code := process.Wait(t)

		if code != 0 || !strings.Contains(string(body), `"host":`) || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(out, "INFO daemon status=stopped") {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with the report posted, status=ok and status=stopped", code, out, body)
		}
	})

	t.Run("Should finish the cycle in flight on SIGTERM and exit 0", func(t *testing.T) {
		dir := t.TempDir()
		release := make(chan struct{})
		webhookURL, requests := startHoldingWebhook(t, release)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		process := cli.Start(t, dir, "daemon")
		_ = receiveRequest(t, requests)

		process.Signal(t, syscall.SIGTERM)
		close(release)
		out, code := process.Wait(t)

		if code != 0 || !strings.Contains(out, "INFO report status=ok") || !strings.Contains(out, "INFO daemon status=stopped") {
			t.Errorf("exit %d, output %q; want exit 0 with status=ok and status=stopped", code, out)
		}
	})

	t.Run("Should log the error of a cycle that runs past its deadline and keep running", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, requests := startHoldingWebhook(t, nil)
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")
		unsetKeys(t)
		t.Setenv("INTERVAL_REPORT", "1s")
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		process := cli.Start(t, dir, "daemon")
		_ = receiveRequest(t, requests)
		_ = receiveRequest(t, requests)

		process.Signal(t, syscall.SIGTERM)
		out, code := process.Wait(t)

		if code != 0 || !strings.Contains(out, `ERROR report error="webhook: Post `) || !strings.Contains(out, "context deadline exceeded") || !strings.Contains(out, "INFO daemon status=stopped") {
			t.Errorf("exit %d, output %q; want exit 0 with context deadline exceeded and status=stopped", code, out)
		}
	})

	t.Run("Should post to the WEBHOOK_URL written to .env after it started", func(t *testing.T) {
		dir := t.TempDir()
		webhookURL, requests := startHoldingWebhook(t, released())
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("INTERVAL_REPORT", "1s")
		t.Setenv("DOCKER_SOCKET", startReportEngine(t))
		t.Setenv("TALON_HOST_ROOT", newHostRoot(t))
		process := cli.Start(t, dir, "daemon")
		writeEnvContent(t, dir, "WEBHOOK_URL="+webhookURL+"\n")

		body := receiveRequest(t, requests)
		process.Signal(t, syscall.SIGTERM)
		out, code := process.Wait(t)

		if code != 0 || !strings.Contains(string(body), `"host":`) || !strings.Contains(out, "INFO daemon status=stopped") {
			t.Errorf("exit %d, output %q, body %s; want exit 0 with the report posted and status=stopped", code, out, body)
		}
	})

	t.Run("Should exit 1 when INTERVAL_REPORT is not a duration", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "")
		unsetKeys(t)
		t.Setenv("INTERVAL_REPORT", "abc")

		out, code := cli.Run(t, dir, "daemon")

		if code != 1 || !strings.Contains(out, `ERROR daemon error="config: INTERVAL_REPORT: time: invalid duration \"abc\""`) {
			t.Errorf("exit %d, output %q; want exit 1 with invalid duration", code, out)
		}
	})

	t.Run("Should exit 1 when INTERVAL_REPORT is not positive", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "INTERVAL_REPORT=0s\n")
		unsetKeys(t)

		out, code := cli.Run(t, dir, "daemon")

		if code != 1 || !strings.Contains(out, `ERROR daemon error="config: INTERVAL_REPORT: 0s is not positive"`) {
			t.Errorf("exit %d, output %q; want exit 1 with 0s is not positive", code, out)
		}
	})

	t.Run("Should exit 1 when .env cannot be parsed", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "INTERVAL_REPORT\n")
		unsetKeys(t)

		out, code := cli.Run(t, dir, "daemon")

		if code != 1 || !strings.Contains(out, `ERROR daemon error="config: .env line 1: want KEY=VALUE"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of line 1", code, out)
		}
	})
}
