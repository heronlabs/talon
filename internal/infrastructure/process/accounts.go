package process

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/atomicfile"
)

// Accounts prepares the deploy user User with home Home and its sudoers file SudoersPath.
// Run and Lookup are Exec and LookupUser in production.
type Accounts struct {
	User        string
	Home        string
	SudoersPath string
	Run         func(ctx context.Context, name string, args ...string) error
	Lookup      func(name string) (uid, gid int, err error)
}

// EnsureDeployUser implements port.SystemAccounts: it creates the system user when missing and
// locks its password either way.
func (a Accounts) EnsureDeployUser(ctx context.Context) error {
	if _, _, err := a.Lookup(a.User); err != nil {
		if err := a.Run(ctx, "useradd", "--system", "--create-home", "--home-dir", a.Home, "--shell", "/bin/sh", a.User); err != nil {
			return err
		}
	}
	return a.Run(ctx, "passwd", "-l", a.User)
}

// WriteAuthorizedKey implements port.SystemAccounts: Home/.ssh (0700) gets an authorized_keys
// (0600) holding only line, both owned by User.
func (a Accounts) WriteAuthorizedKey(line string) error {
	uid, gid, err := a.Lookup(a.User)
	if err != nil {
		return err
	}
	dir := filepath.Join(a.Home, ".ssh")
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	//nolint:gosec // a directory needs its search bit; 0700 keeps it private to its owner
	if err := errors.Join(os.Chmod(dir, 0o700), os.Chown(dir, uid, gid)); err != nil {
		return fmt.Errorf("secure %s: %w", dir, err)
	}
	return atomicfile.Write(dir, "authorized_keys", []byte(line+"\n"), 0o600, func(tmp string) error {
		return os.Chown(tmp, uid, gid)
	})
}

// WriteSudoers implements port.SystemAccounts: the rule is written 0440 and installed only after
// "visudo -cf" accepts it.
func (a Accounts) WriteSudoers(ctx context.Context) error {
	return atomicfile.Write(filepath.Dir(a.SudoersPath), filepath.Base(a.SudoersPath), []byte(domain.SudoersRule), 0o440,
		func(tmp string) error { return a.Run(ctx, "visudo", "-cf", tmp) })
}

// LookupUser returns the numeric ids of the user called name.
func LookupUser(name string) (uid, gid int, err error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, fmt.Errorf("look up user %s: %w", name, err)
	}
	uid, errUID := strconv.Atoi(u.Uid)
	gid, errGID := strconv.Atoi(u.Gid)
	return uid, gid, errors.Join(errUID, errGID)
}
