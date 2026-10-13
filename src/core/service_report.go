package core

import (
	"context"
	"fmt"
	"time"

	"github.com/heronlabs/talon/src/core/entity"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

// endpointKey and tokenKey are the keys of the configuration with the URL of the webhook and its token.
const (
	endpointKey = "WEBHOOK_URL"
	tokenKey    = "WEBHOOK_TOKEN" //nolint:gosec // G101: the name of the environment variable, not a credential
)

// The keys of the configuration with the URL of the manifest and its token, and the URL of the Easypanel API and its token.
const (
	manifestURLKey    = "MANIFEST_URL"
	manifestTokenKey  = "MANIFEST_TOKEN"
	easypanelURLKey   = "EASYPANEL_URL"
	easypanelTokenKey = "EASYPANEL_TOKEN"
)

// ConfigGetter gets the values of keys of the configuration, by their name, in one read.
type ConfigGetter interface {
	Get(names ...string) (map[string]string, error)
}

// DockerCollector collects the snapshot of the host from the Docker Engine.
type DockerCollector interface {
	Collect(ctx context.Context) (*docker.Snapshot, error)
}

// HostCollector collects the snapshot of the metrics of the host.
type HostCollector interface {
	Collect() (*host.Snapshot, error)
}

// EasypanelCollector collects the snapshot of the directories of the Easypanel projects of the host.
type EasypanelCollector interface {
	Collect() (*easypanel.Snapshot, error)
}

// ReportSender sends the report to the webhook at endpoint, with its token, and returns the reply.
type ReportSender interface {
	Send(ctx context.Context, endpoint, token string, report *entity.Report) (*webhook.ReportResponse, error)
}

// ManifestReader reads the manifest of the host at endpoint, with its token.
type ManifestReader interface {
	Read(ctx context.Context, endpoint, token string) (*manifest.Manifest, error)
}

// EasypanelConfigCollector collects the configuration that the Easypanel API at endpoint holds, with its token.
type EasypanelConfigCollector interface {
	Collect(ctx context.Context, endpoint, token string) (*easypanel.ConfigSnapshot, error)
}

// ReportService builds the report of the host from its snapshots and the observation of its manifest and Easypanel, and sends it to the webhook.
type ReportService struct {
	dockerCollector          DockerCollector
	hostCollector            HostCollector
	easypanelCollector       EasypanelCollector
	manifestReader           ManifestReader
	easypanelConfigCollector EasypanelConfigCollector
	sender                   ReportSender
	config                   ConfigGetter
	version                  string
	hostname                 func() (string, error)
	now                      func() time.Time
}

// NewReportService returns a ReportService that collects the snapshot of the Docker Engine through dockerCollector, the snapshot of the metrics of the host through hostCollector and the snapshot of the Easypanel directories through easypanelCollector, reads the manifest through manifestReader and the configuration of Easypanel through easypanelConfigCollector, sends through sender, reads the configuration through config and builds the report with version, the host from hostname and the time from now.
func NewReportService(dockerCollector DockerCollector, hostCollector HostCollector, easypanelCollector EasypanelCollector, manifestReader ManifestReader, easypanelConfigCollector EasypanelConfigCollector, sender ReportSender, config ConfigGetter, version string, hostname func() (string, error), now func() time.Time) *ReportService {
	return &ReportService{dockerCollector: dockerCollector, hostCollector: hostCollector, easypanelCollector: easypanelCollector, manifestReader: manifestReader, easypanelConfigCollector: easypanelConfigCollector, sender: sender, config: config, version: version, hostname: hostname, now: now}
}

// Send builds the report, sends it to WEBHOOK_URL with WEBHOOK_TOKEN and returns it, talking to the Docker Engine, the manifest, Easypanel and the webhook with ctx; without WEBHOOK_URL it builds it, sends nothing and returns it.
func (s *ReportService) Send(ctx context.Context) (*entity.Report, error) {
	config, err := s.readConfig()
	if err != nil {
		return nil, err
	}

	report, err := s.collect(ctx, config)
	if err != nil {
		return nil, err
	}

	if config[endpointKey] == "" {
		return report, nil
	}

	if _, err := s.sender.Send(ctx, config[endpointKey], config[tokenKey], report); err != nil {
		return nil, fmt.Errorf("webhook: %w", err)
	}

	return report, nil
}

// readConfig returns the URLs and tokens of the webhook, the manifest and Easypanel, read together.
func (s *ReportService) readConfig() (map[string]string, error) {
	values, err := s.config.Get(endpointKey, tokenKey, manifestURLKey, manifestTokenKey, easypanelURLKey, easypanelTokenKey)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return values, nil
}

// collect returns the report built, in order, from the hostname, the snapshot of the Docker Engine requested with ctx, the snapshot of the metrics of the host, the snapshot of the Easypanel directories and the observation of the manifest and Easypanel of config, with the time from the clock and the version of talon; a failing observation does not fail it.
func (s *ReportService) collect(ctx context.Context, config map[string]string) (*entity.Report, error) {
	hostname, err := s.hostname()
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}

	snapshot, err := s.dockerCollector.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}

	hostSnapshot, err := s.hostCollector.Collect()
	if err != nil {
		return nil, fmt.Errorf("system: %w", err)
	}

	easypanelSnapshot, err := s.easypanelCollector.Collect()
	if err != nil {
		return nil, fmt.Errorf("easypanel: %w", err)
	}

	declared, configured, observeErr := s.observe(ctx, config)

	return entity.NewReport(hostname, s.now(), s.version, *snapshot, *hostSnapshot, *easypanelSnapshot, declared, configured, observeErr), nil
}

// observe reads, with ctx, the manifest at MANIFEST_URL with MANIFEST_TOKEN and then the configuration of the Easypanel API at EASYPANEL_URL with EASYPANEL_TOKEN, and returns them; it reads nothing without MANIFEST_URL, nothing from Easypanel when the manifest fails, and returns the error of the read that failed, with the manifest when Easypanel failed.
func (s *ReportService) observe(ctx context.Context, config map[string]string) (*manifest.Manifest, *easypanel.ConfigSnapshot, error) {
	if config[manifestURLKey] == "" {
		return nil, nil, nil
	}

	declared, err := s.manifestReader.Read(ctx, config[manifestURLKey], config[manifestTokenKey])
	if err != nil {
		return nil, nil, fmt.Errorf("manifest: %w", err)
	}

	configured, err := s.easypanelConfigCollector.Collect(ctx, config[easypanelURLKey], config[easypanelTokenKey])
	if err != nil {
		return declared, nil, fmt.Errorf("easypanel: %w", err)
	}

	return declared, configured, nil
}
