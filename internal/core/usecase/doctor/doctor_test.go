package doctor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/heronlabs/vps-agent/internal/core/usecase/doctor"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var errBoom = errors.New("boom")

func TestRun_RunsEveryCheckInOrderWhenAllPass(t *testing.T) {
	got := doctor.Run(context.Background(),
		doctor.ConfigCheck(fake.ConfigValidator{}),
		doctor.DockerCheck(fake.Docker{}))

	testkit.Equal(t, []doctor.Result{{Check: "config"}, {Check: "docker"}}, got, cmpopts.EquateErrors())
}

func TestRun_StopsAtTheFirstFailure(t *testing.T) {
	got := doctor.Run(context.Background(),
		doctor.ConfigCheck(fake.ConfigValidator{Err: errBoom}),
		doctor.DockerCheck(fake.Docker{}))

	testkit.Equal(t, []doctor.Result{{Check: "config", Err: errBoom}}, got, cmpopts.EquateErrors())
}

func TestDockerCheck_ReportsPingFailure(t *testing.T) {
	got := doctor.Run(context.Background(), doctor.DockerCheck(fake.Docker{PingErr: errBoom}))

	testkit.Equal(t, []doctor.Result{{Check: "docker", Err: errBoom}}, got, cmpopts.EquateErrors())
}

func TestHealthy_TrueWhenEveryResultPassed(t *testing.T) {
	got := doctor.Healthy([]doctor.Result{{Check: "config"}, {Check: "docker"}})

	testkit.Equal(t, true, got)
}

func TestHealthy_FalseWhenAnyResultFailed(t *testing.T) {
	got := doctor.Healthy([]doctor.Result{{Check: "config"}, {Check: "docker", Err: errBoom}})

	testkit.Equal(t, false, got)
}
