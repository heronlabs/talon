package application

import (
	"fmt"
	"io/fs"
)

type DoctorCommand struct {
	fsys fs.FS
}

func NewDoctorCommand(fsys fs.FS) *DoctorCommand {
	return &DoctorCommand{fsys: fsys}
}

func (c *DoctorCommand) Run() error {
	info, err := fs.Stat(c.fsys, ".env")
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if perm := info.Mode().Perm(); perm != 0o600 {
		return fmt.Errorf("config: .env has mode %o, want 600", perm)
	}

	return nil
}
