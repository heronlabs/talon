# vps-agent

Agent for Heron Labs Easypanel hosts. It runs on the host as a static binary and only makes outbound connections.

Today (phase 1) it measures the host (memory, swap, disk, load, uptime) and every Easypanel service (state, replicas, memory, CPU, disk, network, block I/O) and prints the report the CMS will receive. Sending the report and reconciling Easypanel from the CMS come in the next phases; see `docs/superpowers/specs/2026-10-01-vps-agent-design.md`.

## Install on a VPS (once)

The first install, and the migration from 0.1.0, run the `install.sh` asset of a release (0.2.0 or later):

```bash
sudo -i
read -rs GITHUB_TOKEN && export GITHUB_TOKEN
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
  "<install.sh asset URL of the release>" -o install.sh
AGENT_HOST_NAME=<infra-host name> DEPLOY_SSH_PUBLIC_KEY="ssh-ed25519 AAAA... vps-agent-deploy" bash install.sh
```

- `GITHUB_TOKEN` is a long-lived fine-grained token with only `Contents: read` on this repository. It is kept as `GITHUB_TOKEN` in `/etc/vps-agent/config.env` (`root:root 0600`), and only `update` uses it.
- `VERSION=1.2.3` installs that release instead of the latest.
- The script downloads the binary, `checksums.txt` and `checksums.txt.sig`, checks that `checksums.txt` is signed by the release key and that the binary's sha256 matches, and stops without touching anything otherwise. Then it runs `vps-agent install`, which copies the binary to `/usr/local/bin/vps-agent`, adds to `config.env` the known keys it is missing (`GITHUB_TOKEN`, `AGENT_HOST_NAME`, `CMS_URL`, ...; existing lines are never changed) and runs `vps-agent doctor`.
- `DEPLOY_SSH_PUBLIC_KEY` (optional, never written to `config.env`) prepares the `vps-agent-deploy` user used by the `Deploy` workflow: its `authorized_keys` holds only that key, as `restrict,command="sudo /usr/local/bin/vps-agent deploy"`, and `/etc/sudoers.d/vps-agent-deploy` lets it run only `vps-agent deploy`.

### Release key

Releases are signed with an ed25519 key. The public key is `internal/infrastructure/release/release-signing.pub`; it is built into the binary and written literally in `install.sh`. Its fingerprint (sha256 of the `.pub` file):

```
15044725045f4917db2c097da0483fa3475848732f3656ef51221a1922ecc217
```

The first install trusts the `install.sh` it downloads; to check it before running, compare the `RELEASE_PUBLIC_KEY` block in it with that fingerprint (`sed -n "/^readonly RELEASE_PUBLIC_KEY='/,/^'/p" install.sh | sed "1s/^readonly RELEASE_PUBLIC_KEY='//;\$d" | sha256sum`). From then on the installed binary only accepts releases signed by the key it carries.

### Keys and rotation

`scripts/generate-keys.sh` (run by the operator, needs `gh` logged in and, for the release key, OpenSSL 3) generates the keys, stores the private halves as GitHub Environment secrets and deletes them locally; only the public halves are written, to `~/vps-agent-keys`:

- `scripts/generate-keys.sh deploy` rotates the deploy SSH key (`DEPLOY_SSH_KEY` of `production`) at any time. Rerun `install.sh` (or `vps-agent install`) on each VPS with the new `deploy.pub` as `DEPLOY_SSH_PUBLIC_KEY`.
- `scripts/generate-keys.sh all` also replaces the release signing key (`RELEASE_SIGNING_KEY` of `release`). Its `release-signing.pub` goes, in one PR, to `internal/infrastructure/release/release-signing.pub`, the literal key in `scripts/install.sh` and the fingerprint above.

Rotating the release key on hosts that already run vps-agent: a release must be signed by the key the installed binary embeds. Do not run `all` then. First ship a release signed with the current key that embeds the new public key, update every host, and only then replace `RELEASE_SIGNING_KEY`.

## Update

From GitHub, run the **Deploy** workflow (Actions → Deploy → Run workflow) with a version like `1.2.3`. It connects to every host in the `production` environment's `DEPLOY_HOSTS` over SSH as `vps-agent-deploy` and runs `vps-agent deploy` there; the job fails if the update fails. The same workflow redeploys or rolls back: give it any published version from 0.2.0 on. The CD never deploys by itself.

By hand, on the VPS:

```bash
sudo vps-agent update                  # latest release
sudo vps-agent update --version 1.2.3  # exact release, also to go back
```

`update` takes a lock, downloads the release, checks the signature and the sha256, swaps the binary atomically (keeping `/usr/local/bin/vps-agent.previous`) and runs the new binary's `doctor`. If `doctor` fails, the previous binary is restored and the command exits 1. Exit codes: `0` installed or already current, `1` any failure (rollback included), `2` invalid usage.

## Use

```bash
sudo vps-agent version
sudo vps-agent doctor          # config and Docker socket
sudo vps-agent report --once   # print one report as JSON
```

## Develop

See `CONTRIBUTING.md` and `AGENTS.md`.
