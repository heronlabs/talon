package application

import (
	"encoding/json"
	"log/slog"

	"github.com/heronlabs/talon/src/core/entity"
)

// Report sends the report of the host and returns it.
type Report interface {
	Send() (*entity.Report, error)
}

// ReportCommand runs talon report.
type ReportCommand struct {
	report Report
	logger *slog.Logger
}

// NewReportCommand returns a ReportCommand that sends the report through report and logs through logger.
func NewReportCommand(report Report, logger *slog.Logger) *ReportCommand {
	return &ReportCommand{report: report, logger: logger}
}

// Run sends the report, logs the result with the report as JSON in payload and returns 0 when it is sent or 1 when it is not.
func (c *ReportCommand) Run() int {
	report, err := c.report.Send()
	if err != nil {
		c.logger.Error("report", "error", err)
		return 1
	}

	payload, err := json.Marshal(report)
	if err != nil {
		c.logger.Error("report", "error", err)
		return 1
	}

	c.logger.Info("report", "status", "ok", "payload", string(payload))

	return 0
}
