# Contributing

## Setup

```bash
make hooks
make build
```

Requires the Go toolchain in `go.mod` (fetched automatically), plus `golangci-lint`, `shellcheck` and `actionlint` on the `PATH` (`brew install golangci-lint shellcheck actionlint`).

## Commands

| Command | Purpose |
|---------|---------|
| `make build` | Build `bin/talon` |
| `make lint` | gofmt, vet, golangci-lint, testshape, tidy, shellcheck, actionlint |
| `make test` | Unit tests, 100% coverage of `internal/` |
| `make test-integration` | Integration tests |
| `make test-mutation` | Mutation tests, 100% efficacy |
| `make audit` | govulncheck |

## Conventions

- **Commits**: [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `chore:`, etc.)
- **PRs**: one logical change per PR, squash-merged.
- **Tests**: one scenario and one assertion per test, no loops (see `AGENTS.md`). New behaviour needs tests; bug fixes need a regression test.

## CI

All PRs must pass: Build, Lint, Audit & Supply Chain, Unit Tests, Integration Tests, Mutation Tests.
