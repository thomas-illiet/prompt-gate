# Prompt Gate FAQ

## 1. What is Prompt Gate?

Prompt Gate is a platform that centralizes and secures access to artificial intelligence models. It provides a single entry point to multiple LLM providers while enforcing the access rights, quotas, network rules, and usage tracking defined by your organization.

## 2. Which AI providers are supported?

Prompt Gate supports OpenAI, Anthropic, and Ollama providers. Administrators can configure multiple instances of each type, such as a primary OpenAI service and a local Ollama instance.

## 3. How do I sign in to Prompt Gate?

You sign in through your organization's OIDC identity provider. Select the sign-in button, then authenticate with your usual account. You do not need a separate Prompt Gate password.

## 4. Why am I signed in but unable to access the application?

A new human account normally starts active with the `limited` role. This grants personal dashboard and proxy access through a key issued by an administrator, but it does not allow you to manage your own keys or open setup and monitoring features. If you have no access at all, an administrator may have deactivated the account, assigned the `none` role, or allowed its access period to expire.

## 5. What is the difference between the limited, user, and administrator roles?

The `limited` role can use the proxy with a centrally administered key and view personal profile, usage, quota, quota-reset, dashboard, group, and FAQ information. The `user` role also provides setup and monitoring access and lets you manage your own virtual keys. The `admin` role adds the platform's global configuration and administration features. The `none` role grants no application or proxy access.

## 6. What is a virtual key?

A virtual key is a Prompt Gate API token used by an application, script, or LLM client to call the proxy. It replaces direct use of the AI provider's secret keys.

## 7. Why is my virtual key displayed only once?

The complete key value is returned only to the person who creates the key. Prompt Gate then stores only a cryptographic hash, so the original value cannot be retrieved. Users with role `user` or `admin` should copy it immediately to a secret manager. For a `limited` account, an administrator creates and securely provides the key; contact that administrator if the value is lost.

## 8. How long does a virtual key remain valid?

For a human account, the requested lifetime must be between 1 and 30 days; service-account keys can be requested for up to 365 days. If no lifetime is specified, the default is 7 days for `limited`, `user`, and service accounts, and 30 days for `admin`. Users who can manage keys see the expiration date in the virtual key list; `limited` users can ask an administrator.

## 9. How do I revoke a virtual key?

Users with role `user` or `admin` can open the virtual keys page, find the relevant key, and use the revoke action. A `limited` user must ask an administrator to revoke a centrally managed key. The key will no longer authenticate new requests. Immediately revoke any key that has been exposed, is no longer used, or belongs to a retired integration.

## 10. How do I configure my client to use Prompt Gate?

Users with role `user` or `admin` can open the help and setup page, select a provider and model, then copy the suggested base URL and example. A `limited` user should request the approved endpoint, model, and centrally managed virtual key from an administrator. Use the Prompt Gate virtual key as the authentication token instead of the provider's key.

## 11. Which base URL should I use?

The URL depends on the configured provider type and name. For OpenAI and Ollama, it usually follows the format `<proxy-url>/<provider-name>/v1`. For Anthropic, it follows the format `<proxy-url>/<provider-name>`. The setup page displays the exact URL for `user` and `admin` roles; `limited` users should obtain it from an administrator.

## 12. Can I use every model available from a provider?

Not necessarily. The models you can access depend on the enabled providers and the access groups to which you belong. A group can allow specific providers or models and exclude others.

## 13. Why is access to a provider or model denied?

Your access group may not allow that provider or model, or an exclusion rule may apply. Check the groups shown in your profile and contact an administrator if an expected permission is missing.

## 14. Where can I view my quotas?

Your profile shows your assigned subscription plan, measured usage, and remaining tokens for the configured quota windows. Prompt Gate can track rolling limits over 5 hours and 7 days.

## 15. What happens when I exceed my quota?

The proxy temporarily rejects new requests with a quota exceeded error. If a reset credit is available and your plan has a finite quota, open your profile and use one credit. A reset clears both live 5-hour and 7-day windows without deleting historical usage or dashboard analytics. If the reset is pending, the profile updates automatically while the scheduler retries it. Otherwise, access becomes available again when enough usage falls outside the rolling window or when an administrator changes your subscription plan.

Active human accounts with unexpired `limited`, `user`, or `admin` access and an effective finite quota receive one automatic credit per UTC calendar month, up to a balance of two. Reaching the two-credit maximum still counts as that month's distribution, so spending one later in the same month does not immediately add another. An administrator can also offer one credit to any human account while its balance is below the maximum.

## 16. What can I see on my dashboard?

The dashboard shows token volume, request count, request duration, daily activity, and the most frequently used models and providers. Available views cover the last 7 days, the last 30 days, or the entire retained period. The expandable 365-day activity view is available only on the personal dashboard; the organization overview does not expose a 365-day window.

## 17. Are the displayed costs actual billing amounts?

No. Cost estimates are always calculated from recorded token counts and the rates configured in Prompt Gate. They are provided for monitoring purposes and do not replace provider statements or invoices. The interface marks cost coverage as complete, partial, or unavailable when retained raw usage does not cover the whole requested period; an unavailable estimate is omitted instead of being displayed as zero.

## 18. What information does Prompt Gate record for a request?

Prompt Gate records the identity that initiated the request, provider, model, timestamps, duration, token usage, and provider-reported tool invocation names, arguments, and errors. Raw prompts and assistant responses are not stored in PostgreSQL. If an operator explicitly enables OpenTelemetry prompt, output, or thinking capture, that configured content is exported to the external trace collector; review its access and retention policy. OpenTelemetry spans also include account identity metadata and tool arguments when tracing is enabled, even when raw prompt and output capture are disabled. Separately, the emergency `proxy.debug_requests` mode logs request headers and body bytes consumed by the proxy, which can expose credentials, prompts, and personal data; it must remain disabled outside short-lived diagnosis. The PostgreSQL records power dashboards and administration statistics.

## 19. Is my Prompt Gate key sent to the AI provider?

No. The proxy removes Prompt Gate authentication details before forwarding the request. It then uses the provider credentials stored in encrypted form by the platform.

## 20. What is a service account?

A service account is a non-human identity intended for applications, automations, and shared integrations. It can have its own virtual keys and subscription plan without depending on an employee's personal account. Its proxy access is decided by the external OPA policy.

## 21. How is proxy access authorized?

Prompt Gate sends a minimal identity and request context to an external OPA server. OPA is the sole policy engine and can evaluate roles, identity type, route, method, host, and client IP. Decisions are cached in Redis for the configured TTL.

## 22. Why was my request rejected with `policy_denied` or `policy_unavailable`?

`policy_denied` means OPA explicitly refused the request. `policy_unavailable` means no valid decision could be obtained from either Redis or OPA. The policy reason is retained in server observability but is intentionally not returned to the client.

## 23. Do I need to restart my client when a provider or rule changes?

Usually not. Prompt Gate dynamically reloads providers, access groups, subscription plans and assignments, and authentication state. OPA policy changes take effect after an existing cached decision expires, at most 10 minutes with the default configuration.

## 24. How can I tell whether a service is unavailable or degraded?

The monitoring view displays the current state of monitored services, their latest latency, and any degradation. An incident banner may also notify users about an ongoing issue.

## 25. What can an administrator manage in Prompt Gate?

An administrator can pre-create users, manage roles and access expiration dates, offer quota-reset credits, and manage service accounts, keys, providers, access groups, subscription plans, quotas, setup guides, FAQ entries, and monitored services. OPA policies are operated independently. Administrators can also access global usage statistics and the interactive API documentation, whose Try it out requests affect the current environment.

## 26. How are provider secrets protected?

Provider API keys are encrypted before they are stored in PostgreSQL. Their protection also depends on organizational practices such as using a secret manager, enforcing HTTPS, controlling administrator access, and carefully rotating encryption keys.

## 27. What should I do if a model request fails?

First, verify that your key is active and that neither its expiration nor your quota has been exceeded. Then confirm the URL, provider, and model you are using, and check the service status. If the problem continues, send the administrator the request time, provider, model, error code, and any request identifier, but never share your virtual key.

## 28. Where can I find additional help?

Start with the setup page for examples tailored to the available providers, then review your profile, access groups, quotas, and monitoring status. Contact your organization's Prompt Gate administrator to request changes to permissions, subscription plans, OPA policy, or providers.
