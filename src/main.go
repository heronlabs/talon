package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/heronlabs/talon/src/application"
	"github.com/heronlabs/talon/src/core"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/environment"
	"github.com/heronlabs/talon/src/infrastructure/terminal"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

// defaultDockerSocket and defaultConfigDir are the Docker socket and the configuration directory when DOCKER_SOCKET and TALON_CONFIG_DIR are empty.
const (
	defaultDockerSocket = "/var/run/docker.sock"
	defaultConfigDir    = "/etc/talon"
)

// version is the version of talon, set at build time by -ldflags "-X main.version=<version>".
var version = "0.0.0-dev"

// main runs the command named by the first argument and exits with its code.
func main() {
	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	dockerSocketPath := envOrDefault("DOCKER_SOCKET", defaultDockerSocket)

	configDir := envOrDefault("TALON_CONFIG_DIR", defaultConfigDir)

	logger := slog.Default()

	osDirFs := os.DirFS(configDir)

	terminalPromptService := terminal.NewPromptService(os.Stdin, os.Stdout)

	dockerClient := docker.NewSocketClient(dockerSocketPath, 5*time.Second)
	dockerHealthCheckService := docker.NewHealthCheckService(dockerClient)
	dockerReportCollectorService := docker.NewReportCollectorService(dockerClient)

	environmentKeys := environment.Keys()
	environmentConfigReaderService := environment.NewConfigReaderService(osDirFs, environmentKeys, os.Getenv, terminalPromptService)
	environmentConfigWriterService := environment.NewConfigWriterService(configDir, environmentKeys)

	webhookHTTPClient := webhook.NewHTTPClient(15 * time.Second)
	webhookReportSenderService := webhook.NewReportSenderService(webhookHTTPClient)

	coreDoctorService := core.NewDoctorService(osDirFs, dockerHealthCheckService)
	coreSetupService := core.NewSetupService(environmentConfigReaderService, environmentConfigWriterService)
	coreReportService := core.NewReportService(dockerReportCollectorService, webhookReportSenderService, environmentConfigReaderService, version, os.Hostname, time.Now)

	versionCommand := application.NewVersionCommand(version, logger)
	doctorCommand := application.NewDoctorCommand(coreDoctorService, logger)
	setupCommand := application.NewSetupCommand(coreSetupService, logger)
	reportCommand := application.NewReportCommand(coreReportService, logger)

	switch command {
	case "version":
		os.Exit(versionCommand.Run())

	case "doctor":
		os.Exit(doctorCommand.Run())

	case "setup":
		os.Exit(setupCommand.Run())

	case "report":
		os.Exit(reportCommand.Run())

	default:
		logger.Error("command not found", "command", command)
		os.Exit(1)
	}
}

// envOrDefault returns the environment variable name or, when it is empty, fallback.
func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
