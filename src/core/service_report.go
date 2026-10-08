package core

import (
	"fmt"
	"time"

	"github.com/heronlabs/talon/src/core/entity"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

// endpointKey and tokenKey are the keys of the configuration with the URL of the webhook and its token.
const (
	endpointKey = "WEBHOOK_URL"
	tokenKey    = "WEBHOOK_TOKEN" //nolint:gosec // G101: the name of the environment variable, not a credential
)

// ConfigGetter gets the value of one key of the configuration.
type ConfigGetter interface {
	Get(name string) (string, error)
}

// ReportCollector collects the snapshot of the host from the Docker Engine.
type ReportCollector interface {
	Collect() (*docker.Snapshot, error)
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
	Send(endpoint, token string, report *entity.Report) (*webhook.ReportResponse, error)
}

// ReportService builds the report of the host from its snapshots and sends it to the webhook.
type ReportService struct {
	collector          ReportCollector
	hostCollector      HostCollector
	easypanelCollector EasypanelCollector
	sender             ReportSender
	config             ConfigGetter
	version            string
	hostname           func() (string, error)
	now                func() time.Time
}

// NewReportService returns a ReportService that collects the snapshot of the Docker Engine through collector, the snapshot of the metrics of the host through hostCollector and the snapshot of the Easypanel directories through easypanelCollector, sends through sender, reads the configuration through config and builds the report with version, the host from hostname and the time from now.
func NewReportService(collector ReportCollector, hostCollector HostCollector, easypanelCollector EasypanelCollector, sender ReportSender, config ConfigGetter, version string, hostname func() (string, error), now func() time.Time) *ReportService {
	return &ReportService{collector: collector, hostCollector: hostCollector, easypanelCollector: easypanelCollector, sender: sender, config: config, version: version, hostname: hostname, now: now}
}

// Send builds the report, sends it to WEBHOOK_URL with WEBHOOK_TOKEN and returns it; without WEBHOOK_URL it builds it, sends nothing and returns it.
func (s *ReportService) Send() (*entity.Report, error) {
	endpoint, token, err := s.readConfig()
	if err != nil {
		return nil, err
	}

	report, err := s.collect()
	if err != nil {
		return nil, err
	}

	if endpoint == "" {
		return report, nil
	}

	if _, err := s.sender.Send(endpoint, token, report); err != nil {
		return nil, fmt.Errorf("webhook: %w", err)
	}

	return report, nil
}

// readConfig returns the URL of the webhook and its token, reading the token only when the URL is read.
func (s *ReportService) readConfig() (endpoint, token string, err error) {
	if endpoint, err = s.config.Get(endpointKey); err == nil {
		token, err = s.config.Get(tokenKey)
	}

	if err != nil {
		return "", "", fmt.Errorf("config: %w", err)
	}

	return endpoint, token, nil
}

// collect returns the report built, in order, from the hostname, the snapshot of the Docker Engine, the snapshot of the metrics of the host and the snapshot of the Easypanel directories, with the time from the clock and the version of talon.
func (s *ReportService) collect() (*entity.Report, error) {
	hostname, err := s.hostname()
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}

	snapshot, err := s.collector.Collect()
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

	return entity.NewReport(hostname, s.now(), s.version, *snapshot, *hostSnapshot, *easypanelSnapshot), nil
}
