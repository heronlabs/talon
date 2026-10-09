package host

// Snapshot holds the metrics of the host, read one after the other.
type Snapshot struct {
	// Disk is the total and free bytes of the filesystem of the root.
	Disk Disk
	// Memory is the memory and the swap in proc/meminfo.
	Memory Memory
	// Load is the load average in proc/loadavg.
	Load Load
	// Uptime is the uptime in proc/uptime.
	Uptime Uptime
}
