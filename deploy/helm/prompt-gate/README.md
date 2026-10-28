# Prompt Gate Helm Chart

This chart is designed to be rendered by Argo CD and deploys only the Prompt
Gate application components. PostgreSQL, Redis, and OIDC/Keycloak must already
exist outside the chart.

Set `config.common.environment` to `development`, `staging`, or `production`
for every release. Prompt Gate displays it on the login page and in the
authenticated shell. The chart default is `production`; override it explicitly
for development and staging releases.

## Argo CD Application Example

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: prompt-gate
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/thomas-illiet/prompt-gate
    targetRevision: main
    path: deploy/helm/prompt-gate
    helm:
      releaseName: prompt-gate
      valueFiles:
        - values.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: prompt-gate
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

## HAProxy Technologies Ingress

The chart targets the HAProxy Technologies Kubernetes Ingress Controller by
default. It renders one Ingress, one hostname, and two ordinary `Prefix`
routes: `/remote` goes to the proxy Service and `/` goes to the API/frontend
Service. Kubernetes selects the longest matching prefix. There is no regex or
HAProxy URL rewrite; Prompt Gate removes the configured proxy base path before
authentication, policy, quota, and provider routing.

```yaml
ingress:
  className: haproxy
  host: promptgate.example.com
  proxyPath: /remote
  annotations:
    haproxy.org/forwarded-for: "true"
```

By default, the OIDC callback URL is derived from
`config.common.public_urls.backend` by appending `/auth/callback`. Set an exact
callback URL only when the provider-facing callback uses a different origin or
an externally rewritten path:

```yaml
config:
  api:
    oidc:
      redirect_url: https://promptgate.example.com/auth/callback
```

Keep the value empty to use the derived callback. The explicit URL must exactly
match a redirect URI registered with the OIDC provider. It changes only the
OIDC `redirect_uri`; public frontend/backend URLs and session-cookie security
remain controlled by `config.common.public_urls`.

When TLS terminates at an upstream load balancer, configure all public URLs
with their browser-visible HTTPS values even if the downstream Ingress uses
plain HTTP:

```yaml
config:
  common:
    public_urls:
      frontend: https://promptgate.example.com
      backend: https://promptgate.example.com
      proxy: https://promptgate.example.com/remote
ingress:
  tls:
    enabled: false
```

For a chart-managed configuration Secret, the chart sets
`config.proxy.base_path` from `ingress.proxyPath`. If `existingSecret` is used,
the YAML stored in that Secret must set the same base path. `/health` remains
available directly to Kubernetes probes, while `/remote/health` reaches the
same proxy health handler through the Ingress.

An explicit `config.common.public_urls.proxy` may use a different scheme or
hostname, but its path must equal `ingress.proxyPath`. When the chart-managed
proxy Ingress is disabled, set `config.common.public_urls.proxy` explicitly and
make its path match `config.proxy.base_path`; the chart no longer derives a
misleading URL from the example Ingress hostname.

The chart does not guess trusted-proxy networks. Determine the actual HAProxy
controller or node CIDR and every trusted upstream proxy CIDR before setting
`config.proxy.trusted_proxies`.

## Runtime Configuration Secret

Prompt Gate reads one structured `promptgate.yaml` file. The document contains
both ordinary settings and credentials, so the chart stores the complete file
in a Secret and mounts it read-only in every Prompt Gate workload.

For a chart-managed Secret, configure the nested `config.common`, `config.api`,
`config.proxy`, `config.worker`, and `config.schedule` values. In production,
prefer an encrypted values source or provide an externally managed Secret:

```bash
kubectl create secret generic prompt-gate-config \
  --from-file=promptgate.yaml=./promptgate.yaml \
  --namespace prompt-gate
```

```yaml
configSecret:
  existingSecret: prompt-gate-config
  key: promptgate.yaml
  rolloutToken: "2026-10-rotation-1"
```

When `existingSecret` is set, the chart does not render configuration content.
Keep `config.api.port` and `config.proxy.port` equal to the corresponding values
in the external file because Kubernetes probes use those container ports. When
Ingress and proxy are enabled, also set `proxy.base_path` in the external file
to the exact `ingress.proxyPath` value.
Change `rolloutToken` after replacing the Secret so all workloads restart.
Because the selected design uses one complete file, every Prompt Gate process
can read every secret in it; restrict Secret and values-file access accordingly.

## Limited-Access Administrator Contact

Set `config.api.admin_contact_email` to show a clickable administrator address
in the persistent banner seen by authenticated `limited` users:

```yaml
config:
  api:
    admin_contact_email: admin@example.com
```

The value is optional and is exposed in the authenticated session response, so
it is contact information rather than a secret. An empty value keeps the
generic contact message. Restart or roll out all API replicas after changing
it.

## Quota Reset Scheduling

The scheduler distributes monthly reset credits immediately on startup and then
at `config.common.maintenance.quota_reset_distribution_interval`, which defaults to `1h`. Pending
reset reconciliation and the PostgreSQL quota projection run immediately and
then at `config.schedule.subscription_quota_sync_interval`, which defaults to `5m`;
reconciliation always runs before projection.

Keep `schedule.replicas: 1` unless the deployment adds an external distributed
lock. Complete the release migrations before the scheduler starts because these
jobs use the subscription quota projection, wallet, and ledger tables. With
Argo CD, keep the migration Job enabled and ordered ahead of the workloads. With
direct Helm installs, or when `migrate.enabled=false`, run an external migration
process before starting or upgrading the scheduler.

## Administration API Key

The optional administration API key provides direct access to every
`/api/v1/admin/**` endpoint through the `X-Admin-API-Key` header. These routes
include destructive operations, so use the key only over HTTPS and treat it as
a privileged production credential. It is global: requests made with it do not
carry an individual administrator identity for attribution.

The feature is disabled by default. Configure it inside the shared runtime
document:

```yaml
config:
  api:
    admin_api_key: "replace-with-a-secret-value"
```

An empty or whitespace-only value leaves the feature disabled. The API is the
only runtime that consumes the value, although all workloads can read the
shared file. Do not commit a production administration key to a plain values
file.

Example request:

```bash
curl --fail-with-body \
  -H "X-Admin-API-Key: ${ADMIN_API_KEY:?not set}" \
  https://promptgate.example.com/api/v1/admin/users
```

## Phoenix API Key

Enable OTLP export under `config.proxy.otel`. The API key is stored in the same
runtime configuration Secret:

```yaml
config:
  proxy:
    otel:
      enabled: true
      endpoint: https://phoenix.example.com/v1/traces
      api_key: "replace-with-a-secret-value"
      project_name: prompt-gate
      environment: production
      capture_prompts: true
      capture_output: true
      capture_thinking: true
```

Never commit a production Phoenix key. Prompt capture exports raw, unredacted
content and remains disabled independently of trace export by default. The
single `config.common.tls.ca_file` bundle also applies to OTEL.

## Image Pull Secrets

When the image registry is private, create the pull secret in the destination
namespace and reference it in values:

```yaml
imagePullSecrets:
  - name: ghcr-pull-secret
```

The chart attaches the same pull secret to the Pod specs and to the generated
ServiceAccount.

## Real Client IP

When Prompt Gate runs behind an ingress controller such as HAProxy or
ingress-nginx, prefer explicit trusted proxy CIDRs over global forwarded-header
trust. Do not copy a broad private range from an example: first identify the
direct peer that reaches Prompt Gate.

```yaml
config:
  proxy:
    trust_forward_headers: false
    trusted_proxies:
      - "<narrow-haproxy-controller-or-node-cidr>"
      - "<upstream-hop-seen-by-haproxy-ip/32-or-cidr>"
    client_ip_debug: true # temporary
```

Send one known external request and inspect the proxy's `client IP resolved`
log. Match its `peer_ip` against controller pods and nodes:

```bash
kubectl get pods -A -o wide
kubectl get nodes -o wide
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" podCIDRs="}{.spec.podCIDRs}{"\n"}{end}'
```

If `peer_ip` is a controller pod address, use the narrow stable pod CIDR
allocated to those controller pods. If it is a node or SNAT address, use the
narrow corresponding node/network CIDR. Prefer dedicated controller address
ranges; never trust all cluster-private addresses merely for convenience.
After validation, set `config.proxy.client_ip_debug` back to `false`.

The generated Ingress explicitly enables
`haproxy.org/forwarded-for: "true"`. In the supported F5 topology, the F5 must
overwrite or sanitize client-supplied `X-Forwarded-For`. HAProxy then appends
the TCP source it actually observes as the last occurrence. Depending on the
Service topology, that source can be the F5 or a controlled node/SNAT hop.
Prompt Gate walks every header occurrence from right to left, skips only the
explicitly configured HAProxy and upstream ranges, and selects the nearest
untrusted address as the client.
Do not enable `trust_forward_headers` or configure `haproxy.org/src-ip-header`
for this topology.
`externalTrafficPolicy: Local` is not a substitute for the F5 header contract
when the F5 terminates or SNATs the connection; use it only if the F5 operator
confirms that the original L4 source address is preserved.

Obtain the F5 header contract, inspect the source HAProxy really receives, and
configure every controlled hop that actually appears in the chain; do not guess
any CIDR. When the HAProxy CIDR contains other workloads, add a NetworkPolicy
that permits access to the Prompt Gate proxy only from the HAProxy controller
namespace/pods.

## Argo CD Ordering

The migration Job is an Argo CD `Sync` hook, not a Helm hook. The default sync
waves apply the configuration Secret and ServiceAccount first, run migrations next, then
roll out workloads and Ingress resources. Hooks are not run during selective
syncs, so use a full application sync when migrations must run.

Upgrading from chart `0.1.x` to `0.2.x` changes where `/remote` is removed:
HAProxy stops rewriting it and Prompt Gate starts removing it. Perform that
upgrade during a short maintenance window after the `v2.1.0` image is
available, wait for the proxy rollout and full Argo CD sync to become healthy,
validate `/remote/health`, and only then reopen F5 traffic.
