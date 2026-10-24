# Security Model

Prompt Gate separates browser administration from proxy traffic. Browser users
authenticate with OIDC sessions. Applications and service accounts call the LLM
proxy with Prompt Gate API tokens.

## Identity And Sessions

OIDC login uses:

- authorization code flow
- PKCE verifier
- state
- nonce
- ID token verification
- OIDC access-token validation against the configured JWKS URL

The first account created by an OIDC login in an empty database receives role
`admin`. Administratively pre-created accounts and subsequent OIDC users start
active with role `limited`. This default applies only when a human account is
created: existing roles are not migrated, and later OIDC logins preserve the
stored role, including an explicit `none`. Pre-created accounts are matched by
the exact, case-sensitive `preferred_username` claim. Their OIDC subject, email,
display name, and last-login timestamp are filled or refreshed at login, while
the local ID, role, active state, expiration, subscription, note, groups, and
virtual keys are preserved.

Administrators can issue a virtual key before the first SSO login. Because a
pre-created account is already active with role `limited`, that key can be used
immediately; SSO login is not an extra precondition. Proxy authorization still
uses the stable local account ID and applies normal role, active-state, expiry,
group, quota, and policy checks. The administration user list reports recent
platform activity separately from access: `activeInLast30Days` means the KPI
store contains at least one proxy request for today or the preceding 29 UTC
calendar days. It is eventually consistent and must not be treated as an
authorization signal.

When it succeeds, deleting a local user removes its current local account and
authorization records, but it is not an identity-provider denylist. Deletion
does not cascade retained proxy interceptions or daily KPI/breakdown rows;
PostgreSQL therefore rejects deletion of an account still referenced by that
usage data. The same restriction applies to service accounts. If the same OIDC
identity can sign in again after a successful deletion, Prompt Gate creates a
new active account with a new local ID. Its role is `limited` when another local
identity still exists; if deletion left the users table empty, the normal
first-user bootstrap assigns `admin`. To block future access, keep the account
inactive or assigned role `none`, or revoke the identity's access at the OIDC
provider.

Browser sessions are stored in Redis when Redis is configured. Session cookies
are HTTP-only, use `PROMPTGATE_SESSION_COOKIE_NAME`, and inherit secure-cookie
behavior from `PROMPTGATE_BACKEND_BASE_URL`: HTTPS backend URLs produce secure
cookies.

## Roles

Prompt Gate uses these application roles:

| Role | Meaning |
| --- | --- |
| `none` | Identity exists but has no application or proxy access; used for explicit access removal and expired access. |
| `limited` | Can use the proxy with centrally administered keys and access personal profile, usage, quota, quota-reset, dashboard, group, and FAQ routes. Cannot access setup, monitoring, or personal token-management routes. |
| `user` | Has `limited` access plus setup, monitoring, and personal token management. |
| `admin` | Can access all admin routes. |

Protected routes reject inactive users, users with role `none`, and unknown or
legacy role values. Admin routes require role `admin` unless a valid
administration API key is used.

## Administration API Key

`PROMPTGATE_ADMIN_API_KEY` optionally enables a second authentication method
for `/api/v1/admin/**`. A trusted CLI or server sends the configured value in
exactly one `X-Admin-API-Key` header. Empty or whitespace-only configuration
keeps the feature disabled. OIDC remains required for the API and continues to
authenticate browser users.

The key is deliberately limited to admin routes. It is not accepted by user
profile or token routes, the LLM proxy, or `/auth/**`, and it is not integrated
into the frontend or browser CORS flow. Within the admin route group it grants
unrestricted read and mutation access, including destructive actions. The key
is global rather than user-specific, so key-authenticated requests cannot be
attributed to an individual operator by the authentication mechanism.

Treat the key as a high-impact production secret: store it in a secret manager,
never place it in a URL or frontend bundle, avoid logging it, and transmit it
only over HTTPS. Although the application enforces no minimum length, use a
random high-entropy value in production. The API compares SHA-256 fingerprints
in constant time and does not expose the configured value. Only one value is
accepted at a time; rotation requires replacing the secret and restarting every
API process or triggering an API deployment rollout, with no dual-key grace
period.

## Browser API Documentation

The Swagger UI at `/admin/documentation` and its OpenAPI document at
`/api/v1/admin/openapi.json` are protected by the same administrator session
checks as the rest of the browser administration surface. The UI authenticates
Try it out requests only with the current HTTP-only session cookie. JavaScript
does not read or persist the cookie, and it does not inject bearer credentials
or `X-Admin-API-Key` headers.

Swagger Try it out executes against the configured backend and can perform live
create, update, and delete operations. Keep the frontend and API same-site so
the `SameSite=Lax` session cookie remains usable, serve production deployments
over HTTPS, and grant administrator sessions only to trusted operators. The
administration API key remains reserved for trusted CLI and server-to-server
clients.

## API Tokens

Prompt Gate API tokens are signed JWTs and are used only for proxy
authentication. The raw token is returned once at creation time. The database
stores only a SHA-256 hash of the raw token plus metadata and lifecycle fields.

Token validation checks:

- JWT signature with `PROMPTGATE_JWT_SECRET`
- expected signing method
- required subject and token id claims
- stored token hash
- token ownership
- revoked state
- expiration state
- active user or service account
- role is one of `limited`, `user`, or `admin`

When upgrading from a release that still allowed the legacy `manager` role,
verify that no user retains it. Prompt Gate does not convert legacy `manager`
rows; such a row is treated as an invalid role and grants no application or
proxy access until an administrator assigns a supported role.

Token revocation publishes an `auth` config event. The proxy responds by
moving its Redis auth cache to a new version. Each cache lookup also reads the
authoritative Redis version atomically with the entry from that generation, so
a missed Pub/Sub event cannot keep an older generation active. If Redis cannot
increment the auth version after the database mutation, however, an already
cached principal can remain usable for at most its remaining auth-cache TTL.
Treat the corresponding Redis warning as security-relevant and restore Redis
promptly; `PROMPTGATE_REDIS_CACHE_TTL` bounds this residual stale-cache window.

## Model Lab tickets and privacy

Model Lab uses a separate ticket type from normal API tokens. The API signs
`pgml_` tickets with the exact `PROMPTGATE_SECRETS_KEY` value already used to
encrypt stored provider credentials; API and proxy must therefore share that
value. It remains unrelated to `PROMPTGATE_JWT_SECRET`. Each ticket is bound to one account, role, provider,
model, protocol, operation, method, exact proxy path, comparison, browser
origin, issue time, expiry, and unique ID. Redis stores only its SHA-256 hash
and signed binding summary. Atomic `GETDEL` makes it single-use, and a Redis
failure fails closed.
An atomic per-account reservation ZSET bounds tickets issued but not yet
consumed; reservations are released on consumption or expire with the ticket.

`PROMPTGATE_PROXY_BASE_URL` may include a browser-visible routing prefix such
as `/remote` only when the edge removes it before forwarding. The signed path
is the canonical post-rewrite path observed by the proxy; mixed-case method
variants are rejected before quota or provider middleware can run.

Provider model discovery never follows HTTP redirects, preventing provider
credentials from being forwarded to a redirected origin.

The browser cannot choose an arbitrary upstream path. The proxy rejects query
strings, unknown JSON fields, non-streaming requests, empty text, system,
tool, function, and multimodal content, malformed role sequences, models
different from the ticket, and values outside configured bounds. Text chat
history must alternate strictly between `user` and `assistant`, start with
`user`, and end with the current `user` message. The request-size limit covers
the complete serialized history. The proxy re-encodes the validated message
list, reloads the account, and runs the normal client-IP, OPA, group, quota,
provider, and accounting controls. A Redis lease limits concurrent streams per
account and expires automatically after a crashed request.

Model Lab is metadata-only by construction. A request-scoped policy suppresses
prompts, outputs, reasoning, tool data, browser/client session identifiers,
personal identity fields, user-agent strings, raw tickets, and upstream error
bodies across request debugging, Redis events, PostgreSQL records, and OTel
spans. Token counts, provider/model, local account ID, timing, status, and cost
may remain for quota and operational accounting. This override applies even
when global OTel content capture is enabled. The provider necessarily receives
the prompt selected by the user; metadata-only means Prompt Gate does not
retain or export that content, not that it is hidden from the provider. The
conversation itself exists only in the browser tab. A comparison deliberately
sends the same history snapshot to every provider selected by the user; its
alternative responses are not added to later chat context.

Model Lab is always enabled for `user` and `admin` roles and has no runtime
feature flag. Before deploying a release that includes it, verify a canary
prompt is absent from application logs, Redis payloads, PostgreSQL content
tables, traces, browser storage, URLs, and returned errors.

## Service Accounts

Service accounts are stored as users with type `service`. They can receive
Prompt Gate API tokens and are evaluated by the external OPA policy.

Service account identifiers must be lowercase alphanumeric with dashes or
underscores and have a maximum length of 64 characters.

## External OPA policy

After token authentication, the proxy sends OPA only stable identity IDs,
role/type, sorted technical access-group names in `input.identity.groups`,
virtual-key metadata, resolved client IP, method, path, and host.
Tokens, personal profile fields, headers, and request bodies are never sent.
OPA is the sole policy source and the supplied example denies by default.
`PROMPTGATE_OPA_ENABLED=false` is an emergency fail-open switch for this policy
stage. It must be time-bounded and operationally monitored because every
authenticated request then bypasses OPA; group, quota, and provider controls
still apply.

The proxy normally uses the TCP remote address. In production, prefer
`PROMPTGATE_PROXY_TRUSTED_PROXIES` with explicit ingress or reverse-proxy CIDRs
so forwarded headers are accepted only from known peers.
`PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS` should be enabled only behind trusted
infrastructure that strips or rewrites untrusted forwarding headers.

## Secret Storage

`PROMPTGATE_SECRETS_KEY` must be a base64-encoded 32-byte key. It initializes
an AES-256-GCM cipher used for stored downstream secrets:

- provider API keys

Encrypted values use a versioned envelope with a random nonce. Keep the key
stable across restarts and deployments. Rotating it requires a deliberate data
migration because existing provider secrets must be decrypted and
re-encrypted.

## CORS

The API and proxy use `PROMPTGATE_CORS_ALLOWED_ORIGINS`. When no explicit
origins are configured:

- the API defaults to `PROMPTGATE_FRONTEND_BASE_URL`
- the proxy defaults to `PROMPTGATE_FRONTEND_BASE_URL` when present

Loopback origins are expanded across `localhost`, `127.0.0.1`, and `::1` for
local development. Credentialed browser API requests allow the methods `GET`,
`POST`, `PUT`, `PATCH`, `DELETE`, and `OPTIONS`; browsers manage the `Cookie`
header themselves. Allowed request headers are `Authorization`, `Content-Type`,
`Accept`, and `Idempotency-Key`. Neither `Cookie` nor `X-Admin-API-Key` is
manually exposed through CORS.

## Operational Checklist

- Store `PROMPTGATE_JWT_SECRET`, `PROMPTGATE_SECRETS_KEY`, database
  credentials, Redis credentials, OIDC client secrets, and any configured
  `PROMPTGATE_ADMIN_API_KEY` in a secret manager.
- Use HTTPS origins for production `PROMPTGATE_BACKEND_BASE_URL` and
  `PROMPTGATE_FRONTEND_BASE_URL`.
- Run migrations before serving traffic.
- Keep Redis available for sessions, hot reload, and proxy auth cache
  invalidation.
- Prefer explicit `PROMPTGATE_PROXY_TRUSTED_PROXIES` CIDRs over global
  forwarded-header trust.
- Review OPA rules for service accounts before issuing long-lived tokens.
- Restrict the administration API key to trusted CLI and server workloads, and
  restart or roll out every API replica after rotating it.
