package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/heronlabs/talon/src/application"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/environment"
	"github.com/heronlabs/talon/src/infrastructure/ssh"
	"github.com/heronlabs/talon/src/infrastructure/terminal"
)

// version is set at build time: -ldflags "-X main.version=<version>".
var version = "0.0.0-dev"

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	socketPath := os.Getenv("DOCKER_SOCKET")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	configDir := os.Getenv("TALON_CONFIG_DIR")
	if configDir == "" {
		configDir = "/etc/talon"
	}

	authorizedKeysPath := os.Getenv("TALON_AUTHORIZED_KEYS")
	if authorizedKeysPath == "" {
		authorizedKeysPath = "/root/.ssh/authorized_keys"
	}

	switch command {
	case "version":
		versionCommand := application.NewVersionCommand(version)
		slog.Info("version", "version", versionCommand.GetVersion())
	case "doctor":
		doctorCommand := application.NewDoctorCommand(os.DirFS(configDir), docker.NewHealthCheckService(socketPath, 5*time.Second))
		if err := doctorCommand.Run(); err != nil {
			slog.Error("doctor", "error", err)
			os.Exit(1)
		}
		slog.Info("doctor", "status", "ok")
	case "setup":
		root, err := os.OpenRoot(configDir)
		if err != nil {
			slog.Error("setup", "error", err)
			os.Exit(1)
		}

		setupCommand := application.NewSetupCommand(
			environment.NewConfigReaderService(os.DirFS(configDir), environment.Keys(), os.Getenv, terminal.NewPromptService(os.Stdin, os.Stdout)),
			environment.NewConfigWriterService(root, environment.Keys()),
			ssh.NewAuthorizedKeysService(authorizedKeysPath),
		)
		if err := setupCommand.Run(); err != nil {
			slog.Error("setup", "error", err)
			os.Exit(1)
		}

		slog.Info("setup", "status", "ok")
	default:
		slog.Error("command not found", "command", command)
		os.Exit(1)
	}
}
