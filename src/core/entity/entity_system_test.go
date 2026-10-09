package entity

import (
	"testing"

	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/host"
)

func TestSystem(t *testing.T) {
	t.Run("Should report the images in GB", func(t *testing.T) {
		usage := decode[docker.DiskUsage](t, `{"LayersSize":2500000000}`)
		want := System{DockerImagesGB: 2.5}

		system := NewSystem(usage, host.Snapshot{})

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the build cache in GB, summed over its records", func(t *testing.T) {
		usage := decode[docker.DiskUsage](t, `{"BuildCache":[{"Size":1000000000},{"Size":500000000}]}`)
		want := System{DockerBuildCacheGB: 1.5}

		system := NewSystem(usage, host.Snapshot{})

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the RAM total and used in GiB, used as the total less the available", func(t *testing.T) {
		hostSnapshot := host.Snapshot{Memory: host.Memory{MemTotal: 8388608, MemAvailable: 2097152}}
		want := System{RAMTotalGiB: 8, RAMUsedGiB: 6}

		system := NewSystem(docker.DiskUsage{}, hostSnapshot)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the swap total and used in GiB, used as the total less the free", func(t *testing.T) {
		hostSnapshot := host.Snapshot{Memory: host.Memory{SwapTotal: 3145728, SwapFree: 524288}}
		want := System{SwapTotalGiB: 3, SwapUsedGiB: 2.5}

		system := NewSystem(docker.DiskUsage{}, hostSnapshot)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the disk total and used in GB, used as the total less the free", func(t *testing.T) {
		hostSnapshot := host.Snapshot{Disk: host.Disk{Total: 50000000000, Free: 12500000000}}
		want := System{DiskTotalGB: 50, DiskUsedGB: 37.5}

		system := NewSystem(docker.DiskUsage{}, hostSnapshot)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the load over 1, 5 and 15 minutes", func(t *testing.T) {
		hostSnapshot := host.Snapshot{Load: host.Load{One: 0.52, Five: 0.58, Fifteen: 0.59}}
		want := System{Load1: 0.52, Load5: 0.58, Load15: 0.59}

		system := NewSystem(docker.DiskUsage{}, hostSnapshot)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})

	t.Run("Should report the uptime in whole seconds", func(t *testing.T) {
		hostSnapshot := host.Snapshot{Uptime: host.Uptime{Seconds: 12345.67}}
		want := System{UptimeSeconds: 12345}

		system := NewSystem(docker.DiskUsage{}, hostSnapshot)

		if system != want {
			t.Errorf("NewSystem() = %+v, want %+v", system, want)
		}
	})
}
