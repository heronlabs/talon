package application

import (
	"io/fs"
	"strings"
)

type VersionCommand struct {
	fsys fs.FS
}

func NewVersionCommand(fsys fs.FS) *VersionCommand {
	return &VersionCommand{fsys: fsys}
}

func (c *VersionCommand) GetTxtVersion() string {
	version, err := fs.ReadFile(c.fsys, "version.txt")

	if err != nil {
		return "0.0.0"
	}

	return strings.TrimSpace(string(version))
}
