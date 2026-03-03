package auth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ServerADCProvider implements TokenProvider using Google Application Default Credentials (ADC).
type ServerADCProvider struct {
	scopes []string
}

// NewServerADCProvider creates a new ServerADCProvider with the given scopes.
func NewServerADCProvider(scopes []string) *ServerADCProvider {
	return &ServerADCProvider{
		scopes: scopes,
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
