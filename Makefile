SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
CONFIG_FILE ?= promptgate.yaml
BIN_DIR ?= bin
COVERAGE_MIN ?= 70
GO_PACKAGE_DIRS ?= $(shell find . -path './frontend' -prune -o -name '*.go' -print | xargs -n1 dirname | sort -u)
GO_PACKAGES ?= $(shell $(GO) list $(GO_PACKAGE_DIRS))
GO_FILES ?= $(shell find . -path './frontend' -prune -o -name '*.go' -print)

.PHONY: help all deps fmt fmt-check vet staticcheck test coverage build clean migrate run-api run-proxy run-worker run-schedule run-all

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-12s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: run-all ## Run all backend products.

deps: ## Download Go modules.
	$(GO) mod download

fmt: ## Format Go files.
	$(GO) fmt $(GO_PACKAGES)

fmt-check: ## Check Go file formatting.
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$${unformatted}" ]; then \
		echo "$${unformatted}"; \
		exit 1; \
	fi

vet: ## Run go vet.
	$(GO) vet $(GO_PACKAGES)

staticcheck: ## Run Staticcheck.
	$(GO) tool staticcheck $(GO_PACKAGES)

test: ## Run tests.
	$(GO) test $(GO_PACKAGES)

coverage: ## Run tests and enforce the minimum aggregate statement coverage.
	@GO="$(GO)" GO_PACKAGES="$(GO_PACKAGES)" COVERAGE_MIN="$(COVERAGE_MIN)" bash scripts/check-go-coverage.sh

build: ## Build the backend binary.
	mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(BIN_DIR)/promptgate" .

clean: ## Remove built binaries.
	rm -rf "$(BIN_DIR)"

migrate: ## Run database migrations.
	$(GO) run . --config "$(CONFIG_FILE)" migrate

run-api: ## Run API server.
	$(GO) run . --config "$(CONFIG_FILE)" api

run-proxy: ## Run proxy server.
	$(GO) run . --config "$(CONFIG_FILE)" proxy

run-worker: ## Run generic background worker.
	$(GO) run . --config "$(CONFIG_FILE)" worker

run-schedule: ## Run scheduled background jobs.
	$(GO) run . --config "$(CONFIG_FILE)" schedule

run-all: ## Run migrations, worker, scheduler, API server, and proxy server.
	$(GO) run . --config "$(CONFIG_FILE)" migrate; \
	$(GO) run . --config "$(CONFIG_FILE)" worker & \
	worker_pid=$$!; \
	$(GO) run . --config "$(CONFIG_FILE)" schedule & \
	schedule_pid=$$!; \
	$(GO) run . --config "$(CONFIG_FILE)" api & \
	api_pid=$$!; \
	$(GO) run . --config "$(CONFIG_FILE)" proxy & \
	proxy_pid=$$!; \
	trap 'kill $$worker_pid $$schedule_pid $$api_pid $$proxy_pid 2>/dev/null || true; wait $$worker_pid $$schedule_pid $$api_pid $$proxy_pid 2>/dev/null || true' INT TERM EXIT; \
	wait $$worker_pid $$schedule_pid $$api_pid $$proxy_pid
