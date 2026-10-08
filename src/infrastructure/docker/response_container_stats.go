package docker

// ContainerStats is the answer of the Engine API with the stats of one container: its memory, its two CPU samples, its networks and its block I/O.
type ContainerStats struct {
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	CPUStats    CPUStats `json:"cpu_stats"`
	PreCPUStats CPUStats `json:"precpu_stats"`
	Networks    map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IoServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}
