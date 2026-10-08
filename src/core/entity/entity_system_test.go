package entity

import (
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
)

func TestSystem(t *testing.T) {
	t.Run("Should report the images in GB", func(t *testing.T) {
		usage := decode[docker.DiskUsage](t, `{"LayersSize":2500000000}`)
		want := System{DockerImagesGB: 2.5}

		system := NewSystem(usage)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the build cache in GB, summed over its records", func(t *testing.T) {
		usage := decode[docker.DiskUsage](t, `{"BuildCache":[{"Size":1000000000},{"Size":500000000}]}`)
		want := System{DockerBuildCacheGB: 1.5}

		system := NewSystem(usage)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})
}
