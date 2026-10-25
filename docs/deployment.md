# Deployment

Prompt Gate Backend is deployed as a Docker image published to GitHub Container
Registry:

```text
ghcr.io/thomas-illiet/prompt-gate
```

The same image contains the backend binary and the generated Nuxt static
frontend assets.

## Process Layout

The image exposes:

- `8080` for the API server
- `8081` for the LLM proxy server

The default command expects the shared configuration at
`/etc/prompt-gate/promptgate.yaml` and starts the API:

```text
/app/promptgate --config /etc/prompt-gate/promptgate.yaml api
```

Use explicit commands for the other processes:

```text
/app/promptgate --config /etc/prompt-gate/promptgate.yaml proxy
/app/promptgate --config /etc/prompt-gate/promptgate.yaml migrate
/app/promptgate --config /etc/prompt-gate/promptgate.yaml schedule
/app/promptgate --config /etc/prompt-gate/promptgate.yaml worker
```

Recommended production layout:

| Process | Replica guidance | Notes |
| --- | --- | --- |
| `migrate` | One-off job | Run before API, proxy, worker, or scheduler for each release. |
| `api` | One or more replicas | Serves browser/API traffic and optionally static frontend assets. |
| `proxy` | One or more replicas | Serves LLM traffic, subscribes to Redis reload events, and enqueues usage events. |
| `worker` | One or more replicas | Consumes Redis usage events and updates raw usage plus dashboard KPI tables. |
| `schedule` | One replica | The current scheduler has no distributed lock. |

## Runtime Dependencies

Prompt Gate requires:

- PostgreSQL for durable data
- Redis for sessions, auth and OPA denial caches, group/provider/subscription
  snapshots, Model Lab catalogs, one-time tickets and concurrency leases,
  rolling quota state and reset barriers, config reload events, and usage event
  streams
- an independently operated OPA server for every proxy decision
- Keycloak or another OIDC-compatible provider
- one protected YAML file containing the common and per-runtime settings

Allow API replicas to reach each configured provider's model-catalog endpoint;
Model Lab performs discovery from the API process. Proxy replicas separately
need provider egress for generation traffic. Apply `common.tls.ca_file` to both
paths when those endpoints use a private certificate authority.

See [YAML configuration](environment.md) for the full configuration
reference.

## Deployment Order

1. Provision PostgreSQL, Redis, and OPA.
2. Configure the OIDC client and callback URL:
   `https://api.example.com/auth/callback`.
3. Create `promptgate.yaml`, including database, Redis, signing, encryption,
   OIDC, administration, and telemetry credentials. Protect the entire file as
   a secret because every Prompt Gate process receives it.
4. Run migrations with `promptgate --config /path/promptgate.yaml migrate`.
5. Start the API, proxy, workers, and scheduler with the same required
   `--config` option.
6. Configure providers, users, service accounts, tokens, and the external OPA
   policy through the admin API or frontend.

Model Lab is always enabled for `user` and `admin` roles. Configure its
browser-visible HTTP(S) proxy URL in `common.public_urls.proxy`. If that URL
contains a routing prefix such as the Helm chart's `/remote`, configure the
same value in `proxy.base_path`; the edge must forward the original path
without rewriting it.
Control exposure through roles and access groups. Verify that tickets are
one-time, the configured target and concurrent-stream limits hold, quota and
accounting are exact, and a unique canary prompt is absent from logs, Redis
event payloads, PostgreSQL content, OTel spans, browser storage, URLs, and error
responses before broadening access. Both `common.model_lab.max_targets` and
`common.model_lab.max_concurrent` are configurable from 1 to 3. Rolling back
the release invalidates no durable Model Lab state; outstanding tickets expire
after their configured short TTL.

Historical compatibility note: when upgrading from a release with the original
one-turn playground to the first multi-turn release, deploy the API and proxy
before the frontend. The expanded validator accepts the older one-message
requests, while the multi-turn frontend requires alternating conversation
history support.

Historical compatibility note: during the first upgrade that introduced user
pre-creation, the migration made OIDC subject and last-login fields nullable.
That transition required migrations and every API replica to be replaced before
using the frontend action or `POST /api/v1/admin/users`; older API binaries
expected both fields to be non-null.

The scheduler runs monthly reset-credit distribution and pending-reset
reconciliation immediately on startup. Always complete migrations before
starting the scheduler, and keep it at one replica unless the deployment adds
external job locking.

## Container Examples

Mount the complete configuration read-only. The same mount works for every
runtime; only the final command name changes:

```sh
docker run --rm \
  -v /secure/path/promptgate.yaml:/etc/prompt-gate/promptgate.yaml:ro \
  ghcr.io/thomas-illiet/prompt-gate:v2.1.0 \
  /app/promptgate --config /etc/prompt-gate/promptgate.yaml migrate
```

Run the API:

```sh
docker run --rm \
  -p 8080:8080 \
  -v /secure/path/promptgate.yaml:/etc/prompt-gate/promptgate.yaml:ro \
  -v /path/to/custom-ca.pem:/run/secrets/custom-ca.pem:ro \
  ghcr.io/thomas-illiet/prompt-gate:v2.1.0 \
  /app/promptgate --config /etc/prompt-gate/promptgate.yaml api
```

Set `common.tls.ca_file: /run/secrets/custom-ca.pem` when mounting the optional
CA bundle. This single CA setting applies to OIDC, monitoring, providers, and
OTEL.

## Administration API Key

Set `api.admin_api_key` in the protected YAML. The API is the only runtime that
consumes it, although all runtimes can read the shared configuration file. OIDC
configuration remains required even when administration-key access is enabled.

The credential is global and grants the full `/api/v1/admin/**` surface,
including destructive operations, without per-operator attribution. Store it
in a secret manager, expose the API over HTTPS, and use the header only from
trusted CLI or server workloads. Do not inject it into frontend builds.

The API reads the value at startup. To rotate it, update the backing secret and
restart every API replica or trigger a Deployment rollout. There is no period
where one API process accepts both the old and new keys, so coordinate the
client cutover with the restart.

Run the proxy, worker, or scheduler by changing only the last argument:

```sh
docker run --rm \
  -p 8081:8081 \
  -v /secure/path/promptgate.yaml:/etc/prompt-gate/promptgate.yaml:ro \
  ghcr.io/thomas-illiet/prompt-gate:v2.1.0 \
  /app/promptgate --config /etc/prompt-gate/promptgate.yaml proxy
```

## Static Frontend Assets

The Dockerfile builds the Nuxt frontend and copies generated assets to:

```text
/app/public
```

Configure the bundled directory in the API section:

```yaml
api:
  static_assets_dir: /app/public
```

When set, the API process serves frontend files from `/` and falls back to the
SPA shell for frontend routes.

## Reverse Proxy Notes

- Route browser/API traffic to the API process.
- Route LLM SDK traffic to the proxy process.
- Set `common.public_urls.backend` to the browser-visible API origin.
- Set `common.public_urls.frontend` to the browser-visible frontend origin.
- Set `common.public_urls.proxy` when the proxy is exposed on a different
  origin, path, or externally mapped port.
- Set `proxy.base_path` to the public proxy path when the proxy shares an
  origin with the API. The proxy removes it internally before authorization
  and provider routing.
- Prefer `proxy.trusted_proxies` with explicit ingress or
  reverse-proxy CIDRs when the proxy needs the real client IP.
- Keep `proxy.trust_forward_headers` disabled when using explicit trusted CIDRs;
  the global switch is only for an entirely trusted network boundary that
  sanitizes every forwarded header.

## Kubernetes Helm And HAProxy Technologies Ingress

The maintained Helm chart targets the HAProxy Technologies Kubernetes Ingress
Controller. It renders one `networking.k8s.io/v1` Ingress with one hostname and
ordinary `Prefix` routes:

- `/remote` routes to the proxy Service
- `/` routes to the API Service and bundled frontend

The edge does not use a regular expression or rewrite the URL. Prompt Gate
derives `proxy.base_path` from `ingress.proxyPath`, validates that the public
proxy URL has the same path, and removes the prefix inside the proxy before
authentication, OPA, quota, or provider routing.

```yaml
ingress:
  enabled: true
  className: haproxy
  host: promptgate.example.com
  proxyPath: /remote
  tls:
    enabled: true
    secretName: promptgate-tls
  annotations:
    haproxy.org/forwarded-for: "true"

config:
  proxy:
    trust_forward_headers: false
    trusted_proxies:
      - "YOUR_HAPROXY_PEER_CIDR"
```

The OIDC callback defaults to
`<common.public_urls.backend>/auth/callback`. Set an exact callback only when
the provider-facing URL uses a different origin or an externally rewritten
path:

```yaml
config:
  api:
    oidc:
      redirect_url: https://promptgate.example.com/auth/callback
```

Keep `redirect_url` empty to use the derived callback. The explicit value must
exactly match a redirect URI registered with the OIDC provider. It changes only
the OIDC callback; the frontend return URL and secure-cookie behavior still
follow `common.public_urls.frontend` and `common.public_urls.backend`.

When TLS terminates at an upstream load balancer but the chart-managed Ingress
receives plain HTTP, set the complete browser-visible URL configuration to
HTTPS even though Ingress TLS is disabled:

```yaml
config:
  common:
    public_urls:
      frontend: https://promptgate.example.com
      backend: https://promptgate.example.com
      proxy: https://promptgate.example.com/remote
  api:
    oidc:
      redirect_url: https://promptgate.example.com/auth/callback
ingress:
  tls:
    enabled: false
```

The explicit `redirect_url` is optional in that example because it matches the
derived callback, but keeping it visible makes the identity-provider contract
unambiguous.

Replace the placeholder only after observing the actual direct peer. The chart
does not guess trusted networks. When `configSecret.existingSecret` is used,
the external runtime file must set `proxy.base_path: /remote` and
`common.public_urls.proxy: https://promptgate.example.com/remote` itself. Any
custom `api.oidc.redirect_url` must also be set in that external file.

For F5 -> HAProxy Ingress, require the F5 to overwrite or sanitize inbound
`X-Forwarded-For`. HAProxy appends the TCP source it actually observes as the
last header occurrence; this can be the F5 or a controlled node/SNAT hop when
the Kubernetes Service masks the original source. Configure
`proxy.trusted_proxies` with the narrow direct HAProxy peer range and every
controlled upstream hop that actually appears in the chain. Prompt Gate walks
the chain from right to left to select the nearest untrusted client address.
Do not set `haproxy.org/src-ip-header` or enable the legacy global
`proxy.trust_forward_headers` for this topology.

Temporarily enable `proxy.client_ip_debug: true`, send a known external request,
and use the structured `peer_ip` field to determine whether Prompt Gate sees an
HAProxy pod IP, node IP, or SNAT address. Configure only the narrow stable CIDR
that contains that peer, then disable the diagnostic. If that CIDR is shared
with other workloads, restrict the proxy Service with a NetworkPolicy selecting
the HAProxy namespace and controller pods. `externalTrafficPolicy: Local` is an
optional source-preservation choice only when the F5/operator topology supports
it; it does not replace the sanitized forwarding-header contract.

Chart `0.2.x` moves `/remote` removal from HAProxy into Prompt Gate. Upgrade
from `0.1.x` during a short F5 maintenance window after the selected `v2.1.x`
image exists: close external traffic, perform a full Argo CD sync, wait for the
proxy rollout and unified Ingress to become healthy, validate
`/remote/health` and a provider request, then reopen traffic.

## Kubernetes NGINX Ingress

When the API image serves the generated static frontend, expose it at the host
root without a rewrite:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-api
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - promptgate.example.com
      secretName: promptgate-tls
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: promptgate-api
                port:
                  number: 8080
```

Use matching public URLs:

```yaml
common:
  public_urls:
    frontend: https://promptgate.example.com
    backend: https://promptgate.example.com
  cors:
    allowed_origins:
      - https://promptgate.example.com
```

Register the OIDC redirect URI as:

```text
https://promptgate.example.com/auth/callback
```

For a separate frontend service behind the same host, route `/auth/*` and
`/api/v1/*` to the API service, `/bridge` to the proxy service, and `/` to the
frontend service. Configure `proxy.base_path: /bridge`; no Ingress rewrite is
required.

If `common.public_urls.backend` includes `/api`, add a compatibility rule for
the OIDC callback:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-api-auth-compat
  annotations:
    nginx.ingress.kubernetes.io/use-regex: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /auth/$2
spec:
  ingressClassName: nginx
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /api/auth(/|$)(.*)
            pathType: ImplementationSpecific
            backend:
              service:
                name: promptgate-api
                port:
                  number: 8080
```

For the LLM proxy route:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: promptgate-proxy
spec:
  ingressClassName: nginx
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /bridge
            pathType: Prefix
            backend:
              service:
                name: promptgate-proxy
                port:
                  number: 8081
```

The matching runtime configuration is:

```yaml
common:
  public_urls:
    proxy: https://promptgate.example.com/bridge
proxy:
  base_path: /bridge
```

To preserve the real client IP with ingress-nginx, make sure the controller
actually receives and forwards it before enabling Prompt Gate trust. Depending
on your load balancer, this can require `externalTrafficPolicy: Local` on the
ingress controller Service, `use-forwarded-headers`, `enable-real-ip`, and a
correct `proxy-real-ip-cidr` in the ingress-nginx ConfigMap. Use the real CIDRs
for your load balancer, ingress nodes, or upstream proxies. See the
[ingress-nginx ConfigMap reference](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/),
[source IP notes](https://kubernetes.github.io/ingress-nginx/user-guide/miscellaneous/#source-ip-address),
and [externalTrafficPolicy FAQ](https://kubernetes.github.io/ingress-nginx/faq/#client-ipaddress-on-single-node-cluster).

## Health Checks

API:

```text
GET /health
```

Returns `200 OK` when the API can reach the database and `503 Service
Unavailable` when the database check fails.

Proxy:

```text
GET /health
```

Returns `200 OK` with `{"status":"ok"}` when the proxy process is serving.
When `proxy.base_path` is `/remote`, `GET /remote/health` reaches the same
handler through the public Ingress while the direct path remains available to
Kubernetes probes.

## Production Notes

- Use immutable version tags such as `v2.1.0` for production deployments.
- Avoid deploying `latest` to production unless the platform also records the
  resolved image digest.
- Store all secrets in a secret manager.
- Keep `common.security.secrets_key` stable across deployments unless you are
  intentionally rotating encrypted provider secrets.
- Run exactly one scheduler replica unless you add external job locking.
- Ensure at least one supported enabled provider exists before starting proxy
  replicas.

Related docs:

- [Architecture](architecture.md)
- [Proxy runtime](proxy.md)
- [Scheduler](scheduler.md)
- [Security model](security.md)
- [Release process](release.md)
