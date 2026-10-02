// Package install puts the running binary in place, merges the configuration file and prepares
// the deploy user.
package install

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/port"
)

// Deps are the collaborators of the install use case.
type Deps struct {
	Version    string
	BinaryPath string
	Executable port.Executable
	Binaries   port.BinaryStore
	Config     port.ConfigStore
	Syntax     port.ConfigSyntax
	// Values are the known configuration keys as found in the environment; empty ones are skipped.
	Values map[string]string
	// DeployKey is DEPLOY_SSH_PUBLIC_KEY; empty skips the deploy user. It is never written to the config.
	DeployKey string
	Accounts  port.SystemAccounts
	Out       io.Writer
	Logger    *slog.Logger
}

// UseCase installs the agent.
type UseCase struct{ deps Deps }

// New builds the install use case.
func New(deps Deps) *UseCase { return &UseCase{deps: deps} }

// Run installs the binary, merges the config and, with a deploy key, prepares the deploy user.
func (u *UseCase) Run(ctx context.Context) error {
	err := u.run(ctx)
	if err != nil {
		u.deps.Logger.ErrorContext(ctx, "install failed",
			slog.String("logId", "install.failed"),
			slog.String("version", u.deps.Version),
			slog.String("errorName", fmt.Sprintf("%T", err)),
			slog.String("errorMessage", err.Error()))
	}
	return err
}

func (u *UseCase) run(ctx context.Context) error {
	deployLine, err := u.deployLine()
	if err != nil {
		return err
	}
	if err := u.installBinary(); err != nil {
		return err
	}
	added, err := u.mergeConfig()
	if err != nil {
		return err
	}
	fmt.Fprintf(u.deps.Out, "install: vps-agent %s at %s\n", u.deps.Version, u.deps.BinaryPath)
	if len(added) > 0 {
		fmt.Fprintf(u.deps.Out, "install: added to config.env: %s\n", strings.Join(added, ", "))
	}
	if deployLine != "" {
		if err := u.prepareDeployUser(ctx, deployLine); err != nil {
			return err
		}
	}
	u.deps.Logger.InfoContext(ctx, "install completed",
		slog.String("logId", "install.completed"),
		slog.String("version", u.deps.Version),
		slog.String("addedKeys", strings.Join(added, ",")),
		slog.Bool("deployUser", deployLine != ""))
	return nil
}

// deployLine validates the deploy key before anything is written.
func (u *UseCase) deployLine() (string, error) {
	if u.deps.DeployKey == "" {
		return "", nil
	}
	return domain.AuthorizedKeyLine(u.deps.DeployKey)
}

func (u *UseCase) installBinary() error {
	binary, err := u.deps.Executable.Read()
	if err != nil {
		return fmt.Errorf("read running binary: %w", err)
	}
	if err := u.deps.Binaries.Stage(binary); err != nil {
		return fmt.Errorf("stage binary: %w", err)
	}
	if err := u.deps.Binaries.Swap(); err != nil {
		return fmt.Errorf("swap binary: %w", err)
	}
	return nil
}

// mergeConfig adds the missing keys; a config that would not parse is not written.
func (u *UseCase) mergeConfig() ([]string, error) {
	existing, err := u.deps.Config.Read()
	missing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !missing {
		return nil, fmt.Errorf("read config: %w", err)
	}
	merged, added := domain.MergeConfig(existing, u.deps.Values)
	if err := u.deps.Syntax.CheckSyntax(merged); err != nil {
		return nil, fmt.Errorf("config.env would be invalid, nothing written: %w", err)
	}
	if missing || len(added) > 0 {
		if err := u.deps.Config.Write(merged); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
	}
	return added, nil
}

func (u *UseCase) prepareDeployUser(ctx context.Context, line string) error {
	if err := u.deps.Accounts.EnsureDeployUser(ctx); err != nil {
		return fmt.Errorf("deploy user: %w", err)
	}
	if err := u.deps.Accounts.WriteAuthorizedKey(line); err != nil {
		return fmt.Errorf("deploy authorized_keys: %w", err)
	}
	if err := u.deps.Accounts.WriteSudoers(ctx); err != nil {
		return fmt.Errorf("deploy sudoers: %w", err)
	}
	fmt.Fprintf(u.deps.Out, "install: deploy user %s ready\n", domain.DeployUser)
	return nil
}
