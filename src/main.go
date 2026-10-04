package main

import (
	"log/slog"
	"os"

	"github.com/heronlabs/vps-agent/src/application"
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
	default:
		slog.Error("command not found", "command", command)
		os.Exit(1)
	}
}
