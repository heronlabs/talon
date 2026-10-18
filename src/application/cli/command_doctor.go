package cli

import (
	"context"
	"log/slog"
)

// Doctor checks the configuration and the Docker Engine of the host.
type Doctor interface {
	Check(ctx context.Context) error
}

// DoctorCommand runs talon doctor.
type DoctorCommand struct {
	doctor Doctor
	logger *slog.Logger
}

// NewDoctorCommand returns a DoctorCommand that checks the host through doctor and logs through logger.
func NewDoctorCommand(doctor Doctor, logger *slog.Logger) *DoctorCommand {
	return &DoctorCommand{doctor: doctor, logger: logger}
}

// Run checks the host with ctx, logs the result and returns 0 when the check passes or 1 when it fails.
func (c *DoctorCommand) Run(ctx context.Context) int {
	if err := c.doctor.Check(ctx); err != nil {
		c.logger.Error("doctor", "error", err)
		return 1
	}

	c.logger.Info("doctor", "status", "ok")

	return 0
}
