package application

import (
	"fmt"
	"io/fs"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

type DoctorCommand struct {
	fsys        fs.FS
	healthCheck docker.HealthCheck
}

func NewDoctorCommand(fsys fs.FS, healthCheck docker.HealthCheck) *DoctorCommand {
	return &DoctorCommand{fsys: fsys, healthCheck: healthCheck}
}

func (c *DoctorCommand) Run() error {
	info, err := fs.Stat(c.fsys, ".env")
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		return fmt.Errorf("config: .env has mode %o, want 600", perm)
	}

	if err := c.healthCheck.Ping(); err != nil {
		return fmt.Errorf("docker: %w", err)
	}

	return nil
}
