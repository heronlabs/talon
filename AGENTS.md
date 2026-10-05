# talon

Go agent for Heron Labs Easypanel hosts: one static binary with two commands, `version` and `doctor`. `README.md` describes what they do.

## Map

| Path | Holds |
|---|---|
| `src/main.go` | The entry point: picks the command from `os.Args[1]`, builds its dependencies, logs the result, sets the exit code. |
| `src/application/` | One command per file, `<name>_command.go`. `VersionCommand` and `DoctorCommand` are implemented. `deploy_command.go` and `report_command.go` hold only the package clause; `install_command.go` and `update_command.go` add `TODO` markers for the work to come. |
| `src/core/` | `entity_deploy.go`, `entity_report.go`, `service_deploy.go` and `service_report.go`, each with only the package clause. |
| `src/infrastructure/docker/` | `HealthCheck` interface and `HealthCheckService`, which pings the Engine API over the unix socket with `net/http`. `container_service.go` holds only the package clause. |
| `src/infrastructure/environment/` | `config.go`: `Key` (name and default; required when it has no default) and `Keys()`, every variable of the `.env` in the order `install` asks for them. `.env.example` at the root repeats the list by hand. |
| `src/infrastructure/cms/`, `easypanel/` | `infra_client_service.go` and `provider_service.go`, each with only the package clause. |
| `tests/cli/` | Harness that runs the built binary (`TALON_BIN`) in a directory and returns output and exit code. |
| `tests/commands/` | Integration tests (`//go:build integration`), one file per command, against the real binary, a temp directory and a fake Docker Engine on a unix socket. |
| `.docker/` | The fake VPS: Ubuntu 22.04 with systemd, Docker Engine and Easypanel. The binary is at `/usr/local/bin/talon`, the working directory is `/opt/talon`. |
| `.github/workflows/` | `continuous-integration.yml` (pull requests) and `continuous-deployment.yml` (release on `main`, with the `linux/amd64` binary attached as `talon-linux-amd64`). |
| `.githooks/pre-commit` | gofmt, build, vet and unit tests when staged changes touch Go files, `go.mod` or `go.sum`. |

Import rules, enforced by `depguard` in `.golangci.yml`: `core` imports nothing from `application` or `infrastructure`; `infrastructure` never imports `application`. `DoctorCommand` depends on `docker.HealthCheck`, the interface the `docker` package exports beside its implementation.

## Commands

| Command | Does |
|---|---|
| `make build` | Static binary in `bin/talon` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make lint` | golangci-lint (with gofmt and goimports), actionlint, shellcheck, `go mod tidy -diff` |
| `make lint:fix` | golangci-lint with `--fix` |
| `make test` | Unit tests with the race detector; coverage in `.coverage/unit/` |
| `make test:integration` | Builds an instrumented binary and runs `tests/`; coverage in `.coverage/integration/` |
| `make test:mutation` | gremlins over `src/`, `main.go` excluded; report in `.coverage/mutation/mutation.json` |
| `make audit` | govulncheck |
| `make up` / `make down` | Start and stop the fake VPS |
| `make watch` | Rebuild the Linux binary into `bin/linux` on every `.go` change; the fake VPS mounts it at `/opt/talon/bin`, first on its `PATH` |

`golangci-lint` and `shellcheck` come from brew locally. `actionlint`, `govulncheck`, `gremlins` and `air` are `go tool`s pinned in `go.mod`. The target names use a colon (`test:integration`), not a dash.

## Conventions

- **Tests:** one `Test<Type>` per source file, in the same package (`version_command.go` and `TestVersionCommand` in `version_command_test.go`), and one `t.Run("Should ...")` per scenario. No table tests, no `for`/`range`. Arrange, act and assert are separated by blank lines, and the single assertion is the last statement: one `if` with one `t.Errorf`. Setup failures use `t.Fatal` inside helpers marked `t.Helper()`.
- **Fakes** are written by hand in the test file that uses them (`fakeHealthCheck`). The filesystem enters as `fs.FS` and tests pass `fstest.MapFS`.
- **Integration tests** assert the exit code and the output of the real binary in one condition.
- **Mutants:** a surviving equivalent mutant is fixed by refactoring the code, not by excluding it.
- **Dependencies** enter through constructors (`New<Type>`). Only `main.go` wires.
- **The runtime uses only the standard library.** Every module in `go.mod` is there for a development tool.
- **Errors** wrap with `%w` and a short context prefix (`config:`, `docker:`). `ErrUnexpectedStatus` is a sentinel that tests match with `errors.Is`.
- **Logs** go through `log/slog`, one line per command result.
- **File names:** `<name>_command.go` in `application`, `entity_<name>.go` and `service_<name>.go` in `core`, `<name>_service.go` in `infrastructure`.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- **Every change goes through `/supera:ship`:** worktree, pull request, CI green. Nothing is committed to `main` directly and merging is a human action.
