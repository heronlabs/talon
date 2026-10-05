//go:build integration

package commands

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heronlabs/talon/tests/cli"
)

func writeEnvContent(t *testing.T, dir, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSetupCommand(t *testing.T) {
	t.Run("Should write every key to .env from the existing values and the defaults", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "CMS_URL=https://cms.example.com\nCMS_AGENT_TOKEN=token\nCMS_AGENT_HOST_NAME=vps-01\nEASYPANEL_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\n")
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "")
		t.Setenv("AWS_REGION", "")
		want := "CMS_URL=https://cms.example.com\nCMS_AGENT_TOKEN=token\nCMS_AGENT_HOST_NAME=vps-01\nEASYPANEL_URL=http://127.0.0.1:3000\nEASYPANEL_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=us-east-1\nINTERVAL_RECONCILE=5m\nINTERVAL_REPORT=10m\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should fail listing the required keys when there is no .env and no input", func(t *testing.T) {
		dir := t.TempDir()

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, "config: missing CMS_URL, CMS_AGENT_TOKEN, CMS_AGENT_HOST_NAME, EASYPANEL_TOKEN") {
			t.Errorf("exit %d, output %q; want exit 1 with missing keys", code, out)
		}
	})
}
