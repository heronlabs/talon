# vps-agent

Go agent that runs as a systemd service on an Easypanel VPS. Phase 1 (this release line) measures the host and every Swarm service and prints the CMS report contract; later phases send it to the CMS and reconcile Easypanel from a CMS manifest. Since 0.2.0 it installs and updates itself from signed releases (`install`, `update`, `deploy`). Design: `docs/superpowers/specs/2026-10-01-vps-agent-design.md` and `docs/superpowers/specs/2026-10-02-vps-agent-update-design.md` (install, update, deploy).

## Map

| Path | Holds |
|---|---|
| `cmd/vps-agent/main.go` | The app: flags, signals, builds the graph, runs one use case. No rules here. |
| `internal/core/domain/` | Entities and pure rules (`BuildReport`, `BindMountPaths`, `Version`/`UpdatePlan`, `VerifyRelease`, `MergeConfig`, `AuthorizedKeyLine`). No I/O, no clock. |
| `internal/core/port/` | Interfaces the use cases need: `HostMetrics`, `DirSizer`, `Docker`, `ReportSink`, `Clock`, `ConfigValidator`, and for install/update `ReleaseSource`, `BinaryStore`, `HealthCheck`, `UpdateLock`, `ConfigStore`, `ConfigSyntax`, `SystemAccounts`, `Executable`. |
| `internal/core/usecase/` | `report` (collect and publish), `doctor` (ordered checks), `install` (binary, `config.env` merge, deploy user), `update` (signed release, swap, doctor, rollback), `deploy` (validates the SSH request, runs `update`). |
| `internal/infrastructure/` | Adapters: `hostfs` (/proc, statfs, dir sizes; binary swap, atomic `config.env` writes, `flock` update lock), `docker` (Engine API over the unix socket, no SDK), `cms` (contract DTOs, printer), `config` (`/etc/vps-agent/config.env`, `GITHUB_TOKEN` included), `github` (releases API), `release` (embedded `release-signing.pub`), `process` (`doctor` subprocess, deploy user via `useradd`/`passwd`/`visudo`), `atomicfile` (temp file, fsync, rename; used for the binary, `config.env`, `authorized_keys`, sudoers), `clock`. |
| `internal/testkit/` | The only assertions (`Equal`, `ErrorIs`), setup helpers (`Must`, `MustDo`) and hand-written fakes (`fake/`). |
| `internal/tools/testshape/` | Lint analyzer: one scenario and one assertion per test. |
| `test/integration/` | End-to-end tests (`//go:build integration`) against a fake engine on a unix socket and a fake GitHub API. |
| `test/installer/` | `install.sh` tests in `ubuntu:24.04` containers with a fake `curl` and a throwaway signing key (`make test-installer`). |
| `scripts/install.sh` | Bootstrap for the first install and the 0.1.0 migration: verifies the signed release, runs `vps-agent install`. |
| `.github/workflows/deploy.yml` | Manual `Deploy` workflow: SSH to each `DEPLOY_HOSTS` entry as `vps-agent-deploy`, which can only run `vps-agent deploy <version>`. |

Import direction, enforced by `depguard`: `cmd → usecase → port → domain`, `cmd → infrastructure → (domain, port)`. Infrastructure never imports use cases; use cases never import infrastructure.

## Commands

| Command | Does |
|---|---|
| `make hooks` | Point git at `.githooks/` |
| `make build` | Static binary in `bin/` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make lint` | gofmt, go vet, golangci-lint, testshape, go mod tidy -diff, shellcheck, actionlint. Also run `GOOS=linux golangci-lint run`: CI lints on Linux and Linux-only files are invisible to a macOS lint. |
| `make test` | Unit tests with race detector; fails below 100% statement coverage of `internal/` |
| `make test-integration` | Integration tests |
| `make test-installer` | `install.sh` scenarios in Docker (`ubuntu:24.04`) |
| `make test-mutation` | gremlins, efficacy must be 100%. Run it on Linux with a cold cache (`docker run --rm -v "$PWD":/src -w /src golang:1.27 make test-mutation`); a warm cache makes mutants time out spuriously. |
| `make audit` | govulncheck |

`golangci-lint` and `actionlint` come from brew locally; gremlins and govulncheck are `go tool`s pinned in `go.mod`.

## Conventions

- **Tests:** one `Test<Unit>_<Scenario>` per scenario. No table tests, no `for`/`range`, no `t.Run`. Exactly one assertion, `testkit.Equal` (whole value, go-cmp diff) or `testkit.ErrorIs`, as the last statement. Setup failures use `testkit.Must`/`MustDo`, which are not assertions. Arrange, act, assert separated by blank lines. `make lint` enforces this.
- **Ports** are small interfaces in `core/port`; adapters satisfy them implicitly. Dependencies enter through constructors; only `main.go` wires.
- **DTOs stay in adapters.** Engine, CMS and config shapes never reach `core`.
- **Errors** wrap with `%w` and context; sentinels (`ErrMalformed`, `ErrInvalid`, `ErrUnexpectedStatus`) are what tests match.
- **Logs:** JSON via `log/slog`, `logId` as `<domain>.<event>`, flat fields, `errorName`/`errorMessage`, never a token or env value.
- **Mutants:** a surviving equivalent mutant is fixed by refactoring the code, not by excluding it.
- **Release key:** `internal/infrastructure/release/release-signing.pub` and the `RELEASE_PUBLIC_KEY` literal in `scripts/install.sh` must stay byte-equal (a test checks it). The private key is only the `RELEASE_SIGNING_KEY` secret of the `release` environment.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- From the first release on, every change goes through `/supera:ship`.
