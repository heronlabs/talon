package core

import (
	"fmt"
	"time"

	"github.com/heronlabs/talon/src/core/entity"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

// endpointKey and tokenKey are the keys of the configuration with the URL of the webhook and its token.
const (
	endpointKey = "WEBHOOK_URL"
	tokenKey    = "WEBHOOK_AGENT_TOKEN" //nolint:gosec // G101: the name of the environment variable, not a credential
)

// ConfigGetter gets the value of one key of the configuration.
type ConfigGetter interface {
	Get(name string) (string, error)
}

// ReportCollector collects the snapshot of the host from the Docker Engine.
type ReportCollector interface {
	Collect() (*docker.Snapshot, error)
}

// ReportSender sends the report to the webhook at endpoint, with its token, and returns the reply.
type ReportSender interface {
	Send(endpoint, token string, report *entity.Report) (*webhook.ReportResponse, error)
}

// ReportService builds the report of the host from its snapshot and sends it to the webhook.
type ReportService struct {
	collector ReportCollector
	sender    ReportSender
	config    ConfigGetter
	version   string
	hostname  func() (string, error)
	now       func() time.Time
}

// NewReportService returns a ReportService that collects the snapshot through collector, sends through sender, reads the configuration through config and builds the report with version, the host from hostname and the time from now.
func NewReportService(collector ReportCollector, sender ReportSender, config ConfigGetter, version string, hostname func() (string, error), now func() time.Time) *ReportService {
	return &ReportService{collector: collector, sender: sender, config: config, version: version, hostname: hostname, now: now}
}

// Send builds the report and sends it to WEBHOOK_URL with WEBHOOK_AGENT_TOKEN; without WEBHOOK_URL it builds it and sends nothing.
func (s *ReportService) Send() error {
	endpoint, token, err := s.readConfig()
	if err != nil {
		return err
	}

	report, err := s.collect()
	if err != nil {
		return err
	}

	if endpoint == "" {
		return nil
	}

	if _, err := s.sender.Send(endpoint, token, report); err != nil {
		return fmt.Errorf("webhook: %w", err)
	}

	return nil
}

// readConfig returns the URL of the webhook and its token.
func (s *ReportService) readConfig() (endpoint, token string, err error) {
	if endpoint, err = s.config.Get(endpointKey); err != nil {
		return "", "", fmt.Errorf("config: %w", err)
	}

	if token, err = s.config.Get(tokenKey); err != nil {
		return "", "", fmt.Errorf("config: %w", err)
	}

	return endpoint, token, nil
}

// collect returns the report built from the snapshot of the host, with its hostname, the time from the clock and the version of talon.
func (s *ReportService) collect() (*entity.Report, error) {
	host, err := s.hostname()
	if err != nil {
		return nil, fmt.Errorf("host: %w", err)
	}

	snapshot, err := s.collector.Collect()
	if err != nil {
		return nil, fmt.Errorf("docker: %w", err)
	}

	return entity.NewReport(host, s.now(), s.version, *snapshot), nil
}
