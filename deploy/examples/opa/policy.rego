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
	allowed_path(input.request.path)
}

decision := {"allow": true, "reason": "trusted_user_route"} if {
	input.identity.role == "user"
	input.identity.type == "user"
	net.cidr_contains("192.168.0.0/16", input.request.client_ip)
	input.request.method == "POST"
	allowed_path(input.request.path)
}

allowed_path(path) if {
	startswith(path, "/v1/chat/completions")
}

allowed_path(path) if {
	startswith(path, "/v1/responses")
}
