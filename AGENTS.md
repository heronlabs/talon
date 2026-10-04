# talon

Go agent for Heron Labs Easypanel hosts: one static binary that runs on the VPS. Today it has `version` and `doctor`. Next come `report` (measure the host and every Easypanel service, send to the CMS), `deploy` (create on Easypanel what the CMS manifest asks for), `install` and `update` (signed releases with rollback). `README.md` describes the product and where it is going.

The code was rebuilt from a small core and grows one command at a time. Empty files (`package` line only) are placeholders that mark where a planned piece goes: fill them, do not create a parallel file.

## Map

| Path | Holds |
|---|---|
| `src/main.go` | The entry point: picks the command from `os.Args[1]`, builds its dependencies, logs the result, sets the exit code. No rules here. |
| `src/application/` | One command per file, `<name>_command.go`. `VersionCommand` and `DoctorCommand` exist; `report`, `deploy`, `install` and `update` are placeholders. |
| `src/core/` | Entities (`entity_<name>.go`) and pure rules (`service_<name>.go`) for report and deploy. Placeholders today. No I/O. |
| `src/infrastructure/` | One package per outside system. `docker`: Engine API over the unix socket with `net/http`, no SDK. `cms`, `easypanel` and `environment` are placeholders. |
| `tests/cli/` | Harness that runs the built binary (`TALON_BIN`) in a directory and returns output and exit code. |
| `tests/commands/` | Integration tests (`//go:build integration`), one file per command, against the real binary, a temp directory and a fake Docker Engine on a unix socket. |
| `.docker/` | The fake VPS: Ubuntu 22.04 with systemd, Docker Engine and Easypanel. The binary lives in `/usr/local/bin/talon`, the working directory is `/opt/talon`. |
| `.github/workflows/` | `continuous-integration.yml` (pull requests) and `continuous-deployment.yml` (release on `main`). |
| `.githooks/pre-commit` | gofmt, build, vet and unit tests on staged Go changes. |

Import rules, enforced by `depguard` in `.golangci.yml`: `core` imports nothing from `application` or `infrastructure`; `infrastructure` never imports `application`. Commands in `application` depend on interfaces that each adapter package exports beside its implementation (`docker.HealthCheck` and `docker.HealthCheckService`).

Design notes are local only, under `docs/superpowers/` (gitignored). They were written before the rename and the rebuild, so they still say `vps-agent` and describe an older layout: use them for intent, not for paths.

## Commands

| Command | Does |
|---|---|
| `make build` | Static binary in `bin/talon` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make lint` | golangci-lint (with gofmt and goimports), actionlint, shellcheck, `go mod tidy -diff` |
| `make lint:fix` | golangci-lint with `--fix` |
| `make test` | Unit tests with the race detector; coverage in `.coverage/unit/` |
| `make test:integration` | Builds an instrumented binary and runs `tests/`; coverage in `.coverage/integration/` |
| `make test:mutation` | gremlins over `src/`; report in `.coverage/mutation/mutation.json` |
| `make audit` | govulncheck |
| `make up` / `make down` | Start and stop the fake VPS |
| `make watch` | Rebuild the Linux binary into `bin/linux` on every `.go` change; the fake VPS runs it from there |

`golangci-lint` and `shellcheck` come from brew locally. `actionlint`, `govulncheck`, `gremlins` and `air` are `go tool`s pinned in `go.mod`. The target names use a colon (`test:integration`), not a dash.

## Conventions

- **Tests:** one `Test<Type>` per source file, in the same package (`version_command.go` and `TestVersionCommand` in `version_command_test.go`), and one `t.Run("Should ...")` per scenario. No table tests, no `for`/`range`. Arrange, act and assert are separated by blank lines, and the single assertion is the last statement: one `if` with one `t.Errorf`. Setup failures use `t.Fatal` inside helpers marked `t.Helper()`; they are not assertions.
- **Fakes** are written by hand in the test file that uses them (`fakeHealthCheck`). No mocking library. The filesystem enters as `fs.FS` and tests pass `fstest.MapFS`.
- **Integration tests** assert the exit code and the output of the real binary in one condition.
- **Coverage and mutation:** the aim is 100% statement coverage of `src/` outside `main.go` and every mutant killed. A surviving equivalent mutant is fixed by refactoring the code, not by excluding it. The make targets report; they do not gate.
- **Dependencies** enter through constructors (`New<Type>`). Only `main.go` wires. Anything that touches the outside world sits behind an interface or `fs.FS`.
- **The runtime uses only the standard library.** Every module in `go.mod` is there for a development tool.
- **Errors** wrap with `%w` and a short context prefix (`config:`, `docker:`). Sentinels (`ErrUnexpectedStatus`) are what tests match with `errors.Is`.
- **Logs** go through `log/slog`, one line per command result, and never carry a token or an environment value.
- **No comments in code.** Names and tests explain it; the reasoning goes in the pull request.
- **File names** say what the file is: `<name>_command.go`, `entity_<name>.go`, `service_<name>.go`, `<name>_service.go` in adapters.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- **Every change goes through `/supera:ship`:** worktree, pull request, CI green. Nothing is committed to `main` directly and merging is a human action.
