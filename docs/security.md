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

The first OIDC user synced into an empty database receives role `admin`.
Subsequent users are created with role `none` until an admin grants access.

Browser sessions are stored in Redis when Redis is configured. Session cookies
are HTTP-only, use `PROMPTGATE_SESSION_COOKIE_NAME`, and inherit secure-cookie
behavior from `PROMPTGATE_BACKEND_BASE_URL`: HTTPS backend URLs produce secure
cookies.

## Roles

Prompt Gate uses these application roles:

| Role | Meaning |
| --- | --- |
| `none` | Authenticated identity exists but has no app access. |
| `user` | Can use user routes, issue own tokens, and use the proxy. |
| `manager` | Has user-level access with manager token defaults. |
| `admin` | Can access all admin routes. |

Protected routes reject inactive users and users with role `none`. Admin routes
require role `admin`.

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
- role is one of `user`, `manager`, or `admin`

Token revocation publishes an `auth` config event. The proxy responds by
moving its Redis auth cache to a new version.

## Service Accounts

Service accounts are stored as users with type `service`. They can receive
Prompt Gate API tokens and can be restricted with scoped firewall rules.

Service account identifiers must be lowercase alphanumeric with dashes or
underscores and have a maximum length of 64 characters.

When `firewallOverrideEnabled` is true, the proxy evaluates only the scoped
firewall rules for that user or service account. No scoped match denies by
default.

## Firewall

Firewall rules are evaluated inside the proxy after token authentication.

Supported rule inputs:

- IPv4 address
- IPv4 CIDR range
- priority from `1` to `9999`
- action `allow` or `deny`
- enabled flag

Global rules use first match wins and allow on no match. Scoped user and
service-account rules use first match wins and deny on no match.

The proxy normally uses the TCP remote address. In production, prefer
`PROMPTGATE_PROXY_TRUSTED_PROXIES` with explicit ingress or reverse-proxy CIDRs
so forwarded headers are accepted only from known peers.
`PROMPTGATE_PROXY_TRUST_FORWARD_HEADERS` should be enabled only behind trusted
infrastructure that strips or rewrites untrusted forwarding headers.

## Secret Storage

`PROMPTGATE_SECRETS_KEY` must be a base64-encoded 32-byte key. It initializes
an AES-256-GCM cipher used for stored downstream secrets:

- provider API keys
- sensitive MCP header values

Encrypted values use a versioned envelope with a random nonce. Keep the key
stable across restarts and deployments. Rotating it requires a deliberate data
migration because existing provider and MCP secrets must be decrypted and
re-encrypted.

## CORS

The API and proxy use `PROMPTGATE_CORS_ALLOWED_ORIGINS`. When no explicit
origins are configured:

- the API defaults to `PROMPTGATE_FRONTEND_BASE_URL`
- the proxy defaults to `PROMPTGATE_FRONTEND_BASE_URL` when present

Loopback origins are expanded across `localhost`, `127.0.0.1`, and `::1` for
local development.

## Operational Checklist

- Store `PROMPTGATE_JWT_SECRET`, `PROMPTGATE_SECRETS_KEY`, database
  credentials, Redis credentials, and OIDC client secrets in a secret manager.
- Use HTTPS origins for production `PROMPTGATE_BACKEND_BASE_URL` and
  `PROMPTGATE_FRONTEND_BASE_URL`.
- Run migrations before serving traffic.
- Keep Redis available for sessions, hot reload, and proxy auth cache
  invalidation.
- Prefer explicit `PROMPTGATE_PROXY_TRUSTED_PROXIES` CIDRs over global
  forwarded-header trust.
- Review service-account firewall overrides before issuing long-lived tokens.
