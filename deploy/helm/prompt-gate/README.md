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
