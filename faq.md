# Prompt Gate FAQ

## 1. What is Prompt Gate?

Prompt Gate is a platform that centralizes and secures access to artificial intelligence models. It provides a single entry point to multiple LLM providers while enforcing the access rights, quotas, network rules, and usage tracking defined by your organization.

## 2. Which AI providers are supported?

Prompt Gate supports OpenAI, Anthropic, and Ollama providers. Administrators can configure multiple instances of each type, such as a primary OpenAI service and a local Ollama instance.

## 3. How do I sign in to Prompt Gate?

You sign in through your organization's OIDC identity provider. Select the sign-in button, then authenticate with your usual account. You do not need a separate Prompt Gate password.

## 4. Why am I signed in but unable to access the application?

A new account may be created with the `none` role, which does not grant access to product features. An administrator must activate your account and assign you the `user`, `manager`, or `admin` role.

## 5. What is the difference between the user, manager, and administrator roles?

The `user` and `manager` roles can use the proxy, view their activity, and manage their own virtual keys. The `manager` role has a longer default key lifetime. The `admin` role also provides access to the platform's global configuration and administration features.

## 6. What is a virtual key?

A virtual key is a Prompt Gate API token used by an application, script, or LLM client to call the proxy. It replaces direct use of the AI provider's secret keys.

## 7. Why is my virtual key displayed only once?

The complete key value is returned only when the key is created. Prompt Gate then stores only a cryptographic hash, so the original key cannot be retrieved. Copy it immediately to a secret manager. If you lose it, revoke it and create a new one.

## 8. How long does a virtual key remain valid?

The requested lifetime must be between 1 and 365 days. If you do not choose one, the default is 7 days for a user and 30 days for a manager or administrator. The expiration date is shown in the virtual key list.

## 9. How do I revoke a virtual key?

Open the virtual keys page, find the relevant key, and use the revoke action. The key will no longer authenticate new requests. Immediately revoke any key that has been exposed, is no longer used, or belongs to a retired integration.

## 10. How do I configure my client to use Prompt Gate?

Open the help and setup page, select a provider and model, then copy the suggested base URL and example. Use your Prompt Gate virtual key as the authentication token instead of the provider's key.

## 11. Which base URL should I use?

The URL depends on the configured provider type and name. For OpenAI and Ollama, it usually follows the format `<proxy-url>/<provider-name>/v1`. For Anthropic, it follows the format `<proxy-url>/<provider-name>`. The setup page displays the exact URL to use.

## 12. Can I use every model available from a provider?

Not necessarily. The models you can access depend on the enabled providers and the access groups to which you belong. A group can allow specific providers or models and exclude others.

## 13. Why is access to a provider or model denied?

Your access group may not allow that provider or model, or an exclusion rule may apply. Check the groups shown in your profile and contact an administrator if an expected permission is missing.

## 14. Where can I view my quotas?

Your profile shows your assigned subscription plan, measured usage, and remaining tokens for the configured quota windows. Prompt Gate can track rolling limits over 5 hours and 7 days.

## 15. What happens when I exceed my quota?

The proxy temporarily rejects new requests with a quota exceeded error. Access becomes available again when enough usage falls outside the rolling window or when an administrator changes your subscription plan.

## 16. What can I see on my dashboard?

The dashboard shows token volume, request count, request duration, daily activity, and the most frequently used models and providers. Available views cover the last 7 days, the last 30 days, or the entire retained period.

## 17. Can I view my prompt history?

Yes. The history page shows recorded requests with their provider, model, token usage, duration, and date. The availability of older data depends on the retention policy configured by your organization.

## 18. Are the displayed costs actual billing amounts?

No. When enabled, cost estimates are calculated from recorded token counts and the rates configured in Prompt Gate. They are provided for monitoring purposes and do not replace provider statements or invoices.

## 19. What information does Prompt Gate record for a request?

Prompt Gate can record the identity that initiated the request, provider, model, timestamps, duration, token usage, prompt history, and MCP tool usage. This data powers dashboards, history, and administration statistics.

## 20. Is my Prompt Gate key sent to the AI provider?

No. The proxy removes Prompt Gate authentication details before forwarding the request. It then uses the provider credentials stored in encrypted form by the platform.

## 21. What is a service account?

A service account is a non-human identity intended for applications, automations, and shared integrations. It can have its own virtual keys, subscription plan, and dedicated firewall rules without depending on an employee's personal account.

## 22. What is the Prompt Gate firewall used for?

The firewall allows or blocks proxy calls based on their IPv4 address or CIDR range. Enabled rules are evaluated in priority order, and the first matching rule determines the decision.

## 23. Why was my request rejected with a `firewall_denied` error?

The detected IP address is not allowed by the rules that apply to your identity. When a dedicated firewall is enabled, a request is also denied if no rule matches. Send the source address and network context to an administrator so they can simulate and verify the decision.

## 24. What is MCP in Prompt Gate?

MCP makes external tools available to AI clients through the proxy. Administrators configure MCP servers, their headers, and, when needed, filters that allow or deny specific tools.

## 25. Do I need to restart my client when a provider or rule changes?

Usually not. Prompt Gate dynamically reloads providers, MCP servers, firewall rules, and authentication state. However, you must update your client configuration if a new URL or provider name is introduced.

## 26. How can I tell whether a service is unavailable or degraded?

The monitoring view displays the current state of monitored services, their latest latency, and any degradation. An incident banner may also notify users about an ongoing issue.

## 27. What can an administrator manage in Prompt Gate?

An administrator can manage users, roles, access expiration dates, service accounts, keys, providers, MCP servers, access groups, subscription plans, quotas, firewall rules, setup guides, FAQ entries, and monitored services. Administrators can also access global statistics and cross-user usage history.

## 28. How are provider and MCP server secrets protected?

Provider API keys and sensitive MCP header values are encrypted before they are stored in PostgreSQL. Their protection also depends on organizational practices such as using a secret manager, enforcing HTTPS, controlling administrator access, and carefully rotating encryption keys.

## 29. What should I do if a model request fails?

First, verify that your key is active and that neither its expiration nor your quota has been exceeded. Then confirm the URL, provider, and model you are using, and check the service status. If the problem continues, send the administrator the request time, provider, model, error code, and any request identifier, but never share your virtual key.

## 30. Where can I find additional help?

Start with the setup page for examples tailored to the available providers, then review your profile, access groups, quotas, and monitoring status. Contact your organization's Prompt Gate administrator to request changes to permissions, subscription plans, firewall rules, or providers.
