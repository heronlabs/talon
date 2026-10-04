package application

import (
	"testing"
	"testing/fstest"
)

func TestVersionCommand(t *testing.T) {
	t.Run("Should return version", func(t *testing.T) {
		command := NewVersionCommand(fstest.MapFS{"version.txt": {Data: []byte("1.0.0\n")}})

		version := command.GetTxtVersion()

		if version != "1.0.0" {
			t.Errorf("GetTxtVersion() = %q, want %q", version, "1.0.0")
		}
	})

	t.Run("Should return 0.0.0 due to error on file reading", func(t *testing.T) {
		command := NewVersionCommand(fstest.MapFS{})

		version := command.GetTxtVersion()

		if version != "0.0.0" {
			t.Errorf("GetTxtVersion() = %q, want %q", version, "0.0.0")
		}
	})
}
