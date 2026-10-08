package application

import "log/slog"

// Doctor checks the configuration and the Docker Engine of the host.
type Doctor interface {
	Check() error
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

// Run checks the host, logs the result and returns 0 when the check passes or 1 when it fails.
func (c *DoctorCommand) Run() int {
	if err := c.doctor.Check(); err != nil {
		c.logger.Error("doctor", "error", err)
		return 1
	}

	c.logger.Info("doctor", "status", "ok")

	return 0
}
