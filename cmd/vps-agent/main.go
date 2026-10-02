// Command vps-agent measures an Easypanel VPS and, in later phases, reconciles it from the CMS.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/usecase/doctor"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/infrastructure/clock"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
)

var version = "0.0.0-dev"

const (
	defaultConfigPath = "/etc/vps-agent/config.env"
	dockerSocket      = "/var/run/docker.sock"
	dockerTimeout     = 30 * time.Second
	statsTimeout      = 5 * time.Second
	usage             = "usage: vps-agent <version | report --once | doctor> [--config path]"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "report":
		return runReport(ctx, args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}

func configFile(path string) config.File {
	return config.File{Dir: os.DirFS(filepath.Dir(path)), Name: filepath.Base(path)}
}

func runReport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	once := flags.Bool("once", false, "collect one report, print it and exit")
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*once {
		fmt.Fprintln(stderr, "report: only --once is available in this version")
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	host, err := configFile(*configPath).HostNameOr(os.Hostname)
	if err != nil {
		logger.Error("host name unavailable", slog.String("logId", "report.host-name-failed"), slog.String("errorMessage", err.Error()))
		return 1
	}
	uc := report.New(report.Deps{
		Host:         host,
		AgentVersion: version,
		Clock:        clock.System{},
		Metrics:      hostfs.NewMetrics(os.DirFS("/proc"), hostfs.StatFS{}),
		Docker:       docker.NewUnix(dockerSocket, dockerTimeout, statsTimeout, logger),
		Dirs:         hostfs.DirSizer{},
		Sink:         cms.NewPrinter(stdout),
		Logger:       logger,
	})
	if err := uc.Run(ctx); err != nil {
		logger.Error("report failed", slog.String("logId", "report.failed"), slog.String("errorMessage", err.Error()))
		return 1
	}
	return 0
}

func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	results := doctor.Run(ctx,
		doctor.ConfigCheck(configFile(*configPath)),
		doctor.DockerCheck(docker.NewUnix(dockerSocket, dockerTimeout, statsTimeout, slog.New(slog.NewJSONHandler(stderr, nil)))),
	)
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(stdout, "FAIL %s: %v\n", r.Check, r.Err)
			continue
		}
		fmt.Fprintf(stdout, "ok   %s\n", r.Check)
	}
	if !doctor.Healthy(results) {
		return 1
	}
	return 0
}
