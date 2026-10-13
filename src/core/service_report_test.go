package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/heronlabs/talon/src/core/entity"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

type fakeConfigGetter struct {
	values    map[string]string
	requested [][]string
	err       error
}

func (f *fakeConfigGetter) Get(names ...string) (map[string]string, error) {
	f.requested = append(f.requested, names)

	return f.values, f.err
}

type fakeDockerCollector struct {
	snapshot  docker.Snapshot
	collected bool
	ctx       context.Context
	err       error
}

func (f *fakeDockerCollector) Collect(ctx context.Context) (*docker.Snapshot, error) {
	f.collected = true
	f.ctx = ctx

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

type fakeManifestReader struct {
	manifest *manifest.Manifest
	read     bool
	ctx      context.Context
	endpoint string
	token    string
	err      error
}

func (f *fakeManifestReader) Read(ctx context.Context, endpoint, token string) (*manifest.Manifest, error) {
	f.read = true
	f.ctx = ctx
	f.endpoint = endpoint
	f.token = token

	return f.manifest, f.err
}

type fakeEasypanelConfigCollector struct {
	snapshot  *easypanel.ConfigSnapshot
	collected bool
	ctx       context.Context
	endpoint  string
	token     string
	err       error
}

func (f *fakeEasypanelConfigCollector) Collect(ctx context.Context, endpoint, token string) (*easypanel.ConfigSnapshot, error) {
	f.collected = true
	f.ctx = ctx
	f.endpoint = endpoint
	f.token = token

	return f.snapshot, f.err
}

type fakeReportSender struct {
	ctx      context.Context
	endpoint string
	token    string
	sent     *entity.Report
	err      error
}

func (f *fakeReportSender) Send(ctx context.Context, endpoint, token string, report *entity.Report) (*webhook.ReportResponse, error) {
	f.ctx = ctx
	f.endpoint = endpoint
	f.token = token
	f.sent = report

	return &webhook.ReportResponse{}, f.err
}

func TestReportService(t *testing.T) {
	t.Run("Should send the host from the hostname", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent == nil || sender.sent.Host != "vps-01" {
			t.Errorf("Send() sent %+v, want host %q", sender.sent, "vps-01")
		}
	})

	t.Run("Should send measuredAt as the clock in RFC3339Nano UTC", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		now := func() time.Time {
			return time.Date(2026, 10, 8, 9, 30, 15, 123456789, time.FixedZone("BRT", -3*60*60))
		}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, now)

		_, _ = service.Send(t.Context())

		if sender.sent == nil || sender.sent.MeasuredAt != "2026-10-08T12:30:15.123456789Z" {
			t.Errorf("Send() sent %+v, want measuredAt %q", sender.sent, "2026-10-08T12:30:15.123456789Z")
		}
	})

	t.Run("Should send agent.version", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent == nil || sender.sent.Agent.Version != "1.2.3" {
			t.Errorf("Send() sent %+v, want agent.version %q", sender.sent, "1.2.3")
		}
	})

	t.Run("Should send the report built from the collected snapshot", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{snapshot: docker.Snapshot{DiskUsage: docker.DiskUsage{LayersSize: 2500000000}}}
		sender := &fakeReportSender{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent == nil || sender.sent.System.DockerImagesGB != 2.5 {
			t.Errorf("Send() sent %+v, want system.dockerImagesGB %v", sender.sent, 2.5)
		}
	})

	t.Run("Should send the report built from the collected host snapshot", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		hostCollector := &fakeHostCollector{snapshot: host.Snapshot{Memory: host.Memory{MemTotal: 8388608}}}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, hostCollector, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent == nil || sender.sent.System.RAMTotalGiB != 8 {
			t.Errorf("Send() sent %+v, want system.ramTotalGiB %v", sender.sent, 8)
		}
	})

	t.Run("Should send the report built from the collected easypanel snapshot", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		api := docker.SwarmService{ID: "s1"}
		api.Spec.Name = "shop_api"
		dockerCollector := &fakeDockerCollector{snapshot: docker.Snapshot{Services: []docker.SwarmService{api}}}
		easypanelCollector := &fakeEasypanelCollector{snapshot: easypanel.Snapshot{Sizes: map[string]map[string]int64{"shop": {"api": 1500000000}}}}
		sender := &fakeReportSender{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, easypanelCollector, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)
		want := []entity.Project{{Name: "shop", Services: []entity.Service{{Name: "api", State: "missing", DiskGB: 1.5}}}}

		_, _ = service.Send(t.Context())

		if sender.sent == nil || !reflect.DeepEqual(sender.sent.Projects, want) {
			t.Errorf("Send() sent %+v, want projects %+v", sender.sent, want)
		}
	})

	t.Run("Should send to WEBHOOK_URL", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.endpoint != "https://webhook.example.com" {
			t.Errorf("Send() sent to %q, want %q", sender.endpoint, "https://webhook.example.com")
		}
	})

	t.Run("Should send the WEBHOOK_TOKEN", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "WEBHOOK_TOKEN": "token"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.token != "token" {
			t.Errorf("Send() sent token %q, want %q", sender.token, "token")
		}
	})

	t.Run("Should return nil when the report is sent", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if err != nil {
			t.Errorf("Send() = %v, want nil", err)
		}
	})

	t.Run("Should return the report when it is sent", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		report, _ := service.Send(t.Context())

		if report == nil || report.Host != "vps-01" {
			t.Errorf("Send() = %+v, want host %q", report, "vps-01")
		}
	})

	t.Run("Should return the report when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, &fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		report, _ := service.Send(t.Context())

		if report == nil || report.Host != "vps-01" {
			t.Errorf("Send() = %+v, want host %q", report, "vps-01")
		}
	})

	t.Run("Should not send when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, &fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should still collect when WEBHOOK_URL is empty", func(t *testing.T) {
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, &fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if !dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, true)
		}
	})

	t.Run("Should read the URLs and tokens of the webhook, the manifest and Easypanel in one call", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)
		want := [][]string{{"WEBHOOK_URL", "WEBHOOK_TOKEN", "MANIFEST_URL", "MANIFEST_TOKEN", "EASYPANEL_URL", "EASYPANEL_TOKEN"}}

		_, _ = service.Send(t.Context())

		if !reflect.DeepEqual(config.requested, want) {
			t.Errorf("Send() requested %v, want %v", config.requested, want)
		}
	})

	t.Run("Should collect from the Docker Engine with the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, &fakeConfigGetter{}, "1.2.3", hostname, time.Now)

		_, _ = service.Send(ctx)

		if dockerCollector.ctx != ctx {
			t.Errorf("Send() collected with %v, want %v", dockerCollector.ctx, ctx)
		}
	})

	t.Run("Should send with the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(ctx)

		if sender.ctx != ctx {
			t.Errorf("Send() sent with %v, want %v", sender.ctx, ctx)
		}
	})

	t.Run("Should fail when the configuration cannot be read", func(t *testing.T) {
		errGet := errors.New("get failed")
		config := &fakeConfigGetter{err: errGet}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errGet) || !strings.HasPrefix(err.Error(), "config: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errGet, "config: ")
		}
	})

	t.Run("Should not collect when the configuration cannot be read", func(t *testing.T) {
		config := &fakeConfigGetter{err: errors.New("get failed")}
		hostname := func() (string, error) { return "vps-01", nil }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, false)
		}
	})

	t.Run("Should fail when the hostname cannot be read", func(t *testing.T) {
		errHostname := errors.New("hostname failed")
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "", errHostname }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errHostname) || !strings.HasPrefix(err.Error(), "host: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errHostname, "host: ")
		}
	})

	t.Run("Should not collect when the hostname cannot be read", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "", errors.New("hostname failed") }
		dockerCollector := &fakeDockerCollector{}
		service := NewReportService(dockerCollector, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if dockerCollector.collected {
			t.Errorf("Send() collected = %v, want %v", dockerCollector.collected, false)
		}
	})

	t.Run("Should fail when the report cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{err: errCollect}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "docker: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "docker: ")
		}
	})

	t.Run("Should not send when the report cannot be collected", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{err: errors.New("collect failed")}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the host metrics cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{err: errCollect}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "system: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "system: ")
		}
	})

	t.Run("Should not send when the host metrics cannot be collected", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{err: errors.New("collect failed")}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the easypanel directories cannot be collected", func(t *testing.T) {
		errCollect := errors.New("collect failed")
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{err: errCollect}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errCollect) || !strings.HasPrefix(err.Error(), "easypanel: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errCollect, "easypanel: ")
		}
	})

	t.Run("Should not send when the easypanel directories cannot be collected", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{err: errors.New("collect failed")}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if sender.sent != nil {
			t.Errorf("Send() sent %+v, want nothing", sender.sent)
		}
	})

	t.Run("Should fail when the report cannot be sent", func(t *testing.T) {
		errSend := errors.New("send failed")
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{}, &fakeEasypanelConfigCollector{}, &fakeReportSender{err: errSend}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if !errors.Is(err, errSend) || !strings.HasPrefix(err.Error(), "webhook: ") {
			t.Errorf("Send() = %v, want %v with prefix %q", err, errSend, "webhook: ")
		}
	})

	t.Run("Should not read the manifest without MANIFEST_URL", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com"}}
		hostname := func() (string, error) { return "vps-01", nil }
		manifestReader := &fakeManifestReader{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, manifestReader, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if manifestReader.read {
			t.Errorf("Send() read the manifest = %v, want %v", manifestReader.read, false)
		}
	})

	t.Run("Should read the manifest at MANIFEST_URL with MANIFEST_TOKEN and the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		config := &fakeConfigGetter{values: map[string]string{"MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01", "MANIFEST_TOKEN": "manifest-token"}}
		hostname := func() (string, error) { return "vps-01", nil }
		manifestReader := &fakeManifestReader{manifest: &manifest.Manifest{}}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, manifestReader, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(ctx)

		if manifestReader.ctx != ctx || manifestReader.endpoint != "https://cms.example.com/api/infra-manifests/vps-01" || manifestReader.token != "manifest-token" {
			t.Errorf("Send() read %q with %q and %v, want %q with %q and %v", manifestReader.endpoint, manifestReader.token, manifestReader.ctx, "https://cms.example.com/api/infra-manifests/vps-01", "manifest-token", ctx)
		}
	})

	t.Run("Should collect the configuration of Easypanel at EASYPANEL_URL with EASYPANEL_TOKEN and the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		config := &fakeConfigGetter{values: map[string]string{"MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01", "EASYPANEL_URL": "http://localhost:3000", "EASYPANEL_TOKEN": "easypanel-token"}}
		hostname := func() (string, error) { return "vps-01", nil }
		configCollector := &fakeEasypanelConfigCollector{snapshot: &easypanel.ConfigSnapshot{}}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{manifest: &manifest.Manifest{}}, configCollector, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(ctx)

		if configCollector.ctx != ctx || configCollector.endpoint != "http://localhost:3000" || configCollector.token != "easypanel-token" {
			t.Errorf("Send() collected %q with %q and %v, want %q with %q and %v", configCollector.endpoint, configCollector.token, configCollector.ctx, "http://localhost:3000", "easypanel-token", ctx)
		}
	})

	t.Run("Should send the reconcile of each project and lastReconcile ok from the manifest and the configuration of Easypanel", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01"}}
		hostname := func() (string, error) { return "vps-01", nil }
		now := func() time.Time { return time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC) }
		declared := &manifest.Manifest{}
		declared.Data.Clients = []manifest.Client{{Slug: "shop"}}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{manifest: declared}, &fakeEasypanelConfigCollector{snapshot: &easypanel.ConfigSnapshot{}}, sender, config, "1.2.3", hostname, now)
		wantProjects := []entity.Project{{Name: "shop", Reconcile: &entity.Reconcile{State: "pending", MissingSecrets: []string{}}, Services: []entity.Service{}}}
		wantLastReconcile := &entity.LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "ok"}

		_, _ = service.Send(t.Context())

		if sender.sent == nil || !reflect.DeepEqual(sender.sent.Projects, wantProjects) || !reflect.DeepEqual(sender.sent.Agent.LastReconcile, wantLastReconcile) {
			t.Errorf("Send() sent %+v, want projects %+v and lastReconcile %+v", sender.sent, wantProjects, wantLastReconcile)
		}
	})

	t.Run("Should send the report with lastReconcile failed with the error of the manifest when it cannot be read", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01"}}
		hostname := func() (string, error) { return "vps-01", nil }
		now := func() time.Time { return time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC) }
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{err: errors.New("read failed")}, &fakeEasypanelConfigCollector{}, sender, config, "1.2.3", hostname, now)
		message := "manifest: read failed"
		want := &entity.LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "failed", Error: &message}

		_, _ = service.Send(t.Context())

		if sender.sent == nil || !reflect.DeepEqual(sender.sent.Agent.LastReconcile, want) {
			t.Errorf("Send() sent %+v, want lastReconcile %+v", sender.sent, want)
		}
	})

	t.Run("Should not collect from Easypanel when the manifest cannot be read", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01"}}
		hostname := func() (string, error) { return "vps-01", nil }
		configCollector := &fakeEasypanelConfigCollector{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{err: errors.New("read failed")}, configCollector, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, _ = service.Send(t.Context())

		if configCollector.collected {
			t.Errorf("Send() collected from Easypanel = %v, want %v", configCollector.collected, false)
		}
	})

	t.Run("Should send the projects of the manifest with lastReconcile failed with the error of Easypanel when it cannot be read", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01"}}
		hostname := func() (string, error) { return "vps-01", nil }
		now := func() time.Time { return time.Date(2026, 10, 8, 12, 30, 15, 0, time.UTC) }
		declared := &manifest.Manifest{}
		declared.Data.Clients = []manifest.Client{{Slug: "shop"}}
		sender := &fakeReportSender{}
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{manifest: declared}, &fakeEasypanelConfigCollector{err: errors.New("collect failed")}, sender, config, "1.2.3", hostname, now)
		message := "easypanel: collect failed"
		wantProjects := []entity.Project{{Name: "shop", Services: []entity.Service{}}}
		wantLastReconcile := &entity.LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "failed", Error: &message}

		_, _ = service.Send(t.Context())

		if sender.sent == nil || !reflect.DeepEqual(sender.sent.Projects, wantProjects) || !reflect.DeepEqual(sender.sent.Agent.LastReconcile, wantLastReconcile) {
			t.Errorf("Send() sent %+v, want projects %+v and lastReconcile %+v", sender.sent, wantProjects, wantLastReconcile)
		}
	})

	t.Run("Should return nil when the observation fails", func(t *testing.T) {
		config := &fakeConfigGetter{values: map[string]string{"WEBHOOK_URL": "https://webhook.example.com", "MANIFEST_URL": "https://cms.example.com/api/infra-manifests/vps-01"}}
		hostname := func() (string, error) { return "vps-01", nil }
		service := NewReportService(&fakeDockerCollector{}, &fakeHostCollector{}, &fakeEasypanelCollector{}, &fakeManifestReader{err: errors.New("read failed")}, &fakeEasypanelConfigCollector{}, &fakeReportSender{}, config, "1.2.3", hostname, time.Now)

		_, err := service.Send(t.Context())

		if err != nil {
			t.Errorf("Send() = %v, want nil", err)
		}
	})
}
