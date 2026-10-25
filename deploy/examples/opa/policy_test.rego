package promptgate.proxy_test

import data.promptgate.proxy

test_admin_allowed if {
	proxy.decision with input as sample_input("admin", "user", "admin-id", "203.0.113.4", "GET", "unknown", "", "/anything") == {"allow": true, "reason": "administrator"}
}

test_trusted_service_openai_responses_allowed if {
	proxy.decision with input as sample_input("user", "service", "replace-with-service-account-id", "10.4.3.2", "POST", "openai-main", "openai", "/v1/responses") == {"allow": true, "reason": "trusted_service"}
}

test_trusted_user_prefixed_openai_chat_allowed if {
	request := sample_input("user", "user", "user-id", "192.168.4.2", "POST", "openai-main", "openai", "/v1/chat/completions")
	request.request.path == "/openai-main/v1/chat/completions"
	proxy.decision with input as request == {"allow": true, "reason": "trusted_user_route"}
}

test_trusted_user_ollama_chat_allowed if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "ollama-local", "ollama", "/v1/chat/completions") == {"allow": true, "reason": "trusted_user_route"}
}

test_trusted_user_anthropic_messages_allowed if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "anthropic-main", "anthropic", "/v1/messages") == {"allow": true, "reason": "trusted_user_route"}
}

test_unknown_ip_denied if {
	proxy.decision with input as sample_input("user", "user", "user-id", "203.0.113.4", "POST", "openai-main", "openai", "/v1/chat/completions") == {"allow": false, "reason": "default_deny"}
}

test_unknown_route_denied if {
	proxy.decision with input as sample_input("user", "service", "replace-with-service-account-id", "10.4.3.2", "POST", "openai-main", "openai", "/v1/unknown") == {"allow": false, "reason": "default_deny"}
}

test_unknown_provider_type_denied if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "unknown", "", "/v1/chat/completions") == {"allow": false, "reason": "default_deny"}
}

test_prefix_lookalike_denied if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "openai-main", "openai", "/v1/responses-unsafe") == {"allow": false, "reason": "default_deny"}
}

test_openai_response_subroute_denied if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "POST", "openai-main", "openai", "/v1/responses/response-id") == {"allow": false, "reason": "default_deny"}
}

test_non_post_denied_for_non_admin if {
	proxy.decision with input as sample_input("user", "user", "user-id", "192.168.4.2", "GET", "openai-main", "openai", "/v1/chat/completions") == {"allow": false, "reason": "default_deny"}
}

test_unknown_identity_denied if {
	proxy.decision with input as sample_input("user", "service", "unknown-service", "10.4.3.2", "POST", "openai-main", "openai", "/v1/responses") == {"allow": false, "reason": "default_deny"}
}

sample_input(role, identity_type, id, ip, method, provider_name, provider_type, provider_path) := {
	"identity": {"id": id, "type": identity_type, "role": role, "groups": ["developers", "platform"], "credential_id": "key-id", "credential_name": "ci"},
	"request": {
		"client_ip": ip,
		"method": method,
		"path": sprintf("/%s%s", [provider_name, provider_path]),
		"host": "proxy.example.com",
		"provider": provider_name,
		"provider_type": provider_type,
		"provider_path": provider_path,
	},
}
