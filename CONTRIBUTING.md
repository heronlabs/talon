# Contributing

## Setup

```bash
make hooks
make build
```

Requires the Go toolchain in `go.mod` (fetched automatically), plus `golangci-lint`, `shellcheck` and `actionlint` on the `PATH` (`brew install golangci-lint shellcheck actionlint`), and Docker for `make test-installer`.

## Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build `bin/vps-agent` |
| `make lint` | gofmt, vet, golangci-lint, testshape, tidy, shellcheck, actionlint |
| `make test` | Unit tests, 100% coverage of `internal/` |
| `make test-integration` | Integration tests |
| `make test-installer` | `install.sh` scenarios in Docker |
| `make test-mutation` | Mutation tests, 100% efficacy |
| `make audit` | govulncheck |

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, etc.)
- **PRs**: one logical change per PR, squash-merged.
- **Tests**: one scenario and one assertion per test, no loops (see `AGENTS.md`). New behaviour needs tests; bug fixes need a regression test.

## CI

All PRs must pass: Build, Lint, Audit & Supply Chain, Unit Tests, Integration Tests, Installer, Mutation Tests.

## Releases and deploys

Merging to `main` publishes a release whose `checksums.txt` is signed (`checksums.txt.sig`) with the `RELEASE_SIGNING_KEY` of the `release` environment. Nothing reaches the VPSs until someone runs the manual **Deploy** workflow with that version (see `README.md`).

Keys are generated and rotated with `scripts/generate-keys.sh` (`all` or `deploy`; see _Keys and rotation_ in `README.md`). Rotating the release key on running hosts needs a transition release, signed with the current key, that embeds the new one; only after every host runs it is `RELEASE_SIGNING_KEY` replaced.
