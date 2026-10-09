package core

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

type fakeHealthCheck struct {
	ctx context.Context
	err error
}

func (f *fakeHealthCheck) Ping(ctx context.Context) error {
	f.ctx = ctx

	return f.err
}

func TestDoctorService(t *testing.T) {
	t.Run("Should pass when .env has mode 600 and Docker responds", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{})

		err := service.Check(t.Context())

		if err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{}, &fakeHealthCheck{})

		err := service.Check(t.Context())

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Check() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o644}}, &fakeHealthCheck{})

		err := service.Check(t.Context())

		if err == nil || err.Error() != "config: .env has mode 644, want 600" {
			t.Errorf("Check() = %v, want %q", err, "config: .env has mode 644, want 600")
		}
	})

	t.Run("Should ping with the ctx it was given", func(t *testing.T) {
		ctx := t.Context()
		healthCheck := &fakeHealthCheck{}
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, healthCheck)

		_ = service.Check(ctx)

		if healthCheck.ctx != ctx {
			t.Errorf("Check() pinged with %v, want %v", healthCheck.ctx, ctx)
		}
	})

	t.Run("Should fail when Docker does not respond", func(t *testing.T) {
		errPing := errors.New("ping failed")
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, &fakeHealthCheck{err: errPing})

		err := service.Check(t.Context())

		if !errors.Is(err, errPing) {
			t.Errorf("Check() = %v, want %v", err, errPing)
		}
	})
}
