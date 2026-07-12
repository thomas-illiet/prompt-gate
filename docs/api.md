# API Reference

The API process is started with:

```sh
promptgate api
```

All API responses are JSON unless a route redirects as part of the OIDC browser
flow. Protected browser API routes use the HTTP-only session cookie configured
by `PROMPTGATE_SESSION_COOKIE_NAME`.

## Public And Auth Routes

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Health check. Returns `200` when the API can reach the database and `503` when the database check fails. |
| `GET` | `/auth/login` | Starts OIDC authorization with state, nonce, and PKCE. |
| `GET` | `/auth/callback` | Handles the OIDC callback, syncs the user, creates a session, and redirects to the frontend. |
| `GET` | `/auth/logout` | Clears the local session and redirects through OIDC logout when available. |
| `GET` | `/auth/session` | Returns the current browser session state. |

If `PROMPTGATE_STATIC_ASSETS_DIR` is set, the API also serves static frontend
assets from `/` and falls back to the SPA shell for frontend routes.

## User Routes

These routes require a valid browser session and active app access. The
dashboard, prompt, usage, setup, and token routes require role `user`,
`manager`, or `admin`.

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/me` | Current authenticated user profile. |
| `GET` | `/api/v1/me/usage` | Usage summary for the current user. |
| `GET` | `/api/v1/me/prompts` | Prompt history for the current user. |
| `GET` | `/api/v1/me/quota` | Current subscription quota state and reset windows. |
| `GET` | `/api/v1/me/groups` | Access groups assigned to the current user. |
| `GET` | `/api/v1/me/help/setup` | Provider setup helper with proxy base URLs and available models. |
| `GET` | `/api/v1/me/dashboard/tokens` | Token totals for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/messages` | Message count for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/duration` | Total proxied request duration for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/activity` | Daily usage buckets for a dashboard window. |
| `GET` | `/api/v1/me/dashboard/top-models` | Model usage breakdown. |
| `GET` | `/api/v1/me/dashboard/top-provider-names` | Provider-name usage breakdown. |
| `GET` | `/api/v1/me/dashboard/top-provider-types` | Provider-type usage breakdown. |
| `GET` | `/api/v1/monitoring/status` | Current user-visible monitoring status and degraded service names. |
| `GET` | `/api/v1/faq` | Published frequently asked questions. |

Common list-style routes use query parameters such as `page`, `pageSize`,
`search`, `sortBy`, and `sortDir` where supported. Dashboard routes use the
usage windows implemented by the proxy domain: `7d`, `30d`, and `all`.
When `PROMPTGATE_USAGE_COST_ENABLED=true`, usage summary, dashboard token, and
dashboard activity responses include an optional `estimatedCost` object. Its
`inputUsd`, `outputUsd`, `embeddingUsd`, and `totalUsd` values are indicative
estimates based on configured USD-per-1M-token rates, not billing records. The
same object includes a `rates` field with the rates used for the calculation.

## Token Routes

Prompt Gate API tokens are created from the browser API and used as bearer
tokens against the proxy.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/tokens` | Create a token for the current user. The raw token is returned only once. |
| `GET` | `/api/v1/tokens` | List current-user tokens. |
| `DELETE` | `/api/v1/tokens/{id}` | Revoke one current-user token. |

Token names must be lowercase alphanumeric with dashes or underscores and have
a maximum length of 64 characters. Requested token TTL must be between 1 and
365 days. If omitted, the default TTL is 7 days for users and 30 days for
managers or admins.

## Admin Routes

All `/api/v1/admin/**` routes accept either a browser session with role `admin`
or, when `PROMPTGATE_ADMIN_API_KEY` is configured, exactly one
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

Load the value from a secret manager into the environment rather than putting
the secret itself in a command or script:

```sh
curl --fail-with-body \
  --header "X-Admin-API-Key: ${PROMPTGATE_ADMIN_API_KEY:?not set}" \
  https://promptgate.example.com/api/v1/admin/providers
```

### Dashboard

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/dashboard/tokens` | Global token totals for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/messages` | Global message count for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/duration` | Global total proxied request duration for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/activity` | Global daily usage buckets for a dashboard window. |
| `GET` | `/api/v1/admin/dashboard/top-models` | Global model usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/top-provider-names` | Global provider-name usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/top-provider-types` | Global provider-type usage breakdown. |
| `GET` | `/api/v1/admin/dashboard/adoption` | Active users, active service accounts, and currently valid virtual keys. |
| `GET` | `/api/v1/admin/dashboard/top-identities` | Top users and service accounts by token volume. |

Admin dashboard token and activity responses follow the same optional
`estimatedCost` shape as current-user dashboard responses.

### Users

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/users` | List users with pagination, filters, usage totals, and sorting. |
| `GET` | `/api/v1/admin/users/{id}` | Get one user. |
| `PATCH` | `/api/v1/admin/users/{id}` | Update role, active state, firewall override, or access expiration. |
| `PATCH` | `/api/v1/admin/users/{id}/note` | Update the private administration note. |
| `DELETE` | `/api/v1/admin/users/{id}` | Delete one user and its scoped firewall rules. |
| `GET` | `/api/v1/admin/users/{id}/tokens` | List a user's tokens. |
| `DELETE` | `/api/v1/admin/users/{id}/tokens/{tokenId}` | Revoke one user token. |
| `GET` | `/api/v1/admin/users/{id}/groups` | List the user's access groups. |
| `PUT` | `/api/v1/admin/users/{id}/groups` | Replace the user's access groups. |
| `PUT` | `/api/v1/admin/users/{id}/subscription-plan` | Assign or clear the user's subscription plan. |
| `GET` | `/api/v1/admin/users/{id}/firewall/rules` | List scoped user rules. |
| `POST` | `/api/v1/admin/users/{id}/firewall/rules` | Create a scoped user rule. |
| `GET` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Get a scoped user rule. |
| `PATCH` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Update a scoped user rule. |
| `PATCH` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}/priority` | Move a scoped user rule up or down. |
| `POST` | `/api/v1/admin/users/{id}/firewall/simulate` | Simulate a scoped user decision. |
| `DELETE` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Delete a scoped user rule. |
| `GET` | `/api/v1/admin/prompts` | List prompt history across users. |

The first synced OIDC user is assigned role `admin`. Later users are created
with role `none` until an admin grants access.

### Service Accounts

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/service-accounts` | List service accounts. |
| `POST` | `/api/v1/admin/service-accounts` | Create a service account. |
| `GET` | `/api/v1/admin/service-accounts/{id}` | Get one service account. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}` | Update identifier, name, active state, or firewall override. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}/note` | Update the private administration note. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}` | Delete one service account and its scoped firewall rules. |
| `GET` | `/api/v1/admin/service-accounts/{id}/tokens` | List service-account tokens. |
| `POST` | `/api/v1/admin/service-accounts/{id}/tokens` | Create a service-account token. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}/tokens/{tokenId}` | Revoke a service-account token. |
| `PUT` | `/api/v1/admin/service-accounts/{id}/subscription-plan` | Assign or clear the service account's subscription plan. |

Service account identifiers must be lowercase alphanumeric with dashes or
underscores and have a maximum length of 64 characters.

### Firewall

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/firewall/rules` | List global firewall rules. |
| `POST` | `/api/v1/admin/firewall/rules` | Create a global firewall rule. |
| `GET` | `/api/v1/admin/firewall/rules/{id}` | Get a global firewall rule. |
| `PATCH` | `/api/v1/admin/firewall/rules/{id}` | Update a global firewall rule. |
| `PATCH` | `/api/v1/admin/firewall/rules/{id}/priority` | Move a global firewall rule up or down. |
| `POST` | `/api/v1/admin/firewall/simulate` | Simulate a global firewall decision. |
| `DELETE` | `/api/v1/admin/firewall/rules/{id}` | Delete a global firewall rule. |
| `GET` | `/api/v1/admin/users/{id}/firewall/rules` | List scoped user rules. |
| `POST` | `/api/v1/admin/users/{id}/firewall/rules` | Create a scoped user rule. |
| `GET` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Get a scoped user rule. |
| `PATCH` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Update a scoped user rule. |
| `PATCH` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}/priority` | Move a scoped user rule up or down. |
| `POST` | `/api/v1/admin/users/{id}/firewall/simulate` | Simulate a scoped user decision. |
| `DELETE` | `/api/v1/admin/users/{id}/firewall/rules/{ruleId}` | Delete a scoped user rule. |
| `GET` | `/api/v1/admin/service-accounts/{id}/firewall/rules` | List scoped service-account rules. |
| `POST` | `/api/v1/admin/service-accounts/{id}/firewall/rules` | Create a scoped service-account rule. |
| `GET` | `/api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}` | Get a scoped service-account rule. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}` | Update a scoped service-account rule. |
| `PATCH` | `/api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}/priority` | Move a scoped rule up or down. |
| `POST` | `/api/v1/admin/service-accounts/{id}/firewall/simulate` | Simulate a scoped service-account decision. |
| `DELETE` | `/api/v1/admin/service-accounts/{id}/firewall/rules/{ruleId}` | Delete a scoped service-account rule. |

Firewall rules support `allow` and `deny` actions, priorities from `1` to
`9999`, individual IPv4 addresses, and IPv4 CIDR ranges.

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
| `GET` | `/api/v1/admin/groups` | List access groups. |
| `POST` | `/api/v1/admin/groups` | Create an access group. |
| `POST` | `/api/v1/admin/groups/model-patterns/validate` | Validate model allow and deny patterns. |
| `GET` | `/api/v1/admin/groups/{id}` | Get one access group. |
| `PATCH` | `/api/v1/admin/groups/{id}` | Update group metadata, providers, and model patterns. |
| `DELETE` | `/api/v1/admin/groups/{id}` | Delete an access group. |
| `PUT` | `/api/v1/admin/groups/{id}/members/{userId}` | Add a group member. |
| `DELETE` | `/api/v1/admin/groups/{id}/members/{userId}` | Remove a group member. |

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

### MCP Servers

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/admin/mcp/servers` | List MCP server definitions. |
| `POST` | `/api/v1/admin/mcp/servers` | Create an MCP server. |
| `GET` | `/api/v1/admin/mcp/servers/{id}` | Get one MCP server. |
| `PATCH` | `/api/v1/admin/mcp/servers/{id}` | Update MCP metadata, URL, headers, filters, or enabled state. |
| `DELETE` | `/api/v1/admin/mcp/servers/{id}` | Delete an MCP server. |

Sensitive MCP headers are encrypted before storage. `allowPattern` and
`denyPattern` are validated as regular expressions before they are saved.

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

## Error Shape

Middleware failures use a simple JSON error shape:

```json
{"error":"error_code"}
```

Examples include `missing_auth_credentials`, `invalid_token`,
`invalid_admin_api_key`, `account_inactive`, `account_role_none`,
`insufficient_role`, and `firewall_denied`.
