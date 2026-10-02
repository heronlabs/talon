package cms_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var errWrite = errors.New("write")

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errWrite }

func sample() domain.Report {
	return domain.Report{
		Host:         "vps-heron-02",
		MeasuredAt:   time.Date(2026, 10, 1, 11, 10, 0, 0, time.FixedZone("BRT", -3*3600)),
		AgentVersion: "1.0.0",
		System: domain.System{
			RAMTotal: 8321499136, RAMUsed: 5476083302, SwapTotal: 4 << 30, SwapUsed: 322122547,
			DiskTotal: 97e9, DiskUsed: 41.2e9, Load1: 0.421, Load5: 0.38, Load15: 0.35,
			Uptime: 1209600*time.Second + 500*time.Millisecond, DockerImages: 6.1e9, DockerBuildCache: 2.4e9,
		},
		Projects: []domain.Project{{Name: "cms-heronlabs", Reconcile: domain.StateUnmanaged, Services: []domain.Service{{
			Name: "cms-database", State: domain.StateRunning, ReplicasRunning: 1, ReplicasDesired: 1,
			MemoryUsed: 310 << 20, MemoryLimit: 512 << 20, CPUPercent: 3.204, CPULimit: 0.5,
			Disk: 8.1e9, NetRx: 120.5e6, NetTx: 88.1e6, BlockRead: 50.2e6, BlockWrite: 12.9e6,
		}}}},
	}
}

func TestToDTO_ConvertsUnitsAndRounds(t *testing.T) {
	got := cms.ToDTO(sample())

	testkit.Equal(t, cms.ReportDTO{
		Host:       "vps-heron-02",
		MeasuredAt: "2026-10-01T14:10:00Z",
		Agent:      cms.AgentDTO{Version: "1.0.0"},
		System: cms.SystemDTO{
			RAMTotalGiB: 7.75, RAMUsedGiB: 5.1, SwapTotalGiB: 4, SwapUsedGiB: 0.3,
			DiskTotalGB: 97, DiskUsedGB: 41.2, Load1: 0.42, Load5: 0.38, Load15: 0.35,
			UptimeSeconds: 1209600, DockerImagesGB: 6.1, DockerBuildCacheGB: 2.4,
		},
		Projects: []cms.ProjectDTO{{
			Name:      "cms-heronlabs",
			Reconcile: cms.ReconcileDTO{State: "unmanaged", MissingSecrets: []string{}},
			Services: []cms.ServiceDTO{{
				Name: "cms-database", State: "running", ReplicasRunning: 1, ReplicasDesired: 1,
				MemoryUsedMiB: 310, MemoryLimitMiB: 512, CPUPercent: 3.2, CPULimit: 0.5,
				DiskGB: 8.1, NetRxMB: 120.5, NetTxMB: 88.1, BlockReadMB: 50.2, BlockWriteMB: 12.9,
			}},
		}},
	}, got)
}

func TestToDTO_NoProjectsIsAnEmptyList(t *testing.T) {
	got := cms.ToDTO(domain.Report{})

	testkit.Equal(t, []cms.ProjectDTO{}, got.Projects)
}

func TestPublish_PrintsContractJSON(t *testing.T) {
	var out bytes.Buffer
	r := domain.Report{Host: "h", MeasuredAt: time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC), AgentVersion: "1.0.0",
		Projects: []domain.Project{{Name: "p", Reconcile: domain.StateUnmanaged}}}

	testkit.MustDo(t, cms.NewPrinter(&out).Publish(context.Background(), r))

	testkit.Equal(t, `{
  "host": "h",
  "measuredAt": "2026-10-01T14:10:00Z",
  "agent": {
    "version": "1.0.0",
    "easypanelVersion": null,
    "easypanelSupported": null,
    "lastReconcile": null
  },
  "system": {
    "ramTotalGiB": 0,
    "ramUsedGiB": 0,
    "swapTotalGiB": 0,
    "swapUsedGiB": 0,
    "diskTotalGB": 0,
    "diskUsedGB": 0,
    "load1": 0,
    "load5": 0,
    "load15": 0,
    "uptimeSeconds": 0,
    "dockerImagesGB": 0,
    "dockerBuildCacheGB": 0
  },
  "projects": [
    {
      "name": "p",
      "reconcile": {
        "state": "unmanaged",
        "missingSecrets": [],
        "error": null
      },
      "services": []
    }
  ]
}
`, out.String())
}

func TestPublish_FailsWhenWriterFails(t *testing.T) {
	err := cms.NewPrinter(brokenWriter{}).Publish(context.Background(), domain.Report{})

	testkit.ErrorIs(t, err, errWrite)
}
