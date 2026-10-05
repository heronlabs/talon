package application

import "testing"

func TestVersionCommand(t *testing.T) {
	t.Run("Should return the version it was built with", func(t *testing.T) {
		command := NewVersionCommand("1.0.0")

		version := command.GetVersion()

		if version != "1.0.0" {
			t.Errorf("GetVersion() = %q, want %q", version, "1.0.0")
		}
	})
}
