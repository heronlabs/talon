// Package deploy turns the request of the Deploy workflow, received over SSH, into an update.
package deploy

import (
	"context"
	"errors"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrBadRequest marks a request that is not exactly a version like 1.2.3.
var ErrBadRequest = errors.New("expected a version like 1.2.3")

// Updater installs one version; *update.UseCase is one.
type Updater interface {
	Run(ctx context.Context, v domain.Version) error
}

// UseCase validates deploy requests and hands them to the updater.
type UseCase struct{ updater Updater }

// New builds the deploy use case.
func New(updater Updater) *UseCase { return &UseCase{updater: updater} }

// Run installs the version named by request, which must be exactly MAJOR.MINOR.PATCH.
func (u *UseCase) Run(ctx context.Context, request string) error {
	v, err := domain.ParseVersion(request)
	if err != nil {
		return ErrBadRequest
	}
	return u.updater.Run(ctx, v)
}
