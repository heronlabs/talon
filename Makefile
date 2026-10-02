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
