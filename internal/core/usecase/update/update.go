// Package update installs a signed release of the agent over the running one and rolls back
// when the new binary's doctor fails.
package update

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/port"
)

// ErrRolledBack marks an update whose new binary failed its doctor and was replaced by the previous one.
var ErrRolledBack = errors.New("rolled back")

// Deps are the collaborators of the update use case.
type Deps struct {
	// CurrentVersion is the running agent's version; one that does not parse (a dev build)
	// counts as older than every release.
	CurrentVersion string
	Arch           string
	PublicKey      ed25519.PublicKey
	Lock           port.UpdateLock
	Releases       port.ReleaseSource
	Binaries       port.BinaryStore
	Health         port.HealthCheck
	Out            io.Writer
	Logger         *slog.Logger
}

// UseCase installs releases.
type UseCase struct{ deps Deps }

// New builds the update use case.
func New(deps Deps) *UseCase { return &UseCase{deps: deps} }

// Run installs version v.
func (u *UseCase) Run(ctx context.Context, v domain.Version) error {
	return u.run(ctx, v.String())
}

// RunLatest installs the latest release.
func (u *UseCase) RunLatest(ctx context.Context) error {
	return u.run(ctx, "")
}

func (u *UseCase) run(ctx context.Context, request string) error {
	err := u.locked(ctx, request)
	if err != nil {
		u.deps.Logger.ErrorContext(ctx, "update failed",
			slog.String("logId", "update.failed"),
			slog.String("fromVersion", u.deps.CurrentVersion),
			slog.String("errorName", fmt.Sprintf("%T", err)),
			slog.String("errorMessage", err.Error()))
	}
	return err
}

func (u *UseCase) locked(ctx context.Context, request string) error {
	release, err := u.deps.Lock.Acquire()
	if err != nil {
		return err
	}
	defer release()
	u.deps.Logger.InfoContext(ctx, "update started",
		slog.String("logId", "update.started"),
		slog.String("fromVersion", u.deps.CurrentVersion))
	rel, err := u.deps.Releases.Release(ctx, request)
	if err != nil {
		return fmt.Errorf("read release: %w", err)
	}
	target, err := domain.ParseTag(rel.Tag)
	if err != nil {
		return err
	}
	current, _ := domain.ParseVersion(u.deps.CurrentVersion)
	decision, err := domain.UpdatePlan(current, target)
	if err != nil {
		return err
	}
	if decision == domain.DecisionAlreadyCurrent {
		fmt.Fprintf(u.deps.Out, "update: already at %s\n", target)
		return nil
	}
	return u.install(ctx, rel, target)
}

func (u *UseCase) install(ctx context.Context, rel domain.Release, target domain.Version) error {
	binaryName := domain.BinaryAssetName(target, u.deps.Arch)
	blobs, err := u.download(ctx, rel, domain.ChecksumsAsset, domain.SignatureAsset, binaryName)
	if err != nil {
		return err
	}
	if err := domain.VerifyRelease(u.deps.PublicKey, blobs[0], blobs[1], binaryName, blobs[2]); err != nil {
		return err
	}
	if err := u.deps.Binaries.Stage(blobs[2]); err != nil {
		return fmt.Errorf("stage binary: %w", err)
	}
	if err := u.deps.Binaries.Swap(); err != nil {
		return fmt.Errorf("swap binary: %w", err)
	}
	output, err := u.deps.Health.Check(ctx)
	if err != nil {
		return u.rollBack(ctx, target, output, err)
	}
	fmt.Fprintf(u.deps.Out, "update: %s -> %s\n", u.deps.CurrentVersion, target)
	u.deps.Logger.InfoContext(ctx, "update installed",
		slog.String("logId", "update.installed"),
		slog.String("fromVersion", u.deps.CurrentVersion),
		slog.String("toVersion", target.String()))
	return nil
}

// download fetches the named assets of rel, in order.
func (u *UseCase) download(ctx context.Context, rel domain.Release, names ...string) ([][]byte, error) {
	blobs := make([][]byte, 0, len(names))
	for _, name := range names {
		asset, ok := rel.Asset(name)
		if !ok {
			return nil, domain.IncompleteRelease(rel.Tag)
		}
		blob, err := u.deps.Releases.Download(ctx, asset)
		if err != nil {
			return nil, fmt.Errorf("download %s: %w", name, err)
		}
		blobs = append(blobs, blob)
	}
	return blobs, nil
}

func (u *UseCase) rollBack(ctx context.Context, target domain.Version, output string, doctorErr error) error {
	if err := u.deps.Binaries.Restore(); err != nil {
		return fmt.Errorf("doctor of %s failed (%w) and restoring %s failed: %w", target, doctorErr, u.deps.CurrentVersion, err)
	}
	fmt.Fprintf(u.deps.Out, "update: rolled back to %s: doctor failed\n%s", u.deps.CurrentVersion, output)
	u.deps.Logger.WarnContext(ctx, "update rolled back",
		slog.String("logId", "update.rolled-back"),
		slog.String("fromVersion", u.deps.CurrentVersion),
		slog.String("toVersion", target.String()),
		slog.String("errorName", fmt.Sprintf("%T", doctorErr)),
		slog.String("errorMessage", doctorErr.Error()))
	return fmt.Errorf("%w to %s: doctor of %s failed: %w", ErrRolledBack, u.deps.CurrentVersion, target, doctorErr)
}
