package application

type VersionCommand struct {
	version string
}

func NewVersionCommand(version string) *VersionCommand {
	return &VersionCommand{version: version}
}

func (c *VersionCommand) GetVersion() string {
	return c.version
}
