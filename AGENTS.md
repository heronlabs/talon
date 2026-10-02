# vps-agent

Go agent that runs as a systemd service on an Easypanel VPS. Phase 1 (this release line) measures the host and every Swarm service and prints the CMS report contract; later phases send it to the CMS and reconcile Easypanel from a CMS manifest. Design: `docs/superpowers/specs/2026-10-01-vps-agent-design.md`.

## Map

| Path | Holds |
|---|---|
| `cmd/vps-agent/main.go` | The app: flags, signals, builds the graph, runs one use case. No rules here. |
| `internal/core/domain/` | Entities and pure rules (`BuildReport`, `BindMountPaths`). No I/O, no clock. |
| `internal/core/port/` | Interfaces the use cases need: `HostMetrics`, `DirSizer`, `Docker`, `ReportSink`, `Clock`, `ConfigValidator`. |
| `internal/core/usecase/` | `report` (collect and publish), `doctor` (ordered checks). |
| `internal/infrastructure/` | Adapters: `hostfs` (/proc, statfs, dir sizes), `docker` (Engine API over the unix socket, no SDK), `cms` (contract DTOs, printer), `config` (`/etc/vps-agent/config.env`), `clock`. |
| `internal/testkit/` | The only assertions (`Equal`, `ErrorIs`), setup helpers (`Must`, `MustDo`) and hand-written fakes (`fake/`). |
| `internal/tools/testshape/` | Lint analyzer: one scenario and one assertion per test. |
| `test/integration/` | End-to-end tests (`//go:build integration`) against a fake engine on a unix socket. |
| `scripts/install.sh` | Installer/updater run on the VPS. |

Import direction, enforced by `depguard`: `cmd → usecase → port → domain`, `cmd → infrastructure → (domain, port)`. Infrastructure never imports use cases; use cases never import infrastructure.

## Commands

| Command | Does |
|---|---|
| `make hooks` | Point git at `.githooks/` |
| `make build` | Static binary in `bin/` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make lint` | gofmt, go vet, golangci-lint, testshape, go mod tidy -diff, shellcheck, actionlint |
| `make test` | Unit tests with race detector; fails below 100% statement coverage of `internal/` |
| `make test-integration` | Integration tests |
| `make test-mutation` | gremlins, efficacy must be 100% |
| `make audit` | govulncheck |

`golangci-lint` and `actionlint` come from brew locally; gremlins and govulncheck are `go tool`s pinned in `go.mod`.

## Conventions

- **Tests:** one `Test<Unit>_<Scenario>` per scenario. No table tests, no `for`/`range`, no `t.Run`. Exactly one assertion, `testkit.Equal` (whole value, go-cmp diff) or `testkit.ErrorIs`, as the last statement. Setup failures use `testkit.Must`/`MustDo`, which are not assertions. Arrange, act, assert separated by blank lines. `make lint` enforces this.
- **Ports** are small interfaces in `core/port`; adapters satisfy them implicitly. Dependencies enter through constructors; only `main.go` wires.
- **DTOs stay in adapters.** Engine, CMS and config shapes never reach `core`.
- **Errors** wrap with `%w` and context; sentinels (`ErrMalformed`, `ErrInvalid`, `ErrUnexpectedStatus`) are what tests match.
- **Logs:** JSON via `log/slog`, `logId` as `<domain>.<event>`, flat fields, `errorName`/`errorMessage`, never a token or env value.
- **Mutants:** a surviving equivalent mutant is fixed by refactoring the code, not by excluding it.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- From the first release on, every change goes through `/supera:ship`.
