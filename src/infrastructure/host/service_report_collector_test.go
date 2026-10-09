package host

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"testing/fstest"
)

func TestReportCollectorService(t *testing.T) {
	t.Run("Should read MemTotal, MemAvailable, SwapTotal and SwapFree of proc/meminfo in kB", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal:        8040144 kB\nMemFree:          214512 kB\nMemAvailable:    5011332 kB\nSwapTotal:       2097148 kB\nSwapFree:        1048572 kB\nHugePages_Total:       0\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())
		want := Memory{MemTotal: 8040144, MemAvailable: 5011332, SwapTotal: 2097148, SwapFree: 1048572}

		snapshot, err := service.Collect()

		if err != nil || snapshot.Memory != want {
			t.Errorf("Collect() = %+v, %v; want memory %+v", snapshot, err, want)
		}
	})

	t.Run("Should read the load over 1, 5 and 15 minutes of proc/loadavg", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())
		want := Load{One: 0.52, Five: 0.58, Fifteen: 0.59}

		snapshot, err := service.Collect()

		if err != nil || snapshot.Load != want {
			t.Errorf("Collect() = %+v, %v; want load %+v", snapshot, err, want)
		}
	})

	t.Run("Should read the uptime in seconds of proc/uptime", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())
		want := Uptime{Seconds: 12345.67}

		snapshot, err := service.Collect()

		if err != nil || snapshot.Uptime != want {
			t.Errorf("Collect() = %+v, %v; want uptime %+v", snapshot, err, want)
		}
	})

	t.Run("Should read the total bytes of the filesystem of the root", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		root := t.TempDir()
		var stat syscall.Statfs_t
		if err := syscall.Statfs(root, &stat); err != nil {
			t.Fatal(err)
		}
		want := float64(stat.Blocks) * float64(stat.Bsize)
		service := NewReportCollectorService(fsys, root)

		snapshot, err := service.Collect()

		if err != nil || float64(snapshot.Disk.Total) != want {
			t.Errorf("Collect() = %+v, %v; want disk total %v", snapshot, err, want)
		}
	})

	t.Run("Should count the bytes of the blocks and of the free blocks of the filesystem", func(t *testing.T) {
		stat := syscall.Statfs_t{Blocks: 1000, Bfree: 250, Bsize: 4096}
		want := Disk{Total: 4096000, Free: 1024000}

		disk := newDisk(stat)

		if disk != want {
			t.Errorf("newDisk() = %+v, want %+v", disk, want)
		}
	})

	t.Run("Should fail on statfs first when the root does not exist", func(t *testing.T) {
		service := NewReportCollectorService(fstest.MapFS{}, filepath.Join(t.TempDir(), "missing"))
		var pathErr *fs.PathError

		_, err := service.Collect()

		if !errors.As(err, &pathErr) || pathErr.Op != "statfs" || !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Collect() = %v, want statfs %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when proc/meminfo does not exist", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when a field of proc/meminfo is not a number of kB", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: lots kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if err == nil || err.Error() != `proc/meminfo MemAvailable: strconv.ParseInt: parsing "lots": invalid syntax` {
			t.Errorf("Collect() = %v, want %q", err, `proc/meminfo MemAvailable: strconv.ParseInt: parsing "lots": invalid syntax`)
		}
	})

	t.Run("Should fail when a field of proc/meminfo is missing", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if err == nil || err.Error() != `proc/meminfo SwapFree: strconv.ParseInt: parsing "": invalid syntax` {
			t.Errorf("Collect() = %v, want %q", err, `proc/meminfo SwapFree: strconv.ParseInt: parsing "": invalid syntax`)
		}
	})

	t.Run("Should fail when proc/loadavg does not exist", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when proc/loadavg has fewer than three loads", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58\n")},
			"proc/uptime":  {Data: []byte("12345.67 54321.00\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if err == nil || err.Error() != "proc/loadavg: EOF" {
			t.Errorf("Collect() = %v, want %q", err, "proc/loadavg: EOF")
		}
	})

	t.Run("Should fail when proc/uptime does not exist", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when proc/uptime is not a number of seconds", func(t *testing.T) {
		fsys := fstest.MapFS{
			"proc/meminfo": {Data: []byte("MemTotal: 8040144 kB\nMemAvailable: 5011332 kB\nSwapTotal: 2097148 kB\nSwapFree: 1048572 kB\n")},
			"proc/loadavg": {Data: []byte("0.52 0.58 0.59 1/389 12345\n")},
			"proc/uptime":  {Data: []byte("up\n")},
		}
		service := NewReportCollectorService(fsys, t.TempDir())

		_, err := service.Collect()

		if !errors.Is(err, strconv.ErrSyntax) {
			t.Errorf("Collect() = %v, want %v", err, strconv.ErrSyntax)
		}
	})
}
