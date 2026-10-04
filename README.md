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

A release is a tag with a changelog. No binary is published; build it from this repository.

## Develop

```bash
make build              # static binary in bin/talon
make test               # unit tests with coverage
make test:integration   # runs the real binary against a fake Docker Engine
make up                 # fake VPS: Ubuntu, systemd, Docker and Easypanel in a container
```

`CONTRIBUTING.md` has the setup and the workflow. `AGENTS.md` has the code map and the conventions.
