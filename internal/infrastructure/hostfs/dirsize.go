package hostfs

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// DirSizer sums the sizes of the regular files under a directory, without following links.
type DirSizer struct{}

// DirSize implements port.DirSizer. A root that is a regular file (a single-file bind mount) is
// measured as is; a file that disappears during the walk counts as zero.
func (DirSizer) DirSize(ctx context.Context, root string) (domain.Bytes, error) {
	if info, err := os.Lstat(root); err == nil && info.Mode().IsRegular() {
		return domain.BytesOf(info.Size()), nil
	}
	var total domain.Bytes
	err := fs.WalkDir(os.DirFS(root), ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += domain.BytesOf(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("dir size %s: %w", root, err)
	}
	return total, nil
}
