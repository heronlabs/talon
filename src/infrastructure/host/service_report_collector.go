package host

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"syscall"
)

// The files of proc under the root, the fields of meminfo that fill Memory, the separator of a field of meminfo from its value, the unit of its values and the operation of statfs in its error.
const (
	meminfoPath       = "proc/meminfo"
	loadavgPath       = "proc/loadavg"
	uptimePath        = "proc/uptime"
	memTotalField     = "MemTotal"
	memAvailableField = "MemAvailable"
	swapTotalField    = "SwapTotal"
	swapFreeField     = "SwapFree"
	fieldSeparator    = ":"
	kilobytesSuffix   = " kB"
	statfsOp          = "statfs"
)

// ReportCollectorService collects the metrics of the host under its root.
type ReportCollectorService struct {
	fsys fs.FS
	root string
}

// NewReportCollectorService returns a ReportCollectorService that reads proc from fsys, the root of the host, and runs statfs on root, the path of that root.
func NewReportCollectorService(fsys fs.FS, root string) *ReportCollectorService {
	return &ReportCollectorService{fsys: fsys, root: root}
}

// Collect returns the snapshot of the host: the disk of the root first, then the memory, the load and the uptime in proc.
func (s *ReportCollectorService) Collect() (*Snapshot, error) {
	disk, err := s.disk()
	if err != nil {
		return nil, err
	}

	memory, err := s.memory()
	if err != nil {
		return nil, err
	}

	snapshot := Snapshot{Disk: disk, Memory: memory}
	if err := s.scan(loadavgPath, &snapshot.Load.One, &snapshot.Load.Five, &snapshot.Load.Fifteen); err != nil {
		return nil, err
	}

	if err := s.scan(uptimePath, &snapshot.Uptime.Seconds); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// disk returns the total and free bytes of the filesystem of the root, from statfs.
func (s *ReportCollectorService) disk() (Disk, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(s.root, &stat); err != nil {
		return Disk{}, &fs.PathError{Op: statfsOp, Path: s.root, Err: err}
	}

	return newDisk(stat), nil
}

// newDisk returns the total and free bytes of the filesystem of stat; the free are its free blocks, Bfree, the reserve of root included, so the total less the free is the Used column of df.
func newDisk(stat syscall.Statfs_t) Disk {
	return Disk{Total: blockBytes(stat.Blocks, stat), Free: blockBytes(stat.Bfree, stat)}
}

// blockBytes returns the bytes of blocks blocks of the filesystem of stat.
func blockBytes(blocks uint64, stat syscall.Statfs_t) int64 {
	return int64(blocks) * int64(stat.Bsize) //nolint:gosec // G115: a filesystem has fewer than 2^63 blocks
}

// memory returns the memory in proc/meminfo.
func (s *ReportCollectorService) memory() (Memory, error) {
	data, err := fs.ReadFile(s.fsys, meminfoPath)
	if err != nil {
		return Memory{}, err
	}

	return parseMemory(meminfoFields(data))
}

// parseMemory returns the memory of fields, the values of meminfo by name, failing when MemTotal, MemAvailable, SwapTotal or SwapFree is missing or not a number of kB.
func parseMemory(fields map[string]string) (Memory, error) {
	total, totalErr := kilobytes(fields, memTotalField)
	available, availableErr := kilobytes(fields, memAvailableField)
	swapTotal, swapTotalErr := kilobytes(fields, swapTotalField)
	swapFree, swapFreeErr := kilobytes(fields, swapFreeField)
	if err := errors.Join(totalErr, availableErr, swapTotalErr, swapFreeErr); err != nil {
		return Memory{}, err
	}

	return Memory{MemTotal: total, MemAvailable: available, SwapTotal: swapTotal, SwapFree: swapFree}, nil
}

// meminfoFields returns the value of each line of data, the content of meminfo, by the name before its separator, both trimmed.
func meminfoFields(data []byte) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		name, value, _ := strings.Cut(line, fieldSeparator)
		fields[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}

	return fields
}

// kilobytes returns the kB of the field name of fields, failing when it is missing or not a number of kB.
func kilobytes(fields map[string]string, name string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSuffix(fields[name], kilobytesSuffix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", meminfoPath, name, err)
	}

	return value, nil
}

// scan scans the first space-separated values of the file name of the root into targets.
func (s *ReportCollectorService) scan(name string, targets ...any) error {
	data, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		return err
	}

	if _, err := fmt.Sscan(string(data), targets...); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}
