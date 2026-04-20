package auth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// TokenProvider is an interface for obtaining an OAuth2 TokenSource.
// This allows different authentication strategies (e.g. user OAuth vs server-side ADC).
type TokenProvider interface {
	// GetTokenSource returns a TokenSource based on the request context and/or HTTP request.
	GetTokenSource(ctx context.Context, req *http.Request) (oauth2.TokenSource, error)

	// IsAuthenticated returns true if the current context/environment is authenticated.
	IsAuthenticated(ctx context.Context) bool

	// VerifyAccess performs an active check against the downstream service (GTM).
	// It should implement caching to prevent rate-limiting.
	VerifyAccess(ctx context.Context) error
}
