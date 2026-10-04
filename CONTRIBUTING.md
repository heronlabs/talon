# Contributing

## Setup

```bash
git config core.hooksPath .githooks
make build
```

You need:

- the Go toolchain named in `go.mod` (Go fetches it by itself);
- `golangci-lint` and `shellcheck` on the `PATH` (`brew install golangci-lint shellcheck`);
- Docker, only for the fake VPS.

The other tools (`actionlint`, `govulncheck`, `gremlins`, `air`) are pinned in `go.mod` and run through `go tool`.

## Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build `bin/talon` |
| `make lint` | golangci-lint, actionlint, shellcheck, `go mod tidy -diff` |
| `make lint:fix` | Apply the fixes golangci-lint can make |
| `make test` | Unit tests, coverage report in `.coverage/unit/` |
| `make test:integration` | Integration tests against the real binary |
| `make test:mutation` | Mutation tests, report in `.coverage/mutation/` |
| `make audit` | govulncheck |
| `make up` / `make down` | Start and stop the fake VPS |
| `make watch` | Rebuild the binary for the fake VPS on every change |

## The fake VPS

`make up` builds and starts a container that behaves like a real host: Ubuntu 22.04 with systemd, Docker Engine and Easypanel (served on `http://localhost:3000`). `make watch` rebuilds the Linux binary into `bin/linux` on every `.go` change, and that directory comes first on the container's `PATH`, so the next run uses the new build:

```bash
docker compose -f .docker/compose.yaml exec fake-vps talon version
```

## Workflow

1. Every change starts in a worktree and reaches `main` through a pull request. Nothing is committed to `main` directly.
2. One logical change per pull request, squash-merged.
3. Commits follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, `docs:`). The type decides the next version.
4. Never edit `version.txt` or `CHANGELOG.md`. The release workflow bumps the version, writes the changelog and tags the release when the pull request is merged.

## Conventions

`AGENTS.md` is the reference. In short:

- **One scenario and one assertion per test.** No table tests, no loops. New behaviour needs tests; a bug fix needs a test that fails without it.
- **Unit tests beside the code, integration tests in `tests/`.** Integration tests run the real binary and check its exit code and output.
- **Hand-written fakes, standard library only.** No mocking library and no runtime dependency.
- **No comments in code.** Explain the reasoning in the pull request.
- **Aim for 100%** statement coverage and every mutant killed. A mutant that survives is fixed in the code, not excluded.

## CI

Every pull request must pass: Build, Lint, Audit, Unit Tests, Integration Tests and Mutation Tests. The Mutation Tests job writes the mutation score to its summary.
