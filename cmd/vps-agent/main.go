// Command vps-agent measures an Easypanel VPS and, in later phases, reconciles it from the CMS.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/deploy"
	"github.com/heronlabs/vps-agent/internal/core/usecase/doctor"
	"github.com/heronlabs/vps-agent/internal/core/usecase/install"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/core/usecase/update"
	"github.com/heronlabs/vps-agent/internal/infrastructure/clock"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/infrastructure/github"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/infrastructure/process"
	"github.com/heronlabs/vps-agent/internal/infrastructure/release"
)

var version = "0.0.0-dev"

const (
	defaultConfigPath = "/etc/vps-agent/config.env"
	dockerSocket      = "/var/run/docker.sock"
	dockerTimeout     = 30 * time.Second
	statsTimeout      = 5 * time.Second
	binDir            = "/usr/local/bin"
	binName           = "vps-agent"
	updateLockPath    = "/run/vps-agent-update.lock"
	releasesURL       = "https://api.github.com/repos/heronlabs/vps-agent"
	maxAssetBytes     = 100 << 20
	downloadTimeout   = 2 * time.Minute
	doctorTimeout     = 2 * time.Minute
	deployHome        = "/var/lib/vps-agent-deploy"
	deploySudoers     = "/etc/sudoers.d/vps-agent-deploy"
	usage             = "usage: vps-agent <version | report --once | doctor | install | update [--version X] | deploy> [--config path]"
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
	case "install":
		return runInstall(ctx, args[1:], stdout, stderr)
	case "update":
		return runUpdate(ctx, args[1:], stdout, stderr)
	case "deploy":
		return runDeploy(ctx, args[1:], stdout, stderr)
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

// requireRoot tells whether the process runs as root, and says so on stderr when it does not.
func requireRoot(command string, stderr io.Writer) bool {
	if os.Geteuid() != 0 {
		fmt.Fprintf(stderr, "%s: must run as root\n", command)
		return false
	}
	return true
}

func jsonLogger(stderr io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(stderr, nil))
}

func runInstall(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if !requireRoot("install", stderr) {
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "install: %v\n", err)
		return 1
	}
	values := map[string]string{}
	for _, key := range config.Keys() {
		values[key] = os.Getenv(key)
	}
	uc := install.New(install.Deps{
		Version:    version,
		BinaryPath: filepath.Join(binDir, binName),
		Executable: hostfs.Executable{Path: exe},
		Binaries:   hostfs.Binaries{Dir: binDir, Name: binName},
		Config:     hostfs.ConfigFile{Dir: filepath.Dir(*configPath), Name: filepath.Base(*configPath)},
		Syntax:     config.Syntax{},
		Values:     values,
		DeployKey:  os.Getenv("DEPLOY_SSH_PUBLIC_KEY"),
		Accounts: process.Accounts{
			User:        domain.DeployUser,
			Home:        deployHome,
			SudoersPath: deploySudoers,
			Run:         process.Exec,
			Lookup:      process.LookupUser,
		},
		Out:    stdout,
		Logger: jsonLogger(stderr),
	})
	if err := uc.Run(ctx); err != nil {
		fmt.Fprintf(stderr, "install: %v\n", err)
		return 1
	}
	return runDoctor(ctx, []string{"--config", *configPath}, stdout, stderr)
}

func updateUseCase(configPath string, stdout, stderr io.Writer) (*update.UseCase, error) {
	key, err := release.PublicKey()
	if err != nil {
		return nil, err
	}
	return update.New(update.Deps{
		CurrentVersion: version,
		Arch:           runtime.GOARCH,
		PublicKey:      key,
		Lock:           hostfs.Lock{Path: updateLockPath},
		Releases:       github.New(&http.Client{Timeout: downloadTimeout}, releasesURL, configFile(configPath), maxAssetBytes),
		Binaries:       hostfs.Binaries{Dir: binDir, Name: binName},
		Health:         process.Doctor{Binary: filepath.Join(binDir, binName), ConfigPath: configPath, Timeout: doctorTimeout},
		Out:            stdout,
		Logger:         jsonLogger(stderr),
	}), nil
}

func runUpdate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("version", "", "version to install, like 1.2.3 (default: the latest release)")
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	var v domain.Version
	if *target != "" {
		parsed, err := domain.ParseVersion(*target)
		if err != nil {
			fmt.Fprintln(stderr, "update: --version expects a version like 1.2.3")
			return 2
		}
		v = parsed
	}
	if !requireRoot("update", stderr) {
		return 1
	}
	uc, err := updateUseCase(*configPath, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "update: %v\n", err)
		return 1
	}
	if *target == "" {
		err = uc.RunLatest(ctx)
	} else {
		err = uc.Run(ctx, v)
	}
	if err != nil {
		fmt.Fprintf(stderr, "update: %v\n", err)
		return 1
	}
	return 0
}

// runDeploy is the forced command of the deploy user's SSH key: the request arrives in
// SSH_ORIGINAL_COMMAND and must be a bare version.
func runDeploy(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	if !requireRoot("deploy", stderr) {
		return 1
	}
	uc, err := updateUseCase(defaultConfigPath, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "deploy: %v\n", err)
		return 1
	}
	err = deploy.New(uc).Run(ctx, os.Getenv("SSH_ORIGINAL_COMMAND"))
	if errors.Is(err, deploy.ErrBadRequest) {
		fmt.Fprintf(stderr, "deploy: %v\n", err)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "deploy: %v\n", err)
		return 1
	}
	return 0
}
