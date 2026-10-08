package entity

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
)

func decode[T any](t *testing.T, body string) T {
	t.Helper()

	var value T
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}

	return value
}

func TestReport(t *testing.T) {
	t.Run("Should fill the host", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)

		report := NewReport("vps-01", measuredAt, "1.2.3", docker.Snapshot{}, host.Snapshot{}, easypanel.Snapshot{})

		if report.Host != "vps-01" {
			t.Errorf("NewReport() host = %q, want %q", report.Host, "vps-01")
		}
	})

	t.Run("Should fill measuredAt in RFC3339Nano UTC", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 9, 30, 15, 123456789, time.FixedZone("BRT", -3*60*60))

		report := NewReport("vps-01", measuredAt, "1.2.3", docker.Snapshot{}, host.Snapshot{}, easypanel.Snapshot{})

		if report.MeasuredAt != "2026-10-08T12:30:15.123456789Z" {
			t.Errorf("NewReport() measuredAt = %q, want %q", report.MeasuredAt, "2026-10-08T12:30:15.123456789Z")
		}
	})

	t.Run("Should fill agent.version", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)

		report := NewReport("vps-01", measuredAt, "1.2.3", docker.Snapshot{}, host.Snapshot{}, easypanel.Snapshot{})

		if report.Agent.Version != "1.2.3" {
			t.Errorf("NewReport() agent.version = %q, want %q", report.Agent.Version, "1.2.3")
		}
	})

	t.Run("Should fill agent.easypanelVersion from the Easypanel package", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)
		snapshot := docker.Snapshot{Easypanel: &docker.EasypanelPackage{Version: "2.36.0"}}

		report := NewReport("vps-01", measuredAt, "1.2.3", snapshot, host.Snapshot{}, easypanel.Snapshot{})

		if report.Agent.EasypanelVersion == nil || *report.Agent.EasypanelVersion != "2.36.0" {
			t.Errorf("NewReport() agent = %+v, want easypanelVersion %q", report.Agent, "2.36.0")
		}
	})

	t.Run("Should fill the system from the disk usage", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)
		snapshot := docker.Snapshot{DiskUsage: decode[docker.DiskUsage](t, `{"LayersSize":2500000000}`)}
		want := System{DockerImagesGB: 2.5}

		report := NewReport("vps-01", measuredAt, "1.2.3", snapshot, host.Snapshot{}, easypanel.Snapshot{})

		if report.System != want {
			t.Errorf("NewReport() system = %+v, want %+v", report.System, want)
		}
	})

	t.Run("Should fill the system from the host snapshot", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)
		hostSnapshot := host.Snapshot{Load: host.Load{One: 0.52}}
		want := System{Load1: 0.52}

		report := NewReport("vps-01", measuredAt, "1.2.3", docker.Snapshot{}, hostSnapshot, easypanel.Snapshot{})

		if report.System != want {
			t.Errorf("NewReport() system = %+v, want %+v", report.System, want)
		}
	})

	t.Run("Should fill the projects from the Swarm services", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api"}}]`)}
		want := []Project{{Name: "shop", Services: []Service{{Name: "api", State: "missing"}}}}

		report := NewReport("vps-01", measuredAt, "1.2.3", snapshot, host.Snapshot{}, easypanel.Snapshot{})

		if !reflect.DeepEqual(report.Projects, want) {
			t.Errorf("NewReport() projects = %+v, want %+v", report.Projects, want)
		}
	})

	t.Run("Should fill the disk of the services from the easypanel snapshot", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)
		snapshot := docker.Snapshot{Services: decode[[]docker.SwarmService](t, `[{"ID":"s1","Spec":{"Name":"shop_api"}}]`)}
		easypanelSnapshot := easypanel.Snapshot{Sizes: map[string]map[string]int64{"shop": {"api": 1500000000}}}
		want := []Project{{Name: "shop", Services: []Service{{Name: "api", State: "missing", DiskGB: 1.5}}}}

		report := NewReport("vps-01", measuredAt, "1.2.3", snapshot, host.Snapshot{}, easypanelSnapshot)

		if !reflect.DeepEqual(report.Projects, want) {
			t.Errorf("NewReport() projects = %+v, want %+v", report.Projects, want)
		}
	})

	t.Run("Should encode projects as [] when there is no service", func(t *testing.T) {
		measuredAt := time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC)

		report := NewReport("vps-01", measuredAt, "1.2.3", docker.Snapshot{}, host.Snapshot{}, easypanel.Snapshot{})
		encoded, _ := json.Marshal(report)

		if !strings.Contains(string(encoded), `"projects":[]`) {
			t.Errorf("NewReport() encoded %s, want %s", encoded, `"projects":[]`)
		}
	})
}
