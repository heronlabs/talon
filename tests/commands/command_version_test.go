//go:build integration

package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/heronlabs/talon/tests/cli"
)

func TestVersionCommand(t *testing.T) {
	t.Run("Should print the version set at build time", func(t *testing.T) {
		dir := t.TempDir()

		out, code := cli.Run(t, dir, "version")

		if code != 0 || !strings.Contains(out, "INFO version version=1.2.3") {
			t.Errorf("exit %d, output %q; want exit 0 with version=1.2.3", code, out)
		}
	})

	t.Run("Should print the version when the configuration directory does not exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")

		out, code := cli.Run(t, dir, "version")

		if code != 0 || !strings.Contains(out, "INFO version version=1.2.3") {
			t.Errorf("exit %d, output %q; want exit 0 with version=1.2.3", code, out)
		}
	})
}
