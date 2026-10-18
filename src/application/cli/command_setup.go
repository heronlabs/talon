package cli

import "log/slog"

// Setup writes the configuration of the host.
type Setup interface {
	Configure() error
}

// SetupCommand runs talon setup.
type SetupCommand struct {
	setup  Setup
	logger *slog.Logger
}

// NewSetupCommand returns a SetupCommand that writes the configuration through setup and logs through logger.
func NewSetupCommand(setup Setup, logger *slog.Logger) *SetupCommand {
	return &SetupCommand{setup: setup, logger: logger}
}

// Run writes the configuration, logs the result and returns 0 when it is written or 1 when it is not.
func (c *SetupCommand) Run() int {
	if err := c.setup.Configure(); err != nil {
		c.logger.Error("setup", "error", err)
		return 1
	}

	c.logger.Info("setup", "status", "ok")

	return 0
}
