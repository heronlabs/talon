package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/manifest"
)

func TestLastReconcile(t *testing.T) {
	t.Run("Should be nil without the manifest and without an error", func(t *testing.T) {
		lastReconcile := NewLastReconcile("2026-10-08T12:30:15Z", nil, nil)

		if lastReconcile != nil {
			t.Errorf("NewLastReconcile() = %+v, want nil", lastReconcile)
		}
	})

	t.Run("Should be ok at the time of the report, with no error, when the manifest was read", func(t *testing.T) {
		declared := decode[manifest.Manifest](t, `{"data":{"host":"vps-01","clients":[]}}`)
		want := &LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "ok"}

		lastReconcile := NewLastReconcile("2026-10-08T12:30:15Z", &declared, nil)

		if !reflect.DeepEqual(lastReconcile, want) {
			t.Errorf("NewLastReconcile() = %+v, want %+v", lastReconcile, want)
		}
	})

	t.Run("Should be failed at the time of the report with the error of the observation", func(t *testing.T) {
		message := "manifest: unexpected status: 500"
		want := &LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "failed", Error: &message}

		lastReconcile := NewLastReconcile("2026-10-08T12:30:15Z", nil, errors.New("manifest: unexpected status: 500"))

		if !reflect.DeepEqual(lastReconcile, want) {
			t.Errorf("NewLastReconcile() = %+v, want %+v", lastReconcile, want)
		}
	})

	t.Run("Should be failed when Easypanel failed after the manifest was read", func(t *testing.T) {
		declared := decode[manifest.Manifest](t, `{"data":{"host":"vps-01","clients":[]}}`)
		message := "easypanel: listProjectsAndServices: unexpected status: 401"
		want := &LastReconcile{At: "2026-10-08T12:30:15Z", Outcome: "failed", Error: &message}

		lastReconcile := NewLastReconcile("2026-10-08T12:30:15Z", &declared, errors.New("easypanel: listProjectsAndServices: unexpected status: 401"))

		if !reflect.DeepEqual(lastReconcile, want) {
			t.Errorf("NewLastReconcile() = %+v, want %+v", lastReconcile, want)
		}
	})

	t.Run("Should cut the error to 500 characters", func(t *testing.T) {
		message := strings.Repeat("é", 500)

		lastReconcile := NewLastReconcile("2026-10-08T12:30:15Z", nil, errors.New(strings.Repeat("é", 600)))

		if lastReconcile == nil || lastReconcile.Error == nil || *lastReconcile.Error != message {
			t.Errorf("NewLastReconcile() = %+v, want an error of 500 characters", lastReconcile)
		}
	})
}
