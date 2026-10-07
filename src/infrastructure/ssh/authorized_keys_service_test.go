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
	publicKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr"
	deployLine = `restrict,command="export DEBIAN_FRONTEND=noninteractive; apt-get -qq update && apt-get -qq -y -o DPkg::Lock::Timeout=300 install --only-upgrade talon && talon version" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr talon-deploy` + "\n"
	adminKey   = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl admin@laptop\n"
	opsKey     = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB ops@desktop\n"
	otherKeys  = "# admins\n" + adminKey + "\n" + opsKey
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func makeDir(t *testing.T, path string) {
	t.Helper()

	if err := os.Mkdir(path, 0o700); err != nil {
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

func TestAuthorizedKeysService(t *testing.T) {
	t.Run("Should add the key forced to run the upgrade after the other keys", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != otherKeys+deployLine {
			t.Errorf("Write() wrote %q, want %q", content, otherKeys+deployLine)
		}
	})

	t.Run("Should drop the comment of the key", func(t *testing.T) {
		dir := t.TempDir()
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey + " operator@laptop")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != deployLine {
			t.Errorf("Write() wrote %q, want %q", content, deployLine)
		}
	})

	t.Run("Should replace the deploy key already authorized", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), adminKey+"ssh-ed25519 AAAAold talon-deploy\n"+opsKey, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != adminKey+opsKey+deployLine {
			t.Errorf("Write() wrote %q, want %q", content, adminKey+opsKey+deployLine)
		}
	})

	t.Run("Should end the last key with a newline when the file has none", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB ops@desktop", 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != opsKey+deployLine {
			t.Errorf("Write() wrote %q, want %q", content, opsKey+deployLine)
		}
	})

	t.Run("Should replace a leftover temporary file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys.tmp"), "leftover\n", 0o644)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != deployLine {
			t.Errorf("Write() wrote %q, want %q", content, deployLine)
		}
	})

	t.Run("Should create the directory with mode 700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".ssh")
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		info, err := os.Stat(dir)

		if err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("Write() left the directory as %v, %v; want mode 700", info, err)
		}
	})

	t.Run("Should create authorized_keys with mode 600", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".ssh")
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write(publicKey)
		info, err := os.Stat(filepath.Join(dir, "authorized_keys"))

		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("Write() left authorized_keys as %v, %v; want mode 600", info, err)
		}
	})

	t.Run("Should fail on a key of another type without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should fail on a key without its base64 without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("ssh-ed25519")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should fail on a key that is not base64 without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("ssh-ed25519 AAAA!!!!")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should fail on a key whose blob is not ed25519 without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("ssh-ed25519 AAAAB3NzaC1yc2EAAAADAQAB")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should fail on a key of two lines without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(publicKey + "\n" + publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should fail on options before the type without writing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(`command="sh" ` + publicKey)
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if !errors.Is(err, ErrInvalidPublicKey) || string(content) != otherKeys {
			t.Errorf("Write() = %v and wrote %q, want %v and %q", err, content, ErrInvalidPublicKey, otherKeys)
		}
	})

	t.Run("Should not create the directory for an invalid key", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".ssh")
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write("ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB")
		_, err := os.Stat(dir)

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Write() left the directory with %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should remove only the deploy key when the key is empty", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), adminKey+deployLine+opsKey, 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write("")
		content, _ := fs.ReadFile(os.DirFS(dir), "authorized_keys")

		if string(content) != adminKey+opsKey {
			t.Errorf("Write() wrote %q, want %q", content, adminKey+opsKey)
		}
	})

	t.Run("Should not rewrite authorized_keys without the deploy key when the key is empty", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "authorized_keys"), otherKeys, 0o644)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		_ = service.Write("")
		info, err := os.Stat(filepath.Join(dir, "authorized_keys"))

		if err != nil || info.Mode().Perm() != 0o644 {
			t.Errorf("Write() left authorized_keys as %v, %v; want it untouched with mode 644", info, err)
		}
	})

	t.Run("Should not create authorized_keys when the key is empty", func(t *testing.T) {
		dir := t.TempDir()
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("")
		_, statErr := os.Stat(filepath.Join(dir, "authorized_keys"))

		if err != nil || !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("Write() = %v and left authorized_keys with %v, want nil and %v", err, statErr, fs.ErrNotExist)
		}
	})

	t.Run("Should not create the directory when the key is empty", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".ssh")
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write("")
		_, statErr := os.Stat(dir)

		if err != nil || !errors.Is(statErr, fs.ErrNotExist) {
			t.Errorf("Write() = %v and left the directory with %v, want nil and %v", err, statErr, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when the directory cannot be created", func(t *testing.T) {
		service := NewAuthorizedKeysService(filepath.Join(t.TempDir(), "missing", ".ssh", "authorized_keys"))

		err := service.Write(publicKey)

		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Write() = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("Should fail when the directory is a file", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".ssh")
		writeFile(t, dir, "", 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(publicKey)

		if err == nil {
			t.Errorf("Write() = nil, want an error")
		}
	})

	t.Run("Should fail when authorized_keys cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		makeDir(t, filepath.Join(dir, "authorized_keys"))
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(publicKey)

		if !errors.Is(err, syscall.EISDIR) {
			t.Errorf("Write() = %v, want %v", err, syscall.EISDIR)
		}
	})

	t.Run("Should fail when the leftover temporary file cannot be removed", func(t *testing.T) {
		dir := t.TempDir()
		makeDir(t, filepath.Join(dir, "authorized_keys.tmp"))
		writeFile(t, filepath.Join(dir, "authorized_keys.tmp", "file"), "", 0o600)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(publicKey)

		if !errors.Is(err, syscall.ENOTEMPTY) {
			t.Errorf("Write() = %v, want %v", err, syscall.ENOTEMPTY)
		}
	})

	t.Run("Should fail when the directory cannot be written", func(t *testing.T) {
		dir := t.TempDir()
		makeReadOnly(t, dir)
		service := NewAuthorizedKeysService(filepath.Join(dir, "authorized_keys"))

		err := service.Write(publicKey)

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Write() = %v, want %v", err, fs.ErrPermission)
		}
	})
}
