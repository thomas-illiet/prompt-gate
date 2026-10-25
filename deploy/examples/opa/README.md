# Prompt Gate OPA example

This example deploys an external OPA server with two deny-by-default boundaries:
the Prompt Gate decision policy and an OPA `system.authz` policy that exposes only
the exact decision endpoint plus health. OPA's policy and data mutation APIs are
not remotely accessible. Replace `replace-with-service-account-id` and the
example CIDRs before using it outside a development environment.

```sh
bash deploy/examples/opa/test.sh
kubectl apply -f deploy/examples/opa/kubernetes.yaml
kubectl port-forward service/prompt-gate-opa 8181:8181
```

Test an allowed administrator decision:

```sh
curl -sS http://127.0.0.1:8181/v1/data/promptgate/proxy/decision \
  -H 'Content-Type: application/json' \
  -d '{"input":{"identity":{"id":"admin-id","type":"user","role":"admin","groups":["developers","platform"],"credential_id":"key-id","credential_name":"local"},"request":{"client_ip":"203.0.113.4","method":"POST","path":"/openai-main/v1/responses","host":"proxy.example.com","provider":"openai-main","provider_type":"openai","provider_path":"/v1/responses"}}}'
```

Prompt Gate routes requests through a provider-name prefix, so `request.path`
contains the full public proxy path (for example,
`/openai-main/v1/responses`). It also supplies the parsed `provider`, the
configured `provider_type`, and the provider-relative `provider_path`. The
example policy uses the structured fields so route rules cannot accidentally
match a provider name or a lookalike prefix. Keep `request.path` when a custom
policy needs the original routed path.

Change the role to `user` and the IP to `203.0.113.4` to observe the default
deny response. Configure Prompt Gate with:

```yaml
proxy:
  opa:
    enabled: true
    url: http://prompt-gate-opa:8181
    policy_path: promptgate/proxy/decision
    timeout: 2s
    cache_ttl: 10m
```

`system.authz` is loaded from the read-only ConfigMap before the server starts,
and `--authorization=basic` rejects every other OPA REST route. The example does
not enable bearer authentication because Prompt Gate does not send an OPA bearer
token; instead, the accompanying `NetworkPolicy` limits port 8181 to proxy pods
with labels `app.kubernetes.io/name=prompt-gate` and
`app.kubernetes.io/component=proxy`. Adapt those selectors if the Helm chart uses
`nameOverride`, if OPA is in another namespace, or if a non-Helm proxy is used.
Kubernetes probes use the single authorized `GET /health` route.

The example uses HTTP inside the namespace. Add TLS or mTLS when the cluster
network is not a trusted transport boundary. Do not remove `system.authz` even
when a NetworkPolicy is present: the two controls protect against different
failure modes.

For emergency recovery only, set `proxy.opa.enabled: false` and restart the
proxy. This bypasses OPA authorization while leaving authentication, groups,
quotas, and provider controls enabled.

The example intentionally does not add OPA to the Prompt Gate Helm chart. OPA
remains an independently operated dependency protected by its API authorization
policy and the cluster network boundary.
