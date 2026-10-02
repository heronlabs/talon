# vps-agent — Phase 1 (first release) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the private `heronlabs/vps-agent` repository and ship its first release: a static Go binary with `version`, `doctor` (config + Docker socket) and `report --once` (host + Swarm services, printed in the CMS report contract), plus CI/CD, hooks and the installer.

**Architecture:** Three layers. `cmd/vps-agent` is the app (flags, signals, wiring, no rules). `internal/core` holds pure entities and rules (`domain`), the interfaces of the outside world (`port`) and the use cases (`usecase/report`, `usecase/doctor`). `internal/infrastructure` holds the adapters: `/proc` + statfs + directory sizes, the Docker Engine API over the unix socket (no SDK), the CMS contract DTOs and printer, the `config.env` reader and the clock. `depguard` enforces the import direction.

**Tech Stack:** Go 1.27.1 (stdlib only at runtime), `github.com/google/go-cmp` (tests), `golang.org/x/tools/go/analysis` (the `testshape` lint), gremlins (mutation), govulncheck, golangci-lint v2.14.0, shellcheck, actionlint, GitHub Actions, `heronlabs/action-tag-release-build@v7`.

**Spec:** `docs/superpowers/specs/2026-10-01-vps-agent-design.md` (moved there in Task 1). Read its sections _Medição_, _Layout do repositório_, _Convenções de código_, _Instalação e atualização_, _Testes_ and _Repositório, hooks e CI/CD_ before starting.

**Where to work:** at the root of `/Users/lucaslacerdacl/Workfolder/heronlabs/vps-agent`, on branch `feat/scaffold`, without `/supera:ship` (the user's call for this first release). Never push to `main`. From the first release on, every change goes through `/supera:ship`.

**Fastest path:** while the verified draft still exists at `/private/tmp/claude-501/-Users-lucaslacerdacl-Workfolder-heronlabs-vps-agent/f35d84bc-0fc6-4329-9ce9-e460189d454b/scratchpad/draft` (session scratchpad; it does not survive the session), copy each task's files from there and `diff` them against this plan instead of retyping. The plan is the source of truth if they differ.

**Verified:** every file below was built, linted and tested before this plan was written — `make lint` clean, `make test` 100% coverage on macOS and Linux, `make test-integration` green, gremlins 109 killed / 0 lived / 100% mutator coverage on Linux, `install.sh` exercised in Ubuntu 24.04 with a fake GitHub API, and the Linux binary run against a real Swarm (Docker-in-Docker) printing a correct report.

## Global Constraints

- Module `github.com/heronlabs/vps-agent`; `go 1.27.0`, `toolchain go1.27.1` (current stable on 2026-10-02; 1.26.4 has GO-2026-6218 in `net/url`).
- Runtime dependencies: standard library only. Test-only: `github.com/google/go-cmp v0.7.0`. Lint tool: `golang.org/x/tools v0.39.0`. `go tool`s: `github.com/go-gremlins/gremlins/cmd/gremlins v0.6.0`, `golang.org/x/vuln/cmd/govulncheck v1.8.0`. `golangci-lint` is **not** in `go.mod` (brew locally, `golangci-lint-action` in CI, `v2.14.0`).
- Tests: one `Test<Unit>_<Scenario>` per scenario; no table tests, no `for`/`range`, no `t.Run`; exactly one assertion (`testkit.Equal` or `testkit.ErrorIs`; `analysistest.Run` in the analyzer's own tests); setup via `testkit.Must`/`testkit.MustDo`; arrange, act, assert separated by blank lines. `make lint` enforces it.
- 100% statement coverage of `internal/` (excluding `internal/testkit/` and `internal/tools/testshape/cmd/`), checked from the profile; gremlins efficacy 100%. A surviving equivalent mutant is fixed by refactoring.
- Logs: JSON `log/slog`, `logId` as `<domain>.<event>`, flat fields, `errorName`/`errorMessage`, never a token or env value.
- Actions on floating majors: `actions/checkout@v7`, `actions/setup-go@v7`, `actions/upload-artifact@v7`, `actions/download-artifact@v8`, `golangci/golangci-lint-action@v9`, `heronlabs/action-tag-release-build@v7`; `runs-on: ubuntu-24.04`.
- `version.txt` is created once as `0.0.0` in Task 1; after that only the release action writes it and `CHANGELOG.md`.
- Commits: Conventional Commits, ending with `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`. PR body ends with `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
- Paths in this plan are relative to the repository root unless absolute.

## Review Focus

Inputs the spec implies but does not spell out, most likely to bite first; each is pinned by a test in the task named:

- **Hand-edited `config.env` with quotes, CRLF or spaces around `=`** (`AGENT_HOST_NAME = "vps-heron-02"`, file saved on Windows): values are read without the quotes, the `\r` and the spaces. Task 9 (`TestParse_DropsOnePairOfDoubleQuotes`, `TestParse_AcceptsWindowsLineEndings`, `TestParse_DropsSpacesAroundKeyAndValue`, `TestParse_EmptyQuotedValueIsEmpty`).
- **Single-file bind mount under the service dir** (Easypanel file mounts such as `/etc/easypanel/projects/<p>/<s>/config.yml`): measured as the file's size, not an error. Task 6 (`TestDirSize_MeasuresABindSourceThatIsAFile`). This caught a real bug while the plan was verified.
- **Docker node that is not a Swarm manager** (`/services` answers 503): the report fails with `ErrUnexpectedStatus` naming the path and status, instead of printing an empty host. Task 7 (`TestSnapshot_FailsWhenNodeIsNotASwarmManager`).
- **Container states outside the contract** (`removing`, anything new the engine adds): `removing` reads as `exited`, unknown as `dead`, so the CMS never receives a state its `yup` schema rejects. Task 7 (`TestSnapshot_RemovingContainerReadsAsExited`, `TestSnapshot_UnknownContainerStateReadsAsDead`).
- **Memory accounting on cgroup v1 vs v2 and cache equal to usage**: same numbers as `docker stats` (v1 key first, cache only subtracted when below usage). Task 7 (`TestSnapshot_MemoryPrefersTheCgroupV1Key`, `TestSnapshot_MemoryKeepsUsageWhenInactiveCacheEqualsIt`).

---

### Task 1: Repository, module and branch

**Files:**
- Create: `go.mod`, `go.sum`, `.gitignore`, `.editorconfig`, `version.txt`
- Move: `2026-10-01-vps-agent-design.md` → `docs/superpowers/specs/2026-10-01-vps-agent-design.md`
- Keep: `docs/superpowers/plans/2026-10-02-vps-agent-phase-1.md` (this plan)

**Interfaces:**
- Produces: module path `github.com/heronlabs/vps-agent`, branch `feat/scaffold` tracking `origin/main`.

- [ ] **Step 1: Create the private repository with a README (this creates `main`)**

```bash
gh repo create heronlabs/vps-agent --private --add-readme \
  --description "Agent that provisions and measures an Easypanel VPS from the CMS"
```

Expected: `✓ Created repository heronlabs/vps-agent on GitHub`.

- [ ] **Step 2: Configure merges (squash only, delete merged branches)**

```bash
gh api -X PATCH repos/heronlabs/vps-agent \
  -F allow_squash_merge=true -F allow_merge_commit=false -F allow_rebase_merge=false \
  -F delete_branch_on_merge=true --jq '.allow_squash_merge, .allow_merge_commit, .allow_rebase_merge, .delete_branch_on_merge'
```

Expected: `true`, `false`, `false`, `true`. No branch protection, like every other heronlabs repository (`action-tag-release-build`, `action-cloudfront-publish`, `webapps`, `cloud-iac`, `cloud-iac-heronlabs` were checked on 2026-10-02): the release action pushes `version.txt` and `CHANGELOG.md` to `main` with `github.token`, and a protected `main` would reject that push. "Never push to `main`" is a process rule (PRs, `/supera:ship`), not a GitHub setting.

- [ ] **Step 3: Turn the local folder into a clone on a feature branch**

```bash
cd /Users/lucaslacerdacl/Workfolder/heronlabs/vps-agent
git init -q
git remote add origin https://github.com/heronlabs/vps-agent.git
git fetch -q origin main
git checkout -q -b feat/scaffold origin/main
mkdir -p docs/superpowers/specs
mv 2026-10-01-vps-agent-design.md docs/superpowers/specs/
rm -f .DS_Store
```

Expected: `git status` shows only untracked `docs/`.

- [ ] **Step 4: Create the module and its pinned tools**

```bash
cat > go.mod <<'EOF'
module github.com/heronlabs/vps-agent

go 1.27.0

toolchain go1.27.1
EOF
go get github.com/google/go-cmp@v0.7.0 golang.org/x/tools@v0.39.0
go get -tool github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 golang.org/x/vuln/cmd/govulncheck@v1.8.0
go version
```

Expected: `go version go1.27.1 ...` (the toolchain line makes `go` download 1.27.1 if the local one is older). `go mod tidy` runs at the end of Task 2, once code imports these modules.

- [ ] **Step 5: Add the repository basics**

`.gitignore`:

```text
bin/
coverage.out
coverage.filtered.out
reports/
.DS_Store
dist/
```

`.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab

[{Makefile,*.mk}]
indent_style = tab

[*.{yml,yaml,json,md,sh}]
indent_style = space
indent_size = 2
```

```bash
echo 0.0.0 > version.txt
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: bootstrap go module and design spec

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: testkit and the `testshape` analyzer

The only assertions tests may use, the setup helpers, and the lint analyzer that enforces one scenario and one assertion per test.

**Files:**
- Create: `internal/testkit/assert.go`, `internal/testkit/must.go`
- Create: `internal/tools/testshape/testshape.go`, `internal/tools/testshape/cmd/testshape/main.go`
- Test: `internal/tools/testshape/testshape_test.go` and fixtures under `internal/tools/testshape/testdata/src/`

**Interfaces:**
- Produces: `testkit.Equal[T any](t testing.TB, want, got T, opts ...cmp.Option)`, `testkit.ErrorIs(t testing.TB, err, target error)`, `testkit.Must[T any](v T, err error) T` (panics on error; setup only), `testkit.MustDo(t testing.TB, err error)`; `testshape.Analyzer *analysis.Analyzer`; command `go run ./internal/tools/testshape/cmd/testshape ./...`.

- [ ] **Step 1: Write testkit (no tests of its own; it is excluded from coverage and mutation)**

`internal/testkit/assert.go`:

```go
// Package testkit holds the only assertions tests may use, plus setup helpers.
package testkit

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Equal fails the test with a diff when want and got differ.
func Equal[T any](t testing.TB, want, got T, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(want, got, opts...); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

// ErrorIs fails the test when err does not wrap target.
func ErrorIs(t testing.TB, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("error = %v, want %v", err, target)
	}
}
```

`internal/testkit/must.go`:

```go
package testkit

import (
	"fmt"
	"testing"
)

// Must returns v and panics when err is not nil. Setup only: it is not an assertion.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("setup: %v", err))
	}
	return v
}

// MustDo fails the test when err is not nil. Setup only: it is not an assertion.
func MustDo(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
```

- [ ] **Step 2: Write the analyzer fixtures**

The fixtures run in GOPATH mode under `testdata/src`. The stub `example.com/internal/testkit` matches the analyzer's `internal/testkit` suffix rule, and the stub `golang.org/x/tools/go/analysis/analysistest` stands in for the real one. `// want` comments carry the expected diagnostics.

`internal/tools/testshape/testdata/src/example.com/internal/testkit/testkit.go`:

```go
package testkit

import "testing"

func Equal[T any](t testing.TB, want, got T) {}

func ErrorIs(t testing.TB, err, target error) {}

func Must[T any](v T, err error) T { return v }
```

`internal/tools/testshape/testdata/src/golang.org/x/tools/go/analysis/analysistest/analysistest.go`:

```go
package analysistest

import "testing"

func TestData() string { return "" }

func Run(t *testing.T, dir string) {}
```

`internal/tools/testshape/testdata/src/example.com/valid/valid.go` (a `Test`-named func outside a `_test.go` file is ignored):

```go
package valid

func TestLike(xs []int) {
	for range xs {
	}
}
```

`internal/tools/testshape/testdata/src/example.com/valid/valid_test.go` (helpers with loops, methods, builtins, func values, conversions and `testkit.Must` are not counted):

```go
package valid

import (
	"errors"
	"testing"

	"example.com/internal/testkit"
)

type suite struct{}

func (suite) TestMethod(xs []int) {
	for range xs {
	}
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

func TestValid(t *testing.T) {
	err := errors.New("x")
	_ = err.Error()
	_ = len("x")
	f := func() int { return 1 }
	_ = f()
	_ = string(rune(65))
	got := testkit.Must(sum([]int{1}), nil)

	testkit.Equal(t, 1, got)
}
```

`internal/tools/testshape/testdata/src/example.com/loop/loop_test.go`:

```go
package loop

import (
	"testing"

	"example.com/internal/testkit"
)

func TestLoop(t *testing.T) {
	n := 0
	for i := 0; i < 2; i++ { // want `TestLoop: loop in test; write one test per scenario`
		n += i
	}

	testkit.Equal(t, 1, n)
}
```

`internal/tools/testshape/testdata/src/example.com/ranges/ranges_test.go`:

```go
package ranges

import (
	"testing"

	"example.com/internal/testkit"
)

func TestRange(t *testing.T) {
	n := 0
	for _, x := range []int{1} { // want `TestRange: loop in test; write one test per scenario`
		n += x
	}

	testkit.Equal(t, 1, n)
}
```

`internal/tools/testshape/testdata/src/example.com/subtest/subtest_test.go`:

```go
package subtest

import (
	"testing"

	"example.com/internal/testkit"
)

func TestSubtest(t *testing.T) {
	t.Run("case", func(t *testing.T) {}) // want `TestSubtest: t.Run in test; write one test per scenario`

	testkit.Equal(t, 1, 1)
}
```

`internal/tools/testshape/testdata/src/example.com/twoassertions/twoassertions_test.go`:

```go
package twoassertions

import (
	"testing"

	"example.com/internal/testkit"
)

func TestTwo(t *testing.T) { // want `TestTwo: 2 assertions, want exactly 1`
	testkit.Equal(t, 1, 1)
	testkit.ErrorIs(t, nil, nil)
}
```

`internal/tools/testshape/testdata/src/example.com/noassertion/noassertion_test.go`:

```go
package noassertion

import "testing"

func TestNone(t *testing.T) { // want `TestNone: 0 assertions, want exactly 1`
	_ = t
}
```

`internal/tools/testshape/testdata/src/example.com/analyzertest/analyzertest_test.go` (two `TestData()` calls so that counting `TestData` instead of `Run` would be caught):

```go
package analyzertest

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	dir := analysistest.TestData()
	_ = analysistest.TestData()

	analysistest.Run(t, dir)
}
```

- [ ] **Step 3: Write the failing analyzer tests**

`internal/tools/testshape/testshape_test.go`:

```go
package testshape_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/heronlabs/vps-agent/internal/tools/testshape"
)

func TestAnalyzer_AcceptsOneAssertionAndIgnoresNonTests(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/valid")
}

func TestAnalyzer_ReportsForLoop(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/loop")
}

func TestAnalyzer_ReportsRangeLoop(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/ranges")
}

func TestAnalyzer_ReportsSubtest(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/subtest")
}

func TestAnalyzer_ReportsTwoAssertions(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/twoassertions")
}

func TestAnalyzer_ReportsNoAssertion(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/noassertion")
}

func TestAnalyzer_CountsAnalysistestRun(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testshape.Analyzer, "example.com/analyzertest")
}
```

- [ ] **Step 4: Run them to see them fail**

Run: `go test ./internal/tools/testshape/`
Expected: FAIL — build error, package `testshape` has no non-test Go files / `undefined: testshape.Analyzer`.

- [ ] **Step 5: Write the analyzer and its command**

`internal/tools/testshape/testshape.go`:

```go
// Package testshape reports Test functions that loop, open subtests or do not
// end in exactly one assertion.
package testshape

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is the testshape analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "testshape",
	Doc:      "reports Test functions with loops, t.Run or a number of assertions other than one",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if !isTest(pass, fn) {
			return
		}
		checkTest(pass, fn)
	})
	return nil, nil
}

func isTest(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	file := pass.Fset.File(fn.Pos())
	return fn.Recv == nil &&
		strings.HasSuffix(file.Name(), "_test.go") &&
		strings.HasPrefix(fn.Name.Name, "Test")
}

func checkTest(pass *analysis.Pass, fn *ast.FuncDecl) {
	assertions := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			pass.Reportf(node.Pos(), "%s: loop in test; write one test per scenario", fn.Name.Name)
		case *ast.CallExpr:
			callee := typeutil.Callee(pass.TypesInfo, node)
			if isSubtest(callee) {
				pass.Reportf(node.Pos(), "%s: t.Run in test; write one test per scenario", fn.Name.Name)
			}
			if isAssertion(callee) {
				assertions++
			}
		}
		return true
	})
	if assertions != 1 {
		pass.Reportf(fn.Pos(), "%s: %d assertions, want exactly 1", fn.Name.Name, assertions)
	}
}

func isSubtest(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "testing" || fn.Name() != "Run" {
		return false
	}
	return fn.Signature().Recv() != nil
}

func isAssertion(obj types.Object) bool {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	path, name := fn.Pkg().Path(), fn.Name()
	if strings.HasSuffix(path, "internal/testkit") {
		return name == "Equal" || name == "ErrorIs"
	}
	return path == "golang.org/x/tools/go/analysis/analysistest" && name == "Run"
}
```

`internal/tools/testshape/cmd/testshape/main.go`:

```go
// Command testshape runs the testshape analyzer.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/heronlabs/vps-agent/internal/tools/testshape"
)

func main() { singlechecker.Main(testshape.Analyzer) }
```

- [ ] **Step 6: Run the tests and the analyzer**

```bash
go mod tidy
go test -cover ./internal/tools/testshape/
go run ./internal/tools/testshape/cmd/testshape ./...
```

Expected: `ok ... coverage: 100.0% of statements`; the analyzer prints nothing and exits 0.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "test: add testkit and testshape analyzer

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Build, lint, coverage gate, git hook and CI

**Files:**
- Create: `Makefile`, `.golangci.yml`, `scripts/check-coverage.sh`, `.githooks/pre-commit`, `.github/actionlint.yaml`, `.github/workflows/continuous-integration.yml` (actionlint, part of `make lint`, fails when there is no workflow at all)

**Interfaces:**
- Produces: `make hooks|build|lint|test|test-integration|test-mutation|audit`; `make build` honours `VERSION`, `GOOS`, `GOARCH`, `BIN`.

- [ ] **Step 1: Install the local linters if missing**

```bash
brew install golangci-lint shellcheck actionlint
golangci-lint version
```

Expected: `golangci-lint has version 2.14.0 built with go1.27...` or newer. A build with Go 1.26 refuses `go 1.27.0` modules; `brew upgrade golangci-lint` fixes it.

- [ ] **Step 2: Write the Makefile**

`Makefile`:

```makefile
VERSION ?= 0.0.0-dev
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BIN ?= bin/vps-agent

.PHONY: hooks build lint test test-integration test-mutation audit

hooks:
	git config core.hooksPath .githooks

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/vps-agent

lint:
	@unformatted=$$(gofmt -l .); [ -z "$$unformatted" ] || { echo "gofmt: $$unformatted" >&2; exit 1; }
	go vet -tags integration ./...
	golangci-lint run
	GOFLAGS=-tags=integration go run ./internal/tools/testshape/cmd/testshape ./...
	go mod tidy -diff
	shellcheck scripts/*.sh .githooks/pre-commit
	actionlint

test:
	go test -race -coverpkg=./internal/... -coverprofile=coverage.out ./...
	scripts/check-coverage.sh coverage.out

test-integration:
	go test -race -tags integration ./test/integration/...

test-mutation:
	mkdir -p reports
	go tool gremlins unleash --threshold-efficacy 100 --output reports/mutation.json \
		-E 'cmd/.*' -E 'internal/testkit/.*' -E 'internal/tools/testshape/cmd/.*'


audit:
	go tool govulncheck ./...
```

- [ ] **Step 3: Write the lint configuration**

`.golangci.yml` (depguard holds the import direction; `!$test` lets tests import `testkit`):

```yaml
version: "2"

run:
  build-tags: [integration]

linters:
  default: standard
  enable:
    - depguard
    - errorlint
    - gocritic
    - gosec
    - misspell
    - nolintlint
    - revive
    - unparam
  settings:
    depguard:
      rules:
        domain:
          files: ["**/internal/core/domain/**", "!$test"]
          deny:
            - pkg: github.com/heronlabs/vps-agent/internal
              desc: domain imports nothing internal
        port:
          files: ["**/internal/core/port/**", "!$test"]
          deny:
            - pkg: github.com/heronlabs/vps-agent/internal/core/usecase
              desc: port imports only domain
            - pkg: github.com/heronlabs/vps-agent/internal/infrastructure
              desc: port imports only domain
        usecase:
          files: ["**/internal/core/usecase/**", "!$test"]
          deny:
            - pkg: github.com/heronlabs/vps-agent/internal/infrastructure
              desc: use cases reach the outside world only through ports
        infrastructure:
          files: ["**/internal/infrastructure/**", "!$test"]
          deny:
            - pkg: github.com/heronlabs/vps-agent/internal/core/usecase
              desc: adapters never import use cases
        testkit:
          files: ["!$test", "!**/internal/testkit/**", "!**/test/integration/**"]
          deny:
            - pkg: github.com/heronlabs/vps-agent/internal/testkit
              desc: testkit is for tests only
  exclusions:
    presets: [comments, std-error-handling]
    paths:
      - internal/tools/testshape/testdata

formatters:
  enable: [gofmt, goimports]
  settings:
    goimports:
      local-prefixes: [github.com/heronlabs/vps-agent]
```

`.github/actionlint.yaml` (actionlint 1.7.12 does not know `concurrency.queue`, which the org already uses):

```yaml
# actionlint 1.7.12 does not know concurrency.queue yet; GitHub accepts it.
paths:
  .github/workflows/continuous-deployment.yml:
    ignore:
      - 'unexpected key "queue" for "concurrency" section'
```

- [ ] **Step 4: Write the coverage gate and the hook**

`scripts/check-coverage.sh`:

```bash
#!/usr/bin/env bash
# Fails unless every statement in internal/ is covered, testkit and tool mains excluded.
set -euo pipefail

profile="$1"
filtered="$(mktemp)"
trap 'rm -f "$filtered"' EXIT

grep -v -E '/internal/testkit/|/internal/tools/testshape/cmd/' "$profile" >"$filtered"
total="$(go tool cover -func="$filtered" | awk '/^total:/ {print $3}')"
echo "coverage: $total"
[ "$total" = "100.0%" ] || { echo "coverage below 100%" >&2; exit 1; }
```

`.githooks/pre-commit`:

```bash
#!/usr/bin/env bash
set -euo pipefail

git diff --cached --quiet -- '*.go' go.mod go.sum && exit 0

mapfile -t staged < <(git diff --cached --name-only --diff-filter=ACM -- '*.go')
if [ "${#staged[@]}" -gt 0 ]; then
  unformatted=$(gofmt -l "${staged[@]}")
  [ -z "$unformatted" ] || { echo "gofmt: $unformatted" >&2; exit 1; }
fi

make build >/dev/null
go vet ./...
go test ./... >/dev/null
```

```bash
chmod +x scripts/check-coverage.sh .githooks/pre-commit
make hooks
```

- [ ] **Step 5: Write the CI workflow**

`.github/workflows/continuous-integration.yml` (job names become the required checks in Task 12):

````yaml
name: 'Continuous Integration'

on:
  workflow_dispatch: {}
  pull_request:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  build:
    name: 'Build'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - run: make build

  lint:
    needs: build
    name: 'Lint'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - uses: golangci/golangci-lint-action@v9
        with:
          version: v2.14.0
          install-only: true

      - name: Install actionlint
        run: |
          bash <(curl -fsSL https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash)
          echo "$PWD" >> "$GITHUB_PATH"

      - run: make lint

  audit:
    needs: build
    name: 'Audit & Supply Chain'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - run: make audit

  unit:
    needs: build
    name: 'Unit Tests'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - run: make test

      - if: always()
        name: Upload unit-test coverage
        uses: actions/upload-artifact@v7
        with:
          name: unit-test-coverage
          path: coverage.out
          retention-days: 14
          if-no-files-found: warn

  integration:
    needs: unit
    name: 'Integration Tests'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - run: make test-integration

  mutation:
    needs: unit
    name: 'Mutation Tests'
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Mutation tests
        run: |
          set -o pipefail
          make test-mutation 2>&1 | tee /tmp/mutation.log

      - name: Mutation summary
        if: always()
        run: |
          {
            echo "### Mutation testing"
            echo '```'
            grep -E '^(Killed|Timed out|Test efficacy|Mutator coverage)' /tmp/mutation.log || echo "see uploaded mutation-reports artifact"
            echo '```'
          } >> "$GITHUB_STEP_SUMMARY"

      - if: always()
        name: Upload mutation reports
        uses: actions/upload-artifact@v7
        with:
          name: mutation-reports
          path: reports/
          retention-days: 14
          if-no-files-found: warn
````

- [ ] **Step 6: Run the gates on what exists**

Run: `make lint && make test`
Expected: lint prints `0 issues.` and no other output from testshape, tidy, shellcheck, actionlint; `make test` ends with `coverage: 100.0%`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "build: add Makefile, depguard rules, coverage gate, pre-commit hook and CI

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Domain — samples, report and the report builder

**Files:**
- Create: `internal/core/domain/sample.go`, `internal/core/domain/report.go`, `internal/core/domain/build.go`, `internal/core/domain/mounts.go`
- Test: `internal/core/domain/sample_test.go`, `internal/core/domain/build_test.go`, `internal/core/domain/mounts_test.go`

**Interfaces:**
- Produces:
  - `type Bytes uint64`; `func BytesOf(v int64) Bytes`
  - `HostSample`, `DockerSample`, `SwarmService`, `SwarmTask`, `TaskState` (`TaskRunning`, `TaskFailed`, `TaskRejected`), `Container`, `ContainerStats`
  - `ServiceState` (`StateRunning`, `StateRestarting`, `StatePaused`, `StateCreated`, `StateExited`, `StateDead`, `StateMissing`), `ReconcileState` (`StateUnmanaged`)
  - `Report`, `System`, `Project`, `Service`
  - `type ReportInput struct{ Host, AgentVersion string; Now time.Time; HostSample HostSample; DockerSample DockerSample; DirSizes map[string]Bytes }`
  - `func BuildReport(in ReportInput) Report`
  - `func SplitServiceName(full string) (project, service string, ok bool)`
  - `const EasypanelProjectsDir = "/etc/easypanel/projects"`; `func BindMountPaths(d DockerSample, fullName string) []string`; `func AllBindMountPaths(d DockerSample) []string`

- [ ] **Step 1: Write the failing tests**

`internal/core/domain/sample_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestBytesOf_KeepsPositiveSizes(t *testing.T) {
	got := domain.BytesOf(800)

	testkit.Equal(t, domain.Bytes(800), got)
}

func TestBytesOf_TreatsNegativeAsZero(t *testing.T) {
	got := domain.BytesOf(-1)

	testkit.Equal(t, domain.Bytes(0), got)
}
```

`internal/core/domain/build_test.go`:

```go
package domain_test

import (
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var now = time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)

func input(d domain.DockerSample) domain.ReportInput {
	return domain.ReportInput{Host: "vps-heron-02", AgentVersion: "1.0.0", Now: now, DockerSample: d}
}

func service(id, name string) domain.SwarmService {
	return domain.SwarmService{ID: id, Name: name, ReplicasDesired: 1}
}

func onlyService(r domain.Report) domain.Service {
	return r.Projects[0].Services[0]
}

func TestBuildReport_EmptyDockerGivesHostOnlyReport(t *testing.T) {
	in := input(domain.DockerSample{})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Report{
		Host:         "vps-heron-02",
		MeasuredAt:   now,
		AgentVersion: "1.0.0",
		Projects:     []domain.Project{},
	}, got)
}

func TestBuildReport_DerivesSystemFromHostAndDocker(t *testing.T) {
	in := input(domain.DockerSample{ImagesSize: 6, BuildCacheSize: 2})
	in.HostSample = domain.HostSample{
		MemTotal: 100, MemAvailable: 40, SwapTotal: 10, SwapFree: 7,
		Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: time.Hour,
		DiskTotal: 1000, DiskUsed: 412,
	}

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.System{
		RAMTotal: 100, RAMUsed: 60, SwapTotal: 10, SwapUsed: 3,
		DiskTotal: 1000, DiskUsed: 412,
		Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: time.Hour,
		DockerImages: 6, DockerBuildCache: 2,
	}, got.System)
}

func TestBuildReport_GroupsServicesByProjectInNameOrder(t *testing.T) {
	in := input(domain.DockerSample{Services: []domain.SwarmService{
		service("3", "shop_web"), service("1", "cms-heronlabs_cms-database"), service("2", "cms-heronlabs_cms-api"),
	}})

	got := domain.BuildReport(in)

	testkit.Equal(t, []domain.Project{
		{Name: "cms-heronlabs", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "cms-api", State: domain.StateMissing, ReplicasDesired: 1},
			{Name: "cms-database", State: domain.StateMissing, ReplicasDesired: 1},
		}},
		{Name: "shop", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "web", State: domain.StateMissing, ReplicasDesired: 1},
		}},
	}, got.Projects)
}

func TestBuildReport_LeavesOutServicesWithoutProject(t *testing.T) {
	in := input(domain.DockerSample{Services: []domain.SwarmService{service("1", "easypanel")}})

	got := domain.BuildReport(in)

	testkit.Equal(t, []domain.Project{}, got.Projects)
}

func TestBuildReport_CopiesServiceLimits(t *testing.T) {
	svc := domain.SwarmService{ID: "1", Name: "p_api", ReplicasDesired: 2, MemoryLimit: 512, CPULimit: 0.5}
	in := input(domain.DockerSample{Services: []domain.SwarmService{svc}})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateMissing, ReplicasDesired: 2, MemoryLimit: 512, CPULimit: 0.5,
	}, onlyService(got))
}

func TestBuildReport_CountsRunningTasksAndRecentFailuresOfTheService(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Tasks: []domain.SwarmTask{
			{ServiceID: "1", State: domain.TaskRunning},
			{ServiceID: "1", State: domain.TaskFailed},
			{ServiceID: "1", State: domain.TaskRejected},
			{ServiceID: "1", State: "shutdown"},
			{ServiceID: "2", State: domain.TaskRunning},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateMissing, ReplicasRunning: 1, ReplicasDesired: 1, RecentTaskFailures: 2,
	}, onlyService(got))
}

func TestBuildReport_RunningContainerWinsOverLaterExitedOne(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning},
			{ID: "b", ServiceName: "p_api", State: domain.StateExited},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateRunning, onlyService(got).State)
}

func TestBuildReport_RunningContainerReplacesEarlierExitedOne(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "b", ServiceName: "p_api", State: domain.StateExited},
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateRunning, onlyService(got).State)
}

func TestBuildReport_UnknownContainerStateNeverWins(t *testing.T) {
	in := input(domain.DockerSample{
		Services:   []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_api", State: "unknown"}},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateMissing, onlyService(got).State)
}

func TestBuildReport_DeadContainerBeatsMissing(t *testing.T) {
	in := input(domain.DockerSample{
		Services:   []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_api", State: domain.StateDead}},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.StateDead, onlyService(got).State)
}

func TestBuildReport_SumsStatsOfTheServiceContainers(t *testing.T) {
	stats := &domain.ContainerStats{MemoryUsed: 100, CPUPercent: 1.5, NetRx: 10, NetTx: 20, BlockRead: 30, BlockWrite: 40}
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_api")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_api", State: domain.StateRunning, Stats: stats},
			{ID: "b", ServiceName: "p_api", State: domain.StateRunning, Stats: stats},
			{ID: "c", ServiceName: "p_api", State: domain.StateRunning},
			{ID: "d", ServiceName: "other_api", State: domain.StateRunning, Stats: stats},
		},
	})

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Service{
		Name: "api", State: domain.StateRunning, ReplicasDesired: 1,
		MemoryUsed: 200, CPUPercent: 3, NetRx: 20, NetTx: 40, BlockRead: 60, BlockWrite: 80,
	}, onlyService(got))
}

func TestBuildReport_DiskCountsEachVolumeOnceAndTheServiceBindDirs(t *testing.T) {
	in := input(domain.DockerSample{
		Services: []domain.SwarmService{service("1", "p_db")},
		Containers: []domain.Container{
			{ID: "a", ServiceName: "p_db", State: domain.StateRunning, Volumes: []string{"p_db_data"},
				BindSources: []string{"/etc/easypanel/projects/p/db/conf", "/var/run/docker.sock"}},
			{ID: "b", ServiceName: "p_db", State: domain.StateExited, Volumes: []string{"p_db_data"}},
		},
		VolumeSizes: map[string]domain.Bytes{"p_db_data": 800, "other": 5},
	})
	in.DirSizes = map[string]domain.Bytes{"/etc/easypanel/projects/p/db/conf": 3, "/var/run/docker.sock": 9}

	got := domain.BuildReport(in)

	testkit.Equal(t, domain.Bytes(803), onlyService(got).Disk)
}
```

`internal/core/domain/mounts_test.go`:

```go
package domain_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type split struct {
	Project, Service string
	OK               bool
}

func splitOf(full string) split {
	p, s, ok := domain.SplitServiceName(full)
	return split{p, s, ok}
}

func TestSplitServiceName_CutsAtFirstUnderscore(t *testing.T) {
	got := splitOf("cms-heronlabs_cms_api")

	testkit.Equal(t, split{"cms-heronlabs", "cms_api", true}, got)
}

func TestSplitServiceName_RejectsNameWithoutUnderscore(t *testing.T) {
	got := splitOf("easypanel")

	testkit.Equal(t, split{"easypanel", "", false}, got)
}

func TestSplitServiceName_RejectsEmptyProject(t *testing.T) {
	got := splitOf("_api")

	testkit.Equal(t, split{"", "api", false}, got)
}

func TestSplitServiceName_RejectsEmptyService(t *testing.T) {
	got := splitOf("p_")

	testkit.Equal(t, split{"p", "", false}, got)
}

func TestBindMountPaths_KeepsTheServiceDirsSortedWithoutRepeats(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "p_db", BindSources: []string{
			"/etc/easypanel/projects/p/db/data/",
			"/etc/easypanel/projects/p/db/conf",
			"/etc/easypanel/projects/p/dbx/conf",
			"/etc/easypanel/projects/p/api/conf",
			"/var/run/docker.sock",
		}},
		{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db/data"}},
		{ServiceName: "p_api", BindSources: []string{"/etc/easypanel/projects/p/db/other"}},
	}}

	got := domain.BindMountPaths(d, "p_db")

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/db/conf", "/etc/easypanel/projects/p/db/data"}, got)
}

func TestBindMountPaths_ServiceDirItselfCounts(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db"}},
	}}

	got := domain.BindMountPaths(d, "p_db")

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/db"}, got)
}

func TestBindMountPaths_NameWithoutProjectGivesNothing(t *testing.T) {
	d := domain.DockerSample{Containers: []domain.Container{
		{ServiceName: "easypanel", BindSources: []string{"/etc/easypanel"}},
	}}

	got := domain.BindMountPaths(d, "easypanel")

	testkit.Equal(t, []string(nil), got)
}

func TestAllBindMountPaths_JoinsEveryService(t *testing.T) {
	d := domain.DockerSample{
		Services: []domain.SwarmService{{Name: "p_api"}, {Name: "p_db"}, {Name: "easypanel"}},
		Containers: []domain.Container{
			{ServiceName: "p_api", BindSources: []string{"/etc/easypanel/projects/p/api/a"}},
			{ServiceName: "p_db", BindSources: []string{"/etc/easypanel/projects/p/db/b"}},
		},
	}

	got := domain.AllBindMountPaths(d)

	testkit.Equal(t, []string{"/etc/easypanel/projects/p/api/a", "/etc/easypanel/projects/p/db/b"}, got)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/core/domain/`
Expected: FAIL — `undefined: domain.BuildReport` (and the other domain names).

- [ ] **Step 3: Write the domain**

`internal/core/domain/sample.go`:

```go
// Package domain holds the agent's entities and pure rules. It does no I/O.
package domain

import "time"

// Bytes is a size in bytes.
type Bytes uint64

// BytesOf converts a signed size, as the engine and the filesystem report it, treating negatives as zero.
func BytesOf(v int64) Bytes {
	return Bytes(max(v, 0))
}

// HostSample is one reading of the host, already parsed from /proc and statfs.
type HostSample struct {
	MemTotal     Bytes
	MemAvailable Bytes
	SwapTotal    Bytes
	SwapFree     Bytes
	Load1        float64
	Load5        float64
	Load15       float64
	Uptime       time.Duration
	DiskTotal    Bytes
	DiskUsed     Bytes
}

// DockerSample is one reading of the Docker engine.
type DockerSample struct {
	Services       []SwarmService
	Tasks          []SwarmTask
	Containers     []Container
	VolumeSizes    map[string]Bytes
	ImagesSize     Bytes
	BuildCacheSize Bytes
}

// SwarmService is a Swarm service as the engine declares it.
type SwarmService struct {
	ID              string
	Name            string
	ReplicasDesired int
	MemoryLimit     Bytes
	CPULimit        float64
}

// TaskState is the state of a Swarm task.
type TaskState string

// Task states the report counts.
const (
	TaskRunning  TaskState = "running"
	TaskFailed   TaskState = "failed"
	TaskRejected TaskState = "rejected"
)

// SwarmTask is one task of a Swarm service.
type SwarmTask struct {
	ServiceID string
	State     TaskState
}

// Container is a container with the Swarm service it belongs to, if any.
type Container struct {
	ID          string
	ServiceName string
	State       ServiceState
	Volumes     []string
	BindSources []string
	Stats       *ContainerStats
}

// ContainerStats is one stats reading of a running container.
type ContainerStats struct {
	MemoryUsed Bytes
	CPUPercent float64
	NetRx      Bytes
	NetTx      Bytes
	BlockRead  Bytes
	BlockWrite Bytes
}
```

`internal/core/domain/report.go`:

```go
package domain

import "time"

// ServiceState is the state reported for a service, following its containers.
type ServiceState string

// Service states, in the contract's vocabulary.
const (
	StateRunning    ServiceState = "running"
	StateRestarting ServiceState = "restarting"
	StatePaused     ServiceState = "paused"
	StateCreated    ServiceState = "created"
	StateExited     ServiceState = "exited"
	StateDead       ServiceState = "dead"
	StateMissing    ServiceState = "missing"
)

// ReconcileState is the reconcile outcome of a project.
type ReconcileState string

// StateUnmanaged marks a project measured but not reconciled by the agent.
const StateUnmanaged ReconcileState = "unmanaged"

// Report is one measurement of the host and of every Easypanel project on it.
type Report struct {
	Host         string
	MeasuredAt   time.Time
	AgentVersion string
	System       System
	Projects     []Project
}

// System is the host-wide part of the report.
type System struct {
	RAMTotal         Bytes
	RAMUsed          Bytes
	SwapTotal        Bytes
	SwapUsed         Bytes
	DiskTotal        Bytes
	DiskUsed         Bytes
	Load1            float64
	Load5            float64
	Load15           float64
	Uptime           time.Duration
	DockerImages     Bytes
	DockerBuildCache Bytes
}

// Project is an Easypanel project and its services.
type Project struct {
	Name      string
	Reconcile ReconcileState
	Services  []Service
}

// Service is the measurement of one Swarm service.
type Service struct {
	Name               string
	Managed            bool
	State              ServiceState
	ReplicasRunning    int
	ReplicasDesired    int
	RecentTaskFailures int
	MemoryUsed         Bytes
	MemoryLimit        Bytes
	CPUPercent         float64
	CPULimit           float64
	Disk               Bytes
	NetRx              Bytes
	NetTx              Bytes
	BlockRead          Bytes
	BlockWrite         Bytes
}
```

`internal/core/domain/build.go`:

```go
package domain

import (
	"slices"
	"strings"
	"time"
)

// ReportInput is everything BuildReport needs.
type ReportInput struct {
	Host         string
	AgentVersion string
	Now          time.Time
	HostSample   HostSample
	DockerSample DockerSample
	DirSizes     map[string]Bytes
}

// BuildReport turns one host reading and one Docker reading into a Report.
// Swarm services named without "<project>_<service>" (Easypanel's own) are left out.
func BuildReport(in ReportInput) Report {
	return Report{
		Host:         in.Host,
		MeasuredAt:   in.Now,
		AgentVersion: in.AgentVersion,
		System:       buildSystem(in.HostSample, in.DockerSample),
		Projects:     buildProjects(in.DockerSample, in.DirSizes),
	}
}

func buildSystem(h HostSample, d DockerSample) System {
	return System{
		RAMTotal:         h.MemTotal,
		RAMUsed:          h.MemTotal - h.MemAvailable,
		SwapTotal:        h.SwapTotal,
		SwapUsed:         h.SwapTotal - h.SwapFree,
		DiskTotal:        h.DiskTotal,
		DiskUsed:         h.DiskUsed,
		Load1:            h.Load1,
		Load5:            h.Load5,
		Load15:           h.Load15,
		Uptime:           h.Uptime,
		DockerImages:     d.ImagesSize,
		DockerBuildCache: d.BuildCacheSize,
	}
}

func buildProjects(d DockerSample, dirSizes map[string]Bytes) []Project {
	byName := map[string]*Project{}
	for _, svc := range d.Services {
		project, name, ok := SplitServiceName(svc.Name)
		if !ok {
			continue
		}
		p, found := byName[project]
		if !found {
			p = &Project{Name: project, Reconcile: StateUnmanaged}
			byName[project] = p
		}
		p.Services = append(p.Services, buildService(name, svc, d, dirSizes))
	}
	projects := make([]Project, 0, len(byName))
	for _, p := range byName {
		slices.SortFunc(p.Services, func(a, b Service) int { return strings.Compare(a.Name, b.Name) })
		projects = append(projects, *p)
	}
	slices.SortFunc(projects, func(a, b Project) int { return strings.Compare(a.Name, b.Name) })
	return projects
}

// SplitServiceName splits a Swarm service name "<project>_<service>" at the first underscore.
func SplitServiceName(full string) (project, service string, ok bool) {
	project, service, ok = strings.Cut(full, "_")
	return project, service, ok && project != "" && service != ""
}

func buildService(name string, svc SwarmService, d DockerSample, dirSizes map[string]Bytes) Service {
	out := Service{
		Name:            name,
		State:           StateMissing,
		ReplicasDesired: svc.ReplicasDesired,
		MemoryLimit:     svc.MemoryLimit,
		CPULimit:        svc.CPULimit,
	}
	for _, task := range d.Tasks {
		if task.ServiceID != svc.ID {
			continue
		}
		switch task.State {
		case TaskRunning:
			out.ReplicasRunning++
		case TaskFailed, TaskRejected:
			out.RecentTaskFailures++
		}
	}
	volumes := map[string]bool{}
	for _, c := range d.Containers {
		if c.ServiceName != svc.Name {
			continue
		}
		out.State = higherState(out.State, c.State)
		for _, v := range c.Volumes {
			volumes[v] = true
		}
		if c.Stats != nil {
			out.MemoryUsed += c.Stats.MemoryUsed
			out.CPUPercent += c.Stats.CPUPercent
			out.NetRx += c.Stats.NetRx
			out.NetTx += c.Stats.NetTx
			out.BlockRead += c.Stats.BlockRead
			out.BlockWrite += c.Stats.BlockWrite
		}
	}
	for v := range volumes {
		out.Disk += d.VolumeSizes[v]
	}
	for _, path := range BindMountPaths(d, svc.Name) {
		out.Disk += dirSizes[path]
	}
	return out
}

// stateOrder ranks service states from least to most alive.
var stateOrder = []ServiceState{
	StateMissing, StateDead, StateExited, StateCreated, StatePaused, StateRestarting, StateRunning,
}

// higherState keeps the most alive of two states; a state outside stateOrder never wins.
func higherState(current, candidate ServiceState) ServiceState {
	return stateOrder[max(slices.Index(stateOrder, current), slices.Index(stateOrder, candidate))]
}
```

`internal/core/domain/mounts.go`:

```go
package domain

import (
	"path"
	"slices"
	"strings"
)

// EasypanelProjectsDir is where Easypanel keeps the bind-mounted data of each service.
const EasypanelProjectsDir = "/etc/easypanel/projects"

// BindMountPaths lists, without repeats and sorted, the bind sources of the containers of
// the Swarm service fullName that live under that service's Easypanel directory.
func BindMountPaths(d DockerSample, fullName string) []string {
	project, service, ok := SplitServiceName(fullName)
	if !ok {
		return nil
	}
	prefix := path.Join(EasypanelProjectsDir, project, service) + "/"
	var paths []string
	for _, c := range d.Containers {
		if c.ServiceName != fullName {
			continue
		}
		for _, src := range c.BindSources {
			clean := path.Clean(src)
			if strings.HasPrefix(clean+"/", prefix) && !slices.Contains(paths, clean) {
				paths = append(paths, clean)
			}
		}
	}
	slices.Sort(paths)
	return paths
}

// AllBindMountPaths lists the bind paths of every project service in the sample.
func AllBindMountPaths(d DockerSample) []string {
	var paths []string
	for _, svc := range d.Services {
		paths = append(paths, BindMountPaths(d, svc.Name)...)
	}
	return paths
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -cover ./internal/core/domain/ && make lint`
Expected: `ok ... coverage: 100.0% of statements`; lint clean.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add report domain and builder

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Ports, fakes and the `report` and `doctor` use cases

**Files:**
- Create: `internal/core/port/port.go`, `internal/testkit/fake/fake.go`, `internal/core/usecase/report/report.go`, `internal/core/usecase/doctor/doctor.go`
- Test: `internal/core/usecase/report/report_test.go`, `internal/core/usecase/doctor/doctor_test.go`

**Interfaces:**
- Consumes: Task 4 domain types and `BuildReport`, `AllBindMountPaths`.
- Produces:
  - ports `HostMetrics.Read(ctx) (domain.HostSample, error)`, `DirSizer.DirSize(ctx, path string) (domain.Bytes, error)`, `Docker.Ping(ctx) error` + `Docker.Snapshot(ctx) (domain.DockerSample, error)`, `ReportSink.Publish(ctx, domain.Report) error`, `Clock.Now() time.Time`, `ConfigValidator.Validate() error`
  - fakes `fake.HostMetrics{Sample, Err}`, `fake.Docker{Sample, SnapshotErr, PingErr}`, `fake.DirSizer{Sizes, Errs}`, `*fake.Sink{Published, Err}`, `fake.Clock{T}`, `fake.ConfigValidator{Err}`
  - `report.Deps{Host, AgentVersion, Clock, Metrics, Docker, Dirs, Sink, Logger}`, `report.New(Deps) *report.UseCase`, `(*UseCase).Run(ctx) error`
  - `doctor.Check{Name string; Run func(ctx) error}`, `doctor.Result{Check string; Err error}`, `doctor.Run(ctx, ...Check) []Result`, `doctor.Healthy([]Result) bool`, `doctor.ConfigCheck(port.ConfigValidator) Check`, `doctor.DockerCheck(port.Docker) Check`

- [ ] **Step 1: Write the ports and the fakes**

`internal/core/port/port.go`:

```go
// Package port declares the interfaces the use cases need from the outside world.
package port

import (
	"context"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// HostMetrics reads memory, load, uptime and disk of the host.
type HostMetrics interface {
	Read(ctx context.Context) (domain.HostSample, error)
}

// DirSizer measures the size of a directory tree on the host.
type DirSizer interface {
	DirSize(ctx context.Context, path string) (domain.Bytes, error)
}

// Docker reads the Docker engine.
type Docker interface {
	Ping(ctx context.Context) error
	Snapshot(ctx context.Context) (domain.DockerSample, error)
}

// ReportSink receives a finished report.
type ReportSink interface {
	Publish(ctx context.Context, r domain.Report) error
}

// Clock tells the current time.
type Clock interface {
	Now() time.Time
}

// ConfigValidator checks the agent configuration.
type ConfigValidator interface {
	Validate() error
}
```

`internal/testkit/fake/fake.go`:

```go
// Package fake has hand-written fakes for every port.
package fake

import (
	"context"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// HostMetrics returns Sample or Err.
type HostMetrics struct {
	Sample domain.HostSample
	Err    error
}

// Read implements port.HostMetrics.
func (f HostMetrics) Read(context.Context) (domain.HostSample, error) { return f.Sample, f.Err }

// Docker returns Sample or SnapshotErr, and PingErr on Ping.
type Docker struct {
	Sample      domain.DockerSample
	SnapshotErr error
	PingErr     error
}

// Ping implements port.Docker.
func (f Docker) Ping(context.Context) error { return f.PingErr }

// Snapshot implements port.Docker.
func (f Docker) Snapshot(context.Context) (domain.DockerSample, error) {
	return f.Sample, f.SnapshotErr
}

// DirSizer answers from Sizes, or fails for paths in Errs.
type DirSizer struct {
	Sizes map[string]domain.Bytes
	Errs  map[string]error
}

// DirSize implements port.DirSizer.
func (f DirSizer) DirSize(_ context.Context, path string) (domain.Bytes, error) {
	return f.Sizes[path], f.Errs[path]
}

// Sink records published reports, or fails with Err.
type Sink struct {
	Published []domain.Report
	Err       error
}

// Publish implements port.ReportSink.
func (f *Sink) Publish(_ context.Context, r domain.Report) error {
	f.Published = append(f.Published, r)
	return f.Err
}

// Clock always returns T.
type Clock struct{ T time.Time }

// Now implements port.Clock.
func (f Clock) Now() time.Time { return f.T }

// ConfigValidator returns Err.
type ConfigValidator struct{ Err error }

// Validate implements port.ConfigValidator.
func (f ConfigValidator) Validate() error { return f.Err }
```

- [ ] **Step 2: Write the failing use-case tests**

`internal/core/usecase/report/report_test.go`:

```go
package report_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var (
	now     = time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

const bindDir = "/etc/easypanel/projects/p/db/data"

func dbSample() domain.DockerSample {
	return domain.DockerSample{
		Services:   []domain.SwarmService{{ID: "1", Name: "p_db", ReplicasDesired: 1}},
		Containers: []domain.Container{{ID: "a", ServiceName: "p_db", State: domain.StateRunning, BindSources: []string{bindDir}}},
	}
}

func deps(sink *fake.Sink, logs *bytes.Buffer) report.Deps {
	return report.Deps{
		Host:         "vps-heron-02",
		AgentVersion: "1.0.0",
		Clock:        fake.Clock{T: now},
		Metrics:      fake.HostMetrics{Sample: domain.HostSample{MemTotal: 10, MemAvailable: 4}},
		Docker:       fake.Docker{Sample: dbSample()},
		Dirs:         fake.DirSizer{Sizes: map[string]domain.Bytes{bindDir: 7}},
		Sink:         sink,
		Logger: slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		})),
	}
}

func TestRun_PublishesTheBuiltReport(t *testing.T) {
	sink := &fake.Sink{}
	uc := report.New(deps(sink, &bytes.Buffer{}))

	testkit.MustDo(t, uc.Run(context.Background()))

	testkit.Equal(t, []domain.Report{{
		Host:         "vps-heron-02",
		MeasuredAt:   now,
		AgentVersion: "1.0.0",
		System:       domain.System{RAMTotal: 10, RAMUsed: 6},
		Projects: []domain.Project{{Name: "p", Reconcile: domain.StateUnmanaged, Services: []domain.Service{
			{Name: "db", State: domain.StateRunning, ReplicasDesired: 1, Disk: 7},
		}}},
	}}, sink.Published)
}

func TestRun_FailsWhenHostMetricsFail(t *testing.T) {
	d := deps(&fake.Sink{}, &bytes.Buffer{})
	d.Metrics = fake.HostMetrics{Err: errBoom}

	err := report.New(d).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenDockerFails(t *testing.T) {
	d := deps(&fake.Sink{}, &bytes.Buffer{})
	d.Docker = fake.Docker{SnapshotErr: errBoom}

	err := report.New(d).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailsWhenSinkFails(t *testing.T) {
	err := report.New(deps(&fake.Sink{Err: errBoom}, &bytes.Buffer{})).Run(context.Background())

	testkit.ErrorIs(t, err, errBoom)
}

func TestRun_FailedDirCountsAsZero(t *testing.T) {
	sink := &fake.Sink{}
	d := deps(sink, &bytes.Buffer{})
	d.Dirs = fake.DirSizer{Errs: map[string]error{bindDir: errBoom}}

	testkit.MustDo(t, report.New(d).Run(context.Background()))

	testkit.Equal(t, domain.Bytes(0), sink.Published[0].Projects[0].Services[0].Disk)
}

func TestRun_FailedDirIsLogged(t *testing.T) {
	logs := &bytes.Buffer{}
	d := deps(&fake.Sink{}, logs)
	d.Dirs = fake.DirSizer{Errs: map[string]error{bindDir: errBoom}}

	testkit.MustDo(t, report.New(d).Run(context.Background()))

	testkit.Equal(t,
		`{"level":"WARN","msg":"dir size failed","logId":"report.dir-size-failed","path":"`+bindDir+`","errorName":"*errors.errorString","errorMessage":"boom"}`+"\n",
		logs.String())
}
```

`internal/core/usecase/doctor/doctor_test.go`:

```go
package doctor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/heronlabs/vps-agent/internal/core/usecase/doctor"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

var errBoom = errors.New("boom")

func TestRun_RunsEveryCheckInOrderWhenAllPass(t *testing.T) {
	got := doctor.Run(context.Background(),
		doctor.ConfigCheck(fake.ConfigValidator{}),
		doctor.DockerCheck(fake.Docker{}))

	testkit.Equal(t, []doctor.Result{{Check: "config"}, {Check: "docker"}}, got, cmpopts.EquateErrors())
}

func TestRun_StopsAtTheFirstFailure(t *testing.T) {
	got := doctor.Run(context.Background(),
		doctor.ConfigCheck(fake.ConfigValidator{Err: errBoom}),
		doctor.DockerCheck(fake.Docker{}))

	testkit.Equal(t, []doctor.Result{{Check: "config", Err: errBoom}}, got, cmpopts.EquateErrors())
}

func TestDockerCheck_ReportsPingFailure(t *testing.T) {
	got := doctor.Run(context.Background(), doctor.DockerCheck(fake.Docker{PingErr: errBoom}))

	testkit.Equal(t, []doctor.Result{{Check: "docker", Err: errBoom}}, got, cmpopts.EquateErrors())
}

func TestHealthy_TrueWhenEveryResultPassed(t *testing.T) {
	got := doctor.Healthy([]doctor.Result{{Check: "config"}, {Check: "docker"}})

	testkit.Equal(t, true, got)
}

func TestHealthy_FalseWhenAnyResultFailed(t *testing.T) {
	got := doctor.Healthy([]doctor.Result{{Check: "config"}, {Check: "docker", Err: errBoom}})

	testkit.Equal(t, false, got)
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `go test ./internal/core/usecase/...`
Expected: FAIL — `undefined: report.New`, `undefined: doctor.Run`.

- [ ] **Step 4: Write the use cases**

`internal/core/usecase/report/report.go`:

```go
// Package report collects one measurement of the host and hands it to a sink.
package report

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/port"
)

// Deps are the collaborators of the report use case.
type Deps struct {
	Host         string
	AgentVersion string
	Clock        port.Clock
	Metrics      port.HostMetrics
	Docker       port.Docker
	Dirs         port.DirSizer
	Sink         port.ReportSink
	Logger       *slog.Logger
}

// UseCase collects and publishes reports.
type UseCase struct{ deps Deps }

// New builds the report use case.
func New(deps Deps) *UseCase { return &UseCase{deps: deps} }

// Run collects one report and publishes it.
func (u *UseCase) Run(ctx context.Context) error {
	host, err := u.deps.Metrics.Read(ctx)
	if err != nil {
		return fmt.Errorf("read host metrics: %w", err)
	}
	docker, err := u.deps.Docker.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read docker: %w", err)
	}
	r := domain.BuildReport(domain.ReportInput{
		Host:         u.deps.Host,
		AgentVersion: u.deps.AgentVersion,
		Now:          u.deps.Clock.Now(),
		HostSample:   host,
		DockerSample: docker,
		DirSizes:     u.dirSizes(ctx, domain.AllBindMountPaths(docker)),
	})
	if err := u.deps.Sink.Publish(ctx, r); err != nil {
		return fmt.Errorf("publish report: %w", err)
	}
	return nil
}

// dirSizes measures each path; a path that fails is logged and counts as zero.
func (u *UseCase) dirSizes(ctx context.Context, paths []string) map[string]domain.Bytes {
	sizes := make(map[string]domain.Bytes, len(paths))
	for _, p := range paths {
		size, err := u.deps.Dirs.DirSize(ctx, p)
		if err != nil {
			u.deps.Logger.WarnContext(ctx, "dir size failed",
				slog.String("logId", "report.dir-size-failed"),
				slog.String("path", p),
				slog.String("errorName", fmt.Sprintf("%T", err)),
				slog.String("errorMessage", err.Error()))
			continue
		}
		sizes[p] = size
	}
	return sizes
}
```

`internal/core/usecase/doctor/doctor.go`:

```go
// Package doctor runs the agent's health checks in order.
package doctor

import (
	"context"

	"github.com/heronlabs/vps-agent/internal/core/port"
)

// Check is one named health check.
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// Result is the outcome of one check; Err is nil when it passed.
type Result struct {
	Check string
	Err   error
}

// Run runs checks in order and stops at the first failure.
func Run(ctx context.Context, checks ...Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		err := c.Run(ctx)
		results = append(results, Result{Check: c.Name, Err: err})
		if err != nil {
			break
		}
	}
	return results
}

// Healthy tells whether every result passed.
func Healthy(results []Result) bool {
	for _, r := range results {
		if r.Err != nil {
			return false
		}
	}
	return true
}

// ConfigCheck validates the configuration.
func ConfigCheck(v port.ConfigValidator) Check {
	return Check{Name: "config", Run: func(context.Context) error { return v.Validate() }}
}

// DockerCheck pings the Docker engine through its socket.
func DockerCheck(d port.Docker) Check {
	return Check{Name: "docker", Run: d.Ping}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test -race -cover ./internal/core/... && make lint`
Expected: both use-case packages `coverage: 100.0% of statements`; lint clean.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add ports, fakes and report and doctor use cases

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: `hostfs` adapter — /proc, statfs and directory sizes

**Files:**
- Create: `internal/infrastructure/hostfs/metrics.go`, `internal/infrastructure/hostfs/dirsize.go`, `internal/infrastructure/hostfs/statfs_linux.go`, `internal/infrastructure/hostfs/statfs_other.go`
- Test: `internal/infrastructure/hostfs/metrics_test.go`, `internal/infrastructure/hostfs/dirsize_test.go`, `internal/infrastructure/hostfs/statfs_linux_test.go`, `internal/infrastructure/hostfs/statfs_other_test.go`

**Interfaces:**
- Consumes: `domain.HostSample`, `domain.Bytes`, `domain.BytesOf`.
- Produces: `hostfs.ErrMalformed`; `hostfs.DiskStater` (`Stat(path) (total, used domain.Bytes, err error)`); `hostfs.NewMetrics(proc fs.FS, disk DiskStater) *Metrics` (implements `port.HostMetrics`); `hostfs.DirSizer{}` (implements `port.DirSizer`); `hostfs.StatFS{}` (statfs(2) on Linux; `hostfs.ErrUnsupportedPlatform` elsewhere).

`statfs_linux*.go` only compile on Linux; `statfs_other*.go` everywhere else. Coverage and mutation gates run on `ubuntu-24.04` in CI, where the Linux pair counts. On macOS the other pair keeps `make test` at 100%.

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/hostfs/metrics_test.go`:

```go
package hostfs_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const meminfo = `MemTotal:        8126464 kB
MemFree:          512000 kB
MemAvailable:    2774016 kB
Buffers:          100000 kB
SwapTotal:       4194304 kB
SwapFree:        3879731 kB
HugePages_Total:       0
`

var errDisk = errors.New("disk")

type disk struct {
	total, used domain.Bytes
	err         error
}

func (d disk) Stat(string) (domain.Bytes, domain.Bytes, error) { return d.total, d.used, d.err }

func proc(files map[string]string) fstest.MapFS {
	m := fstest.MapFS{
		"meminfo": {Data: []byte(meminfo)},
		"loadavg": {Data: []byte("0.42 0.38 0.35 1/234 5678\n")},
		"uptime":  {Data: []byte("1209600.57 2400000.00\n")},
	}
	for name, data := range files {
		if data == "" {
			delete(m, name)
			continue
		}
		m[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return m
}

func read(files map[string]string, d disk) (domain.HostSample, error) {
	return hostfs.NewMetrics(proc(files), d).Read(context.Background())
}

func TestRead_ParsesProcAndDisk(t *testing.T) {
	got, err := read(nil, disk{total: 97e9, used: 41e9})
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.HostSample{
		MemTotal: 8126464 * 1024, MemAvailable: 2774016 * 1024,
		SwapTotal: 4194304 * 1024, SwapFree: 3879731 * 1024,
		Load1: 0.42, Load5: 0.38, Load15: 0.35,
		Uptime:    1209600 * time.Second,
		DiskTotal: 97e9, DiskUsed: 41e9,
	}, got)
}

func TestRead_MissingSwapCountsAsZero(t *testing.T) {
	got, err := read(map[string]string{"meminfo": "MemTotal: 1 kB\nMemAvailable: 1 kB\n"}, disk{})
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.HostSample{MemTotal: 1024, MemAvailable: 1024, Load1: 0.42, Load5: 0.38, Load15: 0.35, Uptime: 1209600 * time.Second}, got)
}

func TestRead_FailsWithoutMeminfo(t *testing.T) {
	_, err := read(map[string]string{"meminfo": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnMeminfoWithoutMemAvailable(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemTotal: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnMeminfoWithoutMemTotal(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemAvailable: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnNonNumericMeminfo(t *testing.T) {
	_, err := read(map[string]string{"meminfo": "MemTotal: lots kB\nMemAvailable: 1 kB\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWithoutLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnShortLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": "0.42 0.38\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_AcceptsLoadavgWithExactlyThreeFields(t *testing.T) {
	got := testkit.Must(read(map[string]string{"loadavg": "0.1 0.2 0.3"}, disk{}))

	testkit.Equal(t, 0.3, got.Load15)
}

func TestRead_FailsOnNonNumericLoadavg(t *testing.T) {
	_, err := read(map[string]string{"loadavg": "0.42 high 0.35 1/2 3\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWithoutUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": ""}, disk{})

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestRead_FailsOnEmptyUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": "\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsOnNonNumericUptime(t *testing.T) {
	_, err := read(map[string]string{"uptime": "forever 1\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}

func TestRead_FailsWhenDiskFails(t *testing.T) {
	_, err := read(nil, disk{err: errDisk})

	testkit.ErrorIs(t, err, errDisk)
}

func TestRead_FailsOnOverlongMeminfoLine(t *testing.T) {
	_, err := read(map[string]string{"meminfo": strings.Repeat("x", 70*1024) + "\n"}, disk{})

	testkit.ErrorIs(t, err, hostfs.ErrMalformed)
}
```

`internal/infrastructure/hostfs/dirsize_test.go`:

```go
package hostfs_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.MustDo(t, os.MkdirAll(filepath.Join(root, "sub"), 0o750))
	testkit.MustDo(t, os.WriteFile(filepath.Join(root, "a"), make([]byte, 10), 0o600))
	testkit.MustDo(t, os.WriteFile(filepath.Join(root, "sub", "b"), make([]byte, 5), 0o600))
	testkit.MustDo(t, os.Symlink(filepath.Join(root, "a"), filepath.Join(root, "link")))
	return root
}

func TestDirSize_SumsRegularFilesWithoutFollowingLinks(t *testing.T) {
	root := tree(t)

	got, err := hostfs.DirSizer{}.DirSize(context.Background(), root)
	testkit.MustDo(t, err)

	testkit.Equal(t, domain.Bytes(15), got)
}

func TestDirSize_MeasuresABindSourceThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yml")
	testkit.MustDo(t, os.WriteFile(file, make([]byte, 7), 0o600))

	got := testkit.Must(hostfs.DirSizer{}.DirSize(context.Background(), file))

	testkit.Equal(t, domain.Bytes(7), got)
}

func TestDirSize_FailsOnMissingDir(t *testing.T) {
	_, err := hostfs.DirSizer{}.DirSize(context.Background(), filepath.Join(t.TempDir(), "missing"))

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestDirSize_StopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := hostfs.DirSizer{}.DirSize(ctx, tree(t))

	testkit.ErrorIs(t, err, context.Canceled)
}
```

`internal/infrastructure/hostfs/statfs_linux_test.go` (the oracle reads the same syscall to check the arithmetic):

```go
//go:build linux

package hostfs_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

type usage struct{ Total, Used domain.Bytes }

// statfsOracle reads the same filesystem straight from the syscall, to check the adapter's arithmetic.
func statfsOracle(t *testing.T, path string) usage {
	t.Helper()
	var st syscall.Statfs_t
	testkit.MustDo(t, syscall.Statfs(path, &st))
	return usage{
		Total: domain.Bytes(st.Blocks) * domain.Bytes(st.Bsize),
		Used:  domain.Bytes(st.Blocks-st.Bfree) * domain.Bytes(st.Bsize),
	}
}

func TestStatFS_MatchesTheSyscall(t *testing.T) {
	dir := t.TempDir()
	total, used, err := hostfs.StatFS{}.Stat(dir)
	testkit.MustDo(t, err)

	testkit.Equal(t, statfsOracle(t, dir), usage{total, used})
}

func TestStatFS_FailsOnMissingPath(t *testing.T) {
	_, _, err := hostfs.StatFS{}.Stat(filepath.Join(t.TempDir(), "missing"))

	testkit.ErrorIs(t, err, syscall.ENOENT)
}
```

`internal/infrastructure/hostfs/statfs_other_test.go`:

```go
//go:build !linux

package hostfs_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestStatFS_IsUnsupportedOffLinux(t *testing.T) {
	_, _, err := hostfs.StatFS{}.Stat("/")

	testkit.ErrorIs(t, err, hostfs.ErrUnsupportedPlatform)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/infrastructure/hostfs/`
Expected: FAIL — `undefined: hostfs.NewMetrics`.

- [ ] **Step 3: Write the adapter**

`internal/infrastructure/hostfs/metrics.go`:

```go
// Package hostfs reads the host: /proc, statfs and directory sizes.
package hostfs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrMalformed marks a /proc file that does not have the expected shape.
var ErrMalformed = errors.New("malformed proc file")

// DiskStater measures the filesystem that holds a path.
type DiskStater interface {
	Stat(path string) (total, used domain.Bytes, err error)
}

// Metrics reads the host from a procfs and a DiskStater.
type Metrics struct {
	proc fs.FS
	disk DiskStater
}

// NewMetrics builds Metrics over proc (os.DirFS("/proc") in production) and disk.
func NewMetrics(proc fs.FS, disk DiskStater) *Metrics {
	return &Metrics{proc: proc, disk: disk}
}

// Read implements port.HostMetrics.
func (m *Metrics) Read(context.Context) (domain.HostSample, error) {
	var s domain.HostSample
	if err := m.readMemInfo(&s); err != nil {
		return domain.HostSample{}, err
	}
	if err := m.readLoadAvg(&s); err != nil {
		return domain.HostSample{}, err
	}
	if err := m.readUptime(&s); err != nil {
		return domain.HostSample{}, err
	}
	total, used, err := m.disk.Stat("/")
	if err != nil {
		return domain.HostSample{}, err
	}
	s.DiskTotal, s.DiskUsed = total, used
	return s, nil
}

func (m *Metrics) readMemInfo(s *domain.HostSample) error {
	f, err := m.proc.Open("meminfo")
	if err != nil {
		return fmt.Errorf("open meminfo: %w", err)
	}
	defer f.Close()
	fields := map[string]*domain.Bytes{
		"MemTotal": &s.MemTotal, "MemAvailable": &s.MemAvailable,
		"SwapTotal": &s.SwapTotal, "SwapFree": &s.SwapFree,
	}
	found := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		target, wanted := fields[key]
		if !ok || !wanted {
			continue
		}
		kib, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(rest), " kB"), 10, 64)
		if err != nil {
			return fmt.Errorf("%w: meminfo %s: %w", ErrMalformed, key, err)
		}
		*target = domain.Bytes(kib * 1024)
		found[key] = true
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%w: meminfo: %w", ErrMalformed, err)
	}
	if !found["MemTotal"] || !found["MemAvailable"] {
		return fmt.Errorf("%w: meminfo without MemTotal or MemAvailable", ErrMalformed)
	}
	return nil
}

func (m *Metrics) readLoadAvg(s *domain.HostSample) error {
	fields, err := m.fields("loadavg", 3)
	if err != nil {
		return err
	}
	loads := []*float64{&s.Load1, &s.Load5, &s.Load15}
	for i, target := range loads {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return fmt.Errorf("%w: loadavg: %w", ErrMalformed, err)
		}
		*target = v
	}
	return nil
}

func (m *Metrics) readUptime(s *domain.HostSample) error {
	fields, err := m.fields("uptime", 1)
	if err != nil {
		return err
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return fmt.Errorf("%w: uptime: %w", ErrMalformed, err)
	}
	s.Uptime = time.Duration(secs) * time.Second
	return nil
}

// fields reads a one-line proc file and returns its first n whitespace-separated fields.
func (m *Metrics) fields(name string, n int) ([]string, error) {
	raw, err := fs.ReadFile(m.proc, name)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) < n {
		return nil, fmt.Errorf("%w: %s has %d fields, want %d", ErrMalformed, name, len(fields), n)
	}
	return fields[:n], nil
}
```

`internal/infrastructure/hostfs/dirsize.go`:

```go
package hostfs

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// DirSizer sums the sizes of the regular files under a directory, without following links.
type DirSizer struct{}

// DirSize implements port.DirSizer. A root that is a regular file (a single-file bind mount) is
// measured as is; a file that disappears during the walk counts as zero.
func (DirSizer) DirSize(ctx context.Context, root string) (domain.Bytes, error) {
	if info, err := os.Lstat(root); err == nil && info.Mode().IsRegular() {
		return domain.BytesOf(info.Size()), nil
	}
	var total domain.Bytes
	err := fs.WalkDir(os.DirFS(root), ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += domain.BytesOf(info.Size())
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("dir size %s: %w", root, err)
	}
	return total, nil
}
```

`internal/infrastructure/hostfs/statfs_linux.go`:

```go
//go:build linux

package hostfs

import (
	"fmt"
	"syscall"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// StatFS measures a filesystem with statfs(2).
type StatFS struct{}

// Stat implements DiskStater.
func (StatFS) Stat(path string) (total, used domain.Bytes, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	block := domain.Bytes(st.Bsize)
	return domain.Bytes(st.Blocks) * block, domain.Bytes(st.Blocks-st.Bfree) * block, nil
}
```

`internal/infrastructure/hostfs/statfs_other.go`:

```go
//go:build !linux

package hostfs

import (
	"errors"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrUnsupportedPlatform is returned off Linux, where the agent does not run.
var ErrUnsupportedPlatform = errors.New("statfs is only supported on linux")

// StatFS measures a filesystem with statfs(2); off Linux it always fails.
type StatFS struct{}

// Stat implements DiskStater.
func (StatFS) Stat(string) (total, used domain.Bytes, err error) {
	return 0, 0, ErrUnsupportedPlatform
}
```

- [ ] **Step 4: Run the tests here and on Linux**

```bash
go test -race -cover ./internal/infrastructure/hostfs/
GOOS=linux go vet ./internal/infrastructure/hostfs/
docker run --rm -v "$PWD":/src -w /src golang:1.27 go test -cover ./internal/infrastructure/hostfs/
```

Expected: `coverage: 100.0% of statements` both times (the Docker run exercises `statfs_linux.go`). Skip the Docker line if Docker is not running; CI covers it.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: read host metrics from /proc, statfs and directory sizes

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: `docker` adapter — Engine API over the unix socket

**Files:**
- Create: `internal/infrastructure/docker/client.go`, `internal/infrastructure/docker/dto.go`
- Test: `internal/infrastructure/docker/client_test.go`

**Interfaces:**
- Consumes: domain sample types, `domain.BytesOf`.
- Produces: `docker.ErrUnexpectedStatus`; `docker.New(httpClient *http.Client, baseURL string, statsTimeout time.Duration) *Client`; `docker.NewUnix(socketPath string, requestTimeout, statsTimeout time.Duration) *Client`; `(*Client).Ping(ctx) error`; `(*Client).Snapshot(ctx) (domain.DockerSample, error)` — reads `/services`, `/tasks`, `/containers/json?all=1`, `/system/df`, then `/containers/{id}/stats?stream=false` for running containers in parallel, each bounded by `statsTimeout`; a failed or slow stats leaves `Stats` nil.

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/docker/client_test.go` (the fake socket lives under `os.MkdirTemp("", "d")` because `t.TempDir()` paths exceed the 104-byte unix socket limit on macOS):

```go
package docker_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const statsPath = "/containers/c1/stats?stream=false"

var baseRoutes = map[string]string{
	"/_ping": "OK",
	"/services": `[
		{"ID":"s1","Spec":{"Name":"p_api","Mode":{"Replicated":{"Replicas":2}},
			"TaskTemplate":{"Resources":{"Limits":{"NanoCPUs":500000000,"MemoryBytes":536870912}}}}},
		{"ID":"s2","Spec":{"Name":"p_cron","Mode":{"Global":{}}}},
		{"ID":"s3","Spec":{"Name":"p_job","Mode":{}}}]`,
	"/tasks": `[{"ServiceID":"s1","Status":{"State":"running"}}]`,
	"/containers/json?all=1": `[
		{"Id":"c1","State":"running","Labels":{"com.docker.swarm.service.name":"p_api"},"Mounts":[
			{"Type":"volume","Name":"p_api_data","Source":"/var/lib/docker/volumes/p_api_data/_data"},
			{"Type":"bind","Source":"/etc/easypanel/projects/p/api/code"},
			{"Type":"tmpfs"}]},
		{"Id":"c2","State":"exited","Labels":{}}]`,
	"/system/df": `{"LayersSize":6100000000,"BuildCache":[{"Size":2000000000},{"Size":400000000}],
		"Volumes":[{"Name":"p_api_data","UsageData":{"Size":800}},{"Name":"orphan","UsageData":{"Size":-1}}]}`,
	statsPath: `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000},
		"memory_stats":{"usage":1000,"stats":{"inactive_file":200}},
		"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20},"eth1":{"rx_bytes":1,"tx_bytes":2}},
		"blkio_stats":{"io_service_bytes_recursive":[{"op":"read","value":30},{"op":"Write","value":40},{"op":"discard","value":9}]}}`,
}

func routes(overrides map[string]string) map[string]string {
	r := maps.Clone(baseRoutes)
	for path, body := range overrides {
		if body == "" {
			delete(r, path)
			continue
		}
		r[path] = body
	}
	return r
}

func client(t *testing.T, r map[string]string, statsTimeout time.Duration) *docker.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, ok := r[req.URL.RequestURI()]
		if !ok {
			http.NotFound(w, req)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return docker.New(srv.Client(), srv.URL, statsTimeout)
}

func snapshot(t *testing.T, overrides map[string]string) (domain.DockerSample, error) {
	t.Helper()
	return client(t, routes(overrides), time.Second).Snapshot(context.Background())
}

func statsOf(t *testing.T, statsBody string) *domain.ContainerStats {
	t.Helper()
	s := testkit.Must(snapshot(t, map[string]string{statsPath: statsBody}))
	return s.Containers[0].Stats
}

func TestPing_SucceedsWhenEngineAnswersOK(t *testing.T) {
	err := client(t, baseRoutes, time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, nil)
}

func TestPing_FailsOnNonOKStatus(t *testing.T) {
	err := client(t, routes(map[string]string{"/_ping": ""}), time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestPing_FailsWhenEngineIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := docker.New(srv.Client(), srv.URL, time.Second)
	srv.Close()

	err := c.Ping(context.Background())

	testkit.ErrorIs(t, err, syscall.ECONNREFUSED)
}

func TestPing_FailsOnInvalidBaseURL(t *testing.T) {
	var urlErr *url.Error

	err := docker.New(http.DefaultClient, "http://bad host", time.Second).Ping(context.Background())

	testkit.Equal(t, true, errors.As(err, &urlErr))
}

func TestNewUnix_TalksThroughTheSocket(t *testing.T) {
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	ln := testkit.Must(net.Listen("unix", socket))
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "OK")
	}), ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	err := docker.NewUnix(socket, time.Second, time.Second).Ping(context.Background())

	testkit.ErrorIs(t, err, nil)
}

func TestSnapshot_MapsTheEngineState(t *testing.T) {
	got := testkit.Must(snapshot(t, nil))

	testkit.Equal(t, domain.DockerSample{
		Services: []domain.SwarmService{
			{ID: "s1", Name: "p_api", ReplicasDesired: 2, MemoryLimit: 536870912, CPULimit: 0.5},
			{ID: "s2", Name: "p_cron", ReplicasDesired: 1},
			{ID: "s3", Name: "p_job"},
		},
		Tasks: []domain.SwarmTask{{ServiceID: "s1", State: domain.TaskRunning}},
		Containers: []domain.Container{
			{ID: "c1", ServiceName: "p_api", State: domain.StateRunning,
				Volumes:     []string{"p_api_data"},
				BindSources: []string{"/etc/easypanel/projects/p/api/code"},
				Stats: &domain.ContainerStats{
					MemoryUsed: 800, CPUPercent: 4, NetRx: 11, NetTx: 22, BlockRead: 30, BlockWrite: 40,
				}},
			{ID: "c2", State: domain.StateExited},
		},
		VolumeSizes:    map[string]domain.Bytes{"p_api_data": 800, "orphan": 0},
		ImagesSize:     6100000000,
		BuildCacheSize: 2400000000,
	}, got)
}

func TestSnapshot_FailsWhenServicesFail(t *testing.T) {
	_, err := snapshot(t, map[string]string{"/services": ""})

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsWhenNodeIsNotASwarmManager(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"This node is not a swarm manager."}`, http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	_, err := docker.New(srv.Client(), srv.URL, time.Second).Snapshot(context.Background())

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsWhenDiskUsageFails(t *testing.T) {
	_, err := snapshot(t, map[string]string{"/system/df": ""})

	testkit.ErrorIs(t, err, docker.ErrUnexpectedStatus)
}

func TestSnapshot_FailsOnInvalidJSON(t *testing.T) {
	var syntaxErr *json.SyntaxError

	_, err := snapshot(t, map[string]string{"/tasks": "{"})

	testkit.Equal(t, true, errors.As(err, &syntaxErr) || errors.Is(err, io.ErrUnexpectedEOF))
}

func TestSnapshot_FailedStatsLeaveStatsEmpty(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{statsPath: ""}))

	testkit.Equal(t, (*domain.ContainerStats)(nil), got.Containers[0].Stats)
}

func TestSnapshot_SlowStatsLeaveStatsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.RequestURI() == statsPath {
			<-req.Context().Done()
			return
		}
		_, _ = io.WriteString(w, baseRoutes[req.URL.RequestURI()])
	}))
	t.Cleanup(srv.Close)
	c := docker.New(srv.Client(), srv.URL, 20*time.Millisecond)

	got := testkit.Must(c.Snapshot(context.Background()))

	testkit.Equal(t, (*domain.ContainerStats)(nil), got.Containers[0].Stats)
}

func TestSnapshot_MemoryUsesCgroupV1InactiveFile(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":1000,"stats":{"total_inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(700), got.MemoryUsed)
}

func TestSnapshot_MemoryNeverGoesBelowZero(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":100,"stats":{"inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(100), got.MemoryUsed)
}

func TestSnapshot_CPUIsZeroWithoutSystemDelta(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":100000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_CPUIsZeroWithoutUsageDelta(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_CPUIsZeroWhenUsageCounterGoesBack(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":200000,"online_cpus":2},
		"precpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 0.0, got.CPUPercent)
}

func TestSnapshot_MemoryKeepsUsageWhenInactiveCacheEqualsIt(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":300,"stats":{"inactive_file":300}}}`)

	testkit.Equal(t, domain.Bytes(300), got.MemoryUsed)
}

func TestSnapshot_MemoryPrefersTheCgroupV1Key(t *testing.T) {
	got := statsOf(t, `{"memory_stats":{"usage":1000,"stats":{"total_inactive_file":300,"inactive_file":100}}}`)

	testkit.Equal(t, domain.Bytes(700), got.MemoryUsed)
}

func TestSnapshot_RemovingContainerReadsAsExited(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{"/containers/json?all=1": `[{"Id":"c9","State":"removing"}]`}))

	testkit.Equal(t, domain.StateExited, got.Containers[0].State)
}

func TestSnapshot_UnknownContainerStateReadsAsDead(t *testing.T) {
	got := testkit.Must(snapshot(t, map[string]string{"/containers/json?all=1": `[{"Id":"c9","State":"zombie"}]`}))

	testkit.Equal(t, domain.StateDead, got.Containers[0].State)
}

func TestSnapshot_CPUAssumesOneCoreWhenOnlineCPUsIsMissing(t *testing.T) {
	got := statsOf(t, `{"cpu_stats":{"cpu_usage":{"total_usage":3000},"system_cpu_usage":200000},
		"precpu_stats":{"cpu_usage":{"total_usage":1000},"system_cpu_usage":100000}}`)

	testkit.Equal(t, 2.0, got.CPUPercent)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/infrastructure/docker/`
Expected: FAIL — `undefined: docker.New`.

- [ ] **Step 3: Write the adapter**

`internal/infrastructure/docker/client.go`:

```go
// Package docker reads the Docker Engine API through its unix socket, without the SDK.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// ErrUnexpectedStatus marks an Engine API answer other than 200.
var ErrUnexpectedStatus = errors.New("unexpected docker status")

// Client talks to the Engine API.
type Client struct {
	http         *http.Client
	baseURL      string
	statsTimeout time.Duration
}

// New builds a Client over any HTTP client; baseURL has no trailing slash.
func New(httpClient *http.Client, baseURL string, statsTimeout time.Duration) *Client {
	return &Client{http: httpClient, baseURL: baseURL, statsTimeout: statsTimeout}
}

// NewUnix builds a Client that dials the engine's unix socket. requestTimeout bounds every
// call; statsTimeout bounds each container's stats, which take about a second.
func NewUnix(socketPath string, requestTimeout, statsTimeout time.Duration) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return New(&http.Client{Transport: transport, Timeout: requestTimeout}, "http://docker", statsTimeout)
}

// Ping implements port.Docker.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.get(ctx, "/_ping")
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// Snapshot implements port.Docker. A container whose stats fail or time out has nil Stats.
func (c *Client) Snapshot(ctx context.Context) (domain.DockerSample, error) {
	var (
		services   []serviceDTO
		tasks      []taskDTO
		containers []containerDTO
		df         diskUsageDTO
	)
	reads := []struct {
		path string
		out  any
	}{
		{"/services", &services},
		{"/tasks", &tasks},
		{"/containers/json?all=1", &containers},
		{"/system/df", &df},
	}
	for _, r := range reads {
		if err := c.getJSON(ctx, r.path, r.out); err != nil {
			return domain.DockerSample{}, err
		}
	}
	sample := toSample(services, tasks, containers, df)
	c.fillStats(ctx, sample.Containers)
	return sample, nil
}

func (c *Client) fillStats(ctx context.Context, containers []domain.Container) {
	var wg sync.WaitGroup
	for i := range containers {
		if containers[i].State != domain.StateRunning {
			continue
		}
		wg.Go(func() { containers[i].Stats = c.stats(ctx, containers[i].ID) })
	}
	wg.Wait()
}

func (c *Client) stats(ctx context.Context, id string) *domain.ContainerStats {
	ctx, cancel := context.WithTimeout(ctx, c.statsTimeout)
	defer cancel()
	var s statsDTO
	if err := c.getJSON(ctx, "/containers/"+id+"/stats?stream=false", &s); err != nil {
		return nil
	}
	stats := s.toDomain()
	return &stats
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.get(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode GET %s: %w", path, err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build GET %s: %w", path, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: GET %s: %d", ErrUnexpectedStatus, path, resp.StatusCode)
	}
	return resp, nil
}
```

`internal/infrastructure/docker/dto.go` (memory and CPU follow the `docker stats` formulas):

```go
package docker

import (
	"strings"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

const swarmServiceLabel = "com.docker.swarm.service.name"

type serviceDTO struct {
	ID   string
	Spec struct {
		Name string
		Mode struct {
			Replicated *struct{ Replicas int }
			Global     *struct{}
		}
		TaskTemplate struct {
			Resources struct {
				Limits struct {
					NanoCPUs    int64
					MemoryBytes int64
				}
			}
		}
	}
}

type taskDTO struct {
	ServiceID string
	Status    struct{ State string }
}

type containerDTO struct {
	ID     string `json:"Id"`
	State  string
	Labels map[string]string
	Mounts []struct {
		Type   string
		Name   string
		Source string
	}
}

type diskUsageDTO struct {
	LayersSize int64
	BuildCache []struct{ Size int64 }
	Volumes    []struct {
		Name      string
		UsageData struct{ Size int64 }
	}
}

type cpuDTO struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs  uint64 `json:"online_cpus"`
}

type statsDTO struct {
	CPU         cpuDTO `json:"cpu_stats"`
	PreCPU      cpuDTO `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IOServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
}

func toSample(services []serviceDTO, tasks []taskDTO, containers []containerDTO, df diskUsageDTO) domain.DockerSample {
	s := domain.DockerSample{VolumeSizes: map[string]domain.Bytes{}, ImagesSize: domain.BytesOf(df.LayersSize)}
	for _, svc := range services {
		s.Services = append(s.Services, svc.toDomain())
	}
	for _, t := range tasks {
		s.Tasks = append(s.Tasks, domain.SwarmTask{ServiceID: t.ServiceID, State: domain.TaskState(t.Status.State)})
	}
	for _, c := range containers {
		s.Containers = append(s.Containers, c.toDomain())
	}
	for _, b := range df.BuildCache {
		s.BuildCacheSize += domain.BytesOf(b.Size)
	}
	for _, v := range df.Volumes {
		s.VolumeSizes[v.Name] = domain.BytesOf(v.UsageData.Size)
	}
	return s
}

func (s serviceDTO) toDomain() domain.SwarmService {
	desired := 0
	if s.Spec.Mode.Global != nil {
		desired = 1
	}
	if r := s.Spec.Mode.Replicated; r != nil {
		desired = r.Replicas
	}
	limits := s.Spec.TaskTemplate.Resources.Limits
	return domain.SwarmService{
		ID:              s.ID,
		Name:            s.Spec.Name,
		ReplicasDesired: desired,
		MemoryLimit:     domain.BytesOf(limits.MemoryBytes),
		CPULimit:        float64(limits.NanoCPUs) / 1e9,
	}
}

func (c containerDTO) toDomain() domain.Container {
	out := domain.Container{ID: c.ID, ServiceName: c.Labels[swarmServiceLabel], State: containerState(c.State)}
	for _, m := range c.Mounts {
		switch m.Type {
		case "volume":
			out.Volumes = append(out.Volumes, m.Name)
		case "bind":
			out.BindSources = append(out.BindSources, m.Source)
		}
	}
	return out
}

func (s statsDTO) toDomain() domain.ContainerStats {
	out := domain.ContainerStats{MemoryUsed: domain.Bytes(memoryUsed(s.MemoryStats.Usage, s.MemoryStats.Stats))}
	out.CPUPercent = cpuPercent(s.CPU, s.PreCPU)
	for _, n := range s.Networks {
		out.NetRx += domain.Bytes(n.RxBytes)
		out.NetTx += domain.Bytes(n.TxBytes)
	}
	for _, io := range s.BlkioStats.IOServiceBytesRecursive {
		switch strings.ToLower(io.Op) {
		case "read":
			out.BlockRead += domain.Bytes(io.Value)
		case "write":
			out.BlockWrite += domain.Bytes(io.Value)
		}
	}
	return out
}

// containerState maps the engine's container states to the contract; "removing" reads as exited
// and anything unknown as dead.
func containerState(s string) domain.ServiceState {
	switch s {
	case "running", "restarting", "paused", "created", "exited", "dead":
		return domain.ServiceState(s)
	case "removing":
		return domain.StateExited
	}
	return domain.StateDead
}

// memoryUsed follows the docker CLI: usage minus the inactive page cache (cgroup v1 key first,
// then v2), unless the cache is not below the usage.
func memoryUsed(usage uint64, stats map[string]uint64) uint64 {
	inactive, ok := stats["total_inactive_file"]
	if !ok {
		inactive = stats["inactive_file"]
	}
	if inactive < usage {
		return usage - inactive
	}
	return usage
}

// cpuPercent follows the docker CLI: 100 is one full core.
func cpuPercent(cur, pre cpuDTO) float64 {
	if cur.SystemUsage <= pre.SystemUsage {
		return 0
	}
	cpuDelta := max(float64(cur.CPUUsage.TotalUsage)-float64(pre.CPUUsage.TotalUsage), 0)
	systemDelta := float64(cur.SystemUsage - pre.SystemUsage)
	return cpuDelta / systemDelta * float64(max(cur.OnlineCPUs, 1)) * 100
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race -cover ./internal/infrastructure/docker/ && make lint`
Expected: `coverage: 100.0% of statements`; lint clean.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: read Swarm services, tasks, containers and stats from the Docker socket

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: `cms` contract DTOs and printer, and the system clock

**Files:**
- Create: `internal/infrastructure/cms/dto.go`, `internal/infrastructure/cms/printer.go`, `internal/infrastructure/clock/clock.go`
- Test: `internal/infrastructure/cms/printer_test.go`, `internal/infrastructure/clock/clock_test.go`

**Interfaces:**
- Consumes: `domain.Report`.
- Produces: `cms.ReportDTO` and nested DTOs with the JSON tags of `POST /api/infra-reports`; `cms.ToDTO(domain.Report) cms.ReportDTO` (GiB/GB/MiB/MB, two decimals, `easypanelVersion`/`easypanelSupported`/`lastReconcile` null until phase 3, `missingSecrets: []`, `error: null`); `cms.NewPrinter(io.Writer) *Printer` implementing `port.ReportSink`; `clock.System{}` implementing `port.Clock` in UTC.

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/cms/printer_test.go`:

```go
package cms_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

var errWrite = errors.New("write")

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errWrite }

func sample() domain.Report {
	return domain.Report{
		Host:         "vps-heron-02",
		MeasuredAt:   time.Date(2026, 10, 1, 11, 10, 0, 0, time.FixedZone("BRT", -3*3600)),
		AgentVersion: "1.0.0",
		System: domain.System{
			RAMTotal: 8321499136, RAMUsed: 5476083302, SwapTotal: 4 << 30, SwapUsed: 322122547,
			DiskTotal: 97e9, DiskUsed: 41.2e9, Load1: 0.421, Load5: 0.38, Load15: 0.35,
			Uptime: 1209600*time.Second + 500*time.Millisecond, DockerImages: 6.1e9, DockerBuildCache: 2.4e9,
		},
		Projects: []domain.Project{{Name: "cms-heronlabs", Reconcile: domain.StateUnmanaged, Services: []domain.Service{{
			Name: "cms-database", State: domain.StateRunning, ReplicasRunning: 1, ReplicasDesired: 1,
			MemoryUsed: 310 << 20, MemoryLimit: 512 << 20, CPUPercent: 3.204, CPULimit: 0.5,
			Disk: 8.1e9, NetRx: 120.5e6, NetTx: 88.1e6, BlockRead: 50.2e6, BlockWrite: 12.9e6,
		}}}},
	}
}

func TestToDTO_ConvertsUnitsAndRounds(t *testing.T) {
	got := cms.ToDTO(sample())

	testkit.Equal(t, cms.ReportDTO{
		Host:       "vps-heron-02",
		MeasuredAt: "2026-10-01T14:10:00Z",
		Agent:      cms.AgentDTO{Version: "1.0.0"},
		System: cms.SystemDTO{
			RAMTotalGiB: 7.75, RAMUsedGiB: 5.1, SwapTotalGiB: 4, SwapUsedGiB: 0.3,
			DiskTotalGB: 97, DiskUsedGB: 41.2, Load1: 0.42, Load5: 0.38, Load15: 0.35,
			UptimeSeconds: 1209600, DockerImagesGB: 6.1, DockerBuildCacheGB: 2.4,
		},
		Projects: []cms.ProjectDTO{{
			Name:      "cms-heronlabs",
			Reconcile: cms.ReconcileDTO{State: "unmanaged", MissingSecrets: []string{}},
			Services: []cms.ServiceDTO{{
				Name: "cms-database", State: "running", ReplicasRunning: 1, ReplicasDesired: 1,
				MemoryUsedMiB: 310, MemoryLimitMiB: 512, CPUPercent: 3.2, CPULimit: 0.5,
				DiskGB: 8.1, NetRxMB: 120.5, NetTxMB: 88.1, BlockReadMB: 50.2, BlockWriteMB: 12.9,
			}},
		}},
	}, got)
}

func TestToDTO_NoProjectsIsAnEmptyList(t *testing.T) {
	got := cms.ToDTO(domain.Report{})

	testkit.Equal(t, []cms.ProjectDTO{}, got.Projects)
}

func TestPublish_PrintsContractJSON(t *testing.T) {
	var out bytes.Buffer
	r := domain.Report{Host: "h", MeasuredAt: time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC), AgentVersion: "1.0.0",
		Projects: []domain.Project{{Name: "p", Reconcile: domain.StateUnmanaged}}}

	testkit.MustDo(t, cms.NewPrinter(&out).Publish(context.Background(), r))

	testkit.Equal(t, `{
  "host": "h",
  "measuredAt": "2026-10-01T14:10:00Z",
  "agent": {
    "version": "1.0.0",
    "easypanelVersion": null,
    "easypanelSupported": null,
    "lastReconcile": null
  },
  "system": {
    "ramTotalGiB": 0,
    "ramUsedGiB": 0,
    "swapTotalGiB": 0,
    "swapUsedGiB": 0,
    "diskTotalGB": 0,
    "diskUsedGB": 0,
    "load1": 0,
    "load5": 0,
    "load15": 0,
    "uptimeSeconds": 0,
    "dockerImagesGB": 0,
    "dockerBuildCacheGB": 0
  },
  "projects": [
    {
      "name": "p",
      "reconcile": {
        "state": "unmanaged",
        "missingSecrets": [],
        "error": null
      },
      "services": []
    }
  ]
}
`, out.String())
}

func TestPublish_FailsWhenWriterFails(t *testing.T) {
	err := cms.NewPrinter(brokenWriter{}).Publish(context.Background(), domain.Report{})

	testkit.ErrorIs(t, err, errWrite)
}
```

`internal/infrastructure/clock/clock_test.go`:

```go
package clock_test

import (
	"testing"

	"github.com/heronlabs/vps-agent/internal/infrastructure/clock"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

func TestNow_IsUTC(t *testing.T) {
	got := clock.System{}.Now()

	testkit.Equal(t, "UTC", got.Location().String())
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/infrastructure/cms/ ./internal/infrastructure/clock/`
Expected: FAIL — `undefined: cms.ToDTO`, `undefined: clock.System`.

- [ ] **Step 3: Write the adapters**

`internal/infrastructure/cms/dto.go`:

```go
// Package cms holds the CMS contract DTOs and the adapters that publish reports.
package cms

import (
	"math"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

const (
	gib = 1 << 30
	mib = 1 << 20
	gb  = 1e9
	mb  = 1e6
)

// ReportDTO is the body of POST /api/infra-reports.
type ReportDTO struct {
	Host       string       `json:"host"`
	MeasuredAt string       `json:"measuredAt"`
	Agent      AgentDTO     `json:"agent"`
	System     SystemDTO    `json:"system"`
	Projects   []ProjectDTO `json:"projects"`
}

// AgentDTO describes the agent. Easypanel fields stay null until the agent talks to Easypanel.
type AgentDTO struct {
	Version            string    `json:"version"`
	EasypanelVersion   *string   `json:"easypanelVersion"`
	EasypanelSupported *bool     `json:"easypanelSupported"`
	LastReconcile      *struct{} `json:"lastReconcile"`
}

// SystemDTO is the host block of the report.
type SystemDTO struct {
	RAMTotalGiB        float64 `json:"ramTotalGiB"`
	RAMUsedGiB         float64 `json:"ramUsedGiB"`
	SwapTotalGiB       float64 `json:"swapTotalGiB"`
	SwapUsedGiB        float64 `json:"swapUsedGiB"`
	DiskTotalGB        float64 `json:"diskTotalGB"`
	DiskUsedGB         float64 `json:"diskUsedGB"`
	Load1              float64 `json:"load1"`
	Load5              float64 `json:"load5"`
	Load15             float64 `json:"load15"`
	UptimeSeconds      int64   `json:"uptimeSeconds"`
	DockerImagesGB     float64 `json:"dockerImagesGB"`
	DockerBuildCacheGB float64 `json:"dockerBuildCacheGB"`
}

// ProjectDTO is one Easypanel project of the report.
type ProjectDTO struct {
	Name      string       `json:"name"`
	Reconcile ReconcileDTO `json:"reconcile"`
	Services  []ServiceDTO `json:"services"`
}

// ReconcileDTO is the reconcile outcome of a project.
type ReconcileDTO struct {
	State          string   `json:"state"`
	MissingSecrets []string `json:"missingSecrets"`
	Error          *string  `json:"error"`
}

// ServiceDTO is one service of a project.
type ServiceDTO struct {
	Name               string  `json:"name"`
	Managed            bool    `json:"managed"`
	State              string  `json:"state"`
	ReplicasRunning    int     `json:"replicasRunning"`
	ReplicasDesired    int     `json:"replicasDesired"`
	RecentTaskFailures int     `json:"recentTaskFailures"`
	MemoryUsedMiB      float64 `json:"memoryUsedMiB"`
	MemoryLimitMiB     float64 `json:"memoryLimitMiB"`
	CPUPercent         float64 `json:"cpuPercent"`
	CPULimit           float64 `json:"cpuLimit"`
	DiskGB             float64 `json:"diskGB"`
	NetRxMB            float64 `json:"netRxMB"`
	NetTxMB            float64 `json:"netTxMB"`
	BlockReadMB        float64 `json:"blockReadMB"`
	BlockWriteMB       float64 `json:"blockWriteMB"`
}

// ToDTO converts a domain report into the contract, in its units, rounded to two decimals.
func ToDTO(r domain.Report) ReportDTO {
	projects := make([]ProjectDTO, 0, len(r.Projects))
	for _, p := range r.Projects {
		projects = append(projects, projectDTO(p))
	}
	return ReportDTO{
		Host:       r.Host,
		MeasuredAt: r.MeasuredAt.UTC().Format(time.RFC3339Nano),
		Agent:      AgentDTO{Version: r.AgentVersion},
		System: SystemDTO{
			RAMTotalGiB:        in(r.System.RAMTotal, gib),
			RAMUsedGiB:         in(r.System.RAMUsed, gib),
			SwapTotalGiB:       in(r.System.SwapTotal, gib),
			SwapUsedGiB:        in(r.System.SwapUsed, gib),
			DiskTotalGB:        in(r.System.DiskTotal, gb),
			DiskUsedGB:         in(r.System.DiskUsed, gb),
			Load1:              round2(r.System.Load1),
			Load5:              round2(r.System.Load5),
			Load15:             round2(r.System.Load15),
			UptimeSeconds:      int64(r.System.Uptime / time.Second),
			DockerImagesGB:     in(r.System.DockerImages, gb),
			DockerBuildCacheGB: in(r.System.DockerBuildCache, gb),
		},
		Projects: projects,
	}
}

func projectDTO(p domain.Project) ProjectDTO {
	services := make([]ServiceDTO, 0, len(p.Services))
	for _, s := range p.Services {
		services = append(services, ServiceDTO{
			Name:               s.Name,
			Managed:            s.Managed,
			State:              string(s.State),
			ReplicasRunning:    s.ReplicasRunning,
			ReplicasDesired:    s.ReplicasDesired,
			RecentTaskFailures: s.RecentTaskFailures,
			MemoryUsedMiB:      in(s.MemoryUsed, mib),
			MemoryLimitMiB:     in(s.MemoryLimit, mib),
			CPUPercent:         round2(s.CPUPercent),
			CPULimit:           round2(s.CPULimit),
			DiskGB:             in(s.Disk, gb),
			NetRxMB:            in(s.NetRx, mb),
			NetTxMB:            in(s.NetTx, mb),
			BlockReadMB:        in(s.BlockRead, mb),
			BlockWriteMB:       in(s.BlockWrite, mb),
		})
	}
	return ProjectDTO{
		Name:      p.Name,
		Reconcile: ReconcileDTO{State: string(p.Reconcile), MissingSecrets: []string{}},
		Services:  services,
	}
}

func in(b domain.Bytes, unit float64) float64 { return round2(float64(b) / unit) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
```

`internal/infrastructure/cms/printer.go`:

```go
package cms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/heronlabs/vps-agent/internal/core/domain"
)

// Printer writes reports as indented contract JSON; it is the sink of `report --once`.
type Printer struct{ w io.Writer }

// NewPrinter builds a Printer over w.
func NewPrinter(w io.Writer) *Printer { return &Printer{w: w} }

// Publish implements port.ReportSink.
func (p *Printer) Publish(_ context.Context, r domain.Report) error {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ToDTO(r)); err != nil {
		return fmt.Errorf("print report: %w", err)
	}
	return nil
}
```

`internal/infrastructure/clock/clock.go`:

```go
// Package clock is the real clock.
package clock

import "time"

// System reads the system clock, in UTC.
type System struct{}

// Now implements port.Clock.
func (System) Now() time.Time { return time.Now().UTC() }
```

- [ ] **Step 4: Run the tests**

Run: `go test -race -cover ./internal/infrastructure/cms/ ./internal/infrastructure/clock/ && make lint`
Expected: both `coverage: 100.0% of statements`; lint clean.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: print reports in the CMS contract

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: `config` adapter — `/etc/vps-agent/config.env`

**Files:**
- Create: `internal/infrastructure/config/config.go`
- Test: `internal/infrastructure/config/config_test.go`

**Interfaces:**
- Produces: `config.ErrInvalid`; `config.Config` (all keys of the spec, with defaults `EASYPANEL_URL=http://127.0.0.1:3000`, `AWS_REGION=us-east-1`, `RECONCILE_INTERVAL=5m`, `REPORT_INTERVAL=10m`); `config.Parse(io.Reader) (Config, error)`; `config.Load(fsys fs.FS, name string) (Config, error)` (mode must be 0600); `config.File{Dir fs.FS; Name string}` with `Validate() error` (implements `port.ConfigValidator`) and `HostNameOr(fallback func() (string, error)) (string, error)`.

- [ ] **Step 1: Write the failing tests**

`internal/infrastructure/config/config_test.go`:

```go
package config_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/testkit"
)

const full = `# written by install.sh
CMS_URL=https://cms.heronlabs.com.br

AGENT_TOKEN=token
AGENT_HOST_NAME=vps-heron-02
EASYPANEL_URL=http://127.0.0.1:3001
EASYPANEL_TOKEN=ep
AWS_ACCESS_KEY_ID=AKIA
AWS_SECRET_ACCESS_KEY=secret
AWS_REGION=sa-east-1
RECONCILE_INTERVAL=1m
REPORT_INTERVAL=2m
`

var errFallback = errors.New("fallback")

func dir(content string, mode fs.FileMode) fstest.MapFS {
	return fstest.MapFS{"config.env": {Data: []byte(content), Mode: mode}}
}

func parse(content string) (config.Config, error) {
	return config.Parse(strings.NewReader(content))
}

func hostname() (string, error) { return "fallback-host", nil }

// unreadable stats like a normal file but cannot be opened.
type unreadable struct{ files fstest.MapFS }

func (u unreadable) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func (u unreadable) Stat(name string) (fs.FileInfo, error) { return u.files.Stat(name) }

func TestParse_ReadsEveryKey(t *testing.T) {
	got := testkit.Must(parse(full))

	testkit.Equal(t, config.Config{
		CMSURL: "https://cms.heronlabs.com.br", AgentToken: "token", HostName: "vps-heron-02",
		EasypanelURL: "http://127.0.0.1:3001", EasypanelToken: "ep",
		AWSAccessKeyID: "AKIA", AWSSecretAccessKey: "secret", AWSRegion: "sa-east-1",
		ReconcileInterval: time.Minute, ReportInterval: 2 * time.Minute,
	}, got)
}

func TestParse_EmptyFileGivesDefaults(t *testing.T) {
	got := testkit.Must(parse(""))

	testkit.Equal(t, config.Config{
		EasypanelURL: "http://127.0.0.1:3000", AWSRegion: "us-east-1",
		ReconcileInterval: 5 * time.Minute, ReportInterval: 10 * time.Minute,
	}, got)
}

func TestParse_KeepsEqualsSignsInsideTheValue(t *testing.T) {
	got := testkit.Must(parse("AGENT_TOKEN=a=b\n"))

	testkit.Equal(t, "a=b", got.AgentToken)
}

func TestParse_AcceptsWindowsLineEndings(t *testing.T) {
	got := testkit.Must(parse("CMS_URL=https://cms\r\nREPORT_INTERVAL=1m\r\n"))

	testkit.Equal(t, "https://cms", got.CMSURL)
}

func TestParse_DropsSpacesAroundKeyAndValue(t *testing.T) {
	got := testkit.Must(parse("CMS_URL = https://cms \n"))

	testkit.Equal(t, "https://cms", got.CMSURL)
}

func TestParse_DropsOnePairOfDoubleQuotes(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="a b"` + "\n"))

	testkit.Equal(t, "a b", got.AgentToken)
}

func TestParse_EmptyQuotedValueIsEmpty(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN=""` + "\n"))

	testkit.Equal(t, "", got.AgentToken)
}

func TestParse_KeepsALoneDoubleQuote(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="` + "\n"))

	testkit.Equal(t, `"`, got.AgentToken)
}

func TestParse_KeepsQuotesThatDoNotWrapTheValue(t *testing.T) {
	got := testkit.Must(parse(`AGENT_TOKEN="a"b` + "\n"))

	testkit.Equal(t, `"a"b`, got.AgentToken)
}

func TestParse_RejectsLineWithoutEquals(t *testing.T) {
	_, err := parse("CMS_URL\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_NamesTheLineWithoutEquals(t *testing.T) {
	_, err := parse("# comment\nCMS_URL\n")

	testkit.Equal(t, "invalid config: line 2 has no '='", err.Error())
}

func TestParse_RejectsUnknownKey(t *testing.T) {
	_, err := parse("CMS_ULR=https://cms\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsRepeatedKey(t *testing.T) {
	_, err := parse("CMS_URL=a\nCMS_URL=b\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsInvalidDuration(t *testing.T) {
	_, err := parse("REPORT_INTERVAL=ten\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsZeroDuration(t *testing.T) {
	_, err := parse("RECONCILE_INTERVAL=0s\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestParse_RejectsOverlongLine(t *testing.T) {
	_, err := parse("AGENT_TOKEN=" + strings.Repeat("x", 70*1024) + "\n")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestLoad_ReadsAPrivateFile(t *testing.T) {
	got := testkit.Must(config.Load(dir("AGENT_HOST_NAME=vps-heron-02\n", 0o600), "config.env"))

	testkit.Equal(t, "vps-heron-02", got.HostName)
}

func TestLoad_RejectsFileOthersCanRead(t *testing.T) {
	_, err := config.Load(dir("", 0o644), "config.env")

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestLoad_FailsWhenFileIsMissing(t *testing.T) {
	_, err := config.Load(fstest.MapFS{}, "config.env")

	testkit.ErrorIs(t, err, fs.ErrNotExist)
}

func TestLoad_FailsWhenFileCannotBeRead(t *testing.T) {
	_, err := config.Load(unreadable{dir("", 0o600)}, "config.env")

	testkit.ErrorIs(t, err, fs.ErrPermission)
}

func TestFileValidate_PassesOnValidFile(t *testing.T) {
	err := config.File{Dir: dir(full, 0o600), Name: "config.env"}.Validate()

	testkit.ErrorIs(t, err, nil)
}

func TestFileValidate_FailsOnInvalidFile(t *testing.T) {
	err := config.File{Dir: dir("NOPE=1\n", 0o600), Name: "config.env"}.Validate()

	testkit.ErrorIs(t, err, config.ErrInvalid)
}

func TestHostNameOr_PrefersTheConfiguredName(t *testing.T) {
	got := testkit.Must(config.File{Dir: dir(full, 0o600), Name: "config.env"}.HostNameOr(hostname))

	testkit.Equal(t, "vps-heron-02", got)
}

func TestHostNameOr_FallsBackWhenNameIsUnset(t *testing.T) {
	got := testkit.Must(config.File{Dir: dir("", 0o600), Name: "config.env"}.HostNameOr(hostname))

	testkit.Equal(t, "fallback-host", got)
}

func TestHostNameOr_FallsBackWhenFileIsMissing(t *testing.T) {
	_, err := config.File{Dir: fstest.MapFS{}, Name: "config.env"}.HostNameOr(func() (string, error) {
		return "", errFallback
	})

	testkit.ErrorIs(t, err, errFallback)
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/infrastructure/config/`
Expected: FAIL — `undefined: config.Parse`.

- [ ] **Step 3: Write the adapter**

`internal/infrastructure/config/config.go`:

```go
// Package config reads /etc/vps-agent/config.env.
package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// ErrInvalid marks a configuration file the agent refuses.
var ErrInvalid = errors.New("invalid config")

// Config is the agent configuration. Each subcommand checks only the fields it uses.
type Config struct {
	CMSURL             string
	AgentToken         string
	HostName           string
	EasypanelURL       string
	EasypanelToken     string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	ReconcileInterval  time.Duration
	ReportInterval     time.Duration
}

func defaults() Config {
	return Config{
		EasypanelURL:      "http://127.0.0.1:3000",
		AWSRegion:         "us-east-1",
		ReconcileInterval: 5 * time.Minute,
		ReportInterval:    10 * time.Minute,
	}
}

// Load reads name from fsys (os.DirFS("/etc/vps-agent") in production); the file must be mode 0600.
func Load(fsys fs.FS, name string) (Config, error) {
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return Config{}, fmt.Errorf("stat config: %w", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		return Config{}, fmt.Errorf("%w: %s has mode %o, want 600", ErrInvalid, name, perm)
	}
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return Parse(bytes.NewReader(raw))
}

// Parse reads KEY=VALUE lines; blank lines and lines starting with # are skipped. Spaces around
// the key and the value are dropped, and so is one pair of double quotes around the value.
func Parse(r io.Reader) (Config, error) {
	cfg := defaults()
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return Config{}, fmt.Errorf("%w: line %d has no '='", ErrInvalid, line)
		}
		key, value = strings.TrimSpace(key), unquote(strings.TrimSpace(value))
		if seen[key] {
			return Config{}, fmt.Errorf("%w: %s set twice", ErrInvalid, key)
		}
		seen[key] = true
		if err := cfg.set(key, value); err != nil {
			return Config{}, err
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return cfg, nil
}

func unquote(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		return v[1 : len(v)-1]
	}
	return v
}

func (c *Config) set(key, value string) error {
	texts := map[string]*string{
		"CMS_URL":               &c.CMSURL,
		"AGENT_TOKEN":           &c.AgentToken,
		"AGENT_HOST_NAME":       &c.HostName,
		"EASYPANEL_URL":         &c.EasypanelURL,
		"EASYPANEL_TOKEN":       &c.EasypanelToken,
		"AWS_ACCESS_KEY_ID":     &c.AWSAccessKeyID,
		"AWS_SECRET_ACCESS_KEY": &c.AWSSecretAccessKey,
		"AWS_REGION":            &c.AWSRegion,
	}
	durations := map[string]*time.Duration{
		"RECONCILE_INTERVAL": &c.ReconcileInterval,
		"REPORT_INTERVAL":    &c.ReportInterval,
	}
	if target, ok := texts[key]; ok {
		*target = value
		return nil
	}
	target, ok := durations[key]
	if !ok {
		return fmt.Errorf("%w: unknown key %s", ErrInvalid, key)
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return fmt.Errorf("%w: %s must be a positive duration, got %q", ErrInvalid, key, value)
	}
	*target = d
	return nil
}

// File is the configuration file Name inside Dir.
type File struct {
	Dir  fs.FS
	Name string
}

// Validate implements port.ConfigValidator.
func (f File) Validate() error {
	_, err := Load(f.Dir, f.Name)
	return err
}

// HostNameOr returns AGENT_HOST_NAME when the file loads and sets it, and fallback() otherwise.
func (f File) HostNameOr(fallback func() (string, error)) (string, error) {
	if cfg, err := Load(f.Dir, f.Name); err == nil && cfg.HostName != "" {
		return cfg.HostName, nil
	}
	return fallback()
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race -cover ./internal/infrastructure/config/ && make lint`
Expected: `coverage: 100.0% of statements`; lint clean.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: read and validate config.env

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: The app (`cmd/vps-agent`) and the integration test

**Files:**
- Create: `cmd/vps-agent/main.go`
- Test: `test/integration/report_test.go`, fixtures under `test/integration/testdata/` (`proc/meminfo`, `proc/loadavg`, `proc/uptime`, `docker/*.json`, `docker/ping.txt`, `report-once.golden.json`)

**Interfaces:**
- Consumes: everything above.
- Produces: `vps-agent version`, `vps-agent report --once [--config path]`, `vps-agent doctor [--config path]`; exit codes 0 ok, 1 failure, 2 usage. `main.version` is set by `-ldflags -X`.

- [ ] **Step 1: Write the integration fixtures**

`test/integration/testdata/proc/meminfo`:

```text
MemTotal:        8126464 kB
MemFree:          512000 kB
MemAvailable:    2778726 kB
SwapTotal:       4194304 kB
SwapFree:        3879731 kB
```

`test/integration/testdata/proc/loadavg`:

```text
0.42 0.38 0.35 1/234 5678
```

`test/integration/testdata/proc/uptime`:

```text
1209600.57 2400000.00
```

`test/integration/testdata/docker/ping.txt` contains exactly `OK` with no newline: `printf OK > test/integration/testdata/docker/ping.txt`.

`test/integration/testdata/docker/services.json`:

```json
[
  {"ID": "s-api", "Spec": {"Name": "cms-heronlabs_cms-api", "Mode": {"Replicated": {"Replicas": 1}},
    "TaskTemplate": {"Resources": {"Limits": {"NanoCPUs": 1000000000, "MemoryBytes": 1073741824}}}}},
  {"ID": "s-db", "Spec": {"Name": "cms-heronlabs_cms-database", "Mode": {"Replicated": {"Replicas": 1}},
    "TaskTemplate": {"Resources": {"Limits": {"NanoCPUs": 500000000, "MemoryBytes": 536870912}}}}},
  {"ID": "s-ep", "Spec": {"Name": "easypanel", "Mode": {"Replicated": {"Replicas": 1}}}}
]
```

`test/integration/testdata/docker/tasks.json`:

```json
[
  {"ServiceID": "s-api", "Status": {"State": "running"}},
  {"ServiceID": "s-api", "Status": {"State": "failed"}},
  {"ServiceID": "s-db", "Status": {"State": "running"}},
  {"ServiceID": "s-ep", "Status": {"State": "running"}}
]
```

`test/integration/testdata/docker/containers.json`:

```json
[
  {"Id": "c-api", "State": "running", "Labels": {"com.docker.swarm.service.name": "cms-heronlabs_cms-api"}, "Mounts": []},
  {"Id": "c-db", "State": "running", "Labels": {"com.docker.swarm.service.name": "cms-heronlabs_cms-database"}, "Mounts": [
    {"Type": "volume", "Name": "cms-heronlabs_cms-database_data", "Source": "/var/lib/docker/volumes/cms-heronlabs_cms-database_data/_data"},
    {"Type": "bind", "Source": "/etc/easypanel/projects/cms-heronlabs/cms-database/backups"}
  ]},
  {"Id": "c-ep", "State": "running", "Labels": {"com.docker.swarm.service.name": "easypanel"}, "Mounts": [
    {"Type": "bind", "Source": "/var/run/docker.sock"}
  ]}
]
```

`test/integration/testdata/docker/df.json`:

```json
{"LayersSize": 6100000000, "BuildCache": [{"Size": 2400000000}],
 "Volumes": [{"Name": "cms-heronlabs_cms-database_data", "UsageData": {"Size": 8099000000}}]}
```

`test/integration/testdata/docker/stats-api.json`:

```json
{"cpu_stats": {"cpu_usage": {"total_usage": 700000}, "system_cpu_usage": 20000000, "online_cpus": 2},
 "precpu_stats": {"cpu_usage": {"total_usage": 300000}, "system_cpu_usage": 10000000},
 "memory_stats": {"usage": 400000000, "stats": {"inactive_file": 32995328}},
 "networks": {"eth0": {"rx_bytes": 120500000, "tx_bytes": 88100000}},
 "blkio_stats": {"io_service_bytes_recursive": [{"op": "read", "value": 50200000}, {"op": "write", "value": 12900000}]}}
```

`test/integration/testdata/docker/stats-db.json`:

```json
{"cpu_stats": {"cpu_usage": {"total_usage": 1600000}, "system_cpu_usage": 20000000, "online_cpus": 2},
 "precpu_stats": {"cpu_usage": {"total_usage": 0}, "system_cpu_usage": 10000000},
 "memory_stats": {"usage": 335544320, "stats": {"inactive_file": 10485760}},
 "networks": {"eth0": {"rx_bytes": 1000000, "tx_bytes": 2000000}},
 "blkio_stats": {"io_service_bytes_recursive": [{"op": "read", "value": 3000000}, {"op": "write", "value": 4000000}]}}
```

`test/integration/testdata/report-once.golden.json` (checked by hand: RAM 7.75 − 2.65 = 5.1 GiB; `cms-api` memory (400000000 − 32995328) / 2²⁰ = 350 MiB and CPU 400000 / 10⁷ × 2 × 100 = 8; `cms-database` CPU 32 and disk 8.099 GB volume + 1 MB bind = 8.1 GB; the `easypanel` service is left out):

```json
{
  "host": "vps-heron-02",
  "measuredAt": "2026-10-01T14:10:00Z",
  "agent": {
    "version": "1.0.0",
    "easypanelVersion": null,
    "easypanelSupported": null,
    "lastReconcile": null
  },
  "system": {
    "ramTotalGiB": 7.75,
    "ramUsedGiB": 5.1,
    "swapTotalGiB": 4,
    "swapUsedGiB": 0.3,
    "diskTotalGB": 97,
    "diskUsedGB": 41.2,
    "load1": 0.42,
    "load5": 0.38,
    "load15": 0.35,
    "uptimeSeconds": 1209600,
    "dockerImagesGB": 6.1,
    "dockerBuildCacheGB": 2.4
  },
  "projects": [
    {
      "name": "cms-heronlabs",
      "reconcile": {
        "state": "unmanaged",
        "missingSecrets": [],
        "error": null
      },
      "services": [
        {
          "name": "cms-api",
          "managed": false,
          "state": "running",
          "replicasRunning": 1,
          "replicasDesired": 1,
          "recentTaskFailures": 1,
          "memoryUsedMiB": 350,
          "memoryLimitMiB": 1024,
          "cpuPercent": 8,
          "cpuLimit": 1,
          "diskGB": 0,
          "netRxMB": 120.5,
          "netTxMB": 88.1,
          "blockReadMB": 50.2,
          "blockWriteMB": 12.9
        },
        {
          "name": "cms-database",
          "managed": false,
          "state": "running",
          "replicasRunning": 1,
          "replicasDesired": 1,
          "recentTaskFailures": 0,
          "memoryUsedMiB": 310,
          "memoryLimitMiB": 512,
          "cpuPercent": 32,
          "cpuLimit": 0.5,
          "diskGB": 8.1,
          "netRxMB": 1,
          "netTxMB": 2,
          "blockReadMB": 3,
          "blockWriteMB": 4
        }
      ]
    }
  ]
}
```

- [ ] **Step 2: Write the failing integration test**

`test/integration/report_test.go`:

```go
//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/domain"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
	"github.com/heronlabs/vps-agent/internal/testkit"
	"github.com/heronlabs/vps-agent/internal/testkit/fake"
)

type fixedDisk struct{}

func (fixedDisk) Stat(string) (domain.Bytes, domain.Bytes, error) { return 97e9, 41.2e9, nil }

// fakeEngine serves testdata/docker/<route>.json on a unix socket and returns the socket path.
func fakeEngine(t *testing.T) string {
	t.Helper()
	files := map[string]string{
		"/_ping":                               "ping.txt",
		"/services":                            "services.json",
		"/tasks":                               "tasks.json",
		"/containers/json?all=1":               "containers.json",
		"/system/df":                           "df.json",
		"/containers/c-api/stats?stream=false": "stats-api.json",
		"/containers/c-db/stats?stream=false":  "stats-db.json",
	}
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socket := filepath.Join(dir, "docker.sock")
	ln := testkit.Must(net.Listen("unix", socket))
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := files[r.URL.RequestURI()]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(testkit.Must(os.ReadFile(filepath.Join("testdata", "docker", name))))
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socket
}

func useCase(t *testing.T, socket string, out io.Writer) *report.UseCase {
	t.Helper()
	proc := fstest.MapFS{
		"meminfo": {Data: testkit.Must(os.ReadFile("testdata/proc/meminfo"))},
		"loadavg": {Data: testkit.Must(os.ReadFile("testdata/proc/loadavg"))},
		"uptime":  {Data: testkit.Must(os.ReadFile("testdata/proc/uptime"))},
	}
	return report.New(report.Deps{
		Host:         "vps-heron-02",
		AgentVersion: "1.0.0",
		Clock:        fake.Clock{T: time.Date(2026, 10, 1, 14, 10, 0, 0, time.UTC)},
		Metrics:      hostfs.NewMetrics(proc, fixedDisk{}),
		Docker:       docker.NewUnix(socket, time.Second, time.Second),
		Dirs:         fake.DirSizer{Sizes: map[string]domain.Bytes{"/etc/easypanel/projects/cms-heronlabs/cms-database/backups": 1_000_000}},
		Sink:         cms.NewPrinter(out),
		Logger:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
	})
}

func TestReportOnce_PrintsTheContractForTheFakeHost(t *testing.T) {
	var out bytes.Buffer
	uc := useCase(t, fakeEngine(t), &out)

	testkit.MustDo(t, uc.Run(context.Background()))

	testkit.Equal(t, string(testkit.Must(os.ReadFile("testdata/report-once.golden.json"))), out.String())
}

func TestReportOnce_FailsWithoutDockerSocket(t *testing.T) {
	dir := testkit.Must(os.MkdirTemp("", "d"))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	missing := filepath.Join(dir, "docker.sock")

	err := useCase(t, missing, io.Discard).Run(context.Background())

	testkit.ErrorIs(t, err, syscall.ENOENT)
}
```

- [ ] **Step 3: Run it**

Run: `make test-integration`
Expected: PASS (the adapters exist). If the golden differs, read the diff before touching the golden: a changed number means a changed formula.

- [ ] **Step 4: Write the app**

`cmd/vps-agent/main.go`:

```go
// Command vps-agent measures an Easypanel VPS and, in later phases, reconciles it from the CMS.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/heronlabs/vps-agent/internal/core/usecase/doctor"
	"github.com/heronlabs/vps-agent/internal/core/usecase/report"
	"github.com/heronlabs/vps-agent/internal/infrastructure/clock"
	"github.com/heronlabs/vps-agent/internal/infrastructure/cms"
	"github.com/heronlabs/vps-agent/internal/infrastructure/config"
	"github.com/heronlabs/vps-agent/internal/infrastructure/docker"
	"github.com/heronlabs/vps-agent/internal/infrastructure/hostfs"
)

var version = "0.0.0-dev"

const (
	defaultConfigPath = "/etc/vps-agent/config.env"
	dockerSocket      = "/var/run/docker.sock"
	dockerTimeout     = 30 * time.Second
	statsTimeout      = 5 * time.Second
	usage             = "usage: vps-agent <version | report --once | doctor> [--config path]"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "report":
		return runReport(ctx, args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}

func configFile(path string) config.File {
	return config.File{Dir: os.DirFS(filepath.Dir(path)), Name: filepath.Base(path)}
}

func runReport(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	once := flags.Bool("once", false, "collect one report, print it and exit")
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*once {
		fmt.Fprintln(stderr, "report: only --once is available in this version")
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	host, err := configFile(*configPath).HostNameOr(os.Hostname)
	if err != nil {
		logger.Error("host name unavailable", slog.String("logId", "report.host-name-failed"), slog.String("errorMessage", err.Error()))
		return 1
	}
	uc := report.New(report.Deps{
		Host:         host,
		AgentVersion: version,
		Clock:        clock.System{},
		Metrics:      hostfs.NewMetrics(os.DirFS("/proc"), hostfs.StatFS{}),
		Docker:       docker.NewUnix(dockerSocket, dockerTimeout, statsTimeout),
		Dirs:         hostfs.DirSizer{},
		Sink:         cms.NewPrinter(stdout),
		Logger:       logger,
	})
	if err := uc.Run(ctx); err != nil {
		logger.Error("report failed", slog.String("logId", "report.failed"), slog.String("errorMessage", err.Error()))
		return 1
	}
	return 0
}

func runDoctor(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", defaultConfigPath, "configuration file")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	results := doctor.Run(ctx,
		doctor.ConfigCheck(configFile(*configPath)),
		doctor.DockerCheck(docker.NewUnix(dockerSocket, dockerTimeout, statsTimeout)),
	)
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(stdout, "FAIL %s: %v\n", r.Check, r.Err)
			continue
		}
		fmt.Fprintf(stdout, "ok   %s\n", r.Check)
	}
	if !doctor.Healthy(results) {
		return 1
	}
	return 0
}
```

- [ ] **Step 5: Build and smoke-test**

```bash
make build VERSION=0.1.0 && ./bin/vps-agent version
GOOS=linux GOARCH=arm64 make build BIN=bin/linux-arm64 && file bin/linux-arm64
make lint && make test && make test-integration
```

Expected: `0.1.0`; `ELF 64-bit LSB executable, ARM aarch64, ... statically linked`; all gates green.

- [ ] **Step 6: Optional end-to-end on a real Swarm (Docker-in-Docker)**

```bash
docker run -d --privileged --name vps-dind -v "$PWD/bin/linux-arm64":/usr/local/bin/vps-agent:ro docker:dind
sleep 10
docker exec vps-dind sh -c 'docker swarm init >/dev/null;
  mkdir -p /etc/easypanel/projects/p/api/data;
  docker service create -q --name p_api --limit-memory 64m --limit-cpu 0.5 \
    --mount type=bind,source=/etc/easypanel/projects/p/api/data,target=/data nginx:alpine >/dev/null;
  sleep 20; vps-agent report --once'
docker rm -f vps-dind
```

Expected: project `p`, service `api`, `state: running`, `replicasRunning: 1`, `memoryLimitMiB: 64`, `cpuLimit: 0.5`. Use `GOARCH=amd64` builds on Intel machines.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: add version, doctor and report --once commands

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Installer, CI/CD, Dependabot and repository docs

**Files:**
- Create: `scripts/install.sh`, `.github/workflows/continuous-deployment.yml`, `.github/dependabot.yml`, `.github/CODEOWNERS`, `.claude/supera.json`, `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, `SECURITY.md`
- Modify: `README.md` (replace the one `gh repo create` made)

**Interfaces:**
- Produces: release assets `vps-agent_<version>_linux_amd64`, `vps-agent_<version>_linux_arm64`, `checksums.txt`, `install.sh` on every release.

- [ ] **Step 1: Write the installer**

`scripts/install.sh` (phase 1: binary only, no systemd unit; it needs `python3`, present on Ubuntu, to read the GitHub API JSON):

```bash
#!/usr/bin/env bash
# Installs or updates vps-agent from the private GitHub release. Run as root.
# Keeps an existing /etc/vps-agent/config.env; writes it from the environment only on first install.
set -euo pipefail

readonly REPO="heronlabs/vps-agent"
readonly CONFIG_DIR="/etc/vps-agent"
readonly CONFIG_FILE="$CONFIG_DIR/config.env"
readonly BIN="/usr/local/bin/vps-agent"
readonly CONFIG_KEYS=(CMS_URL AGENT_TOKEN AGENT_HOST_NAME EASYPANEL_URL EASYPANEL_TOKEN
  AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION RECONCILE_INTERVAL REPORT_INTERVAL)

: "${GITHUB_TOKEN:?GITHUB_TOKEN is required (fine-grained, Contents: read on $REPO)}"
VERSION="${VERSION:-latest}"

fail() { echo "install: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run as root"
command -v python3 >/dev/null || fail "python3 is required"

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported architecture $(uname -m)" ;;
esac

api() {
  curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/$REPO/$1"
}

if [ "$VERSION" = "latest" ]; then
  release="$(api releases/latest)"
else
  release="$(api "releases/tags/v${VERSION#v}")"
fi

field() { python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1" <<<"$release"; }
asset_id() {
  python3 -c 'import json,sys; print(next((a["id"] for a in json.load(sys.stdin)["assets"] if a["name"] == sys.argv[1]), ""))' \
    "$1" <<<"$release"
}
download() {
  local id
  id="$(asset_id "$1")"
  [ -n "$id" ] || fail "asset $1 missing from $tag"
  curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
    "https://api.github.com/repos/$REPO/releases/assets/$id" -o "$2"
}

tag="$(field tag_name)"
version="${tag#v}"
binary="vps-agent_${version}_linux_${arch}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
download "$binary" "$work/$binary"
download checksums.txt "$work/checksums.txt"
(cd "$work" && grep " $binary\$" checksums.txt | sha256sum -c --status -) ||
  fail "checksum mismatch for $binary; nothing installed"

install -m 0755 "$work/$binary" "$BIN"

if [ ! -f "$CONFIG_FILE" ]; then
  install -d -m 0700 "$CONFIG_DIR"
  (
    umask 077
    for key in "${CONFIG_KEYS[@]}"; do
      if [ -n "${!key:-}" ]; then
        printf '%s=%s\n' "$key" "${!key}"
      fi
    done >"$CONFIG_FILE"
  )
fi

echo "install: vps-agent $version at $BIN"
exec "$BIN" doctor
```

```bash
chmod +x scripts/install.sh
```

- [ ] **Step 2: Exercise the installer in Ubuntu with a fake GitHub API**

```bash
S="$(mktemp -d)"; mkdir -p "$S/assets" "$S/bin"
GOOS=linux GOARCH=arm64 make build VERSION=0.1.0 BIN="$S/assets/vps-agent_0.1.0_linux_arm64" >/dev/null
(cd "$S/assets" && shasum -a 256 vps-agent_* > checksums.txt)
echo '{"tag_name":"v0.1.0","assets":[{"id":11,"name":"vps-agent_0.1.0_linux_arm64"},{"id":12,"name":"checksums.txt"}]}' > "$S/release.json"
cat > "$S/bin/curl" <<'EOF'
#!/usr/bin/env bash
out=""; url=""
while [ $# -gt 0 ]; do case "$1" in -o) out="$2"; shift 2;; -H) shift 2;; -*) shift;; *) url="$1"; shift;; esac; done
case "$url" in
  */releases/latest) cat /t/release.json ;;
  */releases/assets/11) cp /t/assets/vps-agent_0.1.0_linux_arm64 "$out" ;;
  */releases/assets/12) cp /t/assets/checksums.txt "$out" ;;
  *) echo "unexpected $url" >&2; exit 22 ;;
esac
EOF
chmod +x "$S/bin/curl"
docker run --rm -v "$S":/t -v "$PWD/scripts/install.sh":/install.sh:ro ubuntu:24.04 bash -c '
  apt-get -qq update >/dev/null && apt-get -qq install -y python3 >/dev/null 2>&1
  export PATH=/t/bin:$PATH
  GITHUB_TOKEN=x AGENT_HOST_NAME=vps-heron-02 bash /install.sh; echo "exit=$?"
  stat -c "%a %n" /etc/vps-agent/config.env; cat /etc/vps-agent/config.env
  GITHUB_TOKEN=x AGENT_HOST_NAME=other bash /install.sh >/dev/null; cat /etc/vps-agent/config.env
  echo "0000  vps-agent_0.1.0_linux_arm64" > /t/assets/checksums.txt
  GITHUB_TOKEN=x bash /install.sh; echo "exit=$?"'
```

Expected: `install: vps-agent 0.1.0 at /usr/local/bin/vps-agent`, `ok   config`, `FAIL docker: ...docker.sock...` and `exit=1` (no socket in the container); `600 /etc/vps-agent/config.env` with `AGENT_HOST_NAME=vps-heron-02` both times (the rerun keeps the config); then `install: checksum mismatch ...; nothing installed` and `exit=1`. Use `amd64` on Intel machines.

- [ ] **Step 3: Write the CD workflow and repository config**

`.github/workflows/continuous-deployment.yml` (release, then a build matrix, then one job that checksums and uploads, so the matrix never races on `checksums.txt`):

```yaml
name: 'Continuous Deployment'

on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      semantic:
        description: The SEMVER specification.
        required: true
        type: choice
        default: patch
        options:
          - 'major'
          - 'minor'
          - 'patch'

permissions:
  contents: write

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
  queue: max

jobs:
  release:
    runs-on: ubuntu-24.04
    outputs:
      tag: ${{ steps.release.outputs.tag }}
      version: ${{ steps.release.outputs.version }}
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: heronlabs/action-tag-release-build@v7
        id: release
        with:
          ghToken: ${{ github.token }}
          semantic: ${{ inputs.semantic }}

  binaries:
    needs: release
    runs-on: ubuntu-24.04
    strategy:
      matrix:
        arch: [amd64, arm64]
    steps:
      - uses: actions/checkout@v7
        with:
          ref: ${{ needs.release.outputs.tag }}

      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod

      - name: Build linux/${{ matrix.arch }}
        env:
          VERSION: ${{ needs.release.outputs.version }}
          ARCH: ${{ matrix.arch }}
        run: make build GOOS=linux GOARCH="$ARCH" VERSION="$VERSION" BIN="dist/vps-agent_${VERSION}_linux_${ARCH}"

      - uses: actions/upload-artifact@v7
        with:
          name: binary-${{ matrix.arch }}
          path: dist/
          retention-days: 1
          if-no-files-found: error

  publish:
    needs: [release, binaries]
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
        with:
          ref: ${{ needs.release.outputs.tag }}

      - uses: actions/download-artifact@v8
        with:
          pattern: binary-*
          path: dist/
          merge-multiple: true

      - name: Upload release assets
        env:
          GH_TOKEN: ${{ github.token }}
          TAG: ${{ needs.release.outputs.tag }}
        run: |
          cp scripts/install.sh dist/
          (cd dist && sha256sum vps-agent_* > checksums.txt)
          gh release upload "$TAG" dist/* --clobber
```

`.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: '/'
    schedule:
      interval: daily
    groups:
      actions:
        patterns: ['*']
  - package-ecosystem: gomod
    directory: '/'
    schedule:
      interval: daily
```

`.github/CODEOWNERS`:

```text
# CODEOWNERS
# Docs: https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners

* @heronlabs/devops
```

- [ ] **Step 4: Write the agent and contributor docs**

`.claude/supera.json`:

```json
{
  "baseBranch": "main",
  "buildCommand": "make build",
  "lintCommand": "make lint",
  "mergeMethod": "squash",
  "remote": "origin",
  "testCommands": {
    "integration": "make test-integration",
    "unit": "make test"
  }
}
```

`CLAUDE.md`:

```markdown
@AGENTS.md
```

`AGENTS.md`:

```markdown
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
```

`CONTRIBUTING.md`:

````markdown
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
| `make build` | Build `bin/vps-agent` |
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
````

`SECURITY.md`:

```markdown
# Security Policy

## Reporting a Vulnerability

Report vulnerabilities to **security@heronlabs.com**. Do not open public issues.

Expect a response within 72 hours. After triage we will keep you updated on progress and coordinate disclosure timing.

## Scope

`vps-agent` runs as root on Easypanel hosts. It handles:

- the Docker socket, which is equivalent to root on the host;
- `/etc/vps-agent/config.env` (mode 0600) with the CMS, Easypanel and AWS credentials of the host;
- the release download token used by `scripts/install.sh`.

Key concerns: credential exposure in logs or reports, integrity of the released binary (`checksums.txt`), and anything that would let a CMS manifest make the agent read or write beyond its host.

### Supported versions

| Version | Supported |
|---------|-----------|
| latest minor | ✅ |
| older | ❌ |

## Disclosure

We follow coordinated disclosure. After a fix is released we will publish a GitHub Security Advisory and credit reporters (unless you prefer anonymity).
```

`README.md`:

````markdown
# vps-agent

Agent for Heron Labs Easypanel hosts. It runs on the host as a static binary and only makes outbound connections.

Today (phase 1) it measures the host (memory, swap, disk, load, uptime) and every Easypanel service (state, replicas, memory, CPU, disk, network, block I/O) and prints the report the CMS will receive. Sending the report and reconciling Easypanel from the CMS come in the next phases; see `docs/superpowers/specs/2026-10-01-vps-agent-design.md`.

## Install or update on a VPS

```bash
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
  "<install.sh asset URL of the release>" -o install.sh
sudo GITHUB_TOKEN=... AGENT_HOST_NAME=<infra-host name> bash install.sh
```

`GITHUB_TOKEN` is a fine-grained token with `Contents: read` on this repository. The script verifies the sha256, installs `/usr/local/bin/vps-agent`, writes `/etc/vps-agent/config.env` (0600) only on the first install and runs `vps-agent doctor`.

## Use

```bash
sudo vps-agent version
sudo vps-agent doctor          # config and Docker socket
sudo vps-agent report --once   # print one report as JSON
```

## Develop

See `CONTRIBUTING.md` and `AGENTS.md`.
````

- [ ] **Step 5: Run every gate**

Run: `make lint && make test && make test-integration && make audit && make test-mutation`
Expected: lint clean; `coverage: 100.0%`; integration ok; `No vulnerabilities found.`; gremlins `Test efficacy: 100.00%`. On macOS gremlins lists `statfs_linux.go` as NOT COVERED — expected; Linux CI covers it. To check Linux locally: `docker run --rm -v "$PWD":/src -w /src golang:1.27 make test-mutation`.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "ci: add installer, CD workflow, dependabot and repository docs

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: Pull request and first release

- [ ] **Step 1: Push and open the PR**

```bash
git push -u origin feat/scaffold
gh pr create --base main --title "feat: scaffold vps-agent with report --once and doctor" --body "$(cat <<'EOF'
## Summary
- Go module with domain, ports, use cases and adapters for phase 1 of `docs/superpowers/specs/2026-10-01-vps-agent-design.md`
- `vps-agent version`, `doctor` (config + Docker socket), `report --once` (host + Swarm services in the CMS contract)
- Makefile, depguard layering, testshape lint (one scenario, one assertion), 100% coverage and mutation gates, pre-commit hook
- CI (build, lint, audit, unit, integration, mutation) and CD (release, linux amd64/arm64 binaries, checksums, install.sh)

## Test plan
- [ ] CI green
- [ ] After merge: release assets present and `install.sh` + `vps-agent report --once` on the current VPS

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 2: Wait for CI; fix anything red on the same branch**

Run: `gh pr checks --watch`
Expected: all six checks pass. A red job is fixed with a new commit on `feat/scaffold`, never by weakening a gate.

- [ ] **Step 3: Hand the merge to the user**

The user merges (squash). Do not merge.

- [ ] **Step 4: Confirm the release**

```bash
gh run watch "$(gh run list --workflow continuous-deployment.yml --limit 1 --json databaseId --jq '.[0].databaseId')"
gh release view --json tagName,assets --jq '.tagName, (.assets[].name)'
```

Expected: tag `v0.1.0` (or what the release action infers from `feat:`), assets `vps-agent_<v>_linux_amd64`, `vps-agent_<v>_linux_arm64`, `checksums.txt`, `install.sh`.

---

### Task 13: Validate on the current VPS

Needs the user: SSH access to the current Easypanel host and a fine-grained GitHub token with `Contents: read` on `heronlabs/vps-agent`.

- [ ] **Step 1: Get the installer URL (on the workstation, where `gh` is)**

Run: `gh release view --repo heronlabs/vps-agent --json assets --jq '.assets[] | select(.name=="install.sh") | .apiUrl'`
Expected: `https://api.github.com/repos/heronlabs/vps-agent/releases/assets/<id>`.

- [ ] **Step 2: Install (on the VPS, over SSH; `gh` is not there)**

```bash
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -H "Accept: application/octet-stream" \
  "<URL from Step 1>" -o install.sh
sudo GITHUB_TOKEN="$GITHUB_TOKEN" AGENT_HOST_NAME=srv655164 bash install.sh
```

Expected: `ok   config` and `ok   docker`, exit 0.

- [ ] **Step 3: Measure and compare**

Run: `time sudo vps-agent report --once`
Expected: every Easypanel project and service appears with its own name (confirms the `<project>_<service>` label rule of the spec's _Fase 0_), memory and CPU close to what the Easypanel UI and `docker stats --no-stream` show, and `diskGB` of the database service close to `du -sh` of its volume. Record any mismatch in the spec before phase 2.

Watch the run time: `/system/df` makes dockerd walk every volume, and the whole Engine call is bounded by `dockerTimeout` (30 s in `cmd/vps-agent/main.go`). If the report fails with `context deadline exceeded` on `/system/df`, raise `dockerTimeout` in a follow-up `/supera:ship` (or turn it into a `config.env` key) instead of retrying by hand; note the observed duration in the spec.
