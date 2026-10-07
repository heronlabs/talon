package application

import (
	"fmt"

	"github.com/heronlabs/talon/src/infrastructure/environment"
	"github.com/heronlabs/talon/src/infrastructure/ssh"
)

type SetupCommand struct {
	reader         environment.ConfigReader
	writer         environment.ConfigWriter
	authorizedKeys ssh.AuthorizedKeys
}

func NewSetupCommand(reader environment.ConfigReader, writer environment.ConfigWriter, authorizedKeys ssh.AuthorizedKeys) *SetupCommand {
	return &SetupCommand{reader: reader, writer: writer, authorizedKeys: authorizedKeys}
}

func (c *SetupCommand) Run() error {
	values, err := c.reader.Read()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := c.authorizedKeys.Write(values["DEPLOY_SSH_PUBLIC_KEY"]); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	if err := c.writer.Write(values); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	return nil
}
