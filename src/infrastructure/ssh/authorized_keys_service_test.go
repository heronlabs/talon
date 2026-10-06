package ssh

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const (
	publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr"
	wantLine  = "restrict,command=\"sudo -n /usr/lib/talon/upgrade\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr talon-deploy\n"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func makeDir(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	if err := os.Mkdir(path, mode); err != nil {
		t.Fatal(err)
	}
}

func makeReadOnly(t *testing.T, dir string) {
	t.Helper()

	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // G302: the test needs a directory that cannot be written
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restores the temporary directory so it can be removed
}

func owner(t *testing.T, path string) [2]uint32 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat := info.Sys().(*syscall.Stat_t)

	return [2]uint32{stat.Uid, stat.Gid}
}

func TestAuthorizedKeysService(t *testing.T) {
	t.Run("Should write the key forced to run the upgrade", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(home), ".ssh/authorized_keys")

		if string(content) != wantLine {
			t.Errorf("Write() wrote %q, want %q", content, wantLine)
		}
	})

	t.Run("Should drop the comment of the key", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey + " operator@laptop")
		content, _ := fs.ReadFile(os.DirFS(home), ".ssh/authorized_keys")

		if string(content) != wantLine {
			t.Errorf("Write() wrote %q, want %q", content, wantLine)
		}
	})

	t.Run("Should replace the key already authorized", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys"), "ssh-ed25519 old\n", 0o600)
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(home), ".ssh/authorized_keys")

		if string(content) != wantLine {
			t.Errorf("Write() wrote %q, want %q", content, wantLine)
		}
	})

	t.Run("Should replace a leftover temporary file", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys.tmp"), "leftover\n", 0o644)
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(home), ".ssh/authorized_keys")

		if string(content) != wantLine {
			t.Errorf("Write() wrote %q, want %q", content, wantLine)
		}
	})

	t.Run("Should leave .ssh with mode 700", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		info, err := os.Stat(filepath.Join(home, ".ssh"))

		if err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("Write() left .ssh as %v, %v; want mode 700", info, err)
		}
	})

	t.Run("Should leave .ssh with mode 700 when it had another mode", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o755)
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		info, err := os.Stat(filepath.Join(home, ".ssh"))

		if err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("Write() left .ssh as %v, %v; want mode 700", info, err)
		}
	})

	t.Run("Should leave authorized_keys with mode 600", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		info, err := os.Stat(filepath.Join(home, ".ssh", "authorized_keys"))

		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("Write() left authorized_keys as %v, %v; want mode 600", info, err)
		}
	})

	t.Run("Should give .ssh to the owner of the home", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		got := owner(t, filepath.Join(home, ".ssh"))

		if want := owner(t, home); got != want {
			t.Errorf("Write() left .ssh owned by %v, want %v", got, want)
		}
	})

	t.Run("Should give authorized_keys to the owner of the home", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write(publicKey)
		got := owner(t, filepath.Join(home, ".ssh", "authorized_keys"))

		if want := owner(t, home); got != want {
			t.Errorf("Write() left authorized_keys owned by %v, want %v", got, want)
		}
	})

	t.Run("Should fail on a key of another type", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write("ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB")

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should fail on a key without its base64", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write("ssh-ed25519")

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should fail on a key that is not base64", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write("ssh-ed25519 AAAA!!!!")

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should fail on a key whose blob is not ed25519", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write("ssh-ed25519 AAAAB3NzaC1yc2EAAAADAQAB")

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should fail on a key of two lines", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write(publicKey + "\n" + publicKey)

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should fail on options before the type", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write(`command="sh" ` + publicKey)

		if !errors.Is(err, ErrInvalidPublicKey) {
			t.Errorf("Write() = %v, want %v", err, ErrInvalidPublicKey)
		}
	})

	t.Run("Should not write an invalid key", func(t *testing.T) {
		home := t.TempDir()
		service := NewAuthorizedKeysService(home)

		_ = service.Write("ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB")
		_, err := os.Stat(filepath.Join(home, ".ssh"))

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Write() left .ssh with %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when the home does not exist", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "talon-deploy")
		service := NewAuthorizedKeysService(home)
		want := home + " not found, install the talon package"

		err := service.Write(publicKey)

		if !errors.Is(err, ErrHomeNotFound) || err.Error() != want {
			t.Errorf("Write() = %v, want %q", err, want)
		}
	})

	t.Run("Should fail when the home cannot be inspected", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "file"), "", 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "file", "talon-deploy"))

		err := service.Write(publicKey)

		if !errors.Is(err, syscall.ENOTDIR) {
			t.Errorf("Write() = %v, want %v", err, syscall.ENOTDIR)
		}
	})

	t.Run("Should fail when the home is not a directory", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "talon-deploy")
		writeFile(t, home, "", 0o600)
		service := NewAuthorizedKeysService(home)

		err := service.Write(publicKey)

		if err == nil {
			t.Errorf("Write() = nil, want an error")
		}
	})

	t.Run("Should fail when .ssh cannot be created", func(t *testing.T) {
		home := t.TempDir()
		makeReadOnly(t, home)
		service := NewAuthorizedKeysService(home)

		err := service.Write(publicKey)

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Write() = %v, want %v", err, fs.ErrPermission)
		}
	})

	t.Run("Should fail when the leftover temporary file cannot be removed", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		makeDir(t, filepath.Join(home, ".ssh", "authorized_keys.tmp"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys.tmp", "file"), "", 0o600)
		service := NewAuthorizedKeysService(home)

		err := service.Write(publicKey)

		if !errors.Is(err, syscall.ENOTEMPTY) {
			t.Errorf("Write() = %v, want %v", err, syscall.ENOTEMPTY)
		}
	})

	t.Run("Should fail when .ssh cannot be written", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		makeReadOnly(t, filepath.Join(home, ".ssh"))
		service := NewAuthorizedKeysService(home)

		err := service.Write(publicKey)

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Write() = %v, want %v", err, fs.ErrPermission)
		}
	})

	t.Run("Should fail when authorized_keys cannot be replaced", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		makeDir(t, filepath.Join(home, ".ssh", "authorized_keys"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys", "file"), "", 0o600)
		service := NewAuthorizedKeysService(home)

		err := service.Write(publicKey)

		if err == nil {
			t.Errorf("Write() = nil, want an error")
		}
	})

	t.Run("Should remove authorized_keys when the key is empty", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys"), wantLine, 0o600)
		service := NewAuthorizedKeysService(home)

		_ = service.Write("")
		_, err := os.Stat(filepath.Join(home, ".ssh", "authorized_keys"))

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Write() left authorized_keys with %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should pass when the key is empty and nothing is authorized", func(t *testing.T) {
		service := NewAuthorizedKeysService(t.TempDir())

		err := service.Write("")

		if err != nil {
			t.Errorf("Write() = %v, want nil", err)
		}
	})

	t.Run("Should pass when the key is empty and the home does not exist", func(t *testing.T) {
		service := NewAuthorizedKeysService(filepath.Join(t.TempDir(), "talon-deploy"))

		err := service.Write("")

		if err != nil {
			t.Errorf("Write() = %v, want nil", err)
		}
	})

	t.Run("Should fail when the key is empty and the home is not a directory", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "talon-deploy")
		writeFile(t, home, "", 0o600)
		service := NewAuthorizedKeysService(home)

		err := service.Write("")

		if err == nil {
			t.Errorf("Write() = nil, want an error")
		}
	})

	t.Run("Should fail when authorized_keys cannot be removed", func(t *testing.T) {
		home := t.TempDir()
		makeDir(t, filepath.Join(home, ".ssh"), 0o700)
		makeDir(t, filepath.Join(home, ".ssh", "authorized_keys"), 0o700)
		writeFile(t, filepath.Join(home, ".ssh", "authorized_keys", "file"), "", 0o600)
		service := NewAuthorizedKeysService(home)

		err := service.Write("")

		if !errors.Is(err, syscall.ENOTEMPTY) {
			t.Errorf("Write() = %v, want %v", err, syscall.ENOTEMPTY)
		}
	})
}
