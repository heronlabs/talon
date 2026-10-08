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

func unsetKeys(t *testing.T) {
	t.Helper()

	t.Setenv("WEBHOOK_URL", "")
	t.Setenv("WEBHOOK_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("INTERVAL_REPORT", "")
}

func TestSetupCommand(t *testing.T) {
	t.Run("Should write every key to .env from the existing values and the defaults", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL=https://webhook.example.com\nWEBHOOK_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\n")
		unsetKeys(t)
		want := "WEBHOOK_URL=https://webhook.example.com\nWEBHOOK_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=us-east-1\nINTERVAL_REPORT=10m\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should write every key to .env from the environment variables", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("WEBHOOK_URL", "https://webhook.example.com")
		t.Setenv("WEBHOOK_TOKEN", "webhook-token")
		t.Setenv("AWS_ACCESS_KEY_ID", "id")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		t.Setenv("AWS_REGION", "sa-east-1")
		t.Setenv("INTERVAL_REPORT", "2m")
		want := "WEBHOOK_URL=https://webhook.example.com\nWEBHOOK_TOKEN=webhook-token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=sa-east-1\nINTERVAL_REPORT=2m\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should write every key to .env from the typed answers", func(t *testing.T) {
		dir := t.TempDir()
		unsetKeys(t)
		want := "WEBHOOK_URL=https://webhook.example.com\nWEBHOOK_TOKEN=typed-token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=sa-east-1\nINTERVAL_REPORT=5m\n"

		out, code := cli.RunWithInput(t, dir, "https://webhook.example.com\ntyped-token\nid\nsecret\nsa-east-1\n5m\n", "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should not ask for WEBHOOK_TOKEN when WEBHOOK_URL is empty", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\n")
		unsetKeys(t)
		want := "WEBHOOK_URL=\nWEBHOOK_TOKEN=\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=us-east-1\nINTERVAL_REPORT=10m\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || strings.Contains(out, "WEBHOOK_TOKEN") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 without asking for WEBHOOK_TOKEN and .env %q", code, out, content, want)
		}
	})

	t.Run("Should fail when the configuration directory does not exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")
		unsetKeys(t)
		t.Setenv("AWS_ACCESS_KEY_ID", "id")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, "ERROR setup") || !strings.Contains(out, "no such file or directory") {
			t.Errorf("exit %d, output %q; want exit 1 with a missing directory error", code, out)
		}
	})

	t.Run("Should fail listing the required keys when there is no .env and no input", func(t *testing.T) {
		dir := t.TempDir()
		unsetKeys(t)

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, "config: missing AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY") {
			t.Errorf("exit %d, output %q; want exit 1 with missing keys", code, out)
		}
	})

	t.Run("Should fail when .env cannot be parsed", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "WEBHOOK_URL\n")
		unsetKeys(t)

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, `ERROR setup error="config: .env line 1: want KEY=VALUE"`) {
			t.Errorf("exit %d, output %q; want exit 1 with the parse error of line 1", code, out)
		}
	})

	t.Run("Should fail when an answer is longer than a line can be", func(t *testing.T) {
		dir := t.TempDir()
		unsetKeys(t)

		out, code := cli.RunWithInput(t, dir, strings.Repeat("a", 70000)+"\n", "setup")

		if code != 1 || !strings.Contains(out, `ERROR setup error="config: prompt: bufio.Scanner: token too long"`) {
			t.Errorf("exit %d, output %q; want exit 1 with token too long", code, out)
		}
	})

	t.Run("Should fail when the .env.tmp left behind cannot be removed", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".env.tmp", "left"), 0o700); err != nil {
			t.Fatal(err)
		}
		unsetKeys(t)
		t.Setenv("AWS_ACCESS_KEY_ID", "id")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, `ERROR setup error="config: `) || !strings.Contains(out, ".env.tmp: directory not empty") {
			t.Errorf("exit %d, output %q; want exit 1 with directory not empty", code, out)
		}
	})

	t.Run("Should fail when .env.tmp cannot be written", func(t *testing.T) {
		dir := readOnlyDir(t)
		unsetKeys(t)
		t.Setenv("AWS_ACCESS_KEY_ID", "id")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, `ERROR setup error="config: `) || !strings.Contains(out, ".env.tmp: permission denied") {
			t.Errorf("exit %d, output %q; want exit 1 with permission denied", code, out)
		}
	})
}

func readOnlyDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // G302: a directory needs its execute bit to be entered
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // G302: a directory needs its execute bit to be removed

	return dir
}
