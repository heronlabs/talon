package cli

import "log/slog"

// VersionCommand runs talon version.
type VersionCommand struct {
	version string
	logger  *slog.Logger
}

// NewVersionCommand returns a VersionCommand that logs version through logger.
func NewVersionCommand(version string, logger *slog.Logger) *VersionCommand {
	return &VersionCommand{version: version, logger: logger}
}

// Run logs the version talon was built with and returns 0.
func (c *VersionCommand) Run() int {
	c.logger.Info("version", "version", c.version)

	return 0
}
