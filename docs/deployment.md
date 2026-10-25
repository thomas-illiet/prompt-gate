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
- Redis for sessions, auth and OPA decision caches, group/provider/subscription
  snapshots, rolling quota state and reset barriers, config reload events, and
  usage event streams
- an independently operated OPA server for every proxy decision
- Keycloak or another OIDC-compatible provider
- one protected YAML file containing the common and per-runtime settings

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

Model Lab is always enabled for `user` and `admin` roles. Configure the same
browser-visible HTTP(S) URL in `common.public_urls.proxy`. If
that URL contains a routing prefix such as the
Helm chart's `/remote`, the edge must strip it before forwarding to the proxy.
Control exposure through roles and access groups. Verify that tickets are one-time,
three-stream limits hold, quota/accounting are exact, and a unique canary prompt
is absent from logs, Redis event payloads, PostgreSQL content, OTel spans,
browser storage, URLs, and error responses before broadening access. Rolling
back the release invalidates no durable Model Lab state; outstanding tickets
expire after their configured short TTL.

When upgrading from the one-turn playground to multi-turn chat, deploy the API
and proxy before the frontend. The expanded validator remains compatible with
old one-message requests, while the new frontend requires the backend to accept
alternating conversation history.

The user pre-creation migration makes the OIDC subject and last-login columns
nullable. During an upgrade to the first version that supports pre-creation,
run the migration, replace every API replica, and only then deploy or use the
frontend action and `POST /api/v1/admin/users`. Do not create an active limited,
SSO-unlinked user while older API replicas remain in service, because those binaries
still expect the two columns to be non-null when reading administration responses.

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
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
  /app/promptgate --config /etc/prompt-gate/promptgate.yaml migrate
```

Run the API:

```sh
docker run --rm \
  -p 8080:8080 \
  -v /secure/path/promptgate.yaml:/etc/prompt-gate/promptgate.yaml:ro \
  -v /path/to/custom-ca.pem:/run/secrets/custom-ca.pem:ro \
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
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
  ghcr.io/thomas-illiet/prompt-gate:v0.1.0 \
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
- Prefer `proxy.trusted_proxies` with explicit ingress or
  reverse-proxy CIDRs when the proxy needs the real client IP.
- Enable `proxy.trust_forward_headers` only when the proxy is behind
  trusted infrastructure that sanitizes forwarded headers.

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
`/api/v1/*` to the API service, `/bridge/*` to the proxy service with the
`/bridge` prefix stripped, and `/` to the frontend service. Keep the rules that
need rewrites in separate Ingress objects because nginx applies
`rewrite-target` to every path in the same object.

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
  annotations:
    nginx.ingress.kubernetes.io/use-regex: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /$2
spec:
  ingressClassName: nginx
  rules:
    - host: promptgate.example.com
      http:
        paths:
          - path: /bridge(/|$)(.*)
            pathType: ImplementationSpecific
            backend:
              service:
                name: promptgate-proxy
                port:
                  number: 8081
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

For HAProxy Ingress, ensure the controller overwrites or sanitizes inbound
`X-Forwarded-For` before forwarding the client address. Temporarily enable
`proxy.client_ip_debug: true`, send a known external request, and use
the structured `peer_ip` field to determine whether Prompt Gate sees an HAProxy
pod IP, node IP, or SNAT address. Configure only the narrow stable CIDR that
contains that peer in `proxy.trusted_proxies`, keep
`proxy.trust_forward_headers: false`, then disable the diagnostic.
If the trusted CIDR is shared with other workloads, restrict the proxy Service
with a NetworkPolicy selecting the HAProxy namespace and controller pods.

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

## Production Notes

- Use immutable version tags such as `v0.1.0` for production deployments.
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
