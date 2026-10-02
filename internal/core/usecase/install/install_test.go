package install_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/install"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var errBoom = errors.New("boom")

const (
	deployKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMJO6hd9vIfa3qb8Vilon7lkFnaxu6wTEdEZpWfxv+vn ci"
	deployLine = `restrict,command="sudo /usr/local/bin/vps-agent deploy" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMJO6hd9vIfa3qb8Vilon7lkFnaxu6wTEdEZpWfxv+vn vps-agent-deploy`
)

type world struct {
	calls    *fake.Calls
	binaries *fake.BinaryStore
	config   *fake.ConfigStore
	accounts *fake.SystemAccounts
	out      *bytes.Buffer
	logs     *bytes.Buffer
	deps     install.Deps
}

func newWorld() *world {
	calls := &fake.Calls{}
	w := &world{
		calls:    calls,
		binaries: &fake.BinaryStore{Calls: calls},
		config:   &fake.ConfigStore{Data: []byte("AGENT_HOST_NAME=srv\n")},
		accounts: &fake.SystemAccounts{Calls: calls},
		out:      &bytes.Buffer{},
		logs:     &bytes.Buffer{},
	}
	w.deps = install.Deps{
		Version:    "0.3.0",
		BinaryPath: "/usr/local/bin/vps-agent",
		Executable: fake.Executable{Data: []byte("me")},
		Binaries:   w.binaries,
		Config:     w.config,
		Syntax:     fake.ConfigSyntax{},
		Values:     map[string]string{"AGENT_HOST_NAME": "other", "GITHUB_TOKEN": "secret-token", "CMS_URL": ""},
		Accounts:   w.accounts,
		Out:        w.out,
		Logger: slog.New(slog.NewJSONHandler(w.logs, &slog.HandlerOptions{
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		})),
	}
	return w
}

func (w *world) run() error {
	return install.New(w.deps).Run(context.Background())
}

func withDeployKey(w *world) *world {
	w.deps.DeployKey = deployKey
	return w
}

func complete(w *world) *world {
	w.config.Data = []byte("AGENT_HOST_NAME=srv\nGITHUB_TOKEN=old\n")
	return w
}

func TestRun_StagesTheRunningBinary(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run())

	testkit.Equal(t, []byte("me"), w.binaries.Staged)
}

func TestRun_SwapsInTheBinaryWithoutADeployUser(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run())

	testkit.Equal(t, []string{"stage", "swap"}, w.calls.Names)
}

func TestRun_PreparesTheDeployUserAfterTheBinary(t *testing.T) {
	w := withDeployKey(newWorld())

	testkit.MustDo(t, w.run())

	testkit.Equal(t, []string{"stage", "swap", "user", "authorized-key", "sudoers"}, w.calls.Names)
}

func TestRun_WritesTheForcedCommandLine(t *testing.T) {
	w := withDeployKey(newWorld())

	testkit.MustDo(t, w.run())

	testkit.Equal(t, []string{deployLine}, w.accounts.Lines)
}

func TestRun_RejectsAnInvalidDeployKey(t *testing.T) {
	w := newWorld()
	w.deps.DeployKey = "ssh-rsa AAAA"

	err := w.run()

	testkit.ErrorIs(t, err, domain.ErrInvalidDeployKey)
}

func TestRun_ChecksTheDeployKeyBeforeWritingAnything(t *testing.T) {
	w := newWorld()
	w.deps.DeployKey = "ssh-rsa AAAA"

	_ = w.run()

	testkit.Equal(t, []string(nil), w.calls.Names)
}

func TestRun_PrintsVersionPathAndAddedKeyNames(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run())

	testkit.Equal(t, "install: vps-agent 0.3.0 at /usr/local/bin/vps-agent\ninstall: added to config.env: GITHUB_TOKEN\n", w.out.String())
}

func TestRun_PrintsNoAddedKeysWhenNothingWasAdded(t *testing.T) {
	w := complete(newWorld())

	testkit.MustDo(t, w.run())

	testkit.Equal(t, "install: vps-agent 0.3.0 at /usr/local/bin/vps-agent\n", w.out.String())
}

func TestRun_SaysTheDeployUserIsReady(t *testing.T) {
	w := withDeployKey(complete(newWorld()))

	testkit.MustDo(t, w.run())

	testkit.Equal(t, "install: vps-agent 0.3.0 at /usr/local/bin/vps-agent\ninstall: deploy user vps-agent-deploy ready\n", w.out.String())
}

func TestRun_AppendsTheMissingKeys(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run())

	testkit.Equal(t, [][]byte{[]byte("AGENT_HOST_NAME=srv\nGITHUB_TOKEN=secret-token\n")}, w.config.Written)
}

func TestRun_LeavesACompleteConfigAlone(t *testing.T) {
	w := complete(newWorld())

	testkit.MustDo(t, w.run())

	testkit.Equal(t, [][]byte(nil), w.config.Written)
}

func TestRun_CreatesAMissingConfig(t *testing.T) {
	w := newWorld()
	w.config.Data, w.config.ReadErr = nil, fs.ErrNotExist

	testkit.MustDo(t, w.run())

	testkit.Equal(t, [][]byte{[]byte("AGENT_HOST_NAME=other\nGITHUB_TOKEN=secret-token\n")}, w.config.Written)
}

func TestRun_CreatesAMissingConfigEvenWithNothingToAdd(t *testing.T) {
	w := newWorld()
	w.config.Data, w.config.ReadErr = nil, fs.ErrNotExist
	w.deps.Values = map[string]string{}

	testkit.MustDo(t, w.run())

	testkit.Equal(t, [][]byte{nil}, w.config.Written)
}

func TestRun_FailsWhenTheConfigCannotBeRead(t *testing.T) {
	w := newWorld()
	w.config.ReadErr = errBoom

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_RefusesAConfigThatWouldBeInvalid(t *testing.T) {
	w := newWorld()
	w.deps.Syntax = fake.ConfigSyntax{Err: errBoom}

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_WritesNothingWhenTheConfigWouldBeInvalid(t *testing.T) {
	w := newWorld()
	w.deps.Syntax = fake.ConfigSyntax{Err: errBoom}

	_ = w.run()

	testkit.Equal(t, [][]byte(nil), w.config.Written)
}

func TestRun_FailsWhenTheConfigCannotBeWritten(t *testing.T) {
	w := newWorld()
	w.config.WriteErr = errBoom

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenTheRunningBinaryCannotBeRead(t *testing.T) {
	w := newWorld()
	w.deps.Executable = fake.Executable{Err: errBoom}

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenStagingFails(t *testing.T) {
	w := newWorld()
	w.binaries.StageErr = errBoom

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenTheSwapFails(t *testing.T) {
	w := newWorld()
	w.binaries.SwapErr = errBoom

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenTheDeployUserCannotBeCreated(t *testing.T) {
	w := withDeployKey(newWorld())
	w.accounts.Errs = map[string]error{"user": errBoom}

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenTheAuthorizedKeyCannotBeWritten(t *testing.T) {
	w := withDeployKey(newWorld())
	w.accounts.Errs = map[string]error{"authorized-key": errBoom}

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenTheSudoersCannotBeWritten(t *testing.T) {
	w := withDeployKey(newWorld())
	w.accounts.Errs = map[string]error{"sudoers": errBoom}

	err := w.run()

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_LogsTheCompletionWithoutValues(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run())

	testkit.Equal(t,
		`{"level":"INFO","msg":"install completed","logId":"install.completed","version":"0.3.0","addedKeys":"GITHUB_TOKEN","deployUser":false}`+"\n",
		w.logs.String())
}

func TestRun_LogsTheDeployUser(t *testing.T) {
	w := withDeployKey(complete(newWorld()))

	testkit.MustDo(t, w.run())

	testkit.Equal(t,
		`{"level":"INFO","msg":"install completed","logId":"install.completed","version":"0.3.0","addedKeys":"","deployUser":true}`+"\n",
		w.logs.String())
}

func TestRun_LogsTheFailure(t *testing.T) {
	w := newWorld()
	w.deps.Executable = fake.Executable{Err: errBoom}

	_ = w.run()

	testkit.Equal(t,
		`{"level":"ERROR","msg":"install failed","logId":"install.failed","version":"0.3.0","errorName":"*fmt.wrapError","errorMessage":"read running binary: boom"}`+"\n",
		w.logs.String())
}
