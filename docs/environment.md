# Environment Variables

All application configuration is read from environment variables prefixed with
`PROMPTGATE_`. Durations use Go duration syntax such as `250ms`, `5m`, `1h`, or
`8h`.

The CLI also loads the nearest `.env` file automatically before running any
Prompt Gate command. Use `--env-file /path/to/file.env` to load a specific
dotenv file.

## Required Variables

| Variable | Used by | Default | Required | Description |
| --- | --- | --- | --- | --- |
| `PROMPTGATE_DATABASE_URL` | API, proxy, worker, schedule, migrate | none | Yes | PostgreSQL connection string. Example: `postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable`. |
| `PROMPTGATE_REDIS_URL` | API, proxy, worker, schedule | none | Yes | Redis connection URL. Example: `redis://localhost:6379/0`. |
| `PROMPTGATE_JWT_SECRET` | API, proxy, schedule | none | Yes | Secret used for Prompt Gate API token signing and validation. Must be at least 32 characters. |
| `PROMPTGATE_SECRETS_KEY` | API, proxy, schedule | none | Yes | Base64-encoded 32-byte key used for stored provider and MCP secrets. Keep stable across restarts and deployments. |
| `PROMPTGATE_KEYCLOAK_ISSUER_URL` | API | none | API only | OIDC issuer URL. Example: `https://keycloak.example.com/realms/promptgate`. |
| `PROMPTGATE_KEYCLOAK_JWKS_URL` | API | none | API only | JWKS URL used to validate OIDC access tokens. |
| `PROMPTGATE_KEYCLOAK_CLIENT_ID` | API | none | API only | OIDC client ID used for browser login and ID token verification. |
| `PROMPTGATE_FRONTEND_BASE_URL` | API, proxy | none | API only, optional for proxy | Public frontend origin. Proxy uses it as the default CORS origin when explicit origins are not set. |
| `PROMPTGATE_BACKEND_BASE_URL` | API | none | API only | Public API origin. Used to build `/auth/callback` and decide secure-cookie behavior. |

## Optional Settings

| Variable | Used by | Default | Description |
| --- | --- | --- | --- |
| `PROMPTGATE_PORT` | API | `8080` | API server listen port. Values may be `8080` or `:8080`. |
| `PROMPTGATE_PROXY_PORT` | API, proxy | `8081` | Proxy server listen port. The API also uses it to derive `PROMPTGATE_PROXY_BASE_URL` when no explicit proxy base URL is set. |
| `PROMPTGATE_LOG_LEVEL` | API, proxy, worker, schedule, migrate | `info` | Log level. Supported values are `debug`, `info`, `warn`, `warning`, and `error`; unknown values fall back to `info`. |
| `PROMPTGATE_KEYCLOAK_CLIENT_SECRET` | API | empty | Optional OIDC client secret. Set it when the OIDC client is confidential. |
| `PROMPTGATE_ADMIN_API_KEY` | API | empty (disabled) | Optional global credential accepted in `X-Admin-API-Key` for `/api/v1/admin/**` routes. Intended only for trusted CLI and server-to-server administration. Empty or whitespace-only values disable it; any other trimmed value is accepted. |
| `PROMPTGATE_CA_FILE` | API, schedule | empty | Optional path to a PEM-encoded CA certificate file. API uses it for Keycloak HTTPS endpoints and monitoring checks; schedule uses it for monitoring checks. |
| `PROMPTGATE_PROXY_BASE_URL` | API | derived from `PROMPTGATE_BACKEND_BASE_URL` and `PROMPTGATE_PROXY_PORT` | Public proxy origin shown to clients. Set it explicitly when the proxy is served from a different host, path, or externally mapped port. |
| `PROMPTGATE_STATIC_ASSETS_DIR` | API | empty | Optional directory containing frontend static assets. When set, the API serves files from this directory and falls back to the SPA shell for frontend routes. |
| `PROMPTGATE_SESSION_COOKIE_NAME` | API, proxy | `promptgate_session` | Browser session cookie name. |
| `PROMPTGATE_SESSION_TTL` | API, proxy | `8h` | Browser session lifetime. Must be greater than zero. |
| `PROMPTGATE_CORS_ALLOWED_ORIGINS` | API, proxy | API: `PROMPTGATE_FRONTEND_BASE_URL`; proxy: `PROMPTGATE_FRONTEND_BASE_URL` when present | Allowed browser origins for CORS. Loopback origins are expanded across `localhost`, `127.0.0.1`, and `::1` when possible. |
| `PROMPTGATE_TOKEN_CLEANUP_INTERVAL` | API, schedule | `1h` | Interval for expired token cleanup. |
| `PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL` | API, schedule | `1h` | Interval for user access expiration jobs. |
| `PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS` | proxy | `false` | Whether the proxy trusts `X-Forwarded-For` and `X-Real-IP`. Enable only behind trusted infrastructure. |
| `PROMPTGATE_PROXY_TRUSTED_PROXIES` | proxy | empty | Comma-separated CIDRs for proxies whose `X-Forwarded-For` and `X-Real-IP` headers can be trusted. Prefer this over global forwarded-header trust in production. |
| `PROMPTGATE_REDIS_CACHE_TTL` | API, proxy, worker, schedule | `5m` | TTL for Redis-backed cache entries and snapshots. |
| `PROMPTGATE_PROXY_RELOAD_DEBOUNCE` | API, proxy, schedule | `250ms` | Debounce duration for proxy provider and MCP reload notifications. |
| `PROMPTGATE_WORKER_BATCH_SIZE` | worker | `100` | Maximum Redis Stream events read per worker batch. |
| `PROMPTGATE_WORKER_BLOCK_TIMEOUT` | worker | `5s` | Long-poll timeout for new Redis Stream events. |
| `PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT` | worker | `30s` | Minimum idle time before a worker can reclaim pending usage events. |
| `PROMPTGATE_WORKER_CONSUMER_NAME` | worker | generated | Optional Redis consumer name. Leave empty for `hostname-pid-random`. |
| `PROMPTGATE_USAGE_RAW_RETENTION` | schedule | `2160h` | Retention for raw proxy usage rows used by prompt exploration. |
| `PROMPTGATE_USAGE_RAW_CLEANUP_INTERVAL` | schedule | `1h` | Interval for raw proxy usage cleanup. |
| `PROMPTGATE_SUBSCRIPTION_QUOTA_SYNC_INTERVAL` | schedule | `5m` | Interval for copying live Redis subscription quota state into PostgreSQL. |
| `PROMPTGATE_USAGE_COST_ENABLED` | API | `true` | Enables dashboard usage cost estimates. Set to `false` to omit cost fields from web API responses. |
| `PROMPTGATE_USAGE_COST_INPUT` | API | `5.00` | Estimated USD price per 1M completion input tokens. |
| `PROMPTGATE_USAGE_COST_OUTPUT` | API | `30.00` | Estimated USD price per 1M completion output tokens. |
| `PROMPTGATE_USAGE_COST_EMBEDDING` | API | `0.02` | Estimated USD price per 1M embedding tokens. |

## Per Command Requirements

`promptgate api` requires:

```sh
PROMPTGATE_DATABASE_URL
PROMPTGATE_REDIS_URL
PROMPTGATE_JWT_SECRET
PROMPTGATE_SECRETS_KEY
PROMPTGATE_KEYCLOAK_ISSUER_URL
PROMPTGATE_KEYCLOAK_JWKS_URL
PROMPTGATE_KEYCLOAK_CLIENT_ID
PROMPTGATE_FRONTEND_BASE_URL
PROMPTGATE_BACKEND_BASE_URL
```

`promptgate proxy` requires:

```sh
PROMPTGATE_DATABASE_URL
PROMPTGATE_REDIS_URL
PROMPTGATE_JWT_SECRET
PROMPTGATE_SECRETS_KEY
```

`promptgate schedule` requires:

```sh
PROMPTGATE_DATABASE_URL
PROMPTGATE_REDIS_URL
PROMPTGATE_JWT_SECRET
PROMPTGATE_SECRETS_KEY
```

`promptgate worker` requires:

```sh
PROMPTGATE_DATABASE_URL
PROMPTGATE_REDIS_URL
```

`promptgate migrate` requires:

```sh
PROMPTGATE_DATABASE_URL
```

## Local Example

```sh
PROMPTGATE_PORT=8080
PROMPTGATE_PROXY_PORT=8081
PROMPTGATE_LOG_LEVEL=debug
PROMPTGATE_DATABASE_URL=postgres://postgres:postgres@localhost:5432/promptgate?sslmode=disable
PROMPTGATE_REDIS_URL=redis://localhost:6379/0
PROMPTGATE_KEYCLOAK_ISSUER_URL=http://localhost:8082/realms/promptgate
PROMPTGATE_KEYCLOAK_JWKS_URL=http://localhost:8082/realms/promptgate/protocol/openid-connect/certs
PROMPTGATE_KEYCLOAK_CLIENT_ID=promptgate-backend
PROMPTGATE_KEYCLOAK_CLIENT_SECRET=
PROMPTGATE_ADMIN_API_KEY=
PROMPTGATE_CA_FILE=
PROMPTGATE_FRONTEND_BASE_URL=http://localhost:3000
PROMPTGATE_BACKEND_BASE_URL=http://localhost:8080
PROMPTGATE_PROXY_BASE_URL=http://localhost:8081
PROMPTGATE_STATIC_ASSETS_DIR=
PROMPTGATE_SESSION_COOKIE_NAME=promptgate_session
PROMPTGATE_SESSION_TTL=8h
PROMPTGATE_CORS_ALLOWED_ORIGINS=http://localhost:3000
PROMPTGATE_JWT_SECRET=change-me-change-me-change-me-32
PROMPTGATE_SECRETS_KEY=MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=
PROMPTGATE_TOKEN_CLEANUP_INTERVAL=1h
PROMPTGATE_USER_ACCESS_EXPIRATION_INTERVAL=1h
PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS=false
PROMPTGATE_PROXY_TRUSTED_PROXIES=
PROMPTGATE_REDIS_CACHE_TTL=5m
PROMPTGATE_PROXY_RELOAD_DEBOUNCE=250ms
PROMPTGATE_WORKER_BATCH_SIZE=100
PROMPTGATE_WORKER_BLOCK_TIMEOUT=5s
PROMPTGATE_WORKER_PENDING_IDLE_TIMEOUT=30s
PROMPTGATE_WORKER_CONSUMER_NAME=
PROMPTGATE_USAGE_RAW_RETENTION=2160h
PROMPTGATE_USAGE_RAW_CLEANUP_INTERVAL=1h
PROMPTGATE_SUBSCRIPTION_QUOTA_SYNC_INTERVAL=5m
PROMPTGATE_USAGE_COST_ENABLED=true
PROMPTGATE_USAGE_COST_INPUT=5.00
PROMPTGATE_USAGE_COST_OUTPUT=30.00
PROMPTGATE_USAGE_COST_EMBEDDING=0.02
```

## Notes

- `PROMPTGATE_BACKEND_BASE_URL` must be the browser-visible API URL because the
  OIDC callback URL is computed as `<backend-base-url>/auth/callback`.
- `PROMPTGATE_PROXY_BASE_URL` is optional for the API. When omitted, the API
  derives it from `PROMPTGATE_BACKEND_BASE_URL` and `PROMPTGATE_PROXY_PORT`.
- `PROMPTGATE_CORS_ALLOWED_ORIGINS` should contain origins only, without paths
  or trailing slashes.
- `PROMPTGATE_CA_FILE` must point to a readable PEM file. Mount the file into
  API and schedule containers when Keycloak or monitored HTTPS services use a
  private or internal CA.
- `PROMPTGATE_ADMIN_API_KEY` is read only by the API process. When enabled, it
  grants full access to every admin endpoint, including destructive actions.
  Keep it out of frontend code, browser storage, URLs, logs, and version
  control, and send it only over HTTPS.
- `PROMPTGATE_SECRETS_KEY` protects stored downstream provider credentials and
  sensitive MCP headers. Rotating it requires a deliberate secret migration
  plan.
- `PROMPTGATE_JWT_SECRET` must be shared by API, proxy, and scheduler
  processes so issued tokens can be validated and revoked consistently.
- Dashboard usage cost fields are indicative estimates only. They are
  calculated from recorded token counts and the configured rates, not from an
  OpenAI invoice.

Related docs:

- [Development guide](development.md)
- [Deployment](deployment.md)
- [Security model](security.md)
- [Proxy runtime](proxy.md)
- [Scheduler](scheduler.md)
