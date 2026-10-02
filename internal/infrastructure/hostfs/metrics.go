// Package hostfs reads the host: /proc, statfs and directory sizes.
package hostfs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrMalformed marks a /proc file that does not have the expected shape.
var ErrMalformed = errors.New("malformed proc file")

// DiskStater measures the filesystem that holds a path.
type DiskStater interface {
	Stat(path string) (total, used domain.Bytes, err error)
}

// Metrics reads the host from a procfs and a DiskStater.
type Metrics struct {
	proc fs.FS
	disk DiskStater
}

// NewMetrics builds Metrics over proc (os.DirFS("/proc") in production) and disk.
func NewMetrics(proc fs.FS, disk DiskStater) *Metrics {
	return &Metrics{proc: proc, disk: disk}
}

// Read implements port.HostMetrics.
func (m *Metrics) Read(context.Context) (domain.HostSample, error) {
	var s domain.HostSample
	if err := m.readMemInfo(&s); err != nil {
		return domain.HostSample{}, err
	}
	if err := m.readLoadAvg(&s); err != nil {
		return domain.HostSample{}, err
	}
	if err := m.readUptime(&s); err != nil {
		return domain.HostSample{}, err
	}
	total, used, err := m.disk.Stat("/")
	if err != nil {
		return domain.HostSample{}, err
	}
	s.DiskTotal, s.DiskUsed = total, used
	return s, nil
}

func (m *Metrics) readMemInfo(s *domain.HostSample) error {
	f, err := m.proc.Open("meminfo")
	if err != nil {
		return fmt.Errorf("open meminfo: %w", err)
	}
	defer f.Close()
	fields := map[string]*domain.Bytes{
		"MemTotal": &s.MemTotal, "MemAvailable": &s.MemAvailable,
		"SwapTotal": &s.SwapTotal, "SwapFree": &s.SwapFree,
	}
	found := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		target, wanted := fields[key]
		if !ok || !wanted {
			continue
		}
		kib, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
		if err != nil {
			return fmt.Errorf("%w: meminfo %s: %w", ErrMalformed, key, err)
		}
		*target = domain.Bytes(kib * 1024)
		found[key] = true
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%w: meminfo: %w", ErrMalformed, err)
	}
	if !found["MemTotal"] || !found["MemAvailable"] {
		return fmt.Errorf("%w: meminfo without MemTotal or MemAvailable", ErrMalformed)
	}
	return nil
}

func (m *Metrics) readLoadAvg(s *domain.HostSample) error {
	fields, err := m.fields("loadavg", 3)
	if err != nil {
		return err
	}
	loads := []*float64{&s.Load1, &s.Load5, &s.Load15}
	for i, target := range loads {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return fmt.Errorf("%w: loadavg: %w", ErrMalformed, err)
		}
		*target = v
	}
	return nil
}

func (m *Metrics) readUptime(s *domain.HostSample) error {
	fields, err := m.fields("uptime", 1)
	if err != nil {
		return err
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fmt.Errorf("%w: uptime: %w", ErrMalformed, err)
	}
	s.Uptime = time.Duration(secs) * time.Second
	return nil
}

// fields reads a one-line proc file and returns its first n whitespace-separated fields.
func (m *Metrics) fields(name string, n int) ([]string, error) {
	raw, err := fs.ReadFile(m.proc, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) < n {
		return nil, fmt.Errorf("%w: %s has %d fields, want %d", ErrMalformed, name, len(fields), n)
	}
	return fields[:n], nil
}
