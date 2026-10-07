//go:build integration

package commands

import (
	"errors"
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

func writeAuthorizedKeys(t *testing.T, dir, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setRequiredKeys(t *testing.T) {
	t.Helper()

	unsetKeys(t)
	t.Setenv("CMS_URL", "https://cms.example.com")
	t.Setenv("CMS_AGENT_TOKEN", "cms-token")
	t.Setenv("CMS_AGENT_HOST_NAME", "vps-01")
	t.Setenv("EASYPANEL_TOKEN", "easypanel-token")
	t.Setenv("AWS_ACCESS_KEY_ID", "id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
}

func unsetKeys(t *testing.T) {
	t.Helper()

	t.Setenv("CMS_URL", "")
	t.Setenv("CMS_AGENT_TOKEN", "")
	t.Setenv("CMS_AGENT_HOST_NAME", "")
	t.Setenv("EASYPANEL_URL", "")
	t.Setenv("EASYPANEL_TOKEN", "")
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("INTERVAL_RECONCILE", "")
	t.Setenv("INTERVAL_REPORT", "")
	t.Setenv("DEPLOY_SSH_PUBLIC_KEY", "")
}

func TestSetupCommand(t *testing.T) {
	t.Run("Should write every key to .env from the existing values and the defaults", func(t *testing.T) {
		dir := t.TempDir()
		writeEnvContent(t, dir, "CMS_URL=https://cms.example.com\nCMS_AGENT_TOKEN=token\nCMS_AGENT_HOST_NAME=vps-01\nEASYPANEL_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\n")
		unsetKeys(t)
		want := "CMS_URL=https://cms.example.com\nCMS_AGENT_TOKEN=token\nCMS_AGENT_HOST_NAME=vps-01\nEASYPANEL_URL=http://127.0.0.1:3000\nEASYPANEL_TOKEN=token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=us-east-1\nINTERVAL_RECONCILE=5m\nINTERVAL_REPORT=10m\nDEPLOY_SSH_PUBLIC_KEY=\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should write every key to .env from the environment variables", func(t *testing.T) {
		dir := t.TempDir()
		publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr"
		t.Setenv("CMS_URL", "https://cms.example.com")
		t.Setenv("CMS_AGENT_TOKEN", "cms-token")
		t.Setenv("CMS_AGENT_HOST_NAME", "vps-01")
		t.Setenv("EASYPANEL_URL", "http://127.0.0.1:3001")
		t.Setenv("EASYPANEL_TOKEN", "easypanel-token")
		t.Setenv("AWS_ACCESS_KEY_ID", "id")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
		t.Setenv("AWS_REGION", "sa-east-1")
		t.Setenv("INTERVAL_RECONCILE", "1m")
		t.Setenv("INTERVAL_REPORT", "2m")
		t.Setenv("DEPLOY_SSH_PUBLIC_KEY", publicKey)
		want := "CMS_URL=https://cms.example.com\nCMS_AGENT_TOKEN=cms-token\nCMS_AGENT_HOST_NAME=vps-01\nEASYPANEL_URL=http://127.0.0.1:3001\nEASYPANEL_TOKEN=easypanel-token\nAWS_ACCESS_KEY_ID=id\nAWS_SECRET_ACCESS_KEY=secret\nAWS_REGION=sa-east-1\nINTERVAL_RECONCILE=1m\nINTERVAL_REPORT=2m\nDEPLOY_SSH_PUBLIC_KEY=" + publicKey + "\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(dir), ".env")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, .env %q; want exit 0 with status=ok and .env %q", code, out, content, want)
		}
	})

	t.Run("Should add the deploy key to authorized_keys keeping the other keys", func(t *testing.T) {
		dir := t.TempDir()
		sshDir := t.TempDir()
		publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICD1+7TNFPAfwc1N3JNIqifvcWV9Xy/JV9uBmvTdemqr"
		adminKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB admin@laptop\n"
		writeAuthorizedKeys(t, sshDir, adminKey)
		setRequiredKeys(t)
		t.Setenv("DEPLOY_SSH_PUBLIC_KEY", publicKey+" operator@laptop")
		t.Setenv("TALON_AUTHORIZED_KEYS", filepath.Join(sshDir, "authorized_keys"))
		want := adminKey + `restrict,command="export DEBIAN_FRONTEND=noninteractive; apt-get -qq update && apt-get -qq -y -o DPkg::Lock::Timeout=300 install --only-upgrade talon && talon version" ` + publicKey + " talon-deploy\n"

		out, code := cli.Run(t, dir, "setup")
		content, _ := fs.ReadFile(os.DirFS(sshDir), "authorized_keys")

		if code != 0 || !strings.Contains(out, "INFO setup status=ok") || string(content) != want {
			t.Errorf("exit %d, output %q, authorized_keys %q; want exit 0 with status=ok and authorized_keys %q", code, out, content, want)
		}
	})

	t.Run("Should fail without writing .env when the deploy key is not ssh-ed25519", func(t *testing.T) {
		dir := t.TempDir()
		setRequiredKeys(t)
		t.Setenv("DEPLOY_SSH_PUBLIC_KEY", "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB")

		out, code := cli.Run(t, dir, "setup")
		_, err := os.Stat(filepath.Join(dir, ".env"))

		if code != 1 || !strings.Contains(out, `ERROR setup error="deploy: DEPLOY_SSH_PUBLIC_KEY is not an ssh-ed25519 public key"`) || !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("exit %d, output %q, .env %v; want exit 1 with the deploy key error and no .env", code, out, err)
		}
	})

	t.Run("Should fail when the configuration directory does not exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "missing")

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, "ERROR setup") || !strings.Contains(out, "no such file or directory") {
			t.Errorf("exit %d, output %q; want exit 1 with a missing directory error", code, out)
		}
	})

	t.Run("Should fail listing the required keys when there is no .env and no input", func(t *testing.T) {
		dir := t.TempDir()
		unsetKeys(t)

		out, code := cli.Run(t, dir, "setup")

		if code != 1 || !strings.Contains(out, "config: missing CMS_URL, CMS_AGENT_TOKEN, CMS_AGENT_HOST_NAME, EASYPANEL_TOKEN") {
			t.Errorf("exit %d, output %q; want exit 1 with missing keys", code, out)
		}
	})
}
