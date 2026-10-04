package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/heronlabs/talon/src/application"
	"github.com/heronlabs/talon/src/infrastructure/docker"
)

func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "version":
		versionCommand := application.NewVersionCommand(os.DirFS("."))
		slog.Info("version", "version", versionCommand.GetTxtVersion())
	case "doctor":
		socketPath := os.Getenv("DOCKER_SOCKET")
		if socketPath == "" {
			socketPath = "/var/run/docker.sock"
		}

		doctorCommand := application.NewDoctorCommand(os.DirFS("."), docker.NewHealthCheckService(socketPath, 5*time.Second))
		if err := doctorCommand.Run(); err != nil {
			slog.Error("doctor", "error", err)
			os.Exit(1)
		}

		slog.Info("doctor", "status", "ok")
	// TODO: wire the install command case here.
	default:
		slog.Error("command not found", "command", command)
		os.Exit(1)
	}
}
