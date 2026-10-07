# talon

Go agent for Heron Labs Easypanel hosts: one static binary with three commands, `version`, `setup` and `doctor`. `README.md` describes what they do.

## Map

| Path | Holds |
|---|---|
| `src/main.go` | The entry point: picks the command from `os.Args[1]`, builds its dependencies, logs the result, sets the exit code. Its `version` variable is set at build time by `-X main.version`; without the flag it is `0.0.0-dev`. The configuration directory is `/etc/talon`, or `TALON_CONFIG_DIR`; the Docker socket is `/var/run/docker.sock`, or `DOCKER_SOCKET`; the `authorized_keys` of the deploy key is `/root/.ssh/authorized_keys`, or `TALON_AUTHORIZED_KEYS`. |
| `src/application/` | One command per file, `<name>_command.go`. `VersionCommand`, `DoctorCommand` and `SetupCommand` are implemented. `deploy_command.go` and `report_command.go` hold only the package clause. |
| `src/core/` | `entity_deploy.go`, `entity_report.go`, `service_deploy.go` and `service_report.go`, each with only the package clause. |
| `src/infrastructure/docker/` | `HealthCheck` interface and `HealthCheckService`, which pings the Engine API over the unix socket with `net/http`. `container_service.go` holds only the package clause. |
| `src/infrastructure/environment/` | `config_reader_service.go`: `Key` (name, default, whether it is a secret and whether it is optional; required when it has no default and is not optional), `Keys()`, every variable of the `.env` in the order `setup` asks for them, and `ConfigReader` with `ConfigReaderService`, which takes each key from the environment variable or, when that is empty, from the prompt, with the value in `.env` or the default as fallback, and fails listing the required keys left empty. `config_writer_service.go`: `ConfigWriter` and `ConfigWriterService`, which writes `.env.tmp` with mode `600` through an `os.Root` and renames it to `.env`. `.env.example` at the root repeats the list by hand. |
| `src/infrastructure/ssh/` | `authorized_keys_service.go`: `AuthorizedKeys` and `AuthorizedKeysService`, built on the path of an `authorized_keys` file, which owns one line of it, the one whose last field is `talon-deploy`, and keeps every other line as it is. A key must be one `ssh-ed25519` public key (`ErrInvalidPublicKey`); its line, built from its type and base64 only, is `restrict`, forced to run `apt-get update`, `apt-get install --only-upgrade talon` and `talon version`, and goes at the end of the file, in place of the old one. An empty key removes the line, and leaves the file alone when it has none. The file is written through `.tmp` and a rename in an `os.Root` opened on its directory, with mode `600`; a missing directory is created with mode `700`, only for a key. |
| `src/infrastructure/terminal/` | `Prompt` interface and `PromptService`, which asks one question per line, shows the fallback in brackets (`****` for a secret) and returns it on an empty answer. |
| `src/infrastructure/cms/`, `easypanel/` | `infra_client_service.go` and `provider_service.go`, each with only the package clause. |
| `tests/cli/` | Harness that runs the built binary (`TALON_BIN`) with a configuration directory (`TALON_CONFIG_DIR`), from an empty working directory, and returns output and exit code. `TALON_AUTHORIZED_KEYS` defaults to a file in a temporary directory; a test overrides it with `t.Setenv`. |
| `tests/commands/` | Integration tests (`//go:build integration`), one file per command, against the real binary, a temp directory and a fake Docker Engine on a unix socket. |
| `.docker/` | The fake VPS: Ubuntu 22.04 with systemd, Docker Engine and Easypanel. The image installs the `.deb` built by `make package`: the binary is at `/usr/bin/talon`. `.docker/config`, ignored by git, is mounted at `/etc/talon`. |
| `nfpm.yaml` | The `.deb`: the Linux binary at `/usr/bin/talon` and the directory `/etc/talon`. `make package` passes `ARCH` and `VERSION`. |
| `.github/workflows/` | `continuous-integration.yml` (pull requests) and `continuous-deployment.yml` (on `main`): the `release` job cuts the release and attaches `talon-linux-amd64` and `talon_<version>_amd64.deb`; the `publish` job runs `heronlabs/action-apt-repository@v1`, which builds and signs the apt repository from the `.deb` of the release and publishes it through `heronlabs/action-web-server`. `publish` runs in the `main` environment, assumes its AWS role through OIDC and reads the variables `APT_BUCKET_NAME`, `APT_AWS_REGION`, `APT_AWS_ROLE_TO_ASSUME` and `APT_DISTRIBUTION_ID` of the `main` environment. The signing key is the SSM parameter `/talon/APT_GPG_PRIVATE_KEY`, an armored private key with no passphrase. The `deploy` job runs after `publish`, in the `main` environment, once per host of the JSON array in the variable `DEPLOY_HOSTS`, and is skipped when it is empty: it connects over SSH as `root` with the secret `DEPLOY_SSH_PRIVATE_KEY`, checks the host against the variable `DEPLOY_KNOWN_HOSTS` (lines `<ip> ssh-ed25519 <base64>`), and the forced command of the key runs the upgrade. |
| `.githooks/pre-commit` | gofmt, build, vet and unit tests when staged changes touch Go files, `go.mod` or `go.sum`. |

Import rules, enforced by `depguard` in `.golangci.yml`: `core` imports nothing from `application` or `infrastructure`; `infrastructure` never imports `application`. Commands depend on the interfaces the infrastructure packages export beside their implementations: `DoctorCommand` on `docker.HealthCheck`, `SetupCommand` on `environment.ConfigReader`, `environment.ConfigWriter` and `ssh.AuthorizedKeys`.

## Commands

| Command | Does |
|---|---|
| `make build` | Static binary in `bin/talon` (`GOOS`, `GOARCH`, `VERSION`, `BIN` overridable) |
| `make package` | Linux binary and its `.deb` in `dist/`, through nfpm (`GOARCH` and `VERSION` overridable) |
| `make lint` | golangci-lint (with gofmt and goimports), actionlint, shellcheck, `go mod tidy -diff` |
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
- **Fakes** are written by hand in the test file that uses them (`fakeHealthCheck`). The filesystem enters as `fs.FS` for reads, and tests pass `fstest.MapFS`; writes go through an `*os.Root`, and tests open one on `t.TempDir()`. `AuthorizedKeysService` opens its own on the directory of its file, because it creates that directory only for a key; its tests pass a path in `t.TempDir()`.
- **Integration tests** assert the exit code and the output of the real binary in one condition.
- **Mutants:** a surviving equivalent mutant is fixed by refactoring the code, not by excluding it.
- **Dependencies** enter through constructors (`New<Type>`). Only `main.go` wires.
- **The runtime uses only the standard library.** Every module in `go.mod` is there for a development tool.
- **Errors** wrap with `%w` and a short context prefix (`config:`, `docker:`). `ErrUnexpectedStatus` is a sentinel that tests match with `errors.Is`.
- **Logs** go through `log/slog`, one line per command result.
- **File names:** `<name>_command.go` in `application`, `entity_<name>.go` and `service_<name>.go` in `core`, `<name>_service.go` in `infrastructure`.
- **Never edit `version.txt` or `CHANGELOG.md`:** the release action owns them.
- **Every change goes through `/supera:ship`:** worktree, pull request, CI green. Nothing is committed to `main` directly and merging is a human action.
