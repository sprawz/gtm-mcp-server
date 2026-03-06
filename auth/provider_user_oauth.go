package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/tagmanager/v2"
)

// UserOAuthProvider implements TokenProvider using the context set by the interactive OAuth middleware.
type UserOAuthProvider struct {
	mu            sync.RWMutex
	isValidated   bool
	lastChecked   time.Time
	checkInterval time.Duration
}

// NewUserOAuthProvider creates a new UserOAuthProvider.
func NewUserOAuthProvider() *UserOAuthProvider {
	return &UserOAuthProvider{
		checkInterval: 5 * time.Minute,
	}
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

// IsAuthenticated checks if the current context has a valid token.
func (p *UserOAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return GetTokenInfo(ctx) != nil
}

// VerifyAccess performs an active check against the downstream service (GTM).
func (p *UserOAuthProvider) VerifyAccess(ctx context.Context) error {
	if !p.IsAuthenticated(ctx) {
		return fmt.Errorf("not authenticated")
	}

	p.mu.RLock()
	if time.Since(p.lastChecked) < p.checkInterval {
		if p.isValidated {
			p.mu.RUnlock()
			return nil
		}
		p.mu.RUnlock()
	} else {
		p.mu.RUnlock()
	}

	err := p.performPing(ctx)

	p.mu.Lock()
	p.isValidated = (err == nil)
	p.lastChecked = time.Now()
	p.mu.Unlock()

	return err
}

// performPing attempts a minimal API call to verify actual GTM access.
func (p *UserOAuthProvider) performPing(ctx context.Context) error {
	ts, err := p.GetTokenSource(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get token source for ping: %w", err)
	}

	// 1. Initialize the Tag Manager service client
	srv, err := tagmanager.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return fmt.Errorf("failed to initialize GTM service for ping: %w", err)
	}

	// 2. Perform a lightweight, read-only API call. 
	// Listing accounts is usually the safest "whoami" equivalent for GTM.
	_, err = srv.Accounts.List().Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("GTM access verification failed: %w", err)
	}

	return nil
}
