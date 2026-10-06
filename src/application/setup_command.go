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
	hostKey        ssh.HostKey
}

func NewSetupCommand(reader environment.ConfigReader, writer environment.ConfigWriter, authorizedKeys ssh.AuthorizedKeys, hostKey ssh.HostKey) *SetupCommand {
	return &SetupCommand{reader: reader, writer: writer, authorizedKeys: authorizedKeys, hostKey: hostKey}
}

// Run writes the deploy key and .env, then returns the host's SSH public key,
// "" when the host has none.
func (c *SetupCommand) Run() (string, error) {
	values, err := c.reader.Read()
	if err != nil {
		return "", fmt.Errorf("config: %w", err)
	}

	if err := c.authorizedKeys.Write(values["DEPLOY_SSH_PUBLIC_KEY"]); err != nil {
		return "", fmt.Errorf("deploy: %w", err)
	}

	if err := c.writer.Write(values); err != nil {
		return "", fmt.Errorf("config: %w", err)
	}

	hostKey, err := c.hostKey.Read()
	if err != nil {
		return "", fmt.Errorf("deploy: %w", err)
	}

	return hostKey, nil
}
