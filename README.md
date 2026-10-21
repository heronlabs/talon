# 🦅 talon — Heron Labs agent for Easypanel hosts

[![CI][ci-badge]][ci-url]
[![Version][version-badge]][releases-url]
[![Go][go-badge]][go-url]
[![License: MIT][license-badge]][license-url]

> **Static Go binary** that runs on Heron Labs Easypanel hosts: it measures the host and each Easypanel service and, when configured, compares the host with its manifest and posts the report to a webhook.

One binary with five commands, `version`, `setup`, `doctor`, `report` and `daemon`. The package installs a systemd service that runs the daemon and a timer that keeps talon up to date from the Heron Labs apt repository.

## Contents

- [Install](#install)
- [Quick Start](#quick-start)
- [Commands](#commands)
  - [setup](#setup)
  - [doctor](#doctor)
  - [report](#report)
  - [daemon](#daemon)
- [Configuration](#configuration)
- [Reconcile](#reconcile)
- [Logs](#logs)
- [Service](#service)
  - [Automatic upgrades](#automatic-upgrades)
- [Releases](#releases)
- [Develop](#develop)
- [License](#license)

## Install

The host is an Ubuntu or Debian `amd64` machine. Add the Heron Labs apt repository and install the package:

```bash
sudo curl -fsSL https://apt.heronlabs.com.br/talon.gpg -o /etc/apt/keyrings/talon.gpg
echo "deb [signed-by=/etc/apt/keyrings/talon.gpg] https://apt.heronlabs.com.br stable main" | sudo tee /etc/apt/sources.list.d/talon.list
sudo apt update && sudo apt install talon
```

The binary lands in `/usr/bin/talon` and the package creates `/etc/talon`.

Without the repository, copy `talon_<version>_amd64.deb` from a release to the host and run `sudo dpkg -i` on it.

## Quick Start

```bash
talon version
sudo talon setup
sudo talon doctor
sudo talon report
sudo talon daemon
```

The commands run from any directory. `setup` writes the configuration, `doctor` checks it, and `report` and `daemon` read it without asking anything; [Configuration](#configuration) has the file and the variables.

## Commands

| Command | Description |
|---|---|
| `talon version` | Prints the version the binary was built with. |
| `talon setup` | Writes `/etc/talon/.env` with mode `600`, asking for every value that is not in the environment. |
| `talon doctor` | Checks the configuration, the Docker Engine and, with a token, the Easypanel API. |
| `talon report` | Measures the host and each Easypanel service and posts the report to the webhook. |
| `talon daemon` | Runs `report` at once and then every `INTERVAL_REPORT`, until SIGTERM or SIGINT. |

Any other command, or none, is an error.

### setup

`setup` goes through the keys of `.env.example` in order. For each one it takes the environment variable of the same name; when that is empty it asks on the terminal, showing a fallback in brackets:

```text
WEBHOOK_URL: https://webhook.example.com
AWS_REGION [us-east-1]:
```

The fallback is the value already in `.env` or, without one, the default of the key. The fallback of a token or secret key is shown as `[****]`, and when the input is a terminal what you type for it is not echoed; Ctrl+C turns the echo back on and still ends `setup`. An empty answer keeps the fallback. When `WEBHOOK_URL` ends up empty, `setup` does not ask for `WEBHOOK_TOKEN` and leaves it empty; when `MANIFEST_URL` ends up empty, it does not ask for `MANIFEST_TOKEN`, `EASYPANEL_URL` and `EASYPANEL_TOKEN` and leaves them empty. A key with no default is required, except `WEBHOOK_URL`, `WEBHOOK_TOKEN`, `MANIFEST_URL` and `MANIFEST_TOKEN`, and `EASYPANEL_TOKEN` is required only when `MANIFEST_URL` is set: when any required key ends up empty, `setup` lists them and writes nothing. Otherwise it replaces `.env` with the ten keys, mode `600`. Lines of the old `.env` that are not one of the keys are dropped.

### doctor

`doctor` checks that `/etc/talon/.env` exists with mode `600`, that the Docker Engine answers `GET /_ping` on its socket within 5 seconds and, when `EASYPANEL_TOKEN` is set, that the Easypanel API at `EASYPANEL_URL` answers `getUser` with the user of that token.

### report

`report` collects from the host its RAM and swap, total and used, from `/proc/meminfo`, the total and used size of the disk of `/` from statfs, the load over 1, 5 and 15 minutes from `/proc/loadavg` and the uptime from `/proc/uptime`.

From the Docker Engine it collects the size of the images and build cache, the Easypanel version in `/app/package.json` of the running `easypanel` container, with whether talon supports it (`2.36.0` and `2.37.0`), and, for each service of each Easypanel project, its state, `stopped` when it wants no replica, replicas, the tasks that failed or were rejected in the 24 hours before the measurement, its last deploy, limits, memory, CPU, network, block I/O and disk: its named volumes plus the files of `/etc/easypanel/projects/<project>/<service>`. It asks for the stats of up to 8 containers at once.

The last deploy is `lastDeployAt`, when the oldest task the Swarm still keeps of the version that runs was created, and `lastDeployState`, the state of the last update of the service; each is `null` without one. A recovery or a scale-up keeps `lastDeployAt`; scaling to 0 deletes the tasks, so after 0 and back it is when the service was scaled up, and a stopped service has it `null`.

Without a running `easypanel` container, or when the Docker Engine answers `404` for its `/app/package.json`, the Easypanel version and whether it is supported are null. A container that the Docker Engine no longer finds when talon asks for its stats, as during a deploy, is reported without them.

With `MANIFEST_URL` set, it also compares each project with the manifest of the host, as described in [Reconcile](#reconcile).

Then it posts the report as JSON to `WEBHOOK_URL`, with the hostname of the machine as its host and `Authorization: Bearer` when `WEBHOOK_TOKEN` is set. Without `WEBHOOK_URL` it sends nothing. Either way it logs the report as JSON in `payload`.

### daemon

`daemon` runs `report` at once and then every `INTERVAL_REPORT`, until SIGTERM or SIGINT. It reads `INTERVAL_REPORT` once, when it starts, so a change needs a restart; it reads `WEBHOOK_URL`, `WEBHOOK_TOKEN`, `MANIFEST_URL`, `MANIFEST_TOKEN`, `EASYPANEL_URL` and `EASYPANEL_TOKEN` on every cycle, so what `talon setup` writes counts from the next cycle. Without `.env` it runs on the defaults and sends nothing.

Each cycle runs for at most `INTERVAL_REPORT` or 60 seconds, whichever is shorter; a cycle that fails or runs past that is logged and the daemon goes on. On SIGTERM or SIGINT it starts no other cycle, waits for the one in flight and exits `0`. `INTERVAL_REPORT` is a duration such as `10m` or `90s`; when it is not one, or is not positive, the daemon exits `1`.

## Configuration

The configuration is `/etc/talon/.env`: `setup` writes it, `doctor` checks it, and `report` and `daemon` read it without asking anything. Three environment variables change where the commands look:

| Variable | Default | Sets |
|---|---|---|
| `TALON_CONFIG_DIR` | `/etc/talon` | The directory of `.env`. |
| `DOCKER_SOCKET` | `/var/run/docker.sock` | The socket of the Docker Engine that `doctor`, `report` and `daemon` talk to. |
| `TALON_HOST_ROOT` | `/` | The root of the host, where `report` and `daemon` read `proc` and `etc/easypanel/projects` and run statfs. |

Ctrl+C aborts `doctor` and `report`.

## Reconcile

With `MANIFEST_URL` set, `report`, and each cycle of `daemon`, compare the host with its manifest, and only read. Talon gets the manifest at `MANIFEST_URL`, with `Authorization: Bearer` when `MANIFEST_TOKEN` is set: the clients the host runs and the Easypanel schema of each. Then it reads the Easypanel API at `EASYPANEL_URL` with `Authorization: Bearer` and `EASYPANEL_TOKEN`, `GET /api/<operation>`: `listProjectsAndServices`, then `inspectAppService`, `inspectPostgresService` or `inspectMySQLService` for each app, postgres or mysql service it lists, and `listDomains` for each app. It writes nothing to Easypanel.

- **Projects** — the Swarm projects, in the order they first appear, then each client of the manifest without a Swarm service, in the order of the manifest. Each project lists its Swarm services, then each service its client declares without a Swarm service, `missing`, with every number `0` and its last deploy `null`. A service is `managed` when its client declares it.
- **Outcome** — the `reconcile` of a project is `unmanaged` when the project is not in the manifest, `converged` when its client declares at least one service and Easypanel holds every one, with the same type and the declared configuration, and `pending` otherwise, with `missingSecrets` `[]` and `error` `null`. A client that declares no service is `pending`, and a service Easypanel does not list is `pending`, not an error.
- **Comparison** — a declared configuration matches on the keys it declares, at any depth: a key Easypanel holds that the schema does not declare is ignored, a declared `null` matches `null` or an absent key, numbers compare as numbers, and the items of an array match one to one, in any order. `projectName`, `serviceName`, `env` and `password` are not compared. The domains of an app are compared in the shape of the schema: `port` and `internalProtocol` are those of the service the domain routes to.
- **Failures** — `agent.lastReconcile` is `{at, outcome, error}`: `at` is `measuredAt` and `outcome` is `ok`, or `failed` with the error of the manifest (`manifest: …`) or of Easypanel (`easypanel: …`), cut to 500 characters. When the manifest fails, talon does not call Easypanel; when Easypanel fails, the clients of the manifest, their declared services and `managed` are still reported. Either way every `reconcile` is `null` and the report is still sent.
- **Without a manifest** — without `MANIFEST_URL`, every `reconcile` and `lastReconcile` is `null` and `managed` is `false`.

## Logs

Each command logs one line with its result and exits `0` on success or `1` on failure; `daemon` logs the line of `report` on every cycle and one line when it stops:

```text
2026/10/04 20:05:40 INFO version version=0.3.0
2026/10/04 20:05:40 INFO setup status=ok
2026/10/04 20:05:40 ERROR setup error="config: missing AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY"
2026/10/04 20:05:40 INFO doctor status=ok
2026/10/04 20:05:40 ERROR doctor error="config: .env has mode 644, want 600"
2026/10/04 20:05:40 ERROR doctor error="easypanel: token refused"
2026/10/04 20:05:40 INFO report status=ok payload="{\"host\":\"vps-01\",...}"
2026/10/04 20:05:40 ERROR report error="webhook: unexpected status: 401"
2026/10/04 20:05:40 INFO daemon status=stopped
2026/10/04 20:05:40 ERROR daemon error="config: INTERVAL_REPORT: 0s is not positive"
2026/10/04 20:05:40 ERROR command not found command=foo
```

## Service

The package installs `talon.service`, which runs `/usr/bin/talon daemon` as root, after the network and Docker, and restarts it when it fails. It sees the file system read-only (`ProtectSystem=strict`): it only reads `/etc/talon`, `/proc` and `/etc/easypanel`, talks to the Docker socket and, with `MANIFEST_URL` set, reads the manifest and the Easypanel API. Install enables and starts it; its log is in `journalctl -u talon`.

On `systemctl stop`, the daemon waits for the cycle in flight, at most 60 seconds, and systemd gives it 90 seconds before it kills it. Every upgrade of the package restarts it, which is when the reports start to carry the new version; that restart holds `apt` until the cycle in flight ends. `sudo systemctl disable --now talon` turns it off and `sudo systemctl enable --now talon` turns it back on; upgrades keep that choice. `TALON_NO_UPDATES` affects only the upgrade timer.

### Automatic upgrades

The package installs `talon-upgrade.timer`, which runs every 15 minutes, with a random delay of up to 5 minutes. Each run updates only `/etc/apt/sources.list.d/talon.list`, then runs `apt-get install --only-upgrade talon`; without that file it does nothing. Its log is in `journalctl -u talon-upgrade`.

To install talon with the timer disabled:

```bash
sudo TALON_NO_UPDATES=1 apt install talon
```

`sudo systemctl disable --now talon-upgrade.timer` turns it off later and `sudo systemctl enable --now talon-upgrade.timer` turns it back on; upgrades keep that choice.

To roll back, disable the timer first, then run `sudo apt install talon=<version>`. Otherwise the next run upgrades talon again.

## Releases

A release is a tag with a changelog and two assets for `linux/amd64`: `talon-linux-amd64`, the static binary, and `talon_<version>_amd64.deb`, the package that installs it. The release workflow builds and attaches them; earlier releases have no assets. For any other platform, build from this repository.

Each release is also published to the apt repository at `https://apt.heronlabs.com.br`: its index is rebuilt from every version published so far and signed with the key whose public half is `talon.gpg` at the root of the repository.

## Develop

```bash
make build              # static binary in bin/talon
make package            # Linux binary and its .deb in dist/
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine, CMS and Easypanel API
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`make watch` rebuilds the binary into `/usr/local/bin` of the fake VPS, which comes before `/usr/bin` on the `PATH` of a shell there; `talon.service` runs `/usr/bin/talon`, the packaged binary, so the daemon does not see the rebuilt one.

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.

## License

[MIT](LICENSE) © Heron Labs

[ci-badge]: https://github.com/heronlabs/talon/actions/workflows/continuous-integration.yml/badge.svg
[ci-url]: https://github.com/heronlabs/talon/actions/workflows/continuous-integration.yml
[version-badge]: https://img.shields.io/github/v/release/heronlabs/talon
[releases-url]: https://github.com/heronlabs/talon/releases
[go-badge]: https://img.shields.io/badge/Go-1.27-00ADD8.svg
[go-url]: https://go.dev/
[license-badge]: https://img.shields.io/badge/License-MIT-blue.svg
[license-url]: ./LICENSE
