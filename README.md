<p align="center">
  <img src=".github/assets/logo.png" alt="talon" width="160">
</p>

# talon

The Heron Labs agent for Easypanel hosts. A static Go binary with three commands.

## Commands

| Command | What it does |
|---|---|
| `talon version` | Prints the version the binary was built with. |
| `talon setup` | Writes `/etc/talon/.env` with mode `600`, asking for every value that is not in the environment. |
| `talon doctor` | Checks that `/etc/talon/.env` exists with mode `600` and that the Docker Engine answers `GET /_ping` on its socket within 5 seconds. |

Any other command, or none, is an error.

## Install

Each release carries `talon_<version>_amd64.deb`. Copy it to the host and install it; the binary lands in `/usr/bin/talon` and the package creates `/etc/talon`:

```bash
sudo dpkg -i talon_<version>_amd64.deb
```

## Use

```bash
talon version
sudo talon setup
sudo talon doctor
```

The commands run from any directory. The configuration is `/etc/talon/.env`: `setup` writes it and `doctor` reads it; set `TALON_CONFIG_DIR` to use another directory. `doctor` talks to the Docker Engine at `/var/run/docker.sock`; set `DOCKER_SOCKET` to use another socket.

### Setup

`setup` goes through the keys of `.env.example` in order. For each one it takes the environment variable of the same name; when that is empty it asks on the terminal, showing a fallback in brackets:

```text
CMS_URL: https://cms.example.com
AWS_REGION [us-east-1]:
```

The fallback is the value already in `.env` or, without one, the default of the key. The fallback of a token or secret key is shown as `[****]`; what you type is echoed. An empty answer keeps the fallback. A key with no default is required: when any of them ends up empty, `setup` lists them and writes nothing. Otherwise it replaces `.env` with the ten keys, mode `600`. Lines of the old `.env` that are not one of the keys are dropped.

Each command logs one line with its result and exits `0` on success or `1` on failure:

```text
2026/10/04 20:05:40 INFO version version=0.3.0
2026/10/04 20:05:40 INFO setup status=ok
2026/10/04 20:05:40 ERROR setup error="config: missing CMS_URL, CMS_AGENT_TOKEN"
2026/10/04 20:05:40 INFO doctor status=ok
2026/10/04 20:05:40 ERROR doctor error="config: .env has mode 644, want 600"
2026/10/04 20:05:40 ERROR command not found command=foo
```

## Releases

A release is a tag with a changelog and two assets for `linux/amd64`: `talon-linux-amd64`, the static binary, and `talon_<version>_amd64.deb`, the package that installs it. The release workflow builds and attaches them; earlier releases have no assets. For any other platform, build from this repository.

## Develop

```bash
make build              # static binary in bin/talon
make package            # Linux binary and its .deb in dist/
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.
