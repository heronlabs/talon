# talon

Agent for Heron Labs Easypanel hosts. It runs on the host as a static binary and only makes outbound connections.

Today (phase 1) it measures the host (memory, swap, disk, load, uptime) and every Easypanel service (state, replicas, memory, CPU, disk, network, block I/O) and prints the report the CMS will receive. Sending the report and reconciling Easypanel from the CMS come in the next phases; see `docs/superpowers/specs/2026-10-01-vps-agent-design.md`.

## Install or update on a VPS

```bash
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
  "<install.sh asset URL of the release>" -o install.sh
sudo GITHUB_TOKEN=... AGENT_HOST_NAME=<infra-host name> bash install.sh
```

`GITHUB_TOKEN` is a fine-grained token with `Contents: read` on this repository. The script verifies the sha256, installs `/usr/local/bin/talon`, writes `/etc/talon/config.env` (0600) only on the first install and runs `talon doctor`.

## Use

```bash
sudo talon version
sudo talon doctor          # config and Docker socket
sudo talon report --once   # print one report as JSON
```

## Develop

See `CONTRIBUTING.md` and `AGENTS.md`.
