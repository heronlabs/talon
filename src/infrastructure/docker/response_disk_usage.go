package docker

// DiskUsage is the answer of the Engine API with the disk usage of the images, the build cache and the volumes.
type DiskUsage struct {
	LayersSize int64
	BuildCache []struct {
		Size int64
	}
	Volumes []struct {
		Name      string
		UsageData struct {
			Size int64
		}
	}
}
