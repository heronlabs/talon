package domain

import (
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/host"
)

// System holds the memory, swap, disk, load and uptime of the host and the size of the Docker images and build cache.
type System struct {
	RAMTotalGiB        float64 `json:"ramTotalGiB"`
	RAMUsedGiB         float64 `json:"ramUsedGiB"`
	SwapTotalGiB       float64 `json:"swapTotalGiB"`
	SwapUsedGiB        float64 `json:"swapUsedGiB"`
	DiskTotalGB        float64 `json:"diskTotalGB"`
	DiskUsedGB         float64 `json:"diskUsedGB"`
	Load1              float64 `json:"load1"`
	Load5              float64 `json:"load5"`
	Load15             float64 `json:"load15"`
	UptimeSeconds      int64   `json:"uptimeSeconds"`
	DockerImagesGB     float64 `json:"dockerImagesGB"`
	DockerBuildCacheGB float64 `json:"dockerBuildCacheGB"`
}

// NewSystem returns the system with the memory and the swap of hostSnapshot in GiB, each used as its total less its available or free, the disk of hostSnapshot in GB, used as its total less its free, its load, its uptime in whole seconds, and the size of the Docker images and of the build cache of usage in GB.
func NewSystem(usage docker.DiskUsage, hostSnapshot host.Snapshot) System {
	memory := hostSnapshot.Memory
	disk := hostSnapshot.Disk
	return System{
		RAMTotalGiB:        float64(memory.MemTotal) / kibPerGiB,
		RAMUsedGiB:         float64(memory.MemTotal-memory.MemAvailable) / kibPerGiB,
		SwapTotalGiB:       float64(memory.SwapTotal) / kibPerGiB,
		SwapUsedGiB:        float64(memory.SwapTotal-memory.SwapFree) / kibPerGiB,
		DiskTotalGB:        float64(disk.Total) / bytesPerGB,
		DiskUsedGB:         float64(disk.Total-disk.Free) / bytesPerGB,
		Load1:              hostSnapshot.Load.One,
		Load5:              hostSnapshot.Load.Five,
		Load15:             hostSnapshot.Load.Fifteen,
		UptimeSeconds:      int64(hostSnapshot.Uptime.Seconds),
		DockerImagesGB:     float64(usage.LayersSize) / bytesPerGB,
		DockerBuildCacheGB: float64(buildCacheSize(usage)) / bytesPerGB,
	}
}

// buildCacheSize returns the bytes of every record of the build cache of usage.
func buildCacheSize(usage docker.DiskUsage) int64 {
	var size int64
	for _, record := range usage.BuildCache {
		size += record.Size
	}

	return size
}
