//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

type fixedDisk struct{}

func (fixedDisk) Stat(string) (domain.Bytes, domain.Bytes, error) { return 97e9, 41.2e9, nil }

// fakeEngine serves testdata/docker/<route>.json on a unix socket and returns the socket path.
func fakeEngine(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"/_ping":                               "ping.txt",
		"/services":                            "services.json",
		"/tasks":                               "tasks.json",
		"/containers/json?all=1":               "containers.json",
		"/system/df":                           "df.json",
		"/containers/c-api/stats?stream=false": "stats-api.json",
		"/containers/c-db/stats?stream=false":  "stats-db.json",
	}
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	ln := testkit.Must(net.Listen("unix", socket))
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := files[r.URL.RequestURI()]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(testkit.Must(os.ReadFile(filepath.Join("testdata", "docker", name))))
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socket
}

func useCase(t *testing.T, socket string, out io.Writer) *report.UseCase {
	t.Helper()
	proc := fstest.MapFS{
		"meminfo": {Data: testkit.Must(os.ReadFile("testdata/proc/meminfo"))},
		"loadavg": {Data: testkit.Must(os.ReadFile("testdata/proc/loadavg"))},
		"uptime":  {Data: testkit.Must(os.ReadFile("testdata/proc/uptime"))},
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return report.New(report.Deps{
		Host:         "vps-heron-02",
		AgentVersion: "1.0.0",
		Clock:        fake.Clock{T: time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)},
		Metrics:      hostfs.NewMetrics(proc, fixedDisk{}),
		Docker:       docker.NewUnix(socket, time.Second, time.Second, logger),
		Dirs:         fake.DirSizer{Sizes: map[string]domain.Bytes{"/etc/easypanel/projects/cms-heronlabs/cms-database/backups": 1_000_000}},
		Sink:         cms.NewPrinter(out),
		Logger:       logger,
	})
}

func TestReportOnce_PrintsTheContractForTheFakeHost(t *testing.T) {
	var out bytes.Buffer
	uc := useCase(t, fakeEngine(t), &out)

	testkit.MustDo(t, uc.Run(context.Background()))

	testkit.Equal(t, string(testkit.Must(os.ReadFile("testdata/report-once.golden.json"))), out.String())
}

func TestReportOnce_FailsWithoutDockerSocket(t *testing.T) {
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	missing := filepath.Join(dir, "docker.sock")

	err := useCase(t, missing, io.Discard).Run(context.Background())

	testkit.ErrorIs(t, err, syscall.ENOENT)
}
