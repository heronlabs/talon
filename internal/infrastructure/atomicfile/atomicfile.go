// Package atomicfile replaces files so that readers see either the old or the new contents.
package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Write replaces dir/name with data. It writes a temporary file in dir (its name has a dot, which
// sudoers.d ignores), gives it perm, syncs it, lets check inspect it and renames it over dir/name.
// When any step fails the temporary file is removed and dir/name is left as it was.
func Write(dir, name string, data []byte, perm fs.FileMode, check func(tmpPath string) error) error {
	f, err := os.CreateTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", name, err)
	}
	tmp := f.Name()
	_, err = f.Write(data)
	err = errors.Join(err, f.Chmod(perm), f.Sync(), f.Close())
	if err == nil {
		err = check(tmp)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, name))
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// NoCheck accepts every temporary file.
func NoCheck(string) error { return nil }
