package process_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/process"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var errBoom = errors.New("boom")

type recorder struct {
	calls [][]string
	fail  map[string]error
}

func (r *recorder) run(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.fail[name]
}

func me(string) (uid, gid int, err error) { return os.Getuid(), os.Getgid(), nil }

func nobody(string) (uid, gid int, err error) { return 0, 0, errBoom }

func accounts(t *testing.T, r *recorder) process.Accounts {
	t.Helper()
	home := t.TempDir()
	return process.Accounts{
		User:        "vps-agent-deploy",
		Home:        home,
		SudoersPath: filepath.Join(t.TempDir(), "vps-agent-deploy"),
		Run:         r.run,
		Lookup:      me,
	}
}

func TestEnsureDeployUser_CreatesAMissingUserAndLocksItsPassword(t *testing.T) {
	r := &recorder{}
	a := accounts(t, r)
	a.Lookup = nobody

	testkit.MustDo(t, a.EnsureDeployUser(context.Background()))

	testkit.Equal(t, [][]string{
		{"useradd", "--system", "--create-home", "--home-dir", a.Home, "--shell", "/bin/sh", "vps-agent-deploy"},
		{"passwd", "-l", "vps-agent-deploy"},
	}, r.calls)
}

func TestEnsureDeployUser_ReusesAnExistingUser(t *testing.T) {
	r := &recorder{}

	testkit.MustDo(t, accounts(t, r).EnsureDeployUser(context.Background()))

	testkit.Equal(t, [][]string{{"passwd", "-l", "vps-agent-deploy"}}, r.calls)
}

func TestEnsureDeployUser_FailsWhenUseraddFails(t *testing.T) {
	r := &recorder{fail: map[string]error{"useradd": errBoom}}
	a := accounts(t, r)
	a.Lookup = nobody

	err := a.EnsureDeployUser(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestEnsureDeployUser_DoesNotLockAUserItCouldNotCreate(t *testing.T) {
	r := &recorder{fail: map[string]error{"useradd": errBoom}}
	a := accounts(t, r)
	a.Lookup = nobody

	_ = a.EnsureDeployUser(context.Background())

	testkit.Equal(t, 1, len(r.calls))
}

func TestEnsureDeployUser_FailsWhenPasswdFails(t *testing.T) {
	r := &recorder{fail: map[string]error{"passwd": errBoom}}

	err := accounts(t, r).EnsureDeployUser(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestWriteAuthorizedKey_WritesOnlyTheLine(t *testing.T) {
	a := accounts(t, &recorder{})

	testkit.MustDo(t, a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA"))

	testkit.Equal(t, "restrict ssh-ed25519 AAAA\n", string(testkit.Must(os.ReadFile(filepath.Join(a.Home, ".ssh", "authorized_keys")))))
}

func TestWriteAuthorizedKey_ReplacesThePreviousKeys(t *testing.T) {
	a := accounts(t, &recorder{})
	testkit.MustDo(t, os.Mkdir(filepath.Join(a.Home, ".ssh"), 0o700))
	testkit.MustDo(t, os.WriteFile(filepath.Join(a.Home, ".ssh", "authorized_keys"), []byte("ssh-ed25519 OLD\n"), 0o600))

	testkit.MustDo(t, a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA"))

	testkit.Equal(t, "restrict ssh-ed25519 AAAA\n", string(testkit.Must(os.ReadFile(filepath.Join(a.Home, ".ssh", "authorized_keys")))))
}

func TestWriteAuthorizedKey_WritesAPrivateFile(t *testing.T) {
	a := accounts(t, &recorder{})

	testkit.MustDo(t, a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA"))

	testkit.Equal(t, fs.FileMode(0o600), testkit.Must(os.Stat(filepath.Join(a.Home, ".ssh", "authorized_keys"))).Mode().Perm())
}

func TestWriteAuthorizedKey_MakesAnOpenSSHDirectoryPrivate(t *testing.T) {
	a := accounts(t, &recorder{})
	testkit.MustDo(t, os.Mkdir(filepath.Join(a.Home, ".ssh"), 0o750))

	testkit.MustDo(t, a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA"))

	testkit.Equal(t, fs.FileMode(0o700), testkit.Must(os.Stat(filepath.Join(a.Home, ".ssh"))).Mode().Perm())
}

func TestWriteAuthorizedKey_FailsWhenTheUserIsUnknown(t *testing.T) {
	a := accounts(t, &recorder{})
	a.Lookup = nobody

	err := a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA")

	testkit.ErrorIs(t, err, errBoom)
}

func TestWriteAuthorizedKey_FailsWhenTheHomeIsMissing(t *testing.T) {
	a := accounts(t, &recorder{})
	a.Home = filepath.Join(a.Home, "missing")

	err := a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA")

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestWriteAuthorizedKey_FailsWhenTheSSHDirectoryIsADanglingLink(t *testing.T) {
	a := accounts(t, &recorder{})
	testkit.MustDo(t, os.Symlink(filepath.Join(a.Home, "nowhere"), filepath.Join(a.Home, ".ssh")))

	err := a.WriteAuthorizedKey("restrict ssh-ed25519 AAAA")

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestWriteSudoers_WritesTheDeployRule(t *testing.T) {
	a := accounts(t, &recorder{})

	testkit.MustDo(t, a.WriteSudoers(context.Background()))

	testkit.Equal(t, domain.SudoersRule, string(testkit.Must(os.ReadFile(a.SudoersPath))))
}

func TestWriteSudoers_WritesTheModeSudoWants(t *testing.T) {
	a := accounts(t, &recorder{})

	testkit.MustDo(t, a.WriteSudoers(context.Background()))

	testkit.Equal(t, fs.FileMode(0o440), testkit.Must(os.Stat(a.SudoersPath)).Mode().Perm())
}

func TestWriteSudoers_ValidatesWithVisudoBeforeInstalling(t *testing.T) {
	r := &recorder{}
	a := accounts(t, r)

	testkit.MustDo(t, a.WriteSudoers(context.Background()))

	testkit.Equal(t, []string{"visudo", "-cf"}, r.calls[0][:2])
}

func TestWriteSudoers_ValidatesATemporaryFileSudoIgnores(t *testing.T) {
	r := &recorder{}
	a := accounts(t, r)

	testkit.MustDo(t, a.WriteSudoers(context.Background()))

	testkit.Equal(t, [2]string{filepath.Dir(a.SudoersPath), ".tmp"}, [2]string{filepath.Dir(r.calls[0][2]), filepath.Ext(r.calls[0][2])})
}

func TestWriteSudoers_FailsWhenVisudoRejectsTheRule(t *testing.T) {
	a := accounts(t, &recorder{fail: map[string]error{"visudo": errBoom}})

	err := a.WriteSudoers(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestWriteSudoers_InstallsNothingVisudoRejects(t *testing.T) {
	a := accounts(t, &recorder{fail: map[string]error{"visudo": errBoom}})

	_ = a.WriteSudoers(context.Background())

	testkit.Equal(t, []fs.DirEntry{}, testkit.Must(os.ReadDir(filepath.Dir(a.SudoersPath))))
}

func TestLookupUser_ReadsTheIDsOfTheCurrentUser(t *testing.T) {
	current := testkit.Must(user.Current())
	uid, gid, err := process.LookupUser(current.Username)
	testkit.MustDo(t, err)

	testkit.Equal(t, [2]string{current.Uid, current.Gid}, [2]string{strconv.Itoa(uid), strconv.Itoa(gid)})
}

func TestLookupUser_FailsForAnUnknownUser(t *testing.T) {
	_, _, err := process.LookupUser("no-such-user-vps-agent")

	testkit.ErrorIs(t, err, user.UnknownUserError("no-such-user-vps-agent"))
}
