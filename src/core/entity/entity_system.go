package entity

import "github.com/heronlabs/talon/src/infrastructure/docker"

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

// NewSystem returns the system with the size of the Docker images and of the build cache of usage in GB.
func NewSystem(usage docker.DiskUsage) System {
	return System{
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
