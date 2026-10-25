# Architecture

Prompt Gate Backend is a Go service layer split into five runtime commands from
one binary. Each command has a narrow operational responsibility and shares the
same domain packages.

## Runtime Components

```mermaid
flowchart LR
    Browser["Browser session"] -->|UI, catalog, ticket issuance| API["promptgate api"]
    Browser -->|pgml_ ticket + streamed chat| Proxy["promptgate proxy"]
    Admin["Admin UI"] --> API
    Client["LLM client"] -->|Prompt Gate API token| Proxy
    Worker["promptgate worker"] --> WorkerJobs["Usage event jobs"]
    Schedule["promptgate schedule"] --> Jobs["Maintenance and quota jobs"]
    Migrate["promptgate migrate"] --> DB["PostgreSQL"]

    API --> DB
    WorkerJobs --> DB
    Jobs --> DB

    API -->|sessions, catalogs, tickets, reservations| Redis["Redis"]
    Proxy -->|ticket consumption, leases, quotas, usage| Redis
    Proxy --> OPA["External OPA"]
    WorkerJobs --> Redis
    Jobs --> Redis

    API --> OIDC["OIDC provider"]
    API -->|Model Lab catalog discovery| Providers
    Proxy --> Providers["OpenAI, Anthropic, Ollama"]
```

| Command | Responsibility |
| --- | --- |
| `promptgate api` | HTTP API, OIDC browser login, sessions, user/admin routes, token creation, Model Lab catalog and ticket issuance, provider configuration, optional static frontend hosting. |
| `promptgate proxy` | API-token or Model Lab ticket authentication for LLM traffic, external OPA authorization, provider routing, Redis usage event enqueueing, hot reload. |
| `promptgate worker` | Redis Stream consumer for proxy usage events, raw usage persistence, and dashboard KPI aggregation. |
| `promptgate schedule` | Background token and access expiration, monitoring checks, raw usage cleanup, quota projection, monthly reset-credit distribution, and pending-reset reconciliation. |
| `promptgate migrate` | Ordered migrations for all durable domain models, including subscription and quota-reset state, plus removal of retired schemas. |

## Package Layout

| Area | Main responsibility |
| --- | --- |
| `cmd/` | Cobra command tree and runtime bootstrapping. |
| `internal/runtime/app` | Shared application wiring for API and scheduler. |
| `internal/runtime/proxy` | AIBridge proxy manager, provider adapters, and hot reload. |
| `internal/domain/auth` | OIDC, sessions, roles, user profile context, proxy actor injection. |
| `internal/domain/modellab` | Group-filtered model catalog, one-time browser tickets, text-chat validation, concurrency leases, and metadata-only privacy policy. |
| `internal/domain/tokens` | Prompt Gate API token creation, validation, revocation, cleanup, Redis auth cache. |
| `internal/domain/users` | Human users, service accounts, role management, access expiration. |
| `internal/domain/policy` | OPA client, Redis decision cache, policy middleware, and observability. |
| `internal/domain/provider` | LLM provider configuration, encrypted API keys, setup helper metadata. |
| `internal/domain/proxy` | Usage, tool, and interception recording plus dashboards. |
| `internal/domain/subscriptions` | Subscription plans, rolling quota enforcement, quota projections, and reset-credit wallets. |
| `internal/platform/*` | Configuration, Postgres, Redis, migrations, and secret encryption. |
| `internal/transport/httpapi` | HTTP routes and JSON handlers. |
| `internal/transport/httpapi/openapi` | OpenAPI source, protected document handler, and runtime/spec parity tests. |
| `internal/transport/httpmiddleware` | Session, CORS, authorization, and request logging middleware. |

## Data Stores

PostgreSQL is the source of truth for durable application data:

- users and service accounts
- Prompt Gate API token records and token hashes
- LLM provider definitions
- access groups, provider/model rules, and membership
- FAQ entries, setup guides, pricing, and monitoring definitions
- subscription plans and account assignments
- projected live quota state, quota-reset wallets, and the reset ledger
- proxy interceptions, token usage, and tool usage for operational metrics
- daily dashboard KPI aggregates and processed usage event ids

Redis is required by the current runtime configuration. It is used for:

- browser sessions and OIDC authorization requests
- proxy auth cache entries
- provider snapshots
- access-group and subscription-plan snapshots
- Model Lab catalog last-known-good entries, one-time ticket hashes, pending
  reservations, and active-stream leases
- versioned subscription assignments and rolling 5-hour and 7-day quota state
- quota-reset sequence and cutoff barriers
- cached OPA deny decisions (allows are always re-evaluated)
- config version counters and hot-reload events
- proxy usage event stream `promptgate:usage:events`

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
    participant C as LLM client or Model Lab browser
    participant P as Proxy
    participant R as Redis
    participant D as PostgreSQL
    participant O as External OPA
    participant W as Worker
    participant U as Provider

    C->>P: HTTP request under optional public proxy.base_path
    P->>P: Validate and remove base path
    alt Prompt Gate API token
        P->>R: Check versioned auth cache
        P->>D: Validate token hash and account on cache miss
    else Model Lab browser ticket
        P->>P: Parse ticket and validate canonical text-only payload
        P->>R: Consume bound one-time ticket
        P->>D: Reload authoritative account
        P->>R: Acquire account concurrency lease
    end
    P->>P: Resolve client IP; enforce route and method allowlist
    P->>R: Check cached policy denial
    P->>O: Evaluate policy unless denied by cache
    P->>P: Enforce group, provider, and model access snapshot
    P->>R: Enforce effective subscription and rolling quotas
    P->>U: Forward request through AIBridge
    P->>R: XADD usage events
    W->>R: XREADGROUP usage events
    W->>D: Persist raw rows and increment KPI aggregates
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
    Kind -->|providers| Bridge["Debounced bridge rebuild"]
    Kind -->|groups| Groups["Refresh access snapshot"]
    Kind -->|auth| AuthCache["Update auth cache version"]
    Kind -->|subscriptions| Quotas["Refresh plans and invalidate assignments"]
```

Provider updates trigger a debounced bridge rebuild. Group updates refresh the
proxy access snapshot. Auth updates bump the token auth cache version, which
invalidates old cache keys. A separate subscription watcher refreshes plan and
versioned assignment snapshots; per-user assignment entries expire on a short
independent bound without a global Redis scan. OPA denials expire through
`proxy.opa.cache_ttl`; allows are never cached.

## Migrations

`promptgate migrate` runs GORM migrations in dependency order:

1. users
2. subscriptions, quota projections, reset wallets, and reset events
3. tokens
4. FAQ entries
5. providers
6. setup guides
7. pricing
8. access groups
9. monitoring services
10. proxy recorder and dashboard KPI tables
11. destructive removal of retired firewall and MCP schemas

Run migrations before starting API, proxy, worker, or scheduler processes in a
new environment.
