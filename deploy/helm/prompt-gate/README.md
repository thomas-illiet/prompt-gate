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

## Runtime Secret

Create a Secret in the destination namespace and set `secret.existingSecret` to
its name. It must contain:

- `PROMPTGATE_DATABASE_URL`
- `PROMPTGATE_REDIS_URL`
- `PROMPTGATE_JWT_SECRET`
- `PROMPTGATE_SECRETS_KEY`
- `PROMPTGATE_KEYCLOAK_CLIENT_SECRET` when the OIDC client requires it

## Administration API Key

The optional administration API key provides direct access to every
`/api/v1/admin/**` endpoint through the `X-Admin-API-Key` header. These routes
include destructive operations, so use the key only over HTTPS and treat it as
a privileged production credential. It is global: requests made with it do not
carry an individual administrator identity for attribution.

The feature is disabled by default. For production, create a dedicated Secret
and reference it from the chart:

```yaml
adminApiKey:
  existingSecret:
    name: prompt-gate-admin-api-key
    key: PROMPTGATE_ADMIN_API_KEY
  rolloutToken: "2026-07-rotation-1"
```

Only the API Deployment receives this Secret, through an explicit
`secretKeyRef`; it is not added to the shared runtime Secret or exposed to the
proxy, worker, scheduler, or migration Job. After rotating an externally
managed Secret, change `rolloutToken` or restart the API Deployment so its Pods
load the new value. Do not put `PROMPTGATE_ADMIN_API_KEY` in
`secret.existingSecret`, because that shared Secret is imported by every
Prompt Gate workload. The chart rejects an administration Secret with the same
name as the shared runtime Secret.

For local development only, `value` creates a dedicated chart-managed Secret:

```yaml
adminApiKey:
  value: "local-development-key"
  annotations: {}
```

`value` and `existingSecret.name` are mutually exclusive. An empty or
whitespace-only `value` leaves the feature disabled; any non-empty value is
accepted without a minimum length. A generated Secret automatically rolls the
API Pods when the value changes. Do not commit a real administration key to a
values file.

Example request:

```bash
curl --fail-with-body \
  -H "X-Admin-API-Key: ${PROMPTGATE_ADMIN_API_KEY:?not set}" \
  https://promptgate.example.com/api/v1/admin/users
```

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

When Prompt Gate runs behind ingress-nginx, prefer explicit trusted proxy CIDRs
over global forwarded-header trust:

```yaml
config:
  proxyTrustedProxies: "10.0.0.0/8,192.168.0.0/16"
```

The ingress controller must also preserve or forward the real client IP with
settings appropriate to your load balancer, such as `externalTrafficPolicy`,
`use-forwarded-headers`, `enable-real-ip`, and `proxy-real-ip-cidr`.

## Argo CD Ordering

The migration Job is an Argo CD `Sync` hook, not a Helm hook. The default sync
waves apply the ConfigMap and ServiceAccount first, run migrations next, then
roll out workloads and Ingress resources. Hooks are not run during selective
syncs, so use a full application sync when migrations must run.
