package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/deploy"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

func TestRun_UpdatesToTheRequestedVersion(t *testing.T) {
	updater := &fake.Updater{}

	testkit.MustDo(t, deploy.New(updater).Run(context.Background(), "1.2.3"))

	testkit.Equal(t, []domain.Version{{Major: 1, Minor: 2, Patch: 3}}, updater.Versions)
}

func TestRun_ReturnsTheUpdateError(t *testing.T) {
	errBoom := errors.New("boom")

	err := deploy.New(&fake.Updater{Err: errBoom}).Run(context.Background(), "1.2.3")

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_RejectsARequestWithASpace(t *testing.T) {
	err := deploy.New(&fake.Updater{}).Run(context.Background(), "1.2.3 4.5.6")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestRun_RejectsARequestWithASemicolon(t *testing.T) {
	err := deploy.New(&fake.Updater{}).Run(context.Background(), "1.2.3;id")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestRun_RejectsALeadingV(t *testing.T) {
	err := deploy.New(&fake.Updater{}).Run(context.Background(), "v1.2.3")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestRun_RejectsAnEmptyRequest(t *testing.T) {
	err := deploy.New(&fake.Updater{}).Run(context.Background(), "")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestRun_DoesNotUpdateOnABadRequest(t *testing.T) {
	updater := &fake.Updater{}

	_ = deploy.New(updater).Run(context.Background(), "")

	testkit.Equal(t, []domain.Version(nil), updater.Versions)
}
