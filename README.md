<p align="center">
  <img src=".github/assets/logo.png" alt="talon" width="160">
</p>

# talon

The Heron Labs agent for Easypanel hosts. A static Go binary with two commands.

## Commands

| Command | What it does |
|---|---|
| `talon version` | Prints the version read from `version.txt`, or `0.0.0` when the file cannot be read. |
| `talon doctor` | Checks that `.env` exists with mode `600` and that the Docker Engine answers `GET /_ping` on its socket within 5 seconds. |

Any other command, or none, is an error.

## Install

Each release carries one asset, `talon-linux-amd64`. Run these steps as `root` on the VPS, in `bash`; they need only `curl`.

The repository is private, so the download needs a GitHub token that can read it: a fine-grained personal access token with `Contents: read` on `heronlabs/talon`. Keep it in the environment only. `read -s` keeps it out of the shell history.

```bash
read -rsp 'GitHub token: ' GITHUB_TOKEN && export GITHUB_TOKEN && echo
```

Download the binary of the latest release and install it:

```bash
RELEASE="$(curl -fsS -H "Authorization: Bearer $GITHUB_TOKEN" \
  https://api.github.com/repos/heronlabs/talon/releases/latest)"
ASSET_URL="$(printf '%s' "$RELEASE" \
  | grep -oE 'https://api\.github\.com/repos/heronlabs/talon/releases/assets/[0-9]+' \
  | head -n 1)"
VERSION="$(printf '%s' "$RELEASE" \
  | grep -oE '"tag_name": *"v[^"]+"' \
  | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"

curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H 'Accept: application/octet-stream' \
  -o /tmp/talon "$ASSET_URL"

install -m 755 /tmp/talon /usr/local/bin/talon
rm /tmp/talon
```

Create the working directory with its `.env` and `version.txt`:

```bash
mkdir -p /opt/talon
touch /opt/talon/.env
chmod 600 /opt/talon/.env
echo "$VERSION" > /opt/talon/version.txt
```

Check the installation, then drop the token:

```bash
cd /opt/talon
talon version
talon doctor
unset GITHUB_TOKEN
```

`talon version` must log the version of the release and `talon doctor` must log `status=ok`.

## Use

```bash
talon version
talon doctor
```

Both commands read from the working directory: `version` reads `version.txt` and `doctor` reads `.env`. `doctor` talks to the Docker Engine at `/var/run/docker.sock`; set `DOCKER_SOCKET` to use another socket.

Each command logs one line with its result and exits `0` on success or `1` on failure:

```text
2026/10/04 20:05:40 INFO version version=0.3.0
2026/10/04 20:05:40 INFO doctor status=ok
2026/10/04 20:05:40 ERROR doctor error="config: .env has mode 644, want 600"
2026/10/04 20:05:40 ERROR command not found command=foo
```

## Releases

A release is a tag with a changelog and one asset, `talon-linux-amd64`: the static binary for `linux/amd64`. The release workflow builds and attaches it; earlier releases have no asset. For any other platform, build it from this repository.

## Develop

```bash
make build              # static binary in bin/talon
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.
