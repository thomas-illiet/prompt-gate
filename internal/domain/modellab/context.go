package modellab

import "context"

type validatedTicketKey struct{}

// ContextWithValidatedTicket marks requests only after ticket consumption,
// account validation and concurrency admission succeed. It is never derived
// from HTTP metadata or serialized in the API-token cache.
func ContextWithValidatedTicket(ctx context.Context) context.Context {
	return context.WithValue(ctx, validatedTicketKey{}, true)
}

func HasValidatedTicket(ctx context.Context) bool {
	validated, _ := ctx.Value(validatedTicketKey{}).(bool)
	return validated
}
