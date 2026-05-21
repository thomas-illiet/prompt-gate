# Auth Package

This package owns OIDC login, browser sessions, user identity types, request
context helpers, access-token validation for OIDC, and actor injection for the
proxy bridge.

## OIDC Login Flow

```mermaid
flowchart TD
    A["GET /auth/login"] --> B["OIDCService.AuthorizationURL"]
    B --> C["SessionStore.CreateAuthorizationRequest"]
    C --> D["Generate state, nonce, PKCE verifier"]
    D --> E["Store auth request in Redis or memory"]
    E --> F["Redirect browser to OIDC provider"]

    F --> G["Provider authenticates user"]
    G --> H["GET /auth/callback?state&code"]
    H --> I["OIDCService.ExchangeCode"]
    I --> J{"Authorization request found?"}

    J -->|No| K["Redirect to frontend login error"]
    J -->|Yes| L["Exchange code with PKCE verifier"]
    L --> M["Verify id_token and nonce"]
    M --> N["Validate access token with JWKS"]
    N --> O["Map claims to auth.Identity"]
    O --> P["Sync or create app user"]
    P --> Q["SessionStore.CreateSession"]
    Q --> R["Set HTTP-only session cookie"]
    R --> S["Redirect to frontend path"]
```

## Protected Request Flow

```mermaid
flowchart TD
    A["Incoming API request"] --> B["RequireSession / sessionFromRequest"]
    B --> C{"Session cookie present?"}
    C -->|No| D["401 not authenticated"]
    C -->|Yes| E["SessionStore.Session"]

    E --> F{"Session valid and not expired?"}
    F -->|No| G["Delete/clear session and reject"]
    F -->|Yes| H["Refresh user profile with UserResolver"]

    H --> I{"User still exists?"}
    I -->|No| G
    I -->|Yes| J["Inject auth.UserProfile into context"]

    J --> K["RequireAppAccess / RequireRoles"]
    K --> L{"Access allowed?"}
    L -->|No| M["403 access error"]
    L -->|Yes| N["Route handler"]
```

## Proxy Actor Flow

```mermaid
flowchart TD
    A["Authenticated proxy request"] --> B["auth.UserProfile in context"]
    B --> C["auth.ActorMiddleware"]
    C --> D{"UserProfile present?"}
    D -->|No| E["500 missing_authenticated_user"]
    D -->|Yes| F["Build AIBridge actor metadata"]
    F --> G["coderbridge.AsActor"]
    G --> H["Proxy manager / upstream provider"]
```

## Runtime Behavior

- `OIDCService` creates PKCE-protected authorization requests and validates the
  callback with state, nonce, ID token verification, and access-token validation.
- `Validator` validates OIDC access tokens against the configured issuer and
  JWKS URL, then maps Keycloak claims into `auth.Identity`.
- `UserSynchronizer` turns an external identity into the local `UserProfile`.
- `SessionStore` stores browser sessions in Redis when configured, otherwise in
  process memory.
- Session lookup refreshes the user profile through `UserResolver`. If the user
  no longer exists, the session is deleted and rejected.
- `ContextWithUser` and `UserFromContext` are the shared contract used by API,
  token, firewall, and proxy middleware.
- `ActorMiddleware` requires an authenticated `UserProfile` in context and
  injects the AIBridge actor used by the proxy bridge.

## Package Layout

- `user.go`: roles, user types, `UserProfile`, OIDC identity claim mapping.
- `oidc.go`: OIDC authorization, callback exchange, redirect and logout URL
  helpers.
- `session_store.go`: auth request and session storage, Redis or memory backed.
- `validator.go`: access-token validation with JWKS.
- `context.go`: request context helpers for identity and user profile.
- `middleware.go`: AIBridge actor middleware.
- `user_sync.go`: interfaces implemented by the users domain.
