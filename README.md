<p align="center">
  <img src=".github/assets/logo.png" alt="talon" width="160">
</p>

# talon

The Heron Labs agent for Easypanel hosts. A static Go binary with five commands.

## Commands

| Command | What it does |
|---|---|
| `talon version` | Prints the version the binary was built with. |
| `talon setup` | Writes `/etc/talon/.env` with mode `600`, asking for every value that is not in the environment. |
| `talon doctor` | Checks that `/etc/talon/.env` exists with mode `600` and that the Docker Engine answers `GET /_ping` on its socket within 5 seconds. |
| `talon report` | Collects from the host its RAM and swap, total and used, from `/proc/meminfo`, the total and used size of the disk of `/` from statfs, the load over 1, 5 and 15 minutes from `/proc/loadavg` and the uptime from `/proc/uptime`; from the Docker Engine the size of the images and build cache, the Easypanel version in `/app/package.json` of the running `easypanel` container, with whether talon supports it (`2.36.0`), and, for each service of each Easypanel project, its state, replicas, failed tasks, limits, memory, CPU, network, block I/O and disk: its named volumes plus the files of `/etc/easypanel/projects/<project>/<service>`. Then it posts the report as JSON to `WEBHOOK_URL`, with the hostname of the machine as its host and `Authorization: Bearer` when `WEBHOOK_TOKEN` is set. Without `WEBHOOK_URL` it sends nothing. Either way it logs the report as JSON in `payload`. Without a running `easypanel` container, or when the Docker Engine answers `404` for its `/app/package.json`, the Easypanel version and whether it is supported are null. A container that the Docker Engine no longer finds when talon asks for its stats, as during a deploy, is reported without them. It asks for the stats of up to 8 containers at once. |
| `talon daemon` | Runs `report` at once and then every `INTERVAL_REPORT`, until SIGTERM or SIGINT. It reads `INTERVAL_REPORT` once, when it starts, so a change needs a restart; it reads `WEBHOOK_URL` and `WEBHOOK_TOKEN` on every cycle, so what `talon setup` writes counts from the next cycle. Without `.env` it runs on the defaults and sends nothing. Each cycle runs for at most `INTERVAL_REPORT` or 60 seconds, whichever is shorter; a cycle that fails or runs past that is logged and the daemon goes on. On SIGTERM or SIGINT it starts no other cycle, waits for the one in flight and exits `0`. `INTERVAL_REPORT` is a duration such as `10m` or `90s`; when it is not one, or is not positive, the daemon exits `1`. |

Any other command, or none, is an error.

## Install

The host is an Ubuntu or Debian `amd64` machine. Add the Heron Labs apt repository and install the package:

```bash
sudo curl -fsSL https://apt.heronlabs.com.br/talon.gpg -o /etc/apt/keyrings/talon.gpg
echo "deb [signed-by=/etc/apt/keyrings/talon.gpg] https://apt.heronlabs.com.br stable main" | sudo tee /etc/apt/sources.list.d/talon.list
sudo apt update && sudo apt install talon
```

The binary lands in `/usr/bin/talon` and the package creates `/etc/talon`.

Without the repository, copy `talon_<version>_amd64.deb` from a release to the host and run `sudo dpkg -i` on it.

## Automatic upgrades

The package installs `talon-upgrade.timer`, which runs every 15 minutes, with a random delay of up to 5 minutes. Each run updates only `/etc/apt/sources.list.d/talon.list`, then runs `apt-get install --only-upgrade talon`; without that file it does nothing. Its log is in `journalctl -u talon-upgrade`.

To install talon with the timer disabled:

```bash
sudo TALON_NO_UPDATES=1 apt install talon
```

`sudo systemctl disable --now talon-upgrade.timer` turns it off later and `sudo systemctl enable --now talon-upgrade.timer` turns it back on; upgrades keep that choice.

To roll back, disable the timer first, then run `sudo apt install talon=<version>`. Otherwise the next run upgrades talon again.

## The daemon

The package installs `talon.service`, which runs `/usr/bin/talon daemon` as root, after the network and Docker, and restarts it when it fails. It sees the file system read-only (`ProtectSystem=strict`): it only reads `/etc/talon`, `/proc` and `/etc/easypanel` and talks to the Docker socket. Install enables and starts it; its log is in `journalctl -u talon`.

On `systemctl stop`, the daemon waits for the cycle in flight, at most 60 seconds, and systemd gives it 90 seconds before it kills it. Every upgrade of the package restarts it, which is when the reports start to carry the new version; that restart holds `apt` until the cycle in flight ends. `sudo systemctl disable --now talon` turns it off and `sudo systemctl enable --now talon` turns it back on; upgrades keep that choice. `TALON_NO_UPDATES` affects only the upgrade timer.

## Use

```bash
talon version
sudo talon setup
sudo talon doctor
sudo talon report
sudo talon daemon
```

The commands run from any directory. The configuration is `/etc/talon/.env`: `setup` writes it, `doctor` checks it, and `report` and `daemon` read it without asking anything; set `TALON_CONFIG_DIR` to use another directory. `doctor`, `report` and `daemon` talk to the Docker Engine at `/var/run/docker.sock`, and Ctrl+C aborts `doctor` and `report`; set `DOCKER_SOCKET` to use another socket. `report` and `daemon` read `proc` and `etc/easypanel/projects` under the root of the host, `/`, and run statfs on it; set `TALON_HOST_ROOT` to use another root.

### Setup

`setup` goes through the keys of `.env.example` in order. For each one it takes the environment variable of the same name; when that is empty it asks on the terminal, showing a fallback in brackets:

```text
WEBHOOK_URL: https://webhook.example.com
AWS_REGION [us-east-1]:
```

The fallback is the value already in `.env` or, without one, the default of the key. The fallback of a token or secret key is shown as `[****]`; what you type is echoed. An empty answer keeps the fallback. When `WEBHOOK_URL` ends up empty, `setup` does not ask for `WEBHOOK_TOKEN` and leaves it empty. A key with no default is required, except `WEBHOOK_URL` and `WEBHOOK_TOKEN`: when any required key ends up empty, `setup` lists them and writes nothing. Otherwise it replaces `.env` with the six keys, mode `600`. Lines of the old `.env` that are not one of the keys are dropped.

Each command logs one line with its result and exits `0` on success or `1` on failure; `daemon` logs the line of `report` on every cycle and one line when it stops:

```text
2026/10/04 20:05:40 INFO version version=0.3.0
2026/10/04 20:05:40 INFO setup status=ok
2026/10/04 20:05:40 ERROR setup error="config: missing AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY"
2026/10/04 20:05:40 INFO doctor status=ok
2026/10/04 20:05:40 ERROR doctor error="config: .env has mode 644, want 600"
2026/10/04 20:05:40 INFO report status=ok payload="{\"host\":\"vps-01\",...}"
2026/10/04 20:05:40 ERROR report error="webhook: unexpected status: 401"
2026/10/04 20:05:40 INFO daemon status=stopped
2026/10/04 20:05:40 ERROR daemon error="config: INTERVAL_REPORT: 0s is not positive"
2026/10/04 20:05:40 ERROR command not found command=foo
```

## Releases

A release is a tag with a changelog and two assets for `linux/amd64`: `talon-linux-amd64`, the static binary, and `talon_<version>_amd64.deb`, the package that installs it. The release workflow builds and attaches them; earlier releases have no assets. For any other platform, build from this repository.

Each release is also published to the apt repository at `https://apt.heronlabs.com.br`: its index is rebuilt from every version published so far and signed with the key whose public half is `talon.gpg` at the root of the repository.

## Develop

```bash
make build              # static binary in bin/talon
make package            # Linux binary and its .deb in dist/
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`make watch` rebuilds the binary into `/usr/local/bin` of the fake VPS, which comes before `/usr/bin` on the `PATH` of a shell there; `talon.service` runs `/usr/bin/talon`, the packaged binary, so the daemon does not see the rebuilt one.

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.
