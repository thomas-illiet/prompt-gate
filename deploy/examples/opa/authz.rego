package system.authz

# OPA is a decision service here, not a remote policy-management API. The
# policy is loaded from the read-only ConfigMap at process start, so denying the
# REST mutation endpoints cannot prevent bootstrap.
default allow := false

allow if {
	input.method == "POST"
	input.path == ["v1", "data", "promptgate", "proxy", "decision"]
}

# Kubernetes probes use this exact health endpoint; all broader diagnostic,
# status, query, policy, and data-management routes stay closed.
allow if {
	input.method == "GET"
	input.path == ["health"]
}
