package core

import (
	"fmt"
	"io/fs"
)

// HealthCheck pings the Docker Engine.
type HealthCheck interface {
	Ping() error
}

// DoctorService checks the configuration and the Docker Engine of the host.
type DoctorService struct {
	fsys        fs.FS
	healthCheck HealthCheck
}

// NewDoctorService returns a DoctorService that finds .env in fsys and pings the Docker Engine through healthCheck.
func NewDoctorService(fsys fs.FS, healthCheck HealthCheck) *DoctorService {
	return &DoctorService{fsys: fsys, healthCheck: healthCheck}
}

// Check fails when .env does not exist or its mode is not 600, or when the Docker Engine does not answer the ping.
func (s *DoctorService) Check() error {
	if err := s.checkEnv(); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := s.healthCheck.Ping(); err != nil {
		return fmt.Errorf("docker: %w", err)
	}

	return nil
}

// checkEnv fails when .env does not exist or its mode is not 600.
func (s *DoctorService) checkEnv() error {
	info, err := fs.Stat(s.fsys, ".env")
	if err != nil {
		return err
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		return fmt.Errorf(".env has mode %o, want 600", perm)
	}

	return nil
}
