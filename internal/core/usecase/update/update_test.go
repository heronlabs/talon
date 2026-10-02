package update_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/update"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var (
	errBoom    = errors.New("boom")
	releaseKey = ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	binary     = []byte("new binary")
)

const binaryName = "vps-agent_0.3.0_linux_amd64"

func signedBlobs(bin []byte) map[int64][]byte {
	sum := sha256.Sum256(bin)
	checksums := []byte(hex.EncodeToString(sum[:]) + "  " + binaryName + "\n")
	return map[int64][]byte{1: checksums, 2: ed25519.Sign(releaseKey, checksums), 3: bin}
}

func release030() domain.Release {
	return domain.Release{Tag: "v0.3.0", Assets: []domain.Asset{
		{ID: 1, Name: "checksums.txt"}, {ID: 2, Name: "checksums.txt.sig"}, {ID: 3, Name: binaryName},
	}}
}

type world struct {
	calls    *fake.Calls
	releases *fake.ReleaseSource
	binaries *fake.BinaryStore
	out      *bytes.Buffer
	logs     *bytes.Buffer
	deps     update.Deps
}

func newWorld() *world {
	calls := &fake.Calls{}
	w := &world{
		calls:    calls,
		releases: &fake.ReleaseSource{Rel: release030(), Blobs: signedBlobs(binary), Calls: calls},
		binaries: &fake.BinaryStore{Calls: calls},
		out:      &bytes.Buffer{},
		logs:     &bytes.Buffer{},
	}
	w.deps = update.Deps{
		CurrentVersion: "0.2.0",
		Arch:           "amd64",
		PublicKey:      releaseKey.Public().(ed25519.PublicKey),
		Lock:           fake.UpdateLock{Calls: calls},
		Releases:       w.releases,
		Binaries:       w.binaries,
		Health:         fake.HealthCheck{Output: "ok   config\n", Calls: calls},
		Out:            w.out,
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

func (w *world) run(t *testing.T) error {
	t.Helper()
	return update.New(w.deps).Run(context.Background(), domain.Version{Minor: 3})
}

func doctorFails(w *world) {
	w.deps.Health = fake.HealthCheck{Output: "FAIL config: invalid config\n", Err: errBoom, Calls: w.calls}
}

func TestRun_InstallsAVerifiedRelease(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, []string{
		"lock", "release", "download checksums.txt", "download checksums.txt.sig", "download " + binaryName,
		"stage", "swap", "check", "unlock",
	}, w.calls.Names)
}

func TestRun_StagesTheDownloadedBinary(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, binary, w.binaries.Staged)
}

func TestRun_AsksForTheRequestedVersion(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, []string{"0.3.0"}, w.releases.Requested)
}

func TestRunLatest_AsksForTheLatestRelease(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, update.New(w.deps).RunLatest(context.Background()))

	testkit.Equal(t, []string{""}, w.releases.Requested)
}

func TestRun_PrintsFromAndTo(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, "update: 0.2.0 -> 0.3.0\n", w.out.String())
}

func TestRun_LogsStartAndInstall(t *testing.T) {
	w := newWorld()

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t,
		`{"level":"INFO","msg":"update started","logId":"update.started","fromVersion":"0.2.0"}`+"\n"+
			`{"level":"INFO","msg":"update installed","logId":"update.installed","fromVersion":"0.2.0","toVersion":"0.3.0"}`+"\n",
		w.logs.String())
}

func TestRun_InstallsOverADevBuild(t *testing.T) {
	w := newWorld()
	w.deps.CurrentVersion = "0.0.0-dev"

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, "update: 0.0.0-dev -> 0.3.0\n", w.out.String())
}

func TestRun_FailsWhenAnotherUpdateHoldsTheLock(t *testing.T) {
	w := newWorld()
	w.deps.Lock = fake.UpdateLock{Err: errBoom, Calls: w.calls}

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_TouchesNothingWhenTheLockIsBusy(t *testing.T) {
	w := newWorld()
	w.deps.Lock = fake.UpdateLock{Err: errBoom, Calls: w.calls}

	_ = w.run(t)

	testkit.Equal(t, []string{"lock"}, w.calls.Names)
}

func TestRun_LogsTheFailure(t *testing.T) {
	w := newWorld()
	w.deps.Lock = fake.UpdateLock{Err: errBoom, Calls: w.calls}

	_ = w.run(t)

	testkit.Equal(t,
		`{"level":"ERROR","msg":"update failed","logId":"update.failed","fromVersion":"0.2.0","errorName":"*errors.errorString","errorMessage":"boom"}`+"\n",
		w.logs.String())
}

func TestRun_FailsWhenTheReleaseCannotBeRead(t *testing.T) {
	w := newWorld()
	w.releases.ReleaseErr = errBoom

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsOnATagThatIsNotAVersion(t *testing.T) {
	w := newWorld()
	w.releases.Rel.Tag = "latest"

	err := w.run(t)

	testkit.ErrorIs(t, err, domain.ErrInvalidVersion)
}

func TestRun_RefusesAVersionBelowTheMinimum(t *testing.T) {
	w := newWorld()
	w.releases.Rel.Tag = "v0.1.0"

	err := w.run(t)

	testkit.ErrorIs(t, err, domain.ErrBelowMinimum)
}

func TestRun_StopsWhenAlreadyCurrent(t *testing.T) {
	w := newWorld()
	w.deps.CurrentVersion = "0.3.0"

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, []string{"lock", "release", "unlock"}, w.calls.Names)
}

func TestRun_SaysWhenAlreadyCurrent(t *testing.T) {
	w := newWorld()
	w.deps.CurrentVersion = "0.3.0"

	testkit.MustDo(t, w.run(t))

	testkit.Equal(t, "update: already at 0.3.0\n", w.out.String())
}

func TestRun_FailsOnAReleaseWithoutTheBinary(t *testing.T) {
	w := newWorld()
	w.releases.Rel.Assets = w.releases.Rel.Assets[:2]

	err := w.run(t)

	testkit.ErrorIs(t, err, domain.ErrIncompleteRelease)
}

func TestRun_NamesTheIncompleteRelease(t *testing.T) {
	w := newWorld()
	w.releases.Rel.Assets = w.releases.Rel.Assets[1:]

	err := w.run(t)

	testkit.Equal(t, "release v0.3.0 is incomplete", err.Error())
}

func TestRun_FailsWhenADownloadFails(t *testing.T) {
	w := newWorld()
	w.releases.DownloadErr = errBoom

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_RefusesABadSignature(t *testing.T) {
	w := newWorld()
	w.releases.Blobs[2] = make([]byte, ed25519.SignatureSize)

	err := w.run(t)

	testkit.ErrorIs(t, err, domain.ErrBadSignature)
}

func TestRun_TouchesNoBinaryOnABadChecksum(t *testing.T) {
	w := newWorld()
	w.releases.Blobs[3] = []byte("evil")

	_ = w.run(t)

	testkit.Equal(t, []string{
		"lock", "release", "download checksums.txt", "download checksums.txt.sig", "download " + binaryName, "unlock",
	}, w.calls.Names)
}

func TestRun_RefusesABadChecksum(t *testing.T) {
	w := newWorld()
	w.releases.Blobs[3] = []byte("evil")

	err := w.run(t)

	testkit.ErrorIs(t, err, domain.ErrChecksumMismatch)
}

func TestRun_FailsWhenStagingFails(t *testing.T) {
	w := newWorld()
	w.binaries.StageErr = errBoom

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_DoesNotSwapWhenStagingFails(t *testing.T) {
	w := newWorld()
	w.binaries.StageErr = errBoom

	_ = w.run(t)

	testkit.Equal(t, []string{
		"lock", "release", "download checksums.txt", "download checksums.txt.sig", "download " + binaryName, "stage", "unlock",
	}, w.calls.Names)
}

func TestRun_FailsWhenTheSwapFails(t *testing.T) {
	w := newWorld()
	w.binaries.SwapErr = errBoom

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_RollsBackWhenTheDoctorFails(t *testing.T) {
	w := newWorld()
	doctorFails(w)

	_ = w.run(t)

	testkit.Equal(t, []string{
		"lock", "release", "download checksums.txt", "download checksums.txt.sig", "download " + binaryName,
		"stage", "swap", "check", "restore", "unlock",
	}, w.calls.Names)
}

func TestRun_FailsAfterRollingBack(t *testing.T) {
	w := newWorld()
	doctorFails(w)

	err := w.run(t)

	testkit.ErrorIs(t, err, update.ErrRolledBack)
}

func TestRun_KeepsTheDoctorErrorAfterRollingBack(t *testing.T) {
	w := newWorld()
	doctorFails(w)

	err := w.run(t)

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_PrintsTheRollbackAndTheDoctorOutput(t *testing.T) {
	w := newWorld()
	doctorFails(w)

	_ = w.run(t)

	testkit.Equal(t, "update: rolled back to 0.2.0: doctor failed\nFAIL config: invalid config\n", w.out.String())
}

func TestRun_LogsTheRollback(t *testing.T) {
	w := newWorld()
	doctorFails(w)

	_ = w.run(t)

	testkit.Equal(t,
		`{"level":"INFO","msg":"update started","logId":"update.started","fromVersion":"0.2.0"}`+"\n"+
			`{"level":"WARN","msg":"update rolled back","logId":"update.rolled-back","fromVersion":"0.2.0","toVersion":"0.3.0","errorName":"*errors.errorString","errorMessage":"boom"}`+"\n"+
			`{"level":"ERROR","msg":"update failed","logId":"update.failed","fromVersion":"0.2.0","errorName":"*fmt.wrapErrors","errorMessage":"rolled back to 0.2.0: doctor of 0.3.0 failed: boom"}`+"\n",
		w.logs.String())
}

func TestRun_FailsWhenTheRollbackFails(t *testing.T) {
	w := newWorld()
	doctorFails(w)
	restoreErr := errors.New("restore")
	w.binaries.RestoreErr = restoreErr

	err := w.run(t)

	testkit.ErrorIs(t, err, restoreErr)
}
