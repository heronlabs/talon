package docker

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func startEngine(t *testing.T, ping http.HandlerFunc) string {
	t.Helper()

	socketPath := newSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /_ping", ping)

	server := httptest.NewUnstartedServer(mux)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}

func TestHealthCheckService(t *testing.T) {
	t.Run("Should pass when Docker responds 200", func(t *testing.T) {
		socketPath := startEngine(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		service := NewHealthCheckService(NewSocketClient(socketPath, time.Second))

		err := service.Ping(t.Context())

		if err != nil {
			t.Errorf("Ping() = %v, want nil", err)
		}
	})

	t.Run("Should fail when socket does not exist", func(t *testing.T) {
		service := NewHealthCheckService(NewSocketClient(newSocketPath(t), time.Second))

		err := service.Ping(t.Context())

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Ping() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when Docker responds other than 200", func(t *testing.T) {
		socketPath := startEngine(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		service := NewHealthCheckService(NewSocketClient(socketPath, time.Second))

		err := service.Ping(t.Context())

		if !errors.Is(err, ErrUnexpectedStatus) {
			t.Errorf("Ping() = %v, want %v", err, ErrUnexpectedStatus)
		}
	})

	t.Run("Should fail with ErrNotFound when Docker responds 404", func(t *testing.T) {
		socketPath := startEngine(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		service := NewHealthCheckService(NewSocketClient(socketPath, time.Second))

		err := service.Ping(t.Context())

		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Ping() = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("Should fail when Docker does not respond within timeout", func(t *testing.T) {
		socketPath := startEngine(t, func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})
		service := NewHealthCheckService(NewSocketClient(socketPath, 50*time.Millisecond))

		err := service.Ping(t.Context())

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Ping() = %v, want %v", err, context.DeadlineExceeded)
		}
	})

	t.Run("Should fail when ctx is done", func(t *testing.T) {
		socketPath := startEngine(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		service := NewHealthCheckService(NewSocketClient(socketPath, time.Second))
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := service.Ping(ctx)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Ping() = %v, want %v", err, context.Canceled)
		}
	})
}
