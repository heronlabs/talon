package application

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

func TestDoctorCommand(t *testing.T) {
	t.Run("Should pass when .env has mode 600 and Docker responds", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{".env": {Mode: 0o600}}, fakeHealthCheck{})

		err := command.Run()

		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{}, fakeHealthCheck{})

		err := command.Run()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Run() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{".env": {Mode: 0o644}}, fakeHealthCheck{})

		err := command.Run()

		if err == nil || err.Error() != "config: .env has mode 644, want 600" {
			t.Errorf("Run() = %v, want %q", err, "config: .env has mode 644, want 600")
		}
	})

	t.Run("Should fail when Docker does not respond", func(t *testing.T) {
		errPing := errors.New("ping failed")
		command := NewDoctorCommand(fstest.MapFS{".env": {Mode: 0o600}}, fakeHealthCheck{err: errPing})

		err := command.Run()

		if !errors.Is(err, errPing) {
			t.Errorf("Run() = %v, want %v", err, errPing)
		}
	})
}
