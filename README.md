# talon

The Heron Labs agent for Easypanel hosts.

A heron holds what it catches with its talons. `talon` is the grip Heron Labs keeps on each VPS: one static Go binary on the host that checks it, measures it and, in time, shapes it to match what the CMS says should be running there. It only makes outbound connections; nothing calls the agent and it opens no port.

## Where it stands

The project was rebuilt around a small command structure. Two commands work today; the others have their place reserved in the code and arrive one at a time, each fully tested before the next starts.

| Command | State | What it does |
|---|---|---|
| `talon version` | Available | Prints the version read from `version.txt`. |
| `talon doctor` | Available | Checks that `.env` exists with mode `600` and that the Docker Engine answers on its socket. |
| `talon report` | Planned | Measures the host and every Easypanel service and sends the numbers to the CMS. |
| `talon deploy` | Planned | Creates on Easypanel the projects and services the CMS manifest asks for. |
| `talon install` | Planned | First installation on a VPS. |
| `talon update` | Planned | Installs a signed release and goes back to the previous one if `doctor` fails. |

There is no installer or release binary yet: releases are tags with a changelog. Until `install` lands, the agent runs from a build of this repository.

## Use

```bash
talon version
talon doctor
```

Both commands read from the working directory: `version` reads `version.txt` and `doctor` reads `.env`. `doctor` talks to the Docker Engine at `/var/run/docker.sock`; set `DOCKER_SOCKET` to point it somewhere else.

Each command logs one line with its result and exits `0` on success or `1` on failure:

```text
INFO version version=0.3.0
INFO doctor status=ok
ERROR doctor error="config: .env has mode 644, want 600"
```

## Where it is going

The target is a VPS that builds and reports itself. Buy a VPS with Easypanel, install the agent once, and from then on:

- **Deploy.** The agent reads a manifest from the CMS and creates the projects and services that are missing, with their memory and CPU limits. The manifest is the desired state and the agent converges to it.
- **Report.** At a fixed interval the agent measures the host (memory, swap, disk, load, uptime) and every service (state, replicas, memory, CPU, disk, network, block I/O) and sends the numbers to the CMS, which shows how much of each limit is in use.
- **Update.** A new release reaches the hosts only when an operator asks for it, only if its signature checks out, and it rolls back by itself when the new binary fails `doctor`.

The rules that shape those features:

- **Outbound only.** The agent calls the CMS; nothing calls the agent.
- **It never deletes.** It creates what is missing and adjusts limits. Removing a project or a service is always a human action.
- **It does not redeploy applications.** The image and the environment of an existing service belong to that application's own delivery pipeline.
- **Secrets stay where they are.** The CMS holds references; the agent reads each value from the secret store of the client that owns it.
- **Unknown Easypanel, read only.** On an Easypanel version the agent was not tested against, it measures and does not write.
- **Numbers, not opinions.** The agent sends raw measurements; percentages and alerts are the CMS's job.

## Develop

```bash
make build              # static binary in bin/talon
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions every change follows.
