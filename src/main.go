package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heronlabs/talon/src/application"
	"github.com/heronlabs/talon/src/core"
	"github.com/heronlabs/talon/src/infrastructure/docker"
	"github.com/heronlabs/talon/src/infrastructure/easypanel"
	"github.com/heronlabs/talon/src/infrastructure/environment"
	"github.com/heronlabs/talon/src/infrastructure/host"
	"github.com/heronlabs/talon/src/infrastructure/manifest"
	"github.com/heronlabs/talon/src/infrastructure/terminal"
	"github.com/heronlabs/talon/src/infrastructure/webhook"
)

// defaultDockerSocket, defaultConfigDir and defaultHostRoot are the Docker socket, the configuration directory and the root of the host when DOCKER_SOCKET, TALON_CONFIG_DIR and TALON_HOST_ROOT are empty.
const (
	defaultDockerSocket = "/var/run/docker.sock"
	defaultConfigDir    = "/etc/talon"
	defaultHostRoot     = "/"
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

	hostRoot := envOrDefault("TALON_HOST_ROOT", defaultHostRoot)

	logger := slog.Default()

	osDirFs := os.DirFS(configDir)
	hostRootFs := os.DirFS(hostRoot)

	terminalEchoClient := terminal.NewEchoClient(os.Stdin.Fd())
	terminalPromptService := terminal.NewPromptService(os.Stdin, os.Stdout, terminalEchoClient)

	dockerClient := docker.NewSocketClient(dockerSocketPath, 5*time.Second)
	dockerHealthCheckService := docker.NewHealthCheckService(dockerClient)
	dockerReportCollectorService := docker.NewReportCollectorService(dockerClient)

	hostReportCollectorService := host.NewReportCollectorService(hostRootFs, hostRoot)
	easypanelReportCollectorService := easypanel.NewReportCollectorService(hostRootFs)

	environmentKeys := environment.Keys()
	environmentConfigReaderService := environment.NewConfigReaderService(osDirFs, environmentKeys, os.Getenv, terminalPromptService)
	environmentConfigWriterService := environment.NewConfigWriterService(configDir, environmentKeys)

	httpClient := webhook.NewHTTPClient(15 * time.Second)
	webhookReportSenderService := webhook.NewReportSenderService(httpClient)
	manifestReaderService := manifest.NewReaderService(httpClient)

	easypanelAPIClient := easypanel.NewAPIClient(httpClient)
	easypanelConfigCollectorService := easypanel.NewConfigCollectorService(easypanelAPIClient)
	easypanelHealthCheckService := easypanel.NewHealthCheckService(easypanelAPIClient)

	coreDoctorService := core.NewDoctorService(osDirFs, dockerHealthCheckService, environmentConfigReaderService, easypanelHealthCheckService)
	coreSetupService := core.NewSetupService(environmentConfigReaderService, environmentConfigWriterService)
	coreReportService := core.NewReportService(dockerReportCollectorService, hostReportCollectorService, easypanelReportCollectorService, manifestReaderService, easypanelConfigCollectorService, webhookReportSenderService, environmentConfigReaderService, version, os.Hostname, time.Now)
	coreScheduleService := core.NewScheduleService(environmentConfigReaderService)

	versionCommand := application.NewVersionCommand(version, logger)
	doctorCommand := application.NewDoctorCommand(coreDoctorService, logger)
	setupCommand := application.NewSetupCommand(coreSetupService, logger)
	reportCommand := application.NewReportCommand(coreReportService, logger)
	daemonCommand := application.NewDaemonCommand(coreScheduleService, reportCommand, time.Tick, logger)

	switch command {
	case "version":
		os.Exit(versionCommand.Run())

	case "doctor":
		os.Exit(doctorCommand.Run(signalContext()))

	case "setup":
		os.Exit(setupCommand.Run())

	case "report":
		os.Exit(reportCommand.Run(signalContext()))

	case "daemon":
		os.Exit(daemonCommand.Run(signalContext()))

	default:
		logger.Error("command not found", "command", command)
		os.Exit(1)
	}
}

// signalContext returns a context that is done on SIGTERM or SIGINT; only the commands that take it catch those signals, and the process exits after the command without releasing them.
func signalContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)

	return ctx
}

// envOrDefault returns the environment variable name or, when it is empty, fallback.
func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
