package configevents

import "context"

const (
	DomainFirewall      = "firewall"
	DomainProviders     = "providers"
	DomainMCP           = "mcp"
	DomainAuth          = "auth"
	DomainGroups        = "groups"
	DomainSubscriptions = "subscriptions"
)

type Notifier interface {
	Notify(ctx context.Context, domain string)
}

type NoopNotifier struct{}

// Notify implements a no-op config event publisher.
func (NoopNotifier) Notify(context.Context, string) {}
