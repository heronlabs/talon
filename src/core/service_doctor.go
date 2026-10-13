package core

import (
	"context"
	"fmt"
	"io/fs"
)

// HealthCheck pings the Docker Engine.
type HealthCheck interface {
	Ping(ctx context.Context) error
}

// EasypanelHealthCheck checks the token of the Easypanel API at endpoint.
type EasypanelHealthCheck interface {
	Check(ctx context.Context, endpoint, token string) error
}

// DoctorService checks the configuration, the Docker Engine and the token of the Easypanel API of the host.
type DoctorService struct {
	fsys                 fs.FS
	healthCheck          HealthCheck
	config               ConfigGetter
	easypanelHealthCheck EasypanelHealthCheck
}

// NewDoctorService returns a DoctorService that finds .env in fsys, pings the Docker Engine through healthCheck, reads the configuration through config and checks the token of the Easypanel API through easypanelHealthCheck.
func NewDoctorService(fsys fs.FS, healthCheck HealthCheck, config ConfigGetter, easypanelHealthCheck EasypanelHealthCheck) *DoctorService {
	return &DoctorService{fsys: fsys, healthCheck: healthCheck, config: config, easypanelHealthCheck: easypanelHealthCheck}
}

// Check fails when .env does not exist or its mode is not 600, when the Docker Engine does not answer the ping before ctx is done, when the configuration cannot be read, or, with EASYPANEL_TOKEN set, when the Easypanel API at EASYPANEL_URL refuses it.
func (s *DoctorService) Check(ctx context.Context) error {
	if err := s.checkEnv(); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := s.healthCheck.Ping(ctx); err != nil {
		return fmt.Errorf("docker: %w", err)
	}

	values, err := s.config.Get(easypanelURLKey, easypanelTokenKey)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := s.checkEasypanel(ctx, values[easypanelURLKey], values[easypanelTokenKey]); err != nil {
		return fmt.Errorf("easypanel: %w", err)
	}

	return nil
}

// checkEasypanel checks token against the Easypanel API at endpoint with ctx, and passes without a token.
func (s *DoctorService) checkEasypanel(ctx context.Context, endpoint, token string) error {
	if token == "" {
		return nil
	}

	return s.easypanelHealthCheck.Check(ctx, endpoint, token)
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
