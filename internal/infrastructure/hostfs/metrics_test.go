package hostfs_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const meminfo = `MemTotal:        8126464 kB
MemFree:          512000 kB
MemAvailable:    2774016 kB
Buffers:          100000 kB
SwapTotal:       4194304 kB
SwapFree:        3879731 kB
HugePages_Total:       0
`

var errDisk = errors.New("disk")

type disk struct {
	total, used domain.Bytes
	err         error
}

func (d disk) Stat(string) (domain.Bytes, domain.Bytes, error) { return d.total, d.used, d.err }

func proc(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{
		"meminfo": {Data: []byte(meminfo)},
		"loadavg": {Data: []byte("0.42 0.38 0.35 1/234 5678\n")},
		"uptime":  {Data: []byte("1209600.57 2400000.00\n")},
	}
	for name, data := range files {
		if data == "" {
			delete(m, name)
			continue
		}
		m[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return m
}

func read(files map[string]string, d disk) (domain.HostSample, error) {
	return hostfs.NewMetrics(proc(files), d).Read(context.Background())
}

func TestRead_ParsesProcAndDisk(t *testing.T) {
	got, err := read(nil, disk{total: 97e9, used: 41e9})
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.HostSample{
		MemTotal: 8126464 * 1024, MemAvailable: 2774016 * 1024,
		SwapTotal: 4194304 * 1024, SwapFree: 3879731 * 1024,
		Load1: 0.42, Load5: 0.38, Load15: 0.35,
		Uptime:    1209600 * time.Second,
		DiskTotal: 97e9, DiskUsed: 41e9,
	}, got)
}

func TestRead_MissingSwapCountsAsZero(t *testing.T) {
	got, err := read(map[string]string{"meminfo": "MemTotal: 1 kB\nMemAvailable: 1 kB\n"}, disk{})
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.HostSample{MemTotal: 1024, MemAvailable: 1024, Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: 1209600 * time.Second}, got)
}

func TestRead_FailsWithoutMeminfo(t *testing.T) {
	_, err := read(map[string]string{"meminfo": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnMeminfoWithoutMemAvailable(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemTotal: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnMeminfoWithoutMemTotal(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemAvailable: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnNonNumericMeminfo(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemTotal: lots kB\nMemAvailable: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWithoutLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnShortLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": "0.42 0.38\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_AcceptsLoadavgWithExactlyThreeFields(t *testing.T) {
	got := testkit.Must(read(map[string]string{"loadavg": "0.1 0.2 0.3"}, disk{}))

	testkit.Equal(t, 0.3, got.Load15)
}

func TestRead_FailsOnNonNumericLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": "0.42 high 0.35 1/2 3\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWithoutUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnEmptyUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": "\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnNonNumericUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": "forever 1\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWhenDiskFails(t *testing.T) {
	_, err := read(nil, disk{err: errDisk})

	testkit.ErrorIs(t, err, errDisk)
}

func TestRead_FailsOnOverlongMeminfoLine(t *testing.T) {
	_, err := read(map[string]string{"meminfo": strings.Repeat("x", 70*1024) + "\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}
