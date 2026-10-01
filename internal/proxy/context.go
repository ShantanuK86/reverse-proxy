package proxy

import (
	"context"

	"github.com/shantanuk/reverse-proxy/internal/backend"
)

type contextKey string

const backendContextKey contextKey = "proxy-backend"

// WithBackend stores the selected backend in the request context.
func WithBackend(ctx context.Context, b *backend.Backend) context.Context {
	return context.WithValue(ctx, backendContextKey, b)
}

// BackendFromContext retrieves the backend from the request context.
func BackendFromContext(ctx context.Context) *backend.Backend {
	b, _ := ctx.Value(backendContextKey).(*backend.Backend)
	return b
}
