package host

// Memory is the memory of the host in proc/meminfo, in kB: MemTotal, MemAvailable, SwapTotal and SwapFree.
type Memory struct {
	MemTotal     int64
	MemAvailable int64
	SwapTotal    int64
	SwapFree     int64
}
