//go:build integration

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heronlabs/vps-agent/tests/cli"
)

func TestVersionCommand(t *testing.T) {
	t.Run("Should print version from version.txt", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("1.2.3\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		out, code := cli.Run(t, dir, "version")

		if code != 0 || !strings.Contains(out, "INFO version version=1.2.3") {
			t.Errorf("exit %d, output %q; want exit 0 with version=1.2.3", code, out)
		}
	})

	t.Run("Should print 0.0.0 when version.txt is missing", func(t *testing.T) {
		dir := t.TempDir()

		out, code := cli.Run(t, dir, "version")

		if code != 0 || !strings.Contains(out, "INFO version version=0.0.0") {
			t.Errorf("exit %d, output %q; want exit 0 with version=0.0.0", code, out)
		}
	})
}
