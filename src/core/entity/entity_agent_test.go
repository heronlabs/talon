package entity

import (
	"reflect"
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

func TestAgent(t *testing.T) {
	t.Run("Should leave the Easypanel version and whether it is supported null without the Easypanel package", func(t *testing.T) {
		want := Agent{Version: "1.2.3"}

		agent := NewAgent("1.2.3", nil)

		if !reflect.DeepEqual(agent, want) {
			t.Errorf("NewAgent() = %+v, want %+v", agent, want)
		}
	})

	t.Run("Should report the Easypanel version as supported when it is in the supported versions", func(t *testing.T) {
		easypanelPackage := decode[docker.EasypanelPackage](t, `{"name":"easypanel","version":"2.36.0"}`)
		easypanelVersion := "2.36.0"
		supported := true
		want := Agent{Version: "1.2.3", EasypanelVersion: &easypanelVersion, EasypanelSupported: &supported}

		agent := NewAgent("1.2.3", &easypanelPackage)

		if !reflect.DeepEqual(agent, want) {
			t.Errorf("NewAgent() = %+v, want %+v", agent, want)
		}
	})

	t.Run("Should report the Easypanel version as not supported when it is not in the supported versions", func(t *testing.T) {
		easypanelPackage := decode[docker.EasypanelPackage](t, `{"name":"easypanel","version":"2.35.1"}`)
		easypanelVersion := "2.35.1"
		supported := false
		want := Agent{Version: "1.2.3", EasypanelVersion: &easypanelVersion, EasypanelSupported: &supported}

		agent := NewAgent("1.2.3", &easypanelPackage)

		if !reflect.DeepEqual(agent, want) {
			t.Errorf("NewAgent() = %+v, want %+v", agent, want)
		}
	})
}
