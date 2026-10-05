VERSION ?= 0.0.0-dev
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BIN ?= bin/talon
COMPOSE = docker compose -f .docker/compose.yaml
UNIT_COVERPKG = $(shell go list ./src/... | grep -v '/src$$' | paste -sd, -)
UNIT_COVERAGE = .coverage/unit
INTEGRATION_COVERAGE = .coverage/integration
INTEGRATION_BIN = bin/integration/talon
MUTATION_REPORT = .coverage/mutation

.PHONY: build package up down watch lint lint\:fix audit test test\:integration test\:mutation

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./src

package:
	$(MAKE) build GOOS=linux BIN=dist/talon-linux-$(GOARCH)
	GOOS= GOARCH= ARCH=$(GOARCH) VERSION=$(VERSION) go tool nfpm package --packager deb --target dist/

up:
	mkdir -p bin/linux
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

watch:
	go tool air

lint:
	golangci-lint run ./...
	go tool actionlint
	shellcheck .githooks/pre-commit .docker/fake_vps/easypanel_setup.sh
	go mod tidy -diff

lint\:fix:
	golangci-lint run --fix ./...

audit:
	go tool govulncheck ./...

test:
	rm -rf $(UNIT_COVERAGE) && mkdir -p $(UNIT_COVERAGE)
	go test -race -coverpkg=$(UNIT_COVERPKG) -coverprofile=$(UNIT_COVERAGE)/coverage.out ./src/...
	go tool cover -func=$(UNIT_COVERAGE)/coverage.out
	go tool cover -html=$(UNIT_COVERAGE)/coverage.out -o $(UNIT_COVERAGE)/coverage.html

test\:integration:
	rm -rf $(INTEGRATION_COVERAGE) && mkdir -p $(INTEGRATION_COVERAGE)/covdata
	go build -cover -covermode=atomic -coverpkg=./src/... -o $(INTEGRATION_BIN) ./src
	TALON_BIN=$(CURDIR)/$(INTEGRATION_BIN) GOCOVERDIR=$(CURDIR)/$(INTEGRATION_COVERAGE)/covdata \
		go test -race -count=1 -tags integration ./tests/...
	go tool covdata textfmt -i=$(INTEGRATION_COVERAGE)/covdata -o $(INTEGRATION_COVERAGE)/coverage.out
	go tool cover -func=$(INTEGRATION_COVERAGE)/coverage.out
	go tool cover -html=$(INTEGRATION_COVERAGE)/coverage.out -o $(INTEGRATION_COVERAGE)/coverage.html

test\:mutation:
	rm -rf $(MUTATION_REPORT) && mkdir -p $(MUTATION_REPORT)
	go tool gremlins unleash --output $(MUTATION_REPORT)/mutation.json -E '^main\.go$$' ./src
