package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/heronlabs/talon/src/application"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/environment"
	"github.com/heronlabs/talon/src/infrastructure/terminal"
)

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	socketPath := os.Getenv("DOCKER_SOCKET")
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	switch command {
	case "version":
		versionCommand := application.NewVersionCommand(os.DirFS("."))
		slog.Info("version", "version", versionCommand.GetTxtVersion())
	case "doctor":
		doctorCommand := application.NewDoctorCommand(os.DirFS("."), docker.NewHealthCheckService(socketPath, 5*time.Second))
		if err := doctorCommand.Run(); err != nil {
			slog.Error("doctor", "error", err)
			os.Exit(1)
		}
		slog.Info("doctor", "status", "ok")
	case "setup":
		root, err := os.OpenRoot(".")
		if err != nil {
			slog.Error("setup", "error", err)
			os.Exit(1)
		}

		setupCommand := application.NewSetupCommand(
			environment.NewConfigReaderService(os.DirFS("."), environment.Keys(), os.Getenv, terminal.NewPromptService(os.Stdin, os.Stdout)),
			environment.NewConfigWriterService(root, environment.Keys()),
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
