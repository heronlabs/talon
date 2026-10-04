//go:build integration

package commands

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heronlabs/vps-agent/tests/cli"
)

func newSocketPath(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "docker")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return filepath.Join(dir, "docker.sock")
}

func startEngine(t *testing.T) string {
	t.Helper()

	socketPath := newSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /_ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewUnstartedServer(mux)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}

func writeEnv(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, ".env"), nil, mode); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorCommand(t *testing.T) {
	t.Run("Should pass when .env has mode 600 and Docker responds", func(t *testing.T) {
		dir := t.TempDir()
		writeEnv(t, dir, 0o600)
		t.Setenv("DOCKER_SOCKET", startEngine(t))

		out, code := cli.Run(t, dir, "doctor")

		if code != 0 || !strings.Contains(out, "INFO doctor status=ok") {
			t.Errorf("exit %d, output %q; want exit 0 with status=ok", code, out)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("DOCKER_SOCKET", startEngine(t))

		out, code := cli.Run(t, dir, "doctor")

		if code != 1 || !strings.Contains(out, "ERROR doctor error=\"config: stat .env: no such file or directory\"") {
			t.Errorf("exit %d, output %q; want exit 1 with config error", code, out)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		dir := t.TempDir()
		writeEnv(t, dir, 0o400)
		t.Setenv("DOCKER_SOCKET", startEngine(t))

		out, code := cli.Run(t, dir, "doctor")

		if code != 1 || !strings.Contains(out, "ERROR doctor error=\"config: .env has mode 400, want 600\"") {
			t.Errorf("exit %d, output %q; want exit 1 with mode error", code, out)
		}
	})

	t.Run("Should fail when Docker socket does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeEnv(t, dir, 0o600)
		t.Setenv("DOCKER_SOCKET", newSocketPath(t))

		out, code := cli.Run(t, dir, "doctor")

		if code != 1 || !strings.Contains(out, "connect: no such file or directory") {
			t.Errorf("exit %d, output %q; want exit 1 with docker error", code, out)
		}
	})
}
