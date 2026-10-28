# API Reference

The API process is started with:

```sh
promptgate --config /path/to/promptgate.yaml api
```

All API responses are JSON unless a route redirects as part of the OIDC browser
flow. Protected browser API routes use the HTTP-only session cookie configured
by `common.session.cookie_name`.

## Public And Auth Routes

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Health check. Returns `200` when the API can reach the database and `503` when the database check fails. |
| `GET` | `/auth/login` | Starts OIDC authorization with state, nonce, and PKCE. |
| `GET` | `/auth/callback` | Handles the OIDC callback, syncs the user, creates a session, and redirects to the frontend. |
| `GET` | `/auth/logout` | Clears the local session and redirects through OIDC logout when available. |
| `GET` | `/auth/session` | Returns the current browser session state. |

Successful session responses also include `adminContactEmail`. It contains the
trimmed `api.admin_contact_email` value, or an empty string when no contact is
configured. The frontend uses it only for the persistent limited-access banner.

If `api.static_assets_dir` is set, the API also serves static frontend
assets from `/` and falls back to the SPA shell for frontend routes.

## Interactive API Documentation

Active `limited`, `user`, and `admin` accounts can open the native Swagger UI
at `/documentation` from the user menu. The page documents the complete control
API under `/api/v1/**`; authentication routes, the health check, and the LLM
proxy are intentionally excluded.

The page loads its OpenAPI 3.0 document from the protected
`GET /api/v1/openapi.json` endpoint and uses the current HTTP-only browser
session for Try it out requests. The browser sends the session cookie directly;
the frontend never reads it or copies it into a header. The documentation does
not expose `X-Admin-API-Key` as an interactive authentication option.

Try it out is enabled for reads and mutations. `POST`, `PUT`, `PATCH`, and
`DELETE` requests affect the current environment immediately, but every
operation remains subject to the current user's role and access policy.

## User Routes

These routes require a valid browser session and active app access. Roles
`limited`, `user`, and `admin` can access the personal dashboard, usage,
quota, groups, profile, and FAQ routes. Setup, monitoring, and personal token
routes require `user` or `admin`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/me` | Current authenticated user profile. |
| `GET` | `/api/v1/openapi.json` | Runtime OpenAPI document used by the authenticated Swagger UI. |
| `GET` | `/api/v1/me/usage` | Usage summary for the current user. |
| `GET` | `/api/v1/me/quota` | Current subscription quota state and reset windows. |
| `GET` | `/api/v1/me/quota-resets` | Current quota-reset credit balance, applicability, and pending state. |
| `POST` | `/api/v1/me/quota-resets/use` | Spend one credit to clear both live quota windows. Requires a non-blank `Idempotency-Key` of at most 512 characters. |
| `GET` | `/api/v1/me/quota-resets/history` | List applied credit grants and uses, newest first, with `page` and `pageSize`. |
| `GET` | `/api/v1/me/groups` | Access groups assigned to the current user. |
| `GET` | `/api/v1/me/models` | Group-filtered Model Lab catalog with discovery freshness, price provenance, and metadata-only availability state. Requires role `user` or `admin`. |
| `POST` | `/api/v1/me/model-lab/tickets` | Issues one short-lived, single-use proxy ticket per selected Model Lab target. Requires role `user` or `admin`. |
| `GET` | `/api/v1/me/help/setup` | Provider setup helper with proxy base URLs and available models. |
| `GET` | `/api/v1/me/dashboard/overview` | Current usage overview, optional previous-period comparison, and cost-coverage state. |
| `GET` | `/api/v1/me/dashboard/tokens` | Token totals for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/messages` | Message count for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/duration` | Total proxied request duration for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/activity` | Daily usage buckets for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/top-models` | Model usage breakdown. |
| `GET` | `/api/v1/me/dashboard/top-provider-names` | Provider-name usage breakdown. |
| `GET` | `/api/v1/me/dashboard/top-provider-types` | Provider-type usage breakdown. |
| `GET` | `/api/v1/me/dashboard/top-tools` | Top model-requested tool calls observed for the current user. |
| `GET` | `/api/v1/monitoring/status` | Current user-visible monitoring status and degraded service names. |
| `GET` | `/api/v1/faq` | Published frequently asked questions. |

Common list-style routes use query parameters such as `page`, `pageSize`,
`search`, `sortBy`, and `sortDir` where supported. Dashboard routes use the
usage windows implemented by the proxy domain: `7d`, `30d`, and `all`. The
personal activity route also accepts `365d`. Usage summary, dashboard overview,
dashboard token, and dashboard activity responses include an optional
`estimatedCost` object. Its `inputUsd`, `outputUsd`, `embeddingUsd`, and
`totalUsd` values are indicative estimates based on configured
USD-per-1M-token rates, not billing records. The same object includes a `rates`
field with the rates used for the calculation.
Dashboard overview responses always include `costCoverage`. A value of
`complete` means retained raw usage covers the requested estimate period;
`partial` reports the retained bounds, and `unavailable` omits
`totals.estimatedCost` instead of presenting an incomplete zero value.
Top-tools responses rank invocation names by observed call count. They expose
`totalCalls` from the durable usage KPI and `attributedCalls` from the durable
per-tool breakdown so clients can identify historical periods that predate
name-level aggregation. Tool arguments, server URLs, errors, and metadata are
never included.

### Model Lab

Model Lab is always available to `user` and `admin` roles. Its catalog only
contains providers and models allowed by the current user's groups. Discovery
is concurrent and cached in Redis; a failed refresh can return the last known
good catalog as `stale`, while a provider without usable metadata is reported
as `unavailable`. `observedAvailability.state` remains `unknown` in this MVP:
catalog discovery is not treated as evidence of successful generation traffic.
Discovery uses the provider's OpenAI-compatible `/models` endpoint or
Anthropic `/v1/models`. Administrators can set `config.modelLabModels` to an
array of model IDs as an explicit fallback when live discovery is unavailable.
The response `limits` object exposes the effective target, prompt-byte, and
output-token limits so the Model Lab frontend can enforce the deployed
configuration.

Ticket requests contain one to three distinct `{providerName, model}` targets.
The API chooses each exact proxy path and protocol, binds the ticket to the
current account and browser origin, and returns it only once. Tickets expire
after `common.model_lab.ticket_ttl` and cannot be replayed. Responses from
both Model Lab endpoints use `Cache-Control: private, no-store`.
A Redis-backed per-account budget atomically bounds unused outstanding tickets;
consuming a ticket releases its slot, while abandoned slots expire with it.

The chat accepts a text-only `messages` history with `stream: true`, an
optional temperature from 0 through 1, and a bounded `max_tokens`. Messages
must alternate strictly between `user` and `assistant`, start with `user`, and
end with the current `user` message. The prompt-byte limit applies to the
complete serialized request, not only the newest message. System messages,
tools, files, audio, images, multimodal content, arbitrary paths, and automatic
retries are rejected.

Conversation history exists only in the current browser tab. Each generation
uses a new ticket and the normal governed proxy path. Comparing a completed
turn sends the same history snapshot to each selected alternative; every
alternative is a separate request and consumes quota independently.

## Token Routes

Prompt Gate API tokens are created from the browser API and used as bearer
tokens against the proxy.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/tokens` | Create a token for the current user. The raw token is returned only once. |
| `GET` | `/api/v1/tokens` | List current-user tokens. |
| `DELETE` | `/api/v1/tokens/{id}` | Revoke one current-user token. |

Token names must be lowercase alphanumeric with dashes or underscores and have
a maximum length of 64 characters. Requested human-user token TTL must be
between 1 and 30 days. If omitted, the default TTL is 7 days for `limited` and
`user`, and 30 days for `admin`. Limited users cannot call these personal token
routes; an administrator creates their token through the admin route below.

## Admin Routes

All `/api/v1/admin/**` routes accept either a browser session with role `admin`
or, when `api.admin_api_key` is configured, exactly one
`X-Admin-API-Key` header containing the configured value. If the header is
absent, normal session, app-access, and role checks apply. If it is present but
empty, duplicated, disabled, or invalid, the API returns
`401 {"error":"invalid_admin_api_key"}` and does not fall back to a session
cookie.

The administration key is intended only for trusted CLI and server-to-server
clients. It is accepted only on `/api/v1/admin/**`: it does not authenticate
`/api/v1/me`, user token routes, proxy routes, or `/auth/**`, and it does not
change browser CORS behavior. A valid key grants the complete admin surface,
including create, update, revoke, and delete operations, without an individual
user identity. Always transmit it over HTTPS.

Load a client-side copy from a secret manager rather than putting the secret
itself in a command or script:

```sh
curl --fail-with-body \
  --header "X-Admin-API-Key: ${ADMIN_API_KEY:?not set}" \
  https://promptgate.example.com/api/v1/admin/providers
```

### Documentation

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/openapi.json` | Deprecated administrator-compatible alias for the runtime OpenAPI document. |

### Dashboard

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/dashboard/overview` | Organization usage overview with comparison, adoption, and cost-coverage state. |
| `GET` | `/api/v1/admin/dashboard/quota-risk` | Accounts at or above 80% of their most saturated finite quota window. |
| `GET` | `/api/v1/admin/dashboard/tokens` | Global token totals for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/messages` | Global message count for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/duration` | Global total proxied request duration for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/activity` | Global daily usage buckets for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/top-models` | Global model usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/top-provider-names` | Global provider-name usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/top-provider-types` | Global provider-type usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/top-tools` | Top model-requested tool calls observed across all identities. |
| `GET` | `/api/v1/admin/dashboard/adoption` | Active users, active service accounts, and currently valid virtual keys. |
| `GET` | `/api/v1/admin/dashboard/top-identities` | Top users and service accounts by token volume. |

Admin dashboard token and activity responses follow the same optional
`estimatedCost` shape as current-user dashboard responses.

Quota risk is read from the PostgreSQL projection maintained by the scheduler.
It counts every projected human or service account at or above 80% of a finite
5-hour or 7-day quota, treats 100% and above as critical, and returns at most
the five most saturated accounts. `projectionSyncedAt` and each item `syncedAt`
show the projection age. The scheduler refreshes it every
`schedule.subscription_quota_sync_interval` (`5m` by default); inspect those
timestamps because a scheduler or Redis failure can make it older.

### Quotas and resets

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/quota-resets/quota-states` | Paginated active SQL quota projections, including both 5-hour and 7-day windows per account. |
| `GET` | `/api/v1/admin/quota-resets/events` | Paginated quota-reset ledger with period counters and the global pending-use backlog. |

Both endpoints are read-only snapshots intended for manual administration
refreshes. Pagination defaults to `page=1&pageSize=20`; `pageSize` is capped at
100.

Quota states default to `view=at-risk`, which includes `warning` accounts at or
above 80% and `critical` accounts at or above 100% of at least one finite
window. `view=all` means all active rows in the PostgreSQL quota projection,
not all registered accounts. Optional filters are `accountType=user|service`
and a case-insensitive `search` over account identity fields. Results default
to `sortBy=saturation&sortDir=desc`; `sortBy` also accepts `displayName`,
`resetAt`, and `syncedAt`. The possible account states are `unassigned`,
`unlimited`, `healthy`, `warning`, and `critical`. Quota, remaining, percentage,
and reset values are `null` when a window is unlimited or unassigned. An
invalid view, account type, or sort returns `400` with
`invalid_quota_view`, `invalid_account_type`, or `invalid_sort`.

Reset events default to `window=30d`, with `window=7d` and `window=all` also
available. The 7-day and 30-day bounds are computed in UTC; `startsAt` is
`null` for `all`. Optional filters are
`type=monthly_grant|admin_grant|quota_reset_used`,
`status=pending|applied`, and a case-insensitive target-account identity
`search`.
Results default to `sortBy=createdAt&sortDir=desc`; `sortBy` also accepts
`appliedAt`, `user`, `type`, and `status`. Invalid values return `400` with
`invalid_usage_window`, `invalid_quota_reset_type`,
`invalid_quota_reset_status`, or `invalid_sort`.

`monthlyGrants`, `adminGrants`, and `appliedResetUses` follow the selected
window. `pendingResetUsesTotal` is always the complete current backlog so an
older blocked reset cannot be hidden by a date filter. Actors are classified as
`system`, `user`, `admin_session`, or `admin_api_key`. An API-key or system
actor has null identity fields; an administrator deleted after the event also
has null `actor.userId` and `actor.displayName`, which the administration UI
renders as `Deleted administrator`. Deleting the target account continues to
cascade-delete its ledger events. The response never contains an idempotency
key, including its stored hash.

### Users

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/users/{id}/ips` | List client IP addresses observed for a user, paginated and sortable by `ip` or `lastSeen`. |
| `GET` | `/api/v1/admin/users` | List users with pagination, filters, usage totals, and sorting. |
| `POST` | `/api/v1/admin/users` | Pre-create a human user from an exact OIDC `preferred_username`. |
| `GET` | `/api/v1/admin/users/{id}` | Get one user. |
| `GET` | `/api/v1/admin/users/{id}/statistics` | Get aggregated usage totals, duration, and daily activity for one human user. |
| `PATCH` | `/api/v1/admin/users/{id}` | Update role, active state, or access expiration. |
| `PATCH` | `/api/v1/admin/users/{id}/note` | Update the private administration note. |
| `DELETE` | `/api/v1/admin/users/{id}` | Attempt to delete one local user and its current access data. A later OIDC sign-in can provision a new account under the normal first-user bootstrap rules. |
| `GET` | `/api/v1/admin/users/{id}/tokens` | List a user's tokens. |
| `POST` | `/api/v1/admin/users/{id}/tokens` | Create a user token and return its raw value once. |
| `DELETE` | `/api/v1/admin/users/{id}/tokens/{tokenId}` | Revoke one user token. |
| `GET` | `/api/v1/admin/users/{id}/groups` | List the user's access groups. |
| `PUT` | `/api/v1/admin/users/{id}/groups` | Replace the user's access groups. |
| `PUT` | `/api/v1/admin/users/{id}/subscription-plan` | Assign or clear the user's subscription plan. |
| `POST` | `/api/v1/admin/users/{id}/quota-resets` | Offer exactly one reset credit to a human account. Requires a non-blank `Idempotency-Key` of at most 512 characters. |

The first user created by an OIDC login in an otherwise empty database is
assigned role `admin`. Later OIDC users, and all administratively pre-created
users, start active with role `limited`. This default applies only to newly
created human accounts: existing roles are not migrated, and later OIDC logins
preserve the stored role, including an explicit `none`. Pre-creation accepts only
`{"preferredUsername":"..."}`. The value is trimmed at its edges and then
stored case-sensitively; an empty value returns `invalid_preferred_username`
and an existing human identifier returns `preferred_username_conflict`.
Once `POST /api/v1/admin/users` succeeds, the account exists even if a later
request for its details or subscription plans fails. Clients must resume setup
from that account instead of repeating the create request.

A user or service-account deletion succeeds only when no retained proxy
interception, daily KPI, or daily breakdown row still references the account.
PostgreSQL protects those usage records with restrictive foreign keys; the
current implementation does not cascade them, so an account with retained
usage can make the delete endpoint return `500` and remains unchanged.

Before the first SSO login, `sub` and `lastLoginAt` are `null`, while email and
name are empty. The first login whose exact `preferred_username` matches fills
those identity fields on the same local record and preserves its ID, role,
active state, expiration, subscription, note, groups, and virtual keys.

`isActive` is the administrator-controlled access switch. Separately,
`activeInLast30Days` reports whether the account has at least one proxy request
recorded in the KPI data for today and the preceding 29 UTC calendar days. The
activity value is eventually consistent and does not imply that access is
currently enabled.

Administrators may create a token for any human user, including an inactive
user or one with role `none`. The token is stored but proxy authentication
continues to reject it until the account is active and has a supported access
role. A newly pre-created account already satisfies those two conditions as an
active `limited` user, so an administrator-created token is usable immediately
before the first SSO login. Normal token expiration and revocation, account
expiration, group, quota, and policy checks still apply.

Quota-reset mutations require a caller-generated `Idempotency-Key` containing
at least one non-whitespace character and at most 512 Unicode characters after
trimming. Retrying the same operation with the same key does not debit or grant
twice. A reset clears both the rolling 5-hour and 7-day Redis counters but
preserves all historical usage, interception, and KPI records. Business
conflicts return
`409` with one of `quota_reset_credit_unavailable`,
`quota_reset_not_applicable`, `quota_reset_balance_full`, or
`quota_reset_pending`. A missing key returns
`400 {"error":"missing_idempotency_key"}`. Administrators may grant a credit to
any human account, including an inactive account or one without a subscription,
up to the two-credit wallet maximum.
The private ledger records the connected administrator ID for session grants;
API-key grants record only the `admin_api_key` actor type. Raw administration
keys and raw idempotency keys are never persisted.

When a reset is applied synchronously, the response has `status: "applied"`
and includes the refreshed `quota`. When reconciliation is required, the API
returns HTTP `202` with `status: "pending"` and omits `quota`; clients should
use the wallet `pendingReset` state and poll the quota-reset summary until the
operation completes.

User statistics accept `window=7d`, `window=30d`, or `window=all`, defaulting
to `30d` when omitted. The response contains `window`, `startsAt`, `endsAt`,
`totals`, `totalDurationMs`, `averageDurationMs`, `daily`, and `costCoverage`.
Cost coverage is `complete`, `partial`, or `unavailable`; partial coverage
includes the retained raw-usage bounds, while unavailable coverage omits
`estimatedCost` instead of presenting a zero value. Unknown users and
service-account IDs both return
`404 {"error":"user_not_found"}`.
`averageDurationMs` averages completed requests only. It is `null`, not zero,
when no request completed or when retained raw interceptions do not exactly
cover the durable request count for the selected window.

### Service Accounts

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/service-accounts` | List service accounts. |
| `POST` | `/api/v1/admin/service-accounts` | Create a service account. |
| `GET` | `/api/v1/admin/service-accounts/{id}` | Get one service account. |
| `GET` | `/api/v1/admin/service-accounts/{id}/statistics` | Get aggregated usage totals, duration, and daily activity for one service account. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}` | Update identifier, name, or active state. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}/note` | Update the private administration note. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}` | Attempt to delete one service account, subject to the retained-usage constraint described above. |
| `GET` | `/api/v1/admin/service-accounts/{id}/tokens` | List service-account tokens. |
| `GET` | `/api/v1/admin/service-accounts/{id}/ips` | List client IP addresses observed for a service account, paginated and sortable by `ip` or `lastSeen`. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}/ips` | Clear all recorded client IP addresses for a service account. |
| `POST` | `/api/v1/admin/service-accounts/{id}/tokens` | Create a service-account token. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}/tokens/{tokenId}` | Revoke a service-account token. |
| `PUT` | `/api/v1/admin/service-accounts/{id}/subscription-plan` | Assign or clear the service account's subscription plan. |

Service account identifiers must be lowercase alphanumeric with dashes or
underscores and have a maximum length of 64 characters.

Service-account statistics accept `window=7d`, `window=30d`, or `window=all`,
defaulting to `30d` when omitted. The response has the same shape as user
statistics. Unknown service accounts and human-user IDs both return
`404 {"error":"service_account_not_found"}`.

### Subscriptions

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/subscriptions` | List subscription plans and assignment counts. |
| `POST` | `/api/v1/admin/subscriptions` | Create a subscription plan. |
| `GET` | `/api/v1/admin/subscriptions/{id}` | Get one subscription plan. |
| `PATCH` | `/api/v1/admin/subscriptions/{id}` | Update limits, display metadata, or enabled state. |
| `PUT` | `/api/v1/admin/subscriptions/{id}/default` | Make a plan the default inherited plan. |
| `DELETE` | `/api/v1/admin/subscriptions/{id}` | Delete an unreferenced subscription plan. |

### Access Groups

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/groups` | List access groups, including durable all-time input and output token totals for current members. |
| `POST` | `/api/v1/admin/groups` | Create an access group. |
| `POST` | `/api/v1/admin/groups/model-patterns/validate` | Validate model allow and deny patterns. |
| `GET` | `/api/v1/admin/groups/{id}` | Get one access group. |
| `GET` | `/api/v1/admin/groups/{id}/statistics` | Get aggregated usage totals, duration, and daily activity for the group's current members. |
| `PATCH` | `/api/v1/admin/groups/{id}` | Update group metadata, providers, and model patterns. |
| `DELETE` | `/api/v1/admin/groups/{id}` | Delete an access group. |
| `PUT` | `/api/v1/admin/groups/{id}/members/{userId}` | Add a group member. |
| `DELETE` | `/api/v1/admin/groups/{id}/members/{userId}` | Remove a group member. |

Group statistics accept `window=7d`, `window=30d`, or `window=all`, defaulting
to `30d` when omitted. They aggregate users and service accounts that are
members when the request is made. Usage from before a current member joined is
included, while usage from removed members is excluded. Because an identity can
belong to multiple groups, statistics from different groups can overlap and are
not additive. The response has the same `costCoverage` semantics as user and
service-account statistics. Unknown groups return
`404 {"error":"group_not_found"}`.

Each item returned by the group list also exposes `inputTokens` and
`outputTokens`. These are durable all-time totals from the current human and
service-account members, including inactive accounts. Cache-read and
cache-write token fields are not included. Empty groups return zero. The list
supports `sortBy=inputTokens` and `sortBy=outputTokens`; sorting is applied
before pagination. As with group statistics, usage before a current member
joined is included, removed members are excluded, and totals can overlap
between groups.

### Providers

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/providers` | List LLM provider definitions. |
| `GET` | `/api/v1/admin/providers/model-catalog` | Best-effort provider model catalog for pricing and access previews. |
| `POST` | `/api/v1/admin/providers` | Create a provider. |
| `GET` | `/api/v1/admin/providers/{id}` | Get one provider. |
| `PATCH` | `/api/v1/admin/providers/{id}` | Update provider display name, type, base URL, secret, config, or enabled state. |
| `DELETE` | `/api/v1/admin/providers/{id}` | Delete a provider. |

Supported provider types are `openai`, `anthropic`, and `ollama`. API keys are
stored encrypted and are never returned by the admin API. Provider names are
immutable after creation; update `displayName` to change the label shown in
admin and setup views.

### Pricing

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/pricing` | Get fallback and model-specific prices. |
| `PUT` | `/api/v1/admin/pricing` | Replace the complete pricing configuration. |
| `PATCH` | `/api/v1/admin/pricing/fallback` | Update fallback input and output prices. |
| `POST` | `/api/v1/admin/pricing/models` | Create a model-specific price. |
| `GET` | `/api/v1/admin/pricing/models/{id}` | Get one model-specific price. |
| `PATCH` | `/api/v1/admin/pricing/models/{id}` | Update one model-specific price. |
| `DELETE` | `/api/v1/admin/pricing/models/{id}` | Delete one model-specific price. |
| `GET` | `/api/v1/admin/pricing/check` | Report missing model prices for enabled providers. |

### Monitoring

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/monitoring/services` | List HTTP/S monitoring service definitions. |
| `POST` | `/api/v1/admin/monitoring/services` | Create a monitoring service. |
| `GET` | `/api/v1/admin/monitoring/services/{id}` | Get one monitoring service. |
| `PATCH` | `/api/v1/admin/monitoring/services/{id}` | Update monitoring metadata, URL, expected HTTP code, interval, or enabled state. |
| `DELETE` | `/api/v1/admin/monitoring/services/{id}` | Delete a monitoring service. |
| `POST` | `/api/v1/admin/monitoring/services/{id}/check` | Run one immediate HTTP GET check and persist the result. |

Monitoring service names use lowercase letters, numbers, and single hyphens.
URLs must be `http` or `https`. `expectedStatusCode` must be between `100` and
`599`; omitted create requests default to `200`. `intervalSeconds` must be
between `30` and `86400`; omitted create requests default to `60`.

The app-level `/api/v1/monitoring/status` route returns:

```json
{"status":"ok","services":[]}
```

When any enabled service is degraded, `status` is `degraded` and `services`
contains only enabled degraded services. This response intentionally omits
service URLs.

### FAQs

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/faqs` | List all FAQ entries. |
| `POST` | `/api/v1/admin/faqs` | Create an FAQ entry. |
| `POST` | `/api/v1/admin/faqs/preview` | Render and sanitize FAQ Markdown without saving it. |
| `GET` | `/api/v1/admin/faqs/{id}` | Get one FAQ entry. |
| `PATCH` | `/api/v1/admin/faqs/{id}` | Update an FAQ entry. |
| `PATCH` | `/api/v1/admin/faqs/{id}/position` | Move an FAQ entry. |
| `DELETE` | `/api/v1/admin/faqs/{id}` | Delete an FAQ entry. |

### Setup Guides

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/setup-guides` | List setup guide definitions. |
| `POST` | `/api/v1/admin/setup-guides` | Create a setup guide. |
| `POST` | `/api/v1/admin/setup-guides/validate` | Validate a setup guide template. |
| `PUT` | `/api/v1/admin/setup-guides/reorder` | Replace setup guide ordering. |
| `GET` | `/api/v1/admin/setup-guides/{id}` | Get one setup guide. |
| `PATCH` | `/api/v1/admin/setup-guides/{id}` | Update one setup guide. |
| `DELETE` | `/api/v1/admin/setup-guides/{id}` | Delete one setup guide. |

### Email notifications

Email administration is session-authenticated and restricted to administrators.
Messages are queued in PostgreSQL and delivered asynchronously by the worker;
an accepted response does not mean SMTP delivery has completed.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/admin/mail/preview` | Render a subject and Markdown body into the final branded HTML and plain-text alternatives. |
| `POST` | `/api/v1/admin/mail/audience` | Resolve users, access groups, all human accounts, or an external address. |
| `POST` | `/api/v1/admin/mail/messages` | Queue a manual message. Requires an `Idempotency-Key`. |
| `GET` | `/api/v1/admin/mail/messages` | List message history with type, status, search, and pagination filters. |
| `GET` | `/api/v1/admin/mail/messages/{id}` | Inspect retained content and each recipient attempt. |
| `POST` | `/api/v1/admin/mail/messages/{id}/retry-failed` | Requeue failed deliveries only. |

The `all` audience includes every human account with a valid address, including
inactive and expired accounts. Service accounts are never recipients. Group
and user audiences are deduplicated by account. Markdown is sanitized on the
server; line breaks in mail headers and invalid addresses are rejected. Manual
messages and automatic events share the same responsive email shell. Welcome,
quota, reset, access-group, token lifecycle, monthly-report, and manual emails
each use dedicated English copy and artwork served from `/email/` on the public
frontend URL.

## Error Shape

Middleware failures use a simple JSON error shape:

```json
{"error":"error_code"}
```

Examples include `missing_auth_credentials`, `invalid_token`,
`invalid_admin_api_key`, `account_inactive`, `account_role_none`,
`insufficient_role`, `policy_denied`, and `policy_unavailable`.
