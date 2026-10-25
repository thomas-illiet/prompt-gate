# Prompt Gate Helm Chart

This chart is designed to be rendered by Argo CD and deploys only the Prompt
Gate application components. PostgreSQL, Redis, and OIDC/Keycloak must already
exist outside the chart.

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
in the external file because Kubernetes probes use those container ports.
Change `rolloutToken` after replacing the Secret so all workloads restart.
Because the selected design uses one complete file, every Prompt Gate process
can read every secret in it; restrict Secret and values-file access accordingly.

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
      - "<narrow-controller-or-node-cidr>"
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

HAProxy must overwrite or sanitize client-supplied `X-Forwarded-For` before
appending the real client address. When the trusted CIDR contains workloads
other than the ingress controller, add a NetworkPolicy that permits access to
the Prompt Gate proxy only from the HAProxy controller namespace/pods. The
controller or its upstream load balancer must preserve the source IP; the exact
HAProxy setting depends on that external topology.

## Argo CD Ordering

The migration Job is an Argo CD `Sync` hook, not a Helm hook. The default sync
waves apply the configuration Secret and ServiceAccount first, run migrations next, then
roll out workloads and Ingress resources. Hooks are not run during selective
syncs, so use a full application sync when migrations must run.
