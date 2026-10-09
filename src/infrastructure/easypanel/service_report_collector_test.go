package easypanel

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func chmod(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()

	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: restores the temporary directory so it can be removed
}

func TestReportCollectorService(t *testing.T) {
	t.Run("Should sum the bytes of the regular files under the directory of each service, by project and service", func(t *testing.T) {
		fsys := fstest.MapFS{
			"etc/easypanel/projects/shop/api/config.json":       {Data: []byte("{}")},
			"etc/easypanel/projects/shop/api/uploads/image.png": {Data: []byte("image")},
			"etc/easypanel/projects/shop/database/data/base":    {Data: []byte("postgres")},
			"etc/easypanel/projects/blog/web/index.html":        {Data: []byte("<html>")},
		}
		service := NewReportCollectorService(fsys)
		want := map[string]map[string]int64{"shop": {"api": 7, "database": 8}, "blog": {"web": 6}}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Sizes, want) {
			t.Errorf("Collect() = %+v, %v; want sizes %v", snapshot, err, want)
		}
	})

	t.Run("Should skip the files in etc/easypanel/projects and in the directory of a project", func(t *testing.T) {
		fsys := fstest.MapFS{
			"etc/easypanel/projects/notes.txt":       {Data: []byte("notes")},
			"etc/easypanel/projects/shop/README":     {Data: []byte("readme")},
			"etc/easypanel/projects/shop/api/config": {Data: []byte("{}")},
		}
		service := NewReportCollectorService(fsys)
		want := map[string]map[string]int64{"shop": {"api": 2}}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Sizes, want) {
			t.Errorf("Collect() = %+v, %v; want sizes %v", snapshot, err, want)
		}
	})

	t.Run("Should skip the entries under the directory of a service that are not regular files", func(t *testing.T) {
		fsys := fstest.MapFS{
			"etc/easypanel/projects/shop/api/config": {Data: []byte("{}")},
			"etc/easypanel/projects/shop/api/link":   {Data: []byte("../../../../../../var/lib/big"), Mode: fs.ModeSymlink},
		}
		service := NewReportCollectorService(fsys)
		want := map[string]map[string]int64{"shop": {"api": 2}}

		snapshot, err := service.Collect()

		if err != nil || !reflect.DeepEqual(snapshot.Sizes, want) {
			t.Errorf("Collect() = %+v, %v; want sizes %v", snapshot, err, want)
		}
	})

	t.Run("Should return no sizes when etc/easypanel/projects does not exist", func(t *testing.T) {
		fsys := fstest.MapFS{"etc/easypanel/config.json": {Data: []byte("{}")}}
		service := NewReportCollectorService(fsys)

		snapshot, err := service.Collect()

		if err != nil || len(snapshot.Sizes) != 0 {
			t.Errorf("Collect() = %+v, %v; want no sizes", snapshot, err)
		}
	})

	t.Run("Should fail when etc/easypanel/projects cannot be read", func(t *testing.T) {
		fsys := fstest.MapFS{"etc/easypanel/projects": {Data: []byte("not a directory")}}
		service := NewReportCollectorService(fsys)
		var pathErr *fs.PathError

		_, err := service.Collect()

		if !errors.As(err, &pathErr) || pathErr.Path != "etc/easypanel/projects" {
			t.Errorf("Collect() = %v, want an error on %q", err, "etc/easypanel/projects")
		}
	})

	t.Run("Should fail when the directory of a project cannot be read", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "etc/easypanel/projects/shop/api/config"), "{}")
		chmod(t, filepath.Join(root, "etc/easypanel/projects/shop"), 0o000)
		service := NewReportCollectorService(os.DirFS(root))

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrPermission)
		}
	})

	t.Run("Should fail when the directory of a service cannot be walked", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "etc/easypanel/projects/shop/api/config"), "{}")
		chmod(t, filepath.Join(root, "etc/easypanel/projects/shop/api"), 0o000)
		service := NewReportCollectorService(os.DirFS(root))

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrPermission)
		}
	})

	t.Run("Should fail when the info of a file of a service cannot be read", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "etc/easypanel/projects/shop/api/config"), "{}")
		chmod(t, filepath.Join(root, "etc/easypanel/projects/shop/api"), 0o444)
		service := NewReportCollectorService(os.DirFS(root))

		_, err := service.Collect()

		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("Collect() = %v, want %v", err, fs.ErrPermission)
		}
	})
}
