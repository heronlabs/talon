package application

import (
	"fmt"

	"github.com/heronlabs/talon/src/infrastructure/environment"
)

type SetupCommand struct {
	reader environment.ConfigReader
	writer environment.ConfigWriter
}

func NewSetupCommand(reader environment.ConfigReader, writer environment.ConfigWriter) *SetupCommand {
	return &SetupCommand{reader: reader, writer: writer}
}

func (c *SetupCommand) Run() error {
	values, err := c.reader.Read()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := c.writer.Write(values); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return nil
}
