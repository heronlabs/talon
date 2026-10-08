package core

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

type fakeHealthCheck struct {
	err error
}

func (f fakeHealthCheck) Ping() error {
	return f.err
}

func TestDoctorService(t *testing.T) {
	t.Run("Should pass when .env has mode 600 and Docker responds", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, fakeHealthCheck{})

		err := service.Check()

		if err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{}, fakeHealthCheck{})

		err := service.Check()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Check() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o644}}, fakeHealthCheck{})

		err := service.Check()

		if err == nil || err.Error() != "config: .env has mode 644, want 600" {
			t.Errorf("Check() = %v, want %q", err, "config: .env has mode 644, want 600")
		}
	})

	t.Run("Should fail when Docker does not respond", func(t *testing.T) {
		errPing := errors.New("ping failed")
		service := NewDoctorService(fstest.MapFS{".env": {Mode: 0o600}}, fakeHealthCheck{err: errPing})

		err := service.Check()

		if !errors.Is(err, errPing) {
			t.Errorf("Check() = %v, want %v", err, errPing)
		}
	})
}
