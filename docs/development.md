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

### Local Login Troubleshooting

The first browser user synced into an empty Prompt Gate database becomes
`admin`. If Keycloak is reset while the PostgreSQL volume is kept, the seeded
`admin` user can receive a new OIDC subject and appear as a separate account
with no application role.

When `admin` signs in but sees **Access denied** in local development, either:

- reset both persisted volumes with `docker compose down -v` and start again, or
- promote the current `admin@promptgate.local` account from the User management
  page or directly in PostgreSQL for local recovery.

## Source-Based Setup

Create a local dotenv file:

```sh
cp .env.example .env
```

Install dependencies and run checks:

```sh
make deps
make fmt-check
make vet
make test
make build
```

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

## Environment Loading

The Cobra CLI loads the nearest `.env` file automatically before running a
command. This applies to:

```sh
go run . api
go run . proxy
go run . worker
go run . migrate
go run . schedule
```

Use `--env-file /path/to/file.env` to load a specific dotenv file.

The Makefile also loads `.env` for run targets and fills common local defaults
for ports, base URLs, CORS, session name, and interval values.

## Common Make Targets

| Target | Purpose |
| --- | --- |
| `make deps` | Download Go modules. |
| `make fmt` | Format Go files. |
| `make fmt-check` | Check Go formatting without rewriting files. |
| `make vet` | Run `go vet`. |
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
assets into `/app/public`. The runtime image sets:

```sh
PROMPTGATE_STATIC_ASSETS_DIR=/app/public
```

When this variable is set, the API serves frontend files directly.

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
