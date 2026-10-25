# Development Guide

This guide covers local development for Prompt Gate Backend.

## Local Stack With Docker Compose

Run the full local stack:

```sh
docker compose up --build
```

Compose starts:

- PostgreSQL on `localhost:5432`
- Redis on `localhost:6379`
- Keycloak on `http://keycloak.localhost:8082`
- database migrations
- a seed job for a local Ollama provider
- the API on `http://localhost:8080`
- the proxy on `http://localhost:8081`
- the worker
- the scheduler

Local credentials:

- Keycloak admin: `admin` / `admin`
- Prompt Gate test user: `admin` / `admin`

The seeded provider is named `ollama` and points at
`http://host.docker.internal:11434/v1`.

Compose mounts `deploy/config/promptgate.compose.yaml` read-only into every
Prompt Gate process. That committed file contains development-only credentials;
edit or replace it for local experiments, then restart the affected services.

### Local Login Troubleshooting

The first browser user synced into an empty Prompt Gate database becomes
`admin`. Human users are matched by the exact OIDC `preferred_username` claim.
If Keycloak is reset while the PostgreSQL volume is kept, signing in with the
same username refreshes the stored OIDC subject while preserving the local
account and its application role.

When `admin` signs in but sees **Access denied** in local development:

- verify that Keycloak emits the expected `preferred_username` claim, then
- promote the matching account from the User management page or directly in
  PostgreSQL for local recovery.

## Source-Based Setup

Create a local runtime configuration:

```sh
cp promptgate.example.yaml promptgate.yaml
```

Fill the required database, Redis, OIDC, URL, and secret values. The Makefile
uses `CONFIG_FILE=promptgate.yaml` by default; set `CONFIG_FILE=/other/path.yaml`
to select another file.

Install dependencies and run checks:

```sh
make deps
make fmt-check
make vet
make staticcheck
make test
make build
```

Integration-test service endpoints are read from
[`testdata/promptgate.integration.yaml`](../testdata/promptgate.integration.yaml).
The PostgreSQL- and Redis-specific tests run when those disposable services are
available and otherwise skip locally. CI starts matching services for the
endpoints in this file, so the integration coverage always runs there.

Run the backend processes:

```sh
make migrate
make run-api
make run-proxy
make run-worker
make run-schedule
```

For convenience, run migrations, worker, scheduler, API, and proxy from one terminal:

```sh
make run-all
```

## Configuration Loading

Every runtime requires an explicit YAML path:

```sh
go run . --config promptgate.yaml api
go run . --config promptgate.yaml proxy
go run . --config promptgate.yaml worker
go run . --config promptgate.yaml migrate
go run . --config promptgate.yaml schedule
```

The loader does not inspect `.env`, expand environment placeholders, or accept
environment overrides. It rejects unknown YAML keys and invalid types.

## Common Make Targets

| Target | Purpose |
| --- | --- |
| `make deps` | Download Go modules. |
| `make fmt` | Format Go files. |
| `make fmt-check` | Check Go formatting without rewriting files. |
| `make vet` | Run `go vet`. |
| `make staticcheck` | Run the pinned Staticcheck version. |
| `make test` | Run backend tests. |
| `make build` | Build `bin/promptgate`. |
| `make migrate` | Run database migrations. |
| `make run-api` | Run the API process. |
| `make run-proxy` | Run the proxy process. |
| `make run-worker` | Run the generic worker process. |
| `make run-schedule` | Run the scheduler process. |
| `make run-all` | Run migrations, worker, scheduler, API, and proxy together. |
| `make clean` | Remove local binaries. |

## Frontend Notes

The production Dockerfile builds the Nuxt frontend and copies generated static
assets into `/app/public`. Configure that path in the mounted YAML:

```yaml
api:
  static_assets_dir: /app/public
```

When this path is set, the API serves frontend files directly.

## OpenAPI Contract

`internal/transport/httpapi/openapi/openapi.yaml` is the source document loaded
by the protected administrator explorer at `/admin/documentation`. When a
runtime route under `/api/v1/**`, its request, or its response changes, update
this document in the same change.

The HTTP API tests validate the document, representative schemas, and
bidirectional parity between runtime operations and OpenAPI operations. Run the
targeted checks with:

```sh
go test ./internal/transport/httpapi/...
```

`make test` includes the same checks in the full backend suite.

## Migrations

Local migrations use:

```sh
make migrate
```

The migration command applies all GORM migrations in dependency order. Run it
after changing persistent models and before starting API or proxy processes
against a fresh database.

## Test And Validation Checklist

Before opening a pull request:

```sh
make fmt-check
make vet
make staticcheck
make test
docker compose config --quiet
docker build -t prompt-gate-backend:test .
```

If frontend code changed, also run from `frontend/`:

```sh
npm ci
npm run lint:check
npm run typecheck
npm test -- --run
```
