package auth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// UserOAuthProvider implements TokenProvider using the context set by the interactive OAuth middleware.
type UserOAuthProvider struct{}

// NewUserOAuthProvider creates a new UserOAuthProvider.
func NewUserOAuthProvider() *UserOAuthProvider {
	return &UserOAuthProvider{}
}

// GetTokenSource extracts the token info and dependencies from the context to create a TokenSource.
func (p *UserOAuthProvider) GetTokenSource(ctx context.Context, _ *http.Request) (oauth2.TokenSource, error) {
	tokenInfo := GetTokenInfo(ctx)
	if tokenInfo == nil || tokenInfo.GoogleToken == nil {
		return nil, fmt.Errorf("not authenticated - please authenticate with Google first")
	}

	store := GetTokenStore(ctx)
	googleProvider := GetGoogleProvider(ctx)

	// Create auto-refreshing token source
	tokenSource := NewAutoRefreshTokenSource(
		store,
		tokenInfo.AccessToken,
		googleProvider.Config(),
		tokenInfo.GoogleToken,
	)

	return tokenSource, nil
}
