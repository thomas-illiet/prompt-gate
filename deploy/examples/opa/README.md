# Prompt Gate OPA example

This example deploys an external OPA server with a deny-by-default policy. Replace
`replace-with-service-account-id` and the example CIDRs before using it outside a
development environment.

```sh
opa test deploy/examples/opa/policy.rego deploy/examples/opa/policy_test.rego
kubectl apply -f deploy/examples/opa/kubernetes.yaml
kubectl port-forward service/prompt-gate-opa 8181:8181
```

Test an allowed administrator decision:

```sh
curl -sS http://127.0.0.1:8181/v1/data/promptgate/proxy/decision \
  -H 'Content-Type: application/json' \
  -d '{"input":{"identity":{"id":"admin-id","type":"user","role":"admin","credential_id":"key-id","credential_name":"local"},"request":{"client_ip":"203.0.113.4","method":"POST","path":"/v1/responses","host":"proxy.example.com"}}}'
```

Change the role to `user` and the IP to `203.0.113.4` to observe the default
deny response. Configure Prompt Gate with:

```text
PROMPTGATE_OPA_URL=http://prompt-gate-opa:8181
PROMPTGATE_OPA_POLICY_PATH=promptgate/proxy/decision
PROMPTGATE_OPA_TIMEOUT=2s
PROMPTGATE_OPA_CACHE_TTL=10m
```

The example intentionally does not add OPA to the Prompt Gate Helm chart. OPA
remains an independently operated dependency protected by the cluster network.
