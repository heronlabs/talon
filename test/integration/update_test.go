//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/deploy"
	"github.com/heronlabs/vps-agent/internal/core/usecase/install"
	"github.com/heronlabs/vps-agent/internal/core/usecase/update"
	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/infrastructure/github"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/infrastructure/process"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

const (
	oldBinary     = "#!/bin/sh\n# vps-agent 0.2.0\nexit 0\n"
	healthyBinary = "#!/bin/sh\n# vps-agent 0.3.0\nexit 0\n"
	brokenBinary  = "#!/bin/sh\n# vps-agent 0.3.0\necho 'FAIL config: invalid config'\nexit 1\n"
	fakePAT       = "github-pat-fake"
)

var (
	signingKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	otherKey   = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
)

// published is what the fake GitHub serves for one release.
type published struct {
	tag       string
	binary    string
	checksums string // empty: the binary's real sha256
	signer    ed25519.PrivateKey
	noBinary  bool
}

func (p published) assetName() string {
	return "vps-agent_" + p.tag[1:] + "_linux_amd64"
}

func (p published) checksumsFile() []byte {
	if p.checksums != "" {
		return []byte(p.checksums)
	}
	sum := sha256.Sum256([]byte(p.binary))
	return []byte(hex.EncodeToString(sum[:]) + "  " + p.assetName() + "\n")
}

// fakeGitHub serves p as the latest release and as its tag, and checks the token on every call.
func fakeGitHub(t *testing.T, p published) string {
	t.Helper()
	checksums := p.checksumsFile()
	assets := map[string][]byte{
		"/releases/assets/1": checksums,
		"/releases/assets/2": ed25519.Sign(p.signer, checksums),
		"/releases/assets/3": []byte(p.binary),
	}
	binaryAsset := fmt.Sprintf(`,{"id":3,"name":%q}`, p.assetName())
	if p.noBinary {
		binaryAsset = ""
	}
	releaseJSON := fmt.Sprintf(`{"tag_name":%q,"assets":[{"id":1,"name":"checksums.txt"},{"id":2,"name":"checksums.txt.sig"}%s]}`, p.tag, binaryAsset)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+fakePAT {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/releases/latest" || r.URL.Path == "/releases/tags/"+p.tag {
			_, _ = io.WriteString(w, releaseJSON)
			return
		}
		blob, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(blob)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// host is a temporary stand-in for /usr/local/bin, /etc/vps-agent and /run.
type host struct {
	binDir     string
	configDir  string
	configPath string
	lockPath   string
	out        bytes.Buffer
}

func newHost(t *testing.T, configLines string) *host {
	t.Helper()
	root := t.TempDir()
	h := &host{
		binDir:    filepath.Join(root, "bin"),
		configDir: filepath.Join(root, "etc"),
		lockPath:  filepath.Join(root, "update.lock"),
	}
	h.configPath = filepath.Join(h.configDir, "config.env")
	testkit.MustDo(t, os.MkdirAll(h.binDir, 0o700))
	testkit.MustDo(t, os.WriteFile(filepath.Join(h.binDir, "vps-agent"), []byte(oldBinary), 0o600))
	if configLines != "" {
		testkit.MustDo(t, os.MkdirAll(h.configDir, 0o700))
		testkit.MustDo(t, os.WriteFile(h.configPath, []byte(configLines), 0o600))
	}
	return h
}

func (h *host) installed(t *testing.T) string {
	t.Helper()
	return string(testkit.Must(os.ReadFile(filepath.Join(h.binDir, "vps-agent"))))
}

func (h *host) updater(apiURL, currentVersion string) *update.UseCase {
	return update.New(update.Deps{
		CurrentVersion: currentVersion,
		Arch:           "amd64",
		PublicKey:      signingKey.Public().(ed25519.PublicKey),
		Lock:           hostfs.Lock{Path: h.lockPath},
		Releases: github.New(&http.Client{Timeout: 10 * time.Second}, apiURL,
			config.File{Dir: os.DirFS(h.configDir), Name: "config.env"}, 1<<20),
		Binaries: hostfs.Binaries{Dir: h.binDir, Name: "vps-agent"},
		Health:   process.Doctor{Binary: filepath.Join(h.binDir, "vps-agent"), ConfigPath: h.configPath, Timeout: 10 * time.Second},
		Out:      &h.out,
		Logger:   slog.New(slog.DiscardHandler),
	})
}

func healthy() published {
	return published{tag: "v0.3.0", binary: healthyBinary, signer: signingKey}
}

func updateTo(t *testing.T, h *host, p published, v domain.Version) error {
	t.Helper()
	return h.updater(fakeGitHub(t, p), "0.2.0").Run(context.Background(), v)
}

func TestUpdate_InstallsTheSignedLatestRelease(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	testkit.MustDo(t, h.updater(fakeGitHub(t, healthy()), "0.2.0").RunLatest(context.Background()))

	testkit.Equal(t, healthyBinary, h.installed(t))
}

func TestUpdate_PrintsFromAndTo(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	testkit.MustDo(t, updateTo(t, h, healthy(), domain.Version{Minor: 3}))

	testkit.Equal(t, "update: 0.2.0 -> 0.3.0\n", h.out.String())
}

func TestUpdate_SaysWhenAlreadyCurrent(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	testkit.MustDo(t, h.updater(fakeGitHub(t, healthy()), "0.3.0").RunLatest(context.Background()))

	testkit.Equal(t, "update: already at 0.3.0\n", h.out.String())
}

func TestUpdate_RefusesASignatureFromAnotherKey(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.signer = otherKey

	err := updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestUpdate_KeepsTheBinaryOnABadSignature(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.signer = otherKey

	_ = updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.Equal(t, oldBinary, h.installed(t))
}

func TestUpdate_RefusesAWrongChecksum(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.checksums = hex.EncodeToString(make([]byte, sha256.Size)) + "  " + p.assetName() + "\n"

	err := updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, domain.ErrChecksumMismatch)
}

func TestUpdate_RollsBackWhenTheNewDoctorFails(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.binary = brokenBinary

	_ = updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.Equal(t, oldBinary, h.installed(t))
}

func TestUpdate_FailsAfterRollingBack(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.binary = brokenBinary

	err := updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, update.ErrRolledBack)
}

func TestUpdate_PrintsTheRollbackWithTheDoctorOutput(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.binary = brokenBinary

	_ = updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.Equal(t, "update: rolled back to 0.2.0: doctor failed\nFAIL config: invalid config\n", h.out.String())
}

func TestUpdate_FailsOnAReleaseWithoutTheBinary(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := healthy()
	p.noBinary = true

	err := updateTo(t, h, p, domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, domain.ErrIncompleteRelease)
}

func TestUpdate_RefusesAVersionBelowTheMinimum(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	p := published{tag: "v0.1.0", binary: healthyBinary, signer: signingKey}

	err := updateTo(t, h, p, domain.Version{Minor: 1})

	testkit.ErrorIs(t, err, domain.ErrBelowMinimum)
}

func TestUpdate_FailsWhileAnotherUpdateRuns(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")
	release := testkit.Must(hostfs.Lock{Path: h.lockPath}.Acquire())
	t.Cleanup(release)

	err := updateTo(t, h, healthy(), domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, hostfs.ErrUpdateRunning)
}

func TestUpdate_FailsWithoutAToken(t *testing.T) {
	h := newHost(t, "AGENT_HOST_NAME=srv\n")

	err := updateTo(t, h, healthy(), domain.Version{Minor: 3})

	testkit.ErrorIs(t, err, config.ErrMissingToken)
}

func (h *host) installer(t *testing.T, values map[string]string) *install.UseCase {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "vps-agent")
	testkit.MustDo(t, os.WriteFile(exe, []byte(healthyBinary), 0o600))
	return install.New(install.Deps{
		Version:    "0.3.0",
		BinaryPath: "/usr/local/bin/vps-agent",
		Executable: hostfs.Executable{Path: exe},
		Binaries:   hostfs.Binaries{Dir: h.binDir, Name: "vps-agent"},
		Config:     hostfs.ConfigFile{Dir: h.configDir, Name: "config.env"},
		Syntax:     config.Syntax{},
		Values:     values,
		Accounts:   &fake.SystemAccounts{},
		Out:        &h.out,
		Logger:     slog.New(slog.DiscardHandler),
	})
}

func TestInstall_CreatesTheConfig(t *testing.T) {
	h := newHost(t, "")

	testkit.MustDo(t, h.installer(t, map[string]string{"AGENT_HOST_NAME": "srv655164", "GITHUB_TOKEN": fakePAT, "CMS_URL": ""}).Run(context.Background()))

	testkit.Equal(t, "AGENT_HOST_NAME=srv655164\nGITHUB_TOKEN="+fakePAT+"\n", string(testkit.Must(os.ReadFile(h.configPath))))
}

func TestInstall_MergesTheTokenIntoAnOldConfig(t *testing.T) {
	h := newHost(t, "# written by install.sh 0.1.0\nAGENT_HOST_NAME=srv655164\n")

	testkit.MustDo(t, h.installer(t, map[string]string{"AGENT_HOST_NAME": "other", "GITHUB_TOKEN": fakePAT}).Run(context.Background()))

	testkit.Equal(t, "# written by install.sh 0.1.0\nAGENT_HOST_NAME=srv655164\nGITHUB_TOKEN="+fakePAT+"\n", string(testkit.Must(os.ReadFile(h.configPath))))
}

func TestInstall_InstallsTheRunningBinary(t *testing.T) {
	h := newHost(t, "")

	testkit.MustDo(t, h.installer(t, map[string]string{"GITHUB_TOKEN": fakePAT}).Run(context.Background()))

	testkit.Equal(t, healthyBinary, h.installed(t))
}

func deployRequest(t *testing.T, h *host, request string) error {
	t.Helper()
	return deploy.New(h.updater(fakeGitHub(t, healthy()), "0.2.0")).Run(context.Background(), request)
}

func TestDeploy_InstallsTheRequestedVersion(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	testkit.MustDo(t, deployRequest(t, h, "0.3.0"))

	testkit.Equal(t, healthyBinary, h.installed(t))
}

func TestDeploy_RejectsARequestWithASpace(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	err := deployRequest(t, h, "0.3.0 extra")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestDeploy_RejectsARequestWithASemicolon(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	err := deployRequest(t, h, "0.3.0;id")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestDeploy_RejectsATag(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	err := deployRequest(t, h, "v0.3.0")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestDeploy_RejectsAnEmptyRequest(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	err := deployRequest(t, h, "")

	testkit.ErrorIs(t, err, deploy.ErrBadRequest)
}

func TestDeploy_KeepsTheBinaryOnABadRequest(t *testing.T) {
	h := newHost(t, "GITHUB_TOKEN="+fakePAT+"\n")

	_ = deployRequest(t, h, "0.3.0;id")

	testkit.Equal(t, oldBinary, h.installed(t))
}
