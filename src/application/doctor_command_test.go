package application

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestDoctorCommand(t *testing.T) {
	t.Run("Should pass when .env has mode 600", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{".env": {Mode: 0o600}})

		err := command.Run()

		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	})

	t.Run("Should fail when .env does not exist", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{})

		err := command.Run()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Run() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when .env has mode other than 600", func(t *testing.T) {
		command := NewDoctorCommand(fstest.MapFS{".env": {Mode: 0o644}})

		err := command.Run()

		if err == nil || err.Error() != "config: .env has mode 644, want 600" {
			t.Errorf("Run() = %v, want %q", err, "config: .env has mode 644, want 600")
		}
	})
}
