package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heronlabs/talon/src/core/entity"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

type fakeConfigGetter struct {
	values map[string]string
	errs   map[string]error
}

func (f fakeConfigGetter) Get(name string) (string, error) {
	return f.values[name], f.errs[name]
}

type fakeDockerCollector struct {
	snapshot  docker.Snapshot
	collected bool
	err       error
}

func (f *fakeDockerCollector) Collect() (*docker.Snapshot, error) {
	f.collected = true

	return &f.snapshot, f.err
}

type fakeHostCollector struct {
	snapshot host.Snapshot
	err      error
}

func (f *fakeHostCollector) Collect() (*host.Snapshot, error) {
	return &f.snapshot, f.err
}

type fakeEasypanelCollector struct {
	snapshot easypanel.Snapshot
	err      error
}

func (f *fakeEasypanelCollector) Collect() (*easypanel.Snapshot, error) {
	return &f.snapshot, f.err
}

type fakeReportSender struct {
	endpoint string
	token    string
	sent     *entity.Report
	err      error
}

func (f *fakeReportSender) Send(endpoint, token string, report *entity.Report) (*webhook.ReportResponse, error) {
	f.endpoint = endpoint
	f.token = token
	f.sent = report

	return &webhook.ReportResponse{}, f.err
}

func TestReportService(t *testing.T) {
	t.Run("Should send the host from the hostname", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent == nil || sender.sent.Host != "vps-01" {
			t.Errorf("Send() sent %+v, want host %q", sender.sent, "vps-01")
		}
	})

	t.Run("Should send measuredAt as the clock in RFC3339Nano UTC", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		now := func() time.Time {
			return time.Date(2026, 10, 8, 9, 30, 15, 123456789, time.FixedZone("BRT", -3*60*60))
		}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, now)

		_, _ = service.Send()

		if sender.sent == nil || sender.sent.MeasuredAt != "2026-10-08T12:30:15.123456789Z" {
			t.Errorf("Send() sent %+v, want measuredAt %q", sender.sent, "2026-10-08T12:30:15.123456789Z")
		}
	})

	t.Run("Should send agent.version", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent == nil || sender.sent.Agent.Version != "1.2.3" {
			t.Errorf("Send() sent %+v, want agent.version %q", sender.sent, "1.2.3")
		}
	})

	t.Run("Should send the report built from the collected snapshot", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{snapshot: docker.Snapshot{DiskUsage: docker.DiskUsage{LayersSize: 2500000000}}}
		sender := &fakeReportSender{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent == nil || sender.sent.System.DockerImagesGB != 2.5 {
			t.Errorf("Send() sent %+v, want system.dockerImagesGB %v", sender.sent, 2.5)
		}
	})

	t.Run("Should send the report built from the collected host snapshot", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		hostCollector := &fakeHostCollector{snapshot: host.Snapshot{Memory: host.Memory{MemTotal: 8388608}}}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, hostCollector, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent == nil || sender.sent.System.RAMTotalGiB != 8 {
			t.Errorf("Send() sent %+v, want system.ramTotalGiB %v", sender.sent, 8)
		}
	})

	t.Run("Should send the report built from the collected easypanel snapshot", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		api := docker.SwarmService{ID: "s1"}
		api.Spec.Name = "shop_api"
		dockerCollector := &fakeDockerCollector{snapshot: docker.Snapshot{Services: []docker.SwarmService{api}}}
		easypanelCollector := &fakeEasypanelCollector{snapshot: easypanel.Snapshot{Sizes: map[string]map[string]int64{"shop": {"api": 1500000000}}}}
		sender := &fakeReportSender{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, easypanelCollector, sender, config, "1.2.3", hostname, time.Now)
		want := []entity.Project{{Name: "shop", Services: []entity.Service{{Name: "api", State: "missing", DiskGB: 1.5}}}}

		_, _ = service.Send()

		if sender.sent == nil || !reflect.DeepEqual(sender.sent.Projects, want) {
			t.Errorf("Send() sent %+v, want projects %+v", sender.sent, want)
		}
	})

	t.Run("Should send to WEBHOOK_URL", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.endpoint != "https://webhook.example.com" {
			t.Errorf("Send() sent to %q, want %q", sender.endpoint, "https://webhook.example.com")
		}
	})

	t.Run("Should send the WEBHOOK_TOKEN", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "WEBHOOK_TOKEN": "token"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.token != "token" {
			t.Errorf("Send() sent token %q, want %q", sender.token, "token")
		}
	})

	t.Run("Should return nil when the report is sent", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if err != nil {
			t.Errorf("Send() = %v, want nil", err)
		}
	})

	t.Run("Should return the report when it is sent", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		report, _ := service.Send()

		if report == nil || report.Host != "vps-01" {
			t.Errorf("Send() = %+v, want host %q", report, "vps-01")
		}
	})

	t.Run("Should return the report when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		report, _ := service.Send()

		if report == nil || report.Host != "vps-01" {
			t.Errorf("Send() = %+v, want host %q", report, "vps-01")
		}
	})

	t.Run("Should not send when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should still collect when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if !dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, true)
		}
	})

	t.Run("Should fail when WEBHOOK_URL cannot be read", func(t *testing.T) {
		errGet := errors.New("get failed")
		config := fakeConfigGetter{errs: map[string]error{"WEBHOOK_URL": errGet}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errGet) || !strings.HasPrefix(err.Error(), "config: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errGet, "config: ")
		}
	})

	t.Run("Should fail when WEBHOOK_TOKEN cannot be read", func(t *testing.T) {
		errGet := errors.New("get failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}, errs: map[string]error{"WEBHOOK_TOKEN": errGet}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errGet) || !strings.HasPrefix(err.Error(), "config: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errGet, "config: ")
		}
	})

	t.Run("Should not collect when the configuration cannot be read", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}, errs: map[string]error{"WEBHOOK_TOKEN": errors.New("get failed")}}
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, false)
		}
	})

	t.Run("Should fail when the hostname cannot be read", func(t *testing.T) {
		errHostname := errors.New("hostname failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "", errHostname }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errHostname) || !strings.HasPrefix(err.Error(), "host: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errHostname, "host: ")
		}
	})

	t.Run("Should not collect when the hostname cannot be read", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "", errors.New("hostname failed") }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, false)
		}
	})

	t.Run("Should fail when the report cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{err: errCollect}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "docker: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "docker: ")
		}
	})

	t.Run("Should not send when the report cannot be collected", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{err: errors.New("collect failed")}, &fakeHostCollector{}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the host metrics cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{err: errCollect}, &fakeEasypanelCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "system: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "system: ")
		}
	})

	t.Run("Should not send when the host metrics cannot be collected", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{err: errors.New("collect failed")}, &fakeEasypanelCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the easypanel directories cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{err: errCollect}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "easypanel: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "easypanel: ")
		}
	})

	t.Run("Should not send when the easypanel directories cannot be collected", func(t *testing.T) {
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{err: errors.New("collect failed")}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send()

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the report cannot be sent", func(t *testing.T) {
		errSend := errors.New("send failed")
		config := fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeReportSender{err: errSend}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send()

		if !errors.Is(err, errSend) || !strings.HasPrefix(err.Error(), "webhook: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errSend, "webhook: ")
		}
	})
}
