package promptgate.proxy_test

import data.promptgate.proxy

test_admin_allowed if {
	proxy.decision with input as sample_input("admin", "user", "admin-id", "203.0.113.4", "GET", "/anything") == {"allow": true, "reason": "administrator"}
}

test_trusted_service_allowed if {
	proxy.decision with input as sample_input("user", "service", "replace-with-service-account-id", "10.4.3.2", "POST", "/v1/responses") == {"allow": true, "reason": "trusted_service"}
}

test_trusted_user_route_allowed if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "/v1/chat/completions") == {"allow": true, "reason": "trusted_user_route"}
}

test_unknown_ip_denied if {
	proxy.decision with input as sample_input("user", "user", "user-id", "203.0.113.4", "POST", "/v1/chat/completions") == {"allow": false, "reason": "default_deny"}
}

test_unknown_route_denied if {
	proxy.decision with input as sample_input("user", "service", "replace-with-service-account-id", "10.4.3.2", "POST", "/v1/unknown") == {"allow": false, "reason": "default_deny"}
}

test_unknown_identity_denied if {
	proxy.decision with input as sample_input("user", "service", "unknown-service", "10.4.3.2", "POST", "/v1/responses") == {"allow": false, "reason": "default_deny"}
}

sample_input(role, identity_type, id, ip, method, path) := {
	"identity": {"id": id, "type": identity_type, "role": role, "credential_id": "key-id", "credential_name": "ci"},
	"request": {"client_ip": ip, "method": method, "path": path, "host": "proxy.example.com"},
}
