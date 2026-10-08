package host

// Load is the load average of the host in proc/loadavg over 1, 5 and 15 minutes.
type Load struct {
	One     float64
	Five    float64
	Fifteen float64
}
