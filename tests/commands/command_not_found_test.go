//go:build integration

package commands

import (
	"strings"
	"testing"

	"github.com/heronlabs/vps-agent/tests/cli"
)

func TestCommandNotFound(t *testing.T) {
	t.Run("Should exit 1 when command does not exist", func(t *testing.T) {
		dir := t.TempDir()

		out, code := cli.Run(t, dir, "foo")

		if code != 1 || !strings.Contains(out, `ERROR command not found command=foo`) {
			t.Errorf("exit %d, output %q; want exit 1 with command not found", code, out)
		}
	})

	t.Run("Should exit 1 when no command is given", func(t *testing.T) {
		dir := t.TempDir()

		out, code := cli.Run(t, dir)

		if code != 1 || !strings.Contains(out, `ERROR command not found command=""`) {
			t.Errorf("exit %d, output %q; want exit 1 with command not found", code, out)
		}
	})
}
