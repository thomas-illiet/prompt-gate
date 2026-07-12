SHELL := /bin/bash
.DEFAULT_GOAL := help

GO ?= go
ENV_FILE ?= .env
BIN_DIR ?= bin
GO_PACKAGE_DIRS ?= $(shell find . -path './frontend' -prune -o -name '*.go' -print | xargs -n1 dirname | sort -u)
GO_PACKAGES ?= $(shell $(GO) list $(GO_PACKAGE_DIRS))
GO_FILES ?= $(shell find . -path './frontend' -prune -o -name '*.go' -print)

define LOAD_ENV
set -a; \
if [ -f "$(ENV_FILE)" ]; then source "$(ENV_FILE)"; fi; \
set +a; \
export PROMPTGATE_PORT="$${PROMPTGATE_PORT:-8080}"; \
export PROMPTGATE_LOG_LEVEL="$${PROMPTGATE_LOG_LEVEL:-info}"; \
export PROMPTGATE_BACKEND_BASE_URL="$${PROMPTGATE_BACKEND_BASE_URL:-http://localhost:8080}"; \
export PROMPTGATE_FRONTEND_BASE_URL="$${PROMPTGATE_FRONTEND_BASE_URL:-http://localhost:3000}"; \
export PROMPTGATE_SESSION_COOKIE_NAME="$${PROMPTGATE_SESSION_COOKIE_NAME:-promptgate_session}"; \
export PROMPTGATE_CORS_ALLOWED_ORIGINS="$${PROMPTGATE_CORS_ALLOWED_ORIGINS:-http://localhost:3000}"; \
export PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL="$${PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL:-1h}"; \
export PROMPTGATE_PROXY_PORT="$${PROMPTGATE_PROXY_PORT:-8081}"; \
export PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS="$${PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS:-false}"; \
export PROMPTGATE_REDIS_CACHE_TTL="$${PROMPTGATE_REDIS_CACHE_TTL:-5m}"; \
export PROMPTGATE_PROXY_RELOAD_DEBOUNCE="$${PROMPTGATE_PROXY_RELOAD_DEBOUNCE:-250ms}"; \
export PROMPTGATE_WORKER_BATCH_SIZE="$${PROMPTGATE_WORKER_BATCH_SIZE:-100}"; \
export PROMPTGATE_WORKER_BLOCK_TIMEOUT="$${PROMPTGATE_WORKER_BLOCK_TIMEOUT:-5s}"; \
export PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT="$${PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT:-30s}"; \
export PROMPTGATE_USAGE_RAW_RETENTION="$${PROMPTGATE_USAGE_RAW_RETENTION:-2160h}"; \
export PROMPTGATE_USAGE_RAW_CLEANUP_INTERVAL="$${PROMPTGATE_USAGE_RAW_CLEANUP_INTERVAL:-1h}"
endef

.PHONY: help all deps fmt fmt-check vet staticcheck test build clean migrate run-api run-proxy run-worker run-schedule run-all

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

build: ## Build the backend binary.
	mkdir -p "$(BIN_DIR)"
	$(GO) build -o "$(BIN_DIR)/promptgate" .

clean: ## Remove built binaries.
	rm -rf "$(BIN_DIR)"

migrate: ## Run database migrations.
	@$(LOAD_ENV); \
	$(GO) run . migrate

run-api: ## Run API server.
	@$(LOAD_ENV); \
	$(GO) run . api

run-proxy: ## Run proxy server.
	@$(LOAD_ENV); \
	$(GO) run . proxy

run-worker: ## Run generic background worker.
	@$(LOAD_ENV); \
	$(GO) run . worker

run-schedule: ## Run scheduled background jobs.
	@$(LOAD_ENV); \
	$(GO) run . schedule

run-all: ## Run migrations, worker, scheduler, API server, and proxy server.
	@$(LOAD_ENV); \
	$(GO) run . migrate; \
	$(GO) run . worker & \
	worker_pid=$$!; \
	$(GO) run . schedule & \
	schedule_pid=$$!; \
	$(GO) run . api & \
	api_pid=$$!; \
	$(GO) run . proxy & \
	proxy_pid=$$!; \
	trap 'kill $$worker_pid $$schedule_pid $$api_pid $$proxy_pid 2>/dev/null || true; wait $$worker_pid $$schedule_pid $$api_pid $$proxy_pid 2>/dev/null || true' INT TERM EXIT; \
	wait $$worker_pid $$schedule_pid $$api_pid $$proxy_pid
