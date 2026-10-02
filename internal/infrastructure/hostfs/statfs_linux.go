//go:build linux

package hostfs

import (
	"fmt"
	"syscall"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// StatFS measures a filesystem with statfs(2).
type StatFS struct{}

// Stat implements DiskStater.
func (StatFS) Stat(path string) (total, used domain.Bytes, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	block := domain.Bytes(st.Bsize)
	return domain.Bytes(st.Blocks) * block, domain.Bytes(st.Blocks-st.Bfree) * block, nil
}
