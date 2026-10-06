package environment

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func newRoot(t *testing.T, dir string) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })

	return root
}

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

func TestConfigWriterService(t *testing.T) {
	keys := []Key{{Name: "CMS_URL"}, {Name: "AWS_REGION", Default: "us-east-1"}}
	values := map[string]string{"AWS_REGION": "sa-east-1", "CMS_URL": "https://cms.example.com"}

	t.Run("Should write the keys in order", func(t *testing.T) {
		dir := t.TempDir()
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		_ = writer.Write(values)
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if string(content) != "CMS_URL=https://cms.example.com\nAWS_REGION=sa-east-1\n" {
			t.Errorf("Write() wrote %q, want %q", content, "CMS_URL=https://cms.example.com\nAWS_REGION=sa-east-1\n")
		}
	})

	t.Run("Should leave .env with mode 600 when it had another mode", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".env"), "CMS_URL=old\n", 0o644)
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		_ = writer.Write(values)
		info, err := os.Stat(filepath.Join(dir, ".env"))

		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("Write() left .env as %v, %v; want mode 600", info, err)
		}
	})

	t.Run("Should replace a leftover temporary file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".env.tmp"), "leftover\n", 0o644)
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		_ = writer.Write(values)
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if string(content) != "CMS_URL=https://cms.example.com\nAWS_REGION=sa-east-1\n" {
			t.Errorf("Write() wrote %q, want %q", content, "CMS_URL=https://cms.example.com\nAWS_REGION=sa-east-1\n")
		}
	})

	t.Run("Should fail when the leftover temporary file cannot be removed", func(t *testing.T) {
		dir := t.TempDir()
		makeDir(t, filepath.Join(dir, ".env.tmp"))
		writeFile(t, filepath.Join(dir, ".env.tmp", "file"), "", 0o600)
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		err := writer.Write(values)

		if !errors.Is(err, syscall.ENOTEMPTY) {
			t.Errorf("Write() = %v, want %v", err, syscall.ENOTEMPTY)
		}
	})

	t.Run("Should fail when the directory cannot be written", func(t *testing.T) {
		dir := t.TempDir()
		makeReadOnly(t, dir)
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		err := writer.Write(values)

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Write() = %v, want %v", err, fs.ErrPermission)
		}
	})

	t.Run("Should fail when .env cannot be replaced", func(t *testing.T) {
		dir := t.TempDir()
		makeDir(t, filepath.Join(dir, ".env"))
		writer := NewConfigWriterService(newRoot(t, dir), keys)

		err := writer.Write(values)

		if err == nil {
			t.Errorf("Write() = nil, want an error")
		}
	})
}
