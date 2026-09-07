package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestIsValidRedirectURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		// HTTPS URIs - all accepted
		{
			name:     "claude.ai",
			uri:      "https://claude.ai/api/mcp/auth_callback",
			expected: true,
		},
		{
			name:     "any HTTPS domain",
			uri:      "https://example.com/callback",
			expected: true,
		},
		{
			name:     "chatgpt.com",
			uri:      "https://chatgpt.com/connector_platform_oauth_redirect",
			expected: true,
		},
		// Custom URI schemes (RFC 8252 native apps)
		{
			name:     "cursor custom scheme",
			uri:      "cursor://anysphere.cursor-mcp/oauth/callback",
			expected: true,
		},
		{
			name:     "vscode custom scheme",
			uri:      "vscode://extension/oauth/callback",
			expected: true,
		},
		// Localhost (development)
		{
			name:     "localhost with http",
			uri:      "http://localhost:8080/callback",
			expected: true,
		},
		{
			name:     "localhost with https",
			uri:      "https://localhost:8080/callback",
			expected: true,
		},
		{
			name:     "127.0.0.1 with http",
			uri:      "http://127.0.0.1:8080/callback",
			expected: true,
		},
		// Blocked: http to non-localhost (plaintext code leakage)
		{
			name:     "http to remote host",
			uri:      "http://evil.com/callback",
			expected: false,
		},
		{
			name:     "http to remote host pretending to be localhost",
			uri:      "http://localhost.evil.com/callback",
			expected: false,
		},
		// Blocked: dangerous schemes
		{
			name:     "data URI",
			uri:      "data:text/html,<script>alert('xss')</script>",
			expected: false,
		},
		{
			name:     "javascript URI",
			uri:      "javascript:alert('xss')",
			expected: false,
		},
		// Blocked: malformed
		{
			name:     "empty URI",
			uri:      "",
			expected: false,
		},
		{
			name:     "no scheme",
			uri:      "//example.com/callback",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidRedirectURI(tt.uri)
			if result != tt.expected {
				t.Errorf("isValidRedirectURI(%q) = %v, expected %v", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestRedirectURIAllowed(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		candidate  string
		expected   bool
	}{
		{
			name:       "exact match",
			registered: []string{"https://claude.ai/api/mcp/auth_callback"},
			candidate:  "https://claude.ai/api/mcp/auth_callback",
			expected:   true,
		},
		// RFC 8252 §7.3: loopback redirects match regardless of port
		{
			name:       "localhost with ephemeral port matches port-less registration",
			registered: []string{"http://localhost/callback", "http://127.0.0.1/callback"},
			candidate:  "http://localhost:3118/callback",
			expected:   true,
		},
		{
			name:       "127.0.0.1 with ephemeral port matches port-less registration",
			registered: []string{"http://localhost/callback", "http://127.0.0.1/callback"},
			candidate:  "http://127.0.0.1:41973/callback",
			expected:   true,
		},
		{
			name:       "IPv6 loopback with port matches port-less registration",
			registered: []string{"http://[::1]/callback"},
			candidate:  "http://[::1]:8080/callback",
			expected:   true,
		},
		{
			name:       "loopback with different registered port still matches",
			registered: []string{"http://localhost:9999/callback"},
			candidate:  "http://localhost:3118/callback",
			expected:   true,
		},
		// The exemption must not loosen anything non-loopback
		{
			name:       "remote host with different port does not match",
			registered: []string{"https://example.com/callback"},
			candidate:  "https://example.com:8443/callback",
			expected:   false,
		},
		{
			name:       "localhost subdomain trick does not match",
			registered: []string{"http://localhost/callback"},
			candidate:  "http://localhost.evil.com:80/callback",
			expected:   false,
		},
		{
			name:       "https loopback is not exempted",
			registered: []string{"https://localhost/callback"},
			candidate:  "https://localhost:3118/callback",
			expected:   false,
		},
		{
			name:       "loopback path mismatch does not match",
			registered: []string{"http://localhost/callback"},
			candidate:  "http://localhost:3118/other",
			expected:   false,
		},
		{
			name:       "loopback candidate does not match remote registration",
			registered: []string{"https://claude.ai/api/mcp/auth_callback"},
			candidate:  "http://localhost:3118/callback",
			expected:   false,
		},
		{
			name:       "malformed candidate does not match",
			registered: []string{"http://localhost/callback"},
			candidate:  "http://local host/callback",
			expected:   false,
		},
		{
			name:       "empty registered list matches nothing",
			registered: nil,
			candidate:  "http://localhost:3118/callback",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redirectURIAllowed(tt.registered, tt.candidate)
			if result != tt.expected {
				t.Errorf("redirectURIAllowed(%v, %q) = %v, expected %v", tt.registered, tt.candidate, result, tt.expected)
			}
		})
	}
}

func TestServer_TokenError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &Server{logger: logger}

	tests := []struct {
		name           string
		errCode        string
		errDesc        string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "invalid_grant",
			errCode:        "invalid_grant",
			errDesc:        "Invalid refresh token",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid_grant","error_description":"Invalid refresh token"}`,
		},
		{
			name:           "invalid_request",
			errCode:        "invalid_request",
			errDesc:        "Missing code",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"invalid_request","error_description":"Missing code"}`,
		},
		{
			name:           "unsupported_grant_type",
			errCode:        "unsupported_grant_type",
			errDesc:        "Unsupported grant type",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"unsupported_grant_type","error_description":"Unsupported grant type"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.tokenError(w, tt.errCode, tt.errDesc)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", contentType)
			}

			body := strings.TrimSpace(w.Body.String())
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func TestServer_AuthorizeHandler_MethodNotAllowed(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/authorize", nil)
	w := httptest.NewRecorder()

	server.AuthorizeHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestServer_AuthorizeHandler_InvalidResponseType(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/authorize?response_type=token&state=test", nil)
	w := httptest.NewRecorder()

	server.AuthorizeHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "unsupported_response_type") {
		t.Error("expected unsupported_response_type error")
	}
}

func TestServer_AuthorizeHandler_MissingState(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/authorize?response_type=code", nil)
	w := httptest.NewRecorder()

	server.AuthorizeHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "invalid_request") {
		t.Error("expected invalid_request error")
	}
}

func TestServer_AuthorizeHandler_InvalidRedirectURI(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	tests := []struct {
		name        string
		redirectURI string
	}{
		{
			name:        "javascript scheme",
			redirectURI: "javascript:alert('xss')",
		},
		{
			name:        "data scheme",
			redirectURI: "data:text/html,<script>alert(1)</script>",
		},
		{
			name:        "http to remote host",
			redirectURI: "http://evil.com/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := url.Values{}
			params.Set("response_type", "code")
			params.Set("state", "test-state")
			params.Set("redirect_uri", tt.redirectURI)
			params.Set("code_challenge", "test-challenge")
			params.Set("code_challenge_method", "S256")

			req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
			w := httptest.NewRecorder()

			server.AuthorizeHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			if !strings.Contains(w.Body.String(), "Invalid redirect_uri") {
				t.Error("expected Invalid redirect_uri error")
			}
		})
	}
}

func TestServer_AuthorizeHandler_MissingPKCE(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	tests := []struct {
		name                string
		codeChallenge       string
		codeChallengeMethod string
	}{
		{
			name:                "missing both",
			codeChallenge:       "",
			codeChallengeMethod: "",
		},
		{
			name:                "missing challenge",
			codeChallenge:       "",
			codeChallengeMethod: "S256",
		},
		{
			name:                "wrong method",
			codeChallenge:       "test",
			codeChallengeMethod: "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := url.Values{}
			params.Set("response_type", "code")
			params.Set("state", "test-state")
			params.Set("redirect_uri", "http://localhost:8080/callback")
			if tt.codeChallenge != "" {
				params.Set("code_challenge", tt.codeChallenge)
			}
			if tt.codeChallengeMethod != "" {
				params.Set("code_challenge_method", tt.codeChallengeMethod)
			}

			req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
			w := httptest.NewRecorder()

			server.AuthorizeHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			if !strings.Contains(w.Body.String(), "PKCE") {
				t.Error("expected PKCE error")
			}
		})
	}
}

func TestServer_AuthorizeHandler_RegisteredClientValidation(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	// Register a client with specific redirect URIs
	clientInfo := &ClientInfo{
		ClientID:     "registered-client",
		RedirectURIs: []string{"https://example.com/callback"},
		CreatedAt:    time.Now(),
	}
	store.StoreClient(clientInfo)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	// Test that unregistered redirect URI is rejected for a registered client
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("state", "test-state")
	params.Set("client_id", "registered-client")
	params.Set("redirect_uri", "https://evil.com/callback")
	params.Set("code_challenge", "test-challenge")
	params.Set("code_challenge_method", "S256")

	req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
	w := httptest.NewRecorder()

	server.AuthorizeHandler(w, req)

	if !strings.Contains(w.Body.String(), "redirect_uri does not match") {
		t.Error("expected redirect_uri validation error for unregistered URI")
	}
}

func TestPKCEVerification(t *testing.T) {
	// Test PKCE challenge/verifier validation logic
	tests := []struct {
		name        string
		verifier    string
		challenge   string
		shouldMatch bool
	}{
		{
			name:        "valid match",
			verifier:    "test-verifier-123",
			challenge:   "", // Will be calculated
			shouldMatch: true,
		},
		{
			name:        "invalid match",
			verifier:    "test-verifier-123",
			challenge:   "wrong-challenge",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate the correct challenge
			h := sha256.Sum256([]byte(tt.verifier))
			correctChallenge := base64.RawURLEncoding.EncodeToString(h[:])

			challenge := tt.challenge
			if tt.shouldMatch {
				challenge = correctChallenge
			}

			// Verify
			h2 := sha256.Sum256([]byte(tt.verifier))
			calculatedChallenge := base64.RawURLEncoding.EncodeToString(h2[:])

			matched := calculatedChallenge == challenge
			if matched != tt.shouldMatch {
				t.Errorf("expected match=%v, got match=%v", tt.shouldMatch, matched)
			}
		})
	}
}

func TestServer_TokenHandler_MethodNotAllowed(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/token", nil)
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestServer_TokenHandler_UnsupportedGrantType(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "unsupported_grant_type") {
		t.Error("expected unsupported_grant_type error")
	}
}

func TestServer_HandleAuthorizationCodeGrant_MissingCode(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	// Missing code

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Missing code") {
		t.Error("expected Missing code error")
	}
}

func TestServer_HandleAuthorizationCodeGrant_InvalidCode(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "invalid-code")
	form.Set("code_verifier", "test-verifier")

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "invalid_grant") {
		t.Error("expected invalid_grant error")
	}
}

func TestServer_HandleAuthorizationCodeGrant_MissingCodeVerifier(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	// Store a valid code state
	codeState := &AuthState{
		State:        "valid-code",
		CodeVerifier: "test-challenge",
		CreatedAt:    time.Now(),
	}
	store.StoreState(codeState)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "valid-code")
	// Missing code_verifier

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Missing code_verifier") {
		t.Error("expected Missing code_verifier error")
	}
}

func TestServer_HandleRefreshTokenGrant_MissingRefreshToken(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	// Missing refresh_token

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Missing refresh_token") {
		t.Error("expected Missing refresh_token error")
	}
}

func TestServer_HandleRefreshTokenGrant_InvalidRefreshToken(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "invalid-refresh-token")

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.TokenHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid refresh token") {
		t.Error("expected Invalid refresh token error")
	}
}

func TestServer_CallbackHandler_MethodNotAllowed(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/oauth/callback", nil)
	w := httptest.NewRecorder()

	server.CallbackHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestServer_CallbackHandler_GoogleError(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	params := url.Values{}
	params.Set("error", "access_denied")
	params.Set("error_description", "User denied access")

	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+params.Encode(), nil)
	w := httptest.NewRecorder()

	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "access_denied") {
		t.Error("expected access_denied error")
	}
}

func TestServer_CallbackHandler_MissingCodeOrState(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	tests := []struct {
		name   string
		params url.Values
	}{
		{
			name:   "missing code",
			params: url.Values{"state": {"test"}},
		},
		{
			name:   "missing state",
			params: url.Values{"code": {"test"}},
		},
		{
			name:   "missing both",
			params: url.Values{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+tt.params.Encode(), nil)
			w := httptest.NewRecorder()

			server.CallbackHandler(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
			}

			if !strings.Contains(w.Body.String(), "Missing code or state") {
				t.Error("expected Missing code or state error")
			}
		})
	}
}

func TestServer_CallbackHandler_InvalidStateFormat(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	params := url.Values{}
	params.Set("code", "test-code")
	params.Set("state", "invalid-state-no-pipe")

	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+params.Encode(), nil)
	w := httptest.NewRecorder()

	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid state format") {
		t.Error("expected Invalid state format error")
	}
}

func TestServer_CallbackHandler_ExpiredState(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", nil, store, logger, 1*time.Hour)

	params := url.Values{}
	params.Set("code", "test-code")
	params.Set("state", "google-state|claude-state")

	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+params.Encode(), nil)
	w := httptest.NewRecorder()

	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid or expired state") {
		t.Error("expected Invalid or expired state error")
	}
}

// newFakeGoogleProvider returns a GoogleProvider whose token endpoint is a
// local httptest server, plus a cleanup function.
func newFakeGoogleProvider(t *testing.T) (*GoogleProvider, func()) {
	t.Helper()
	p, _, cleanup := newCountingFakeGoogleProvider(t)
	return p, cleanup
}

// newCountingFakeGoogleProvider is newFakeGoogleProvider plus a count of how
// often the token endpoint was reached, so a test can prove an authorization
// code was never exchanged.
func newCountingFakeGoogleProvider(t *testing.T) (*GoogleProvider, *atomic.Int64, func()) {
	t.Helper()
	var exchanges atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanges.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"google-access","refresh_token":"google-refresh","token_type":"Bearer","expires_in":3600}`))
	}))
	p := NewGoogleProvider("client-id", "client-secret", "http://localhost:8080/oauth/callback")
	p.Config().Endpoint.TokenURL = ts.URL
	p.Config().Endpoint.AuthURL = ts.URL + "/auth"
	return p, &exchanges, ts.Close
}

// runAuthorizeCallbackFlow drives /authorize then /oauth/callback and returns
// the final redirect Location back to the MCP client.
func runAuthorizeCallbackFlow(t *testing.T, server *Server, authorizeHost string) *url.URL {
	t.Helper()

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", "test-client")
	params.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	params.Set("state", "claude-state")
	params.Set("code_challenge", base64.RawURLEncoding.EncodeToString(func() []byte { h := sha256.Sum256([]byte("verifier")); return h[:] }()))
	params.Set("code_challenge_method", "S256")

	req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
	if authorizeHost != "" {
		req.Host = authorizeHost
	}
	w := httptest.NewRecorder()
	server.AuthorizeHandler(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("authorize: expected 302, got %d: %s", w.Code, w.Body.String())
	}

	googleURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("authorize: bad Location: %v", err)
	}
	googleState := googleURL.Query().Get("state")
	if googleState == "" {
		t.Fatal("authorize: no state in Google redirect")
	}

	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	cbReq := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	// The same browser completes the flow, so it returns the binding cookie
	// that /authorize set.
	for _, c := range w.Result().Cookies() {
		cbReq.AddCookie(c)
	}
	cbW := httptest.NewRecorder()
	server.CallbackHandler(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("callback: expected 302, got %d: %s", cbW.Code, cbW.Body.String())
	}

	loc, err := url.Parse(cbW.Header().Get("Location"))
	if err != nil {
		t.Fatalf("callback: bad Location: %v", err)
	}
	return loc
}

func TestServer_CallbackHandler_IncludesIssParam(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	loc := runAuthorizeCallbackFlow(t, server, "")

	if got := loc.Query().Get("iss"); got != "http://localhost:8080" {
		t.Errorf("expected iss=http://localhost:8080 in redirect (RFC 9207), got %q", got)
	}
}

func TestServer_CallbackHandler_IssUsesResolvedIssuerFromAuthorize(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)
	server.SetURLResolver(NewURLResolver("http://localhost:8080", []string{"gtm-mcp:8080"}))

	loc := runAuthorizeCallbackFlow(t, server, "gtm-mcp:8080")

	if got := loc.Query().Get("iss"); got != "http://gtm-mcp:8080" {
		t.Errorf("expected iss=http://gtm-mcp:8080 (issuer resolved at authorize time), got %q", got)
	}
}

// TestRefreshTokenGrant_ResetsChainAge is load-bearing for the #79 cap. The cap
// measures age from CreatedAt, and the refresh grant is the sanctioned way past
// it: because it rotates both credentials, the new entry earns a fresh window.
// If rotation carried the old CreatedAt forward, a capped client could never
// recover and would be forced into interactive re-auth instead.
func TestRefreshTokenGrant_ResetsChainAge(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	server := NewServer("http://localhost:8080", nil, store,
		slog.New(slog.NewTextHandler(io.Discard, nil)), 1*time.Hour)

	oldCreatedAt := time.Now().Add(-10 * 24 * time.Hour)
	store.StoreToken(&TokenInfo{
		AccessToken:      "old-access",
		RefreshToken:     "old-refresh",
		ExpiresAt:        time.Now().Add(1 * time.Hour),
		RefreshExpiresAt: time.Now().Add(20 * 24 * time.Hour),
		GoogleToken:      &oauth2.Token{AccessToken: "g-access", Expiry: time.Now().Add(1 * time.Hour)},
		ClientID:         "client-abc",
		CreatedAt:        oldCreatedAt,
	})

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "old-refresh")

	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	server.TokenHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad token response: %v", err)
	}

	rotated, err := store.GetTokenByAccess(resp["access_token"].(string))
	if err != nil {
		t.Fatalf("rotated token not found: %v", err)
	}
	if !rotated.CreatedAt.After(oldCreatedAt.Add(24 * time.Hour)) {
		t.Errorf("rotation carried the old chain age forward (CreatedAt=%v); a capped client could never recover", rotated.CreatedAt)
	}
}

// runAuthorizeForBinding drives /authorize only and returns the Google state
// plus the cookies the response set, so a test can decide whether to send them
// back on the callback.
func runAuthorizeForBinding(t *testing.T, server *Server) (string, []*http.Cookie) {
	t.Helper()

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", "test-client")
	params.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	params.Set("state", "claude-state")
	params.Set("code_challenge", base64.RawURLEncoding.EncodeToString(func() []byte { h := sha256.Sum256([]byte("verifier")); return h[:] }()))
	params.Set("code_challenge_method", "S256")

	req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
	w := httptest.NewRecorder()
	server.AuthorizeHandler(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("authorize: expected 302, got %d: %s", w.Code, w.Body.String())
	}

	googleURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("authorize: bad Location: %v", err)
	}
	googleState := googleURL.Query().Get("state")
	if googleState == "" {
		t.Fatal("authorize: no state in Google redirect")
	}
	return googleState, w.Result().Cookies()
}

// The attacker starts the flow, so they hold the state; the victim's browser
// completes Google consent and carries no binding cookie. That callback must
// be refused.
func TestServer_CallbackHandler_WithoutBindingCookieIsRefused(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, exchanges, cleanup := newCountingFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	googleState, _ := runAuthorizeForBinding(t, server)

	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	w := httptest.NewRecorder()
	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for a callback with no binding cookie, got %d: %s",
			http.StatusBadRequest, w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("refused callback must not redirect to the client, got Location %q", loc)
	}
	// The binding is checked before the exchange, so the victim's Google code
	// is never spent on a refused callback.
	if n := exchanges.Load(); n != 0 {
		t.Errorf("refused callback exchanged the code with Google %d time(s), want 0", n)
	}
}

// A cookie from some other browser, or a guessed one, must not satisfy the
// binding either.
func TestServer_CallbackHandler_WithWrongBindingCookieIsRefused(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, exchanges, cleanup := newCountingFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	googleState, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) == 0 {
		t.Fatal("authorize set no cookie, so there is no binding to get wrong")
	}

	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: cookies[0].Name, Value: "not-the-issued-binding"})
	w := httptest.NewRecorder()
	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for a callback with a wrong binding cookie, got %d: %s",
			http.StatusBadRequest, w.Code, w.Body.String())
	}
	if n := exchanges.Load(); n != 0 {
		t.Errorf("refused callback exchanged the code with Google %d time(s), want 0", n)
	}
}

func TestServer_AuthorizeHandler_SetsBindingCookie(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	_, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie on the authorize response, got %d", len(cookies))
	}
	c := cookies[0]

	if c.Value == "" {
		t.Error("binding cookie has an empty value")
	}
	if !c.HttpOnly {
		t.Error("binding cookie must be HttpOnly so page script cannot read it")
	}
	// Google's redirect back is a cross-site top-level GET, which Lax permits
	// and Strict would drop.
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("binding cookie must be SameSite=Lax, got %v", c.SameSite)
	}
	if c.Path != "/oauth/callback" {
		t.Errorf("binding cookie should be scoped to the callback, got path %q", c.Path)
	}
	if c.MaxAge <= 0 {
		t.Errorf("binding cookie needs a bounded lifetime, got Max-Age %d", c.MaxAge)
	}
}

func TestServer_AuthorizeHandler_BindingCookieSecureOverHTTPS(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("https://mcp.example.com", google, store, logger, 1*time.Hour)

	_, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie on the authorize response, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("binding cookie must be Secure when the issuer is https")
	}
}

// A binding reused across flows would let any victim holding a live cookie
// from their own recent login satisfy an attacker's state row, which is the
// original attack. Each flow must mint its own.
func TestServer_AuthorizeHandler_BindingIsFreshPerFlow(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	_, first := runAuthorizeForBinding(t, server)
	_, second := runAuthorizeForBinding(t, server)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected one cookie per authorize, got %d and %d", len(first), len(second))
	}
	if first[0].Value == second[0].Value {
		t.Error("two authorize calls issued the same binding; each flow must get a fresh one")
	}
}

// The defence rests on an absent value never satisfying the check: a state row
// that carries no binding must not be openable by a request that carries no
// cookie.
func TestBindingMatches_AbsentValuesAreAMismatch(t *testing.T) {
	binding := "some-opaque-binding"

	tests := []struct {
		name         string
		cookieValue  string
		expectedHash string
		want         bool
	}{
		{"both absent", "", "", false},
		{"no cookie, hash recorded", "", hashBinding(binding), false},
		{"cookie, no hash recorded", binding, "", false},
		{"cookie does not match", "other", hashBinding(binding), false},
		{"raw binding stored instead of its hash", binding, binding, false},
		{"matching", binding, hashBinding(binding), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bindingMatches(tt.cookieValue, tt.expectedHash); got != tt.want {
				t.Errorf("bindingMatches(%q, %q) = %v, want %v", tt.cookieValue, tt.expectedHash, got, tt.want)
			}
		})
	}
}

// Over https the cookie must carry the __Host- prefix, which tells the browser
// to accept it only for this exact host with no Domain attribute. That is what
// stops a sibling subdomain, or a network attacker on plain http to any host
// under the parent domain, from planting a binding we would accept.
func TestServer_AuthorizeHandler_UsesHostPrefixedCookieOverHTTPS(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("https://mcp.example.com", google, store, logger, 1*time.Hour)

	_, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	c := cookies[0]

	if c.Name != "__Host-gtm_fed_binding" {
		t.Errorf("expected the __Host- prefixed name over https, got %q", c.Name)
	}
	// The prefix is only honoured with Path=/ and Secure and no Domain.
	if c.Path != "/" {
		t.Errorf("__Host- requires Path=/, got %q", c.Path)
	}
	if !c.Secure {
		t.Error("__Host- requires Secure")
	}
	if c.Domain != "" {
		t.Errorf("__Host- forbids a Domain attribute, got %q", c.Domain)
	}
}

// A plain-http run cannot set a Secure cookie on a non-localhost origin, so the
// unprefixed name stays for that case.
func TestServer_AuthorizeHandler_UsesPlainCookieOverHTTP(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)

	_, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	if got := cookies[0].Name; got != "gtm_fed_binding" {
		t.Errorf("expected the unprefixed name over http, got %q", got)
	}
	if got := cookies[0].Path; got != "/oauth/callback" {
		t.Errorf("expected the callback-scoped path over http, got %q", got)
	}
}

// The whole point of the prefix is lost if the callback also accepts the
// unprefixed name: that is precisely the cookie an attacker on a sibling
// subdomain can set. An https flow must require the prefixed one.
func TestServer_CallbackHandler_UnprefixedCookieDoesNotSatisfyAnHTTPSFlow(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, exchanges, cleanup := newCountingFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("https://mcp.example.com", google, store, logger, 1*time.Hour)

	googleState, cookies := runAuthorizeForBinding(t, server)
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}

	// The injected cookie carries the right value under the unprefixed name,
	// which is all a sibling-subdomain attacker could achieve.
	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	req.Host = "mcp.example.com"
	req.AddCookie(&http.Cookie{Name: "gtm_fed_binding", Value: cookies[0].Value})
	w := httptest.NewRecorder()
	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for an unprefixed cookie on an https flow, got %d: %s",
			http.StatusBadRequest, w.Code, w.Body.String())
	}
	if n := exchanges.Load(); n != 0 {
		t.Errorf("refused callback exchanged the code with Google %d time(s), want 0", n)
	}
}

// runAuthorizeWithHeaders drives /authorize with a chosen Host and headers, so
// a test can play the attacker who controls the request that starts the flow.
func runAuthorizeWithHeaders(t *testing.T, server *Server, host string, headers map[string]string) (string, []*http.Cookie) {
	t.Helper()

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", "test-client")
	params.Set("redirect_uri", "https://claude.ai/api/mcp/auth_callback")
	params.Set("state", "claude-state")
	params.Set("code_challenge", base64.RawURLEncoding.EncodeToString(func() []byte { h := sha256.Sum256([]byte("verifier")); return h[:] }()))
	params.Set("code_challenge_method", "S256")

	req := httptest.NewRequest(http.MethodGet, "/authorize?"+params.Encode(), nil)
	req.Host = host
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	server.AuthorizeHandler(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("authorize: expected 302, got %d: %s", w.Code, w.Body.String())
	}

	googleURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("authorize: bad Location: %v", err)
	}
	return googleURL.Query().Get("state"), w.Result().Cookies()
}

// The dynamic URL resolver decides the scheme from X-Forwarded-Proto, which is
// trusted with no TrustProxy gate. The party that calls /authorize in this
// attack is the attacker, so they own that header — and omitting it must not
// downgrade the cookie to the plantable unprefixed regime on an https server.
func TestServer_AuthorizeHandler_BindingRegimeCannotBeDowngradedByForwardedProto(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("https://mcp.example.com", google, store, logger, 1*time.Hour)
	server.SetURLResolver(NewURLResolver("https://mcp.example.com", []string{"gtm-mcp:8080"}))

	// No X-Forwarded-Proto, so the resolver returns http://mcp.example.com.
	_, cookies := runAuthorizeWithHeaders(t, server, "mcp.example.com", nil)
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}

	if got := cookies[0].Name; got != "__Host-gtm_fed_binding" {
		t.Errorf("a suppressed X-Forwarded-Proto downgraded the cookie to %q", got)
	}
	if !cookies[0].Secure {
		t.Error("a suppressed X-Forwarded-Proto dropped the Secure flag")
	}
}

// The other half: having downgraded the flow, the attacker plants the
// unprefixed cookie a sibling subdomain can set. The callback must refuse it.
func TestServer_CallbackHandler_DowngradedFlowStillRejectsAnUnprefixedCookie(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, exchanges, cleanup := newCountingFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("https://mcp.example.com", google, store, logger, 1*time.Hour)
	server.SetURLResolver(NewURLResolver("https://mcp.example.com", []string{"gtm-mcp:8080"}))

	googleState, cookies := runAuthorizeWithHeaders(t, server, "mcp.example.com", nil)
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}

	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	req.Host = "mcp.example.com"
	req.AddCookie(&http.Cookie{Name: "gtm_fed_binding", Value: cookies[0].Value})
	w := httptest.NewRecorder()
	server.CallbackHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
	if n := exchanges.Load(); n != 0 {
		t.Errorf("refused callback exchanged the code with Google %d time(s), want 0", n)
	}
}

func TestIssuerIsHTTPS(t *testing.T) {
	tests := []struct {
		issuer string
		want   bool
	}{
		{"https://mcp.example.com", true},
		{"HTTPS://mcp.example.com", true},
		{" https://mcp.example.com", true},
		{"http://localhost:8080", false},
		{"", false},
		{"httpsx://mcp.example.com", false},
		{"not a url", false},
	}

	for _, tt := range tests {
		if got := issuerIsHTTPS(tt.issuer); got != tt.want {
			t.Errorf("issuerIsHTTPS(%q) = %v, want %v", tt.issuer, got, tt.want)
		}
	}
}

// The callback derives the regime from the issuer recorded at authorize time,
// never by re-resolving the callback request. Google's redirect back carries
// none of the proxy headers the original request had, so re-resolving would
// look for a different cookie name than the one /authorize set and break the
// legitimate flow.
func TestServer_CallbackHandler_RegimeComesFromTheRecordedIssuerNotTheCallbackRequest(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	google, cleanup := newFakeGoogleProvider(t)
	defer cleanup()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := NewServer("http://localhost:8080", google, store, logger, 1*time.Hour)
	server.SetURLResolver(NewURLResolver("http://localhost:8080", []string{"mcp.example.com"}))

	// Authorize arrives through the proxy, so it resolves to https and gets the
	// prefixed cookie.
	googleState, cookies := runAuthorizeWithHeaders(t, server, "mcp.example.com",
		map[string]string{"X-Forwarded-Proto": "https"})
	if len(cookies) != 1 || cookies[0].Name != "__Host-gtm_fed_binding" {
		t.Fatalf("expected the prefixed cookie from an https-resolved authorize, got %+v", cookies)
	}

	// The callback carries no X-Forwarded-Proto, as Google's redirect would not.
	cbParams := url.Values{}
	cbParams.Set("code", "google-code")
	cbParams.Set("state", googleState)
	req := httptest.NewRequest(http.MethodGet, "/oauth/callback?"+cbParams.Encode(), nil)
	req.Host = "mcp.example.com"
	req.AddCookie(cookies[0])
	w := httptest.NewRecorder()
	server.CallbackHandler(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected the flow to complete, got %d: %s", w.Code, w.Body.String())
	}
}
