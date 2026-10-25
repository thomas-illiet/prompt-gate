package system.authz_test

import data.system.authz

test_exact_decision_endpoint_allowed if {
	authz.allow with input as {"method": "POST", "path": ["v1", "data", "promptgate", "proxy", "decision"]}
}

test_health_allowed if {
	authz.allow with input as {"method": "GET", "path": ["health"]}
}

test_policy_put_denied if {
	not authz.allow with input as {"method": "PUT", "path": ["v1", "policies", "promptgate.proxy"]}
}

test_policy_delete_denied if {
	not authz.allow with input as {"method": "DELETE", "path": ["v1", "policies", "promptgate.proxy"]}
}

test_data_write_denied if {
	not authz.allow with input as {"method": "PUT", "path": ["v1", "data", "promptgate", "proxy"]}
}

test_arbitrary_query_denied if {
	not authz.allow with input as {"method": "GET", "path": ["v1", "query"]}
}

test_decision_subpath_denied if {
	not authz.allow with input as {"method": "POST", "path": ["v1", "data", "promptgate", "proxy", "decision", "extra"]}
}

test_decision_get_denied if {
	not authz.allow with input as {"method": "GET", "path": ["v1", "data", "promptgate", "proxy", "decision"]}
}
