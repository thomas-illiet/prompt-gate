# YAML Configuration

Prompt Gate reads all runtime configuration from one YAML document. Every
command requires its path explicitly:

```sh
promptgate --config /etc/prompt-gate/promptgate.yaml api
promptgate --config /etc/prompt-gate/promptgate.yaml proxy
promptgate --config /etc/prompt-gate/promptgate.yaml worker
promptgate --config /etc/prompt-gate/promptgate.yaml schedule
promptgate --config /etc/prompt-gate/promptgate.yaml migrate
```

The application does not search for `.env`, read `PROMPTGATE_*` runtime
variables, interpolate `${ENV}` placeholders, or merge multiple sources. The
loader rejects unknown fields, duplicate keys, multiple YAML documents, and
incorrect types. Durations use Go syntax such as `250ms`, `5m`, `1h`, or `8h`.

Copy [`promptgate.example.yaml`](../promptgate.example.yaml) to
`promptgate.yaml` for local source development. The local file is ignored by
Git because it contains credentials.

## Common Settings

| YAML path | Used by | Default | Description |
| --- | --- | --- | --- |
| `common.log.level` | all | `info` | `debug`, `info`, `warn`, `warning`, or `error`; unknown values fall back to `info`. |
| `common.database.url` | all | none | Required PostgreSQL connection URL. |
| `common.database.connection_max_lifetime` | all | `30m` | Maximum lifetime of a pooled PostgreSQL connection. |
| `common.database.max_idle_connections` | all | `5` | Maximum idle PostgreSQL connections. |
| `common.database.max_open_connections` | all | `10` | Maximum open PostgreSQL connections. |
| `common.redis.url` | API, proxy, worker, schedule | none | Required Redis connection URL. |
| `common.redis.cache_ttl` | API, proxy, worker, schedule | `5m` | Shared cache and snapshot TTL. |
| `common.redis.pool_size` | Redis runtimes | driver default | Optional positive pool-size override. |
| `common.redis.min_idle_connections` | Redis runtimes | driver default | Optional positive idle-connection override. |
| `common.redis.pool_timeout` | Redis runtimes | driver default | Optional positive pool wait timeout. |
| `common.redis.read_timeout` | Redis runtimes | driver default | Optional positive read timeout. |
| `common.redis.write_timeout` | Redis runtimes | driver default | Optional positive write timeout. |
| `common.tls.ca_file` | API, proxy, schedule | empty | Single CA bundle for OIDC, monitoring, providers, and OTEL. |
| `common.security.jwt_secret` | API, proxy, schedule | none | Required signing secret of at least 32 characters. |
| `common.security.secrets_key` | API, proxy, schedule | none | Required base64-encoded 32-byte provider-secret and Model Lab ticket key. |
| `common.public_urls.frontend` | API, proxy | none | Browser-visible frontend origin; required by API. |
| `common.public_urls.backend` | API | none | Browser-visible API origin; required by API. |
| `common.public_urls.proxy` | API, proxy | derived for API | Browser-visible proxy URL; required by proxy. |
| `common.session.cookie_name` | API, proxy | `promptgate_session` | Browser session cookie name. |
| `common.session.ttl` | API, proxy | `8h` | Positive browser session lifetime. |
| `common.cors.allowed_origins` | API, proxy | frontend URL | YAML list of browser origins. Loopback aliases are expanded. |
| `common.proxy_reload_debounce` | API, proxy, schedule | `250ms` | Provider reload-event debounce. |

`common.model_lab` contains `ticket_ttl`, `max_prompt_bytes`,
`max_output_tokens`, `max_targets`, `max_concurrent`, `catalog_fresh_ttl`, and
`catalog_stale_ttl`. The existing limits remain: tickets live at most `5m`,
targets and concurrency are between 1 and 3, and stale catalog retention cannot
be shorter than fresh retention.

`common.usage_cost` contains the non-negative fallback USD rates `input`,
`output`, and `embedding`, each expressed per one million tokens. Cost
estimation is always enabled; there is no `enabled` setting. Database-backed
per-model pricing takes precedence over these fallback rates.

`common.maintenance` contains `token_cleanup_interval`,
`user_access_expiration_interval`, and `quota_reset_distribution_interval`.
All default to `1h` and must be positive.

## API Settings

| YAML path | Default | Description |
| --- | --- | --- |
| `api.port` | `8080` | API listen port. |
| `api.static_assets_dir` | empty | Optional frontend asset directory. |
| `api.admin_api_key` | empty | Optional global credential for `/api/v1/admin/**`. |
| `api.admin_contact_email` | empty | Optional administrator contact address shown in the limited-access banner. |
| `api.oidc.issuer_url` | none | Required OIDC issuer URL. |
| `api.oidc.jwks_url` | none | Required JWKS URL. |
| `api.oidc.client_id` | none | Required OIDC client ID. |
| `api.oidc.client_secret` | empty | Optional confidential-client secret. |

When `api.admin_contact_email` is set, the API exposes it only in the
authenticated session response. The frontend turns it into the contact link in
the persistent banner shown to active `limited` users. An empty value keeps the
generic administrator message. Restart every API replica after changing this
setting.

## Proxy Settings

The `proxy` section contains `port`, the optional `base_path`,
`trust_forward_headers`, the YAML CIDR list `trusted_proxies`,
`client_ip_debug`, `debug_requests`, request and response buffer limits, and
`upstream_timeout`. `base_path` is empty by default. When set, it must be a
normalized absolute non-root path such as `/remote`; the proxy requires and
removes it before applying Model Lab, authentication, policy, quota, and
provider routing. The direct `/health` endpoint remains available to probes,
and `<base_path>/health` exposes the same check through an Ingress. The path in
`common.public_urls.proxy` must match `proxy.base_path` so generated SDK and
Model Lab URLs reach the mounted proxy.

`proxy.opa` contains `enabled`, `url`, `policy_path`, `timeout`, and
`cache_ttl`. The URL is required and must be absolute HTTP(S) when OPA is
enabled. `cache_ttl` applies only to cached denials; allow decisions are always
re-evaluated.

`proxy.otel` contains `enabled`, `endpoint`, `api_key`, `project_name`,
`service_name`, `environment`, the three `capture_*` consent switches,
`export_timeout`, `batch_timeout`, queue and batch sizes, and `insecure`.
There is deliberately no OTEL-specific CA path: the exporter uses
`common.tls.ca_file`.

## Worker And Scheduler Settings

`worker` contains `batch_size`, `block_timeout`, `pending_idle_timeout`, and
the optional `consumer_name`.

`schedule` contains `usage_raw_retention`, `usage_raw_cleanup_interval`, and
`subscription_quota_sync_interval`. All durations must be positive. The
`migrate` section is currently empty; migrations consume common logging and
database settings.

## Relative Paths And Secrets

Relative values for `common.tls.ca_file` and `api.static_assets_dir` resolve
against the directory containing the selected YAML file, not the process
working directory.

The document contains database credentials, signing and encryption keys, the
OIDC client secret, administration key, and optional OTEL key. Store and mount
the complete file as a secret, make it readable only by the Prompt Gate
process, and never print or commit it. Configuration is read once at startup;
restart every affected process after changing it.

## Per-Command Requirements

- `api`: database, Redis, JWT and secrets keys, public frontend/backend URLs,
  and the three required OIDC fields.
- `proxy`: database, Redis, JWT and secrets keys, and a public proxy URL; OPA
  additionally requires its URL when enabled.
- `schedule`: database, Redis, JWT and secrets keys.
- `worker`: database and Redis.
- `migrate`: database only.

Integration-test PostgreSQL and Redis endpoints are also file-backed in
[`testdata/promptgate.integration.yaml`](../testdata/promptgate.integration.yaml).
The test harness does not read configuration from environment variables.
