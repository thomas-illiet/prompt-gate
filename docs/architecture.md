# Architecture

Prompt Gate Backend is a Go service layer split into four runtime commands from
one binary. Each command has a narrow operational responsibility and shares the
same domain packages.

## Runtime Components

```mermaid
flowchart LR
    Browser["Browser session"] --> API["promptgate api"]
    Admin["Admin UI"] --> API
    Client["LLM client"] --> Proxy["promptgate proxy"]
    Schedule["promptgate schedule"] --> Jobs["Cleanup jobs"]
    Migrate["promptgate migrate"] --> DB["PostgreSQL"]

    API --> DB
    Proxy --> DB
    Jobs --> DB

    API --> Redis["Redis"]
    Proxy --> Redis
    Jobs --> Redis

    API --> OIDC["OIDC provider"]
    Proxy --> Providers["OpenAI, Anthropic, Ollama"]
    Proxy --> MCP["MCP servers"]
```

| Command | Responsibility |
| --- | --- |
| `promptgate api` | HTTP API, OIDC browser login, sessions, user/admin routes, token creation, provider and MCP configuration, firewall management, optional static frontend hosting. |
| `promptgate proxy` | API-token authentication for LLM traffic, firewall enforcement, provider routing, MCP proxying, usage and prompt recording, hot reload. |
| `promptgate schedule` | Background token expiration marking and user access expiration. |
| `promptgate migrate` | GORM migrations for users, tokens, firewall rules, providers, MCP servers, and proxy recorder tables. |

## Package Layout

| Area | Main responsibility |
| --- | --- |
| `cmd/` | Cobra command tree and runtime bootstrapping. |
| `internal/runtime/app` | Shared application wiring for API and scheduler. |
| `internal/runtime/proxy` | AIBridge proxy manager, provider adapters, MCP proxy construction, hot reload. |
| `internal/domain/auth` | OIDC, sessions, roles, user profile context, proxy actor injection. |
| `internal/domain/tokens` | Prompt Gate API token creation, validation, revocation, cleanup, Redis auth cache. |
| `internal/domain/users` | Human users, service accounts, role management, access expiration. |
| `internal/domain/firewall` | Global and service-account firewall rules, snapshots, middleware. |
| `internal/domain/provider` | LLM provider configuration, encrypted API keys, setup helper metadata. |
| `internal/domain/mcp` | MCP server configuration, encrypted sensitive headers, regex filters. |
| `internal/domain/proxy` | Usage, prompt, tool, and interception recording plus dashboards. |
| `internal/platform/*` | Configuration, Postgres, Redis, migrations, and secret encryption. |
| `internal/transport/httpapi` | HTTP routes and JSON handlers. |
| `internal/transport/httpmiddleware` | Session, CORS, authorization, and request logging middleware. |

## Data Stores

PostgreSQL is the source of truth for durable application data:

- users and service accounts
- Prompt Gate API token records and token hashes
- firewall rules
- LLM provider definitions
- MCP server definitions
- proxy interceptions, token usage, prompts, and tool usage

Redis is required by the current runtime configuration. It is used for:

- browser sessions and OIDC authorization requests
- proxy auth cache entries
- provider, MCP, and firewall snapshots
- config version counters and hot-reload events

## Request Flows

### Browser API Flow

```mermaid
sequenceDiagram
    participant B as Browser
    participant A as API
    participant O as OIDC provider
    participant R as Redis
    participant D as PostgreSQL

    B->>A: GET /auth/login
    A->>R: Store state, nonce, PKCE verifier
    A-->>B: Redirect to OIDC provider
    B->>O: Authenticate
    O-->>B: Redirect to /auth/callback
    B->>A: GET /auth/callback?state&code
    A->>O: Exchange code and verify tokens
    A->>D: Sync user profile
    A->>R: Store session
    A-->>B: Set HTTP-only session cookie
```

Protected API routes then use the session cookie, refresh the user profile from
the database, reject inactive or `none` users, and enforce route-level roles.

### Proxy Flow

```mermaid
sequenceDiagram
    participant C as LLM client
    participant P as Proxy
    participant R as Redis
    participant D as PostgreSQL
    participant U as Provider or MCP

    C->>P: Bearer Prompt Gate API token
    P->>R: Check auth cache
    P->>D: Validate token hash and user when cache misses
    P->>P: Apply firewall snapshot
    P->>U: Forward request through AIBridge
    P->>D: Record interception, prompts, tokens, tools
    P-->>C: Stream or return provider response
```

The proxy removes `Authorization` and `X-Api-Key` before forwarding so Prompt
Gate credentials are not leaked to upstream providers.

## Configuration Reload

Configuration mutations publish Redis events on `promptgate:config:events`.
The proxy subscribes to those events and reacts without a process restart.

```mermaid
flowchart TD
    Admin["Admin API mutation"] --> DB["Write PostgreSQL"]
    DB --> Event["Redis version bump and event"]
    Event --> Proxy["Proxy watcher"]
    Proxy --> Kind{"Domain"}
    Kind -->|firewall| Snapshot["Refresh firewall snapshot"]
    Kind -->|providers or mcp| Bridge["Debounced bridge rebuild"]
    Kind -->|auth| AuthCache["Update auth cache version"]
```

Provider and MCP updates trigger a debounced bridge rebuild. Firewall updates
refresh the in-memory snapshot only. Auth updates bump the token auth cache
version, which invalidates old cache keys.

## Migrations

`promptgate migrate` runs GORM migrations in dependency order:

1. users
2. tokens
3. firewall
4. providers
5. MCP
6. proxy recorder tables

Run migrations before starting API, proxy, or scheduler processes in a new
environment.
