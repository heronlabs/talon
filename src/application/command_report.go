package application

import "log/slog"

// Report sends the report of the host.
type Report interface {
	Send() error
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

// Run sends the report, logs the result and returns 0 when it is sent or 1 when it is not.
func (c *ReportCommand) Run() int {
	if err := c.report.Send(); err != nil {
		c.logger.Error("report", "error", err)
		return 1
	}

	c.logger.Info("report", "status", "ok")

	return 0
}
