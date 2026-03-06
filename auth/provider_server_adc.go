package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/tagmanager/v2"
)

// ServerADCProvider implements TokenProvider using Google Application Default Credentials (ADC).
type ServerADCProvider struct {
	scopes []string

	mu            sync.RWMutex
	isValidated   bool
	lastChecked   time.Time
	checkInterval time.Duration
}

// NewServerADCProvider creates a new ServerADCProvider with the given scopes.
func NewServerADCProvider(scopes []string) *ServerADCProvider {
	return &ServerADCProvider{
		scopes:        scopes,
		checkInterval: 5 * time.Minute,
	}
}

// GetTokenSource ignores the request and context, and uses the server's environment credentials.
func (p *ServerADCProvider) GetTokenSource(ctx context.Context, _ *http.Request) (oauth2.TokenSource, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, p.scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to get ADC token source: %w", err)
	}
	return tokenSource, nil
}

// IsAuthenticated returns true for ADC as it uses server-side credentials.
func (p *ServerADCProvider) IsAuthenticated(ctx context.Context) bool {
	return true
}

// VerifyAccess performs an active check against the downstream service (GTM).
func (p *ServerADCProvider) VerifyAccess(ctx context.Context) error {
	p.mu.RLock()
	if time.Since(p.lastChecked) < p.checkInterval {
		if p.isValidated {
			p.mu.RUnlock()
			return nil
		}
		// If it previously failed but the interval hasn't passed, we still might want to re-check, 
		// but typically we'd cache the failure too. For simplicity, let's cache success only or both.
		// Let's cache both: if it was validated, return nil. If not, maybe retry or return error.
		// Actually, let's just cache the validation state regardless.
		p.mu.RUnlock()
		// If not validated and we are within checkInterval, we could return a cached error,
		// but let's just perform ping if it's not validated.
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
func (p *ServerADCProvider) performPing(ctx context.Context) error {
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
