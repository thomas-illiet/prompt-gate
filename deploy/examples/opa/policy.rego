package promptgate.proxy

default decision := {"allow": false, "reason": "default_deny"}

decision := {"allow": true, "reason": "administrator"} if {
	input.identity.role == "admin"
}

decision := {"allow": true, "reason": "trusted_service"} if {
	input.identity.role != "admin"
	input.identity.type == "service"
	input.identity.id == "replace-with-service-account-id"
	net.cidr_contains("10.0.0.0/8", input.request.client_ip)
	input.request.method == "POST"
	allowed_route(input.request)
}

decision := {"allow": true, "reason": "trusted_user_route"} if {
	input.identity.role in {"limited", "user"}
	input.identity.type == "user"
	net.cidr_contains("192.168.0.0/16", input.request.client_ip)
	input.request.method == "POST"
	allowed_route(input.request)
}

allowed_route(request) if {
	request.provider_type in {"openai", "ollama"}
	request.provider_path == "/v1/chat/completions"
}

allowed_route(request) if {
	request.provider_type == "openai"
	request.provider_path == "/v1/responses"
}

allowed_route(request) if {
	request.provider_type == "anthropic"
	request.provider_path == "/v1/messages"
}
