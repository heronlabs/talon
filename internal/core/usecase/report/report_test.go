package report_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var (
	now     = time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

const bindDir = "/etc/easypanel/projects/p/db/data"

func dbSample() domain.DockerSample {
	return domain.DockerSample{
		Services:   []domain.SwarmService{{ID: "1", Name: "p_db", ReplicasDesired: 1}},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_db", State: domain.StateRunning, BindSources: []string{bindDir}}},
	}
}

func deps(sink *fake.Sink, logs *bytes.Buffer) report.Deps {
	return report.Deps{
		Host:         "vps-heron-02",
		AgentVersion: "1.0.0",
		Clock:        fake.Clock{T: now},
		Metrics:      fake.HostMetrics{Sample: domain.HostSample{MemTotal: 10, MemAvailable: 4}},
		Docker:       fake.Docker{Sample: dbSample()},
		Dirs:         fake.DirSizer{Sizes: map[string]domain.Bytes{bindDir: 7}},
		Sink:         sink,
		Logger: slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		})),
	}
}

func TestRun_PublishesTheBuiltReport(t *testing.T) {
	sink := &fake.Sink{}
	uc := report.New(deps(sink, &bytes.Buffer{}))

	testkit.MustDo(t, uc.Run(context.Background()))

	testkit.Equal(t, []domain.Report{{
		Host:         "vps-heron-02",
		MeasuredAt:   now,
		AgentVersion: "1.0.0",
		System:       domain.System{RAMTotal: 10, RAMUsed: 6},
		Projects: []domain.Project{{Name: "p", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "db", State: domain.StateRunning, ReplicasDesired: 1, Disk: 7},
		}}},
	}}, sink.Published)
}

func TestRun_FailsWhenHostMetricsFail(t *testing.T) {
	d := deps(&fake.Sink{}, &bytes.Buffer{})
	d.Metrics = fake.HostMetrics{Err: errBoom}

	err := report.New(d).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenDockerFails(t *testing.T) {
	d := deps(&fake.Sink{}, &bytes.Buffer{})
	d.Docker = fake.Docker{SnapshotErr: errBoom}

	err := report.New(d).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenSinkFails(t *testing.T) {
	err := report.New(deps(&fake.Sink{Err: errBoom}, &bytes.Buffer{})).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailedDirCountsAsZero(t *testing.T) {
	sink := &fake.Sink{}
	d := deps(sink, &bytes.Buffer{})
	d.Dirs = fake.DirSizer{Errs: map[string]error{bindDir: errBoom}}

	testkit.MustDo(t, report.New(d).Run(context.Background()))

	testkit.Equal(t, domain.Bytes(0), sink.Published[0].Projects[0].Services[0].Disk)
}

func TestRun_FailedDirIsLogged(t *testing.T) {
	logs := &bytes.Buffer{}
	d := deps(&fake.Sink{}, logs)
	d.Dirs = fake.DirSizer{Errs: map[string]error{bindDir: errBoom}}

	testkit.MustDo(t, report.New(d).Run(context.Background()))

	testkit.Equal(t,
		`{"level":"WARN","msg":"dir size failed","logId":"report.dir-size-failed","path":"`+bindDir+`","errorName":"*errors.errorString","errorMessage":"boom"}`+"\n",
		logs.String())
}
