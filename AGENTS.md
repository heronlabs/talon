# talon

Go agent for Heron Labs Easypanel hosts: one static binary with three commands, `version`, `setup` and `doctor`. `README.md` describes what they do.

## Map

| Path | Holds |
|---|---|
| `src/main.go` | The entry point: picks the command from `os.Args[1]`, builds its dependencies, logs the result, sets the exit code. Its `version` variable is set at build time by `-X main.version`; without the flag it is `0.0.0-dev`. The configuration directory is `/etc/talon`, or `TALON_CONFIG_DIR`; the Docker socket is `/var/run/docker.sock`, or `DOCKER_SOCKET`. |
| `src/application/` | One command per file, `<name>_command.go`. `VersionCommand`, `DoctorCommand` and `SetupCommand` are implemented. `deploy_command.go`, `report_command.go` and `update_command.go` hold only the package clause. |
| `src/core/` | `entity_deploy.go`, `entity_report.go`, `service_deploy.go` and `service_report.go`, each with only the package clause. |
| `src/infrastructure/docker/` | `HealthCheck` interface and `HealthCheckService`, which pings the Engine API over the unix socket with `net/http`. `container_service.go` holds only the package clause. |
| `src/infrastructure/environment/` | `config_reader_service.go`: `Key` (name, default and whether it is a secret; required when it has no default), `Keys()`, every variable of the `.env` in the order `setup` asks for them, and `ConfigReader` with `ConfigReaderService`, which takes each key from the environment variable or, when that is empty, from the prompt, with the value in `.env` or the default as fallback, and fails listing the required keys left empty. `config_writer_service.go`: `ConfigWriter` and `ConfigWriterService`, which writes `.env.tmp` with mode `600` through an `os.Root` and renames it to `.env`. `.env.example` at the root repeats the list by hand. |
| `src/infrastructure/terminal/` | `Prompt` interface and `PromptService`, which asks one question per line, shows the fallback in brackets (`****` for a secret) and returns it on an empty answer. |
| `src/infrastructure/cms/`, `easypanel/` | `infra_client_service.go` and `provider_service.go`, each with only the package clause. |
| `tests/cli/` | Harness that runs the built binary (`TALON_BIN`) with a configuration directory (`TALON_CONFIG_DIR`), from an empty working directory, and returns output and exit code. |
| `tests/commands/` | Integration tests (`//go:build integration`), one file per command, against the real binary, a temp directory and a fake Docker Engine on a unix socket. |
| `.docker/` | The fake VPS: Ubuntu 22.04 with systemd, Docker Engine and Easypanel. The image installs the `.deb` built by `make package`: the binary is at `/usr/bin/talon`. `.docker/config`, ignored by git, is mounted at `/etc/talon`. |
| `nfpm.yaml` | The `.deb`: the Linux binary at `/usr/bin/talon` and the directory `/etc/talon`. `make package` passes `ARCH` and `VERSION`. |
| `.github/workflows/` | `continuous-integration.yml` (pull requests) and `continuous-deployment.yml` (on `main`): the `release` job cuts the release and attaches `talon-linux-amd64` and `talon_<version>_amd64.deb`; the `publish` job adds the `.deb` to the apt repository in S3 and invalidates the CDN. `publish` runs in the `main` environment, assumes its AWS role through OIDC and is skipped while the repository variable `APT_BUCKET_NAME` is empty. It also reads the variables `APT_AWS_REGION`, `APT_AWS_ROLE_TO_ASSUME` and `APT_DISTRIBUTION_ID` and the secret `APT_GPG_PRIVATE_KEY`, an armored private key with no passphrase. |
| `.github/scripts/` | `build_apt_repository.sh`: copies `.deb` files into `pool/`, rebuilds `dists/stable` from the whole pool with `apt-ftparchive`, signs it with the default gpg key and exports the public key as `talon.gpg`. |
| `.githooks/pre-commit` | gofmt, build, vet and unit tests when staged changes touch Go files, `go.mod` or `go.sum`. |

Import rules, enforced by `depguard` in `.golangci.yml`: `core` imports nothing from `application` or `infrastructure`; `infrastructure` never imports `application`. Commands depend on the interfaces the infrastructure packages export beside their implementations: `DoctorCommand` on `docker.HealthCheck`, `SetupCommand` on `environment.ConfigReader` and `environment.ConfigWriter`.

## Commands

| Command | Does |
|---|---|
| `make build` | Static binary in `bin/talon` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make package` | Linux binary and its `.deb` in `dist/`, through nfpm (`GOARCH` and `VERSION` overridable) |
| `make lint` | golangci-lint (with gofmt and goimports), actionlint, shellcheck over the hook and the scripts, `go mod tidy -diff` |
| `make lint:fix` | golangci-lint with `--fix` |
| `make test` | Unit tests with the race detector; coverage in `.coverage/unit/` |
| `make test:integration` | Builds an instrumented binary with version `1.2.3` and runs `tests/`; coverage in `.coverage/integration/` |
| `make test:mutation` | gremlins over `src/`, `main.go` excluded; report in `.coverage/mutation/mutation.json` |
| `make audit` | govulncheck |
| `make up` / `make down` | Start and stop the fake VPS |
| `make watch` | Rebuild the Linux binary into `bin/linux` on every `.go` change; the fake VPS mounts it at `/usr/local/bin`, ahead of the packaged binary on its `PATH` |

`golangci-lint` and `shellcheck` come from brew locally. `actionlint`, `govulncheck`, `gremlins`, `air` and `nfpm` are `go tool`s pinned in `go.mod`. The target names use a colon (`test:integration`), not a dash.

## Conventions

- **Tests:** one `Test<Type>` per source file, in the same package (`version_command.go` and `TestVersionCommand` in `version_command_test.go`), and one `t.Run("Should ...")` per scenario. No table tests, no `for`/`range`. Arrange, act and assert are separated by blank lines, and the single assertion is the last statement: one `if` with one `t.Errorf`. Setup failures use `t.Fatal` inside helpers marked `t.Helper()`.
- **Fakes** are written by hand in the test file that uses them (`fakeHealthCheck`). The filesystem enters as `fs.FS` for reads, and tests pass `fstest.MapFS`; writes go through an `*os.Root`, and tests open one on `t.TempDir()`.
- **Integration tests** assert the exit code and the output of the real binary in one condition.
- **Mutants:** a surviving equivalent mutant is fixed by refactoring the code, not by excluding it.
- **Dependencies** enter through constructors (`New<Type>`). Only `main.go` wires.
- **The runtime uses only the standard library.** Every module in `go.mod` is there for a development tool.
- **Errors** wrap with `%w` and a short context prefix (`config:`, `docker:`). `ErrUnexpectedStatus` is a sentinel that tests match with `errors.Is`.
- **Logs** go through `log/slog`, one line per command result.
- **File names:** `<name>_command.go` in `application`, `entity_<name>.go` and `service_<name>.go` in `core`, `<name>_service.go` in `infrastructure`.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- **Every change goes through `/supera:ship`:** worktree, pull request, CI green. Nothing is committed to `main` directly and merging is a human action.
