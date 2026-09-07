package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Server handles OAuth2 authorization endpoints.
type Server struct {
	baseURL        string
	google         *GoogleProvider
	store          TokenStore
	logger         *slog.Logger
	accessTokenTTL time.Duration
	resolver       *URLResolver
	cimd           *CIMDFetcher
}

// SetCIMDFetcher overrides the Client ID Metadata Document fetcher (tests).
func (s *Server) SetCIMDFetcher(f *CIMDFetcher) {
	s.cimd = f
}

// SetURLResolver enables per-request issuer resolution (Docker-to-Docker
// contexts), matching the dynamic behavior of the metadata endpoints.
func (s *Server) SetURLResolver(r *URLResolver) {
	s.resolver = r
}

// issuerForRequest returns the issuer identifier for the given request,
// resolved dynamically when a URLResolver is configured.
func (s *Server) issuerForRequest(r *http.Request) string {
	if s.resolver != nil {
		return s.resolver.Resolve(r)
	}
	return s.baseURL
}

// NewServer creates a new OAuth server.
func NewServer(baseURL string, google *GoogleProvider, store TokenStore, logger *slog.Logger, accessTokenTTL time.Duration) *Server {
	return &Server{
		baseURL:        baseURL,
		google:         google,
		store:          store,
		logger:         logger,
		accessTokenTTL: accessTokenTTL,
		cimd:           NewCIMDFetcher(),
	}
}

// AuthorizeHandler handles GET /authorize - redirects to Google OAuth.
func (s *Server) AuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse OAuth parameters from client (Claude or ChatGPT)
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	responseType := r.URL.Query().Get("response_type")
	state := r.URL.Query().Get("state")
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")
	resource := r.URL.Query().Get("resource") // RFC 9728: resource indicator

	// Validate required parameters
	if responseType != "code" {
		s.errorResponse(w, "unsupported_response_type", "Only 'code' response type is supported")
		return
	}

	if state == "" {
		s.errorResponse(w, "invalid_request", "State parameter is required")
		return
	}

	// Validate redirect URI
	// If client is registered via DCR, validate against their registered URIs
	// Otherwise accept any syntactically valid URI (PKCE provides code binding)
	if IsClientIDMetadataURL(clientID) {
		// Client ID Metadata Document (MCP 2026-07-28): client_id is an HTTPS
		// URL; fetch the document and validate redirect_uri against it.
		client, err := s.cimd.Fetch(r.Context(), clientID)
		if err != nil {
			s.logger.Error("failed to fetch client metadata document", "client_id", clientID, "error", err)
			s.errorResponse(w, "invalid_client", "Could not resolve client_id metadata document")
			return
		}
		if !redirectURIAllowed(client.RedirectURIs, redirectURI) {
			s.errorResponse(w, "invalid_request", "redirect_uri does not match client metadata document")
			return
		}
	} else if clientID != "" {
		if client, err := s.store.GetClient(clientID); err == nil {
			// Client is registered, validate against registered redirect_uris
			if !redirectURIAllowed(client.RedirectURIs, redirectURI) {
				s.errorResponse(w, "invalid_request", "redirect_uri does not match registered URIs")
				return
			}
		} else {
			// Client not registered via DCR, fall back to default validation
			if !isValidRedirectURI(redirectURI) {
				s.errorResponse(w, "invalid_request", "Invalid redirect_uri")
				return
			}
		}
	} else if !isValidRedirectURI(redirectURI) {
		s.errorResponse(w, "invalid_request", "Invalid redirect_uri")
		return
	}

	// PKCE is required
	if codeChallenge == "" || codeChallengeMethod != "S256" {
		s.errorResponse(w, "invalid_request", "PKCE with S256 is required")
		return
	}

	// Generate our own state for Google OAuth
	googleState, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate state", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	// Bind this flow to the browser that started it (see statebinding.go).
	binding, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate state binding", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	// Store the auth state for later verification
	authState := &AuthState{
		State:        googleState,
		CodeVerifier: codeChallenge, // Store the challenge, we'll verify later
		RedirectURI:  redirectURI,
		ClientID:     clientID,
		Resource:     resource, // Store resource for audience binding
		Issuer:       s.issuerForRequest(r),
		BindingHash:  hashBinding(binding),
		CreatedAt:    time.Now(),
	}

	// Also store Claude's state so we can pass it back
	authState.State = googleState + "|" + state

	if err := s.store.StoreState(authState); err != nil {
		s.logger.Error("failed to store state", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	setBindingCookie(w, binding, s.bindingRegimeIsHTTPS(authState.Issuer))

	// Redirect to Google OAuth
	googleAuthURL := s.google.AuthCodeURL(authState.State)

	s.logger.Info("redirecting to Google OAuth",
		"client_id", clientID,
		"redirect_uri", redirectURI,
	)

	http.Redirect(w, r, googleAuthURL, http.StatusFound)
}

// CallbackHandler handles GET /oauth/callback - receives code from Google.
func (s *Server) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for errors from Google
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		errDesc := r.URL.Query().Get("error_description")
		s.logger.Error("Google OAuth error", "error", errCode, "description", errDesc)
		s.errorResponse(w, errCode, errDesc)
		return
	}

	code := r.URL.Query().Get("code")
	combinedState := r.URL.Query().Get("state")

	if code == "" || combinedState == "" {
		s.errorResponse(w, "invalid_request", "Missing code or state")
		return
	}

	// Split the combined state
	parts := strings.SplitN(combinedState, "|", 2)
	if len(parts) != 2 {
		s.errorResponse(w, "invalid_request", "Invalid state format")
		return
	}
	claudeState := parts[1]

	// Retrieve and atomically consume stored auth state (single-use)
	authState, err := s.store.ConsumeState(combinedState)
	if err != nil {
		s.logger.Error("failed to get state", "error", err)
		s.errorResponse(w, "invalid_request", "Invalid or expired state")
		return
	}

	// The browser that completed Google consent must be the one that started
	// the flow, or an attacker can have a victim's code minted against their
	// own registered redirect_uri. Checked before the code is spent.
	binding := bindingFromRequest(r, s.bindingRegimeIsHTTPS(authState.Issuer))
	if !bindingMatches(binding, authState.BindingHash) {
		s.logger.Error("federation state binding mismatch",
			"client_id", authState.ClientID,
			"has_cookie", binding != "",
		)
		s.errorResponse(w, "invalid_request", "Invalid or expired state")
		return
	}

	// Exchange code with Google
	googleToken, err := s.google.Exchange(r.Context(), code)
	if err != nil {
		s.logger.Error("failed to exchange code with Google", "error", err)
		s.errorResponse(w, "server_error", "Failed to exchange authorization code")
		return
	}

	// Generate our own authorization code to return to Claude
	ourCode, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate code", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	// Store temporarily with the Google token (code is short-lived)
	tempToken := &TokenInfo{
		AccessToken: ourCode, // Temporary: using code as key
		GoogleToken: googleToken,
		ClientID:    authState.ClientID,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(5 * time.Minute), // Code expires in 5 min
	}

	// Store code verifier for PKCE verification
	codeState := &AuthState{
		State:        ourCode,
		CodeVerifier: authState.CodeVerifier,
		RedirectURI:  authState.RedirectURI,
		ClientID:     authState.ClientID,
		Resource:     authState.Resource, // Preserve resource for token endpoint
		CreatedAt:    time.Now(),
	}

	if err := s.store.StoreState(codeState); err != nil {
		s.logger.Error("failed to store code state", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	if err := s.store.StoreToken(tempToken); err != nil {
		s.logger.Error("failed to store temp token", "error", err)
		s.errorResponse(w, "server_error", "Internal server error")
		return
	}

	// Build the redirect URL with the authorization code
	redirectURL, _ := url.Parse(authState.RedirectURI)
	q := redirectURL.Query()
	q.Set("code", ourCode)
	q.Set("state", claudeState)
	// RFC 9207: identify the issuer so clients can detect mix-up attacks.
	// Use the issuer the client saw at authorize time (resolver-aware).
	issuer := authState.Issuer
	if issuer == "" {
		issuer = s.baseURL
	}
	q.Set("iss", issuer)
	redirectURL.RawQuery = q.Encode()
	finalURL := redirectURL.String()

	s.logger.Info("OAuth callback successful, redirecting to client",
		"redirect_uri", authState.RedirectURI,
	)

	// For HTTP/HTTPS redirect URIs (e.g. Claude.ai), a plain 302 is correct —
	// the client handles the response directly.
	// For custom-scheme URIs (cursor://, vscode://), a 302 causes the browser
	// to get stuck on the previous page (it can't render a custom-scheme URL).
	// JS navigation to custom schemes is also blocked by browsers without a
	// real user gesture (click). So we serve an HTML page with a clickable
	// button — the click satisfies the browser's user-gesture requirement and
	// the OS intercepts the custom scheme to open the editor.
	scheme := strings.ToLower(redirectURL.Scheme)
	if scheme == "http" || scheme == "https" {
		http.Redirect(w, r, finalURL, http.StatusFound)
		return
	}

	// Infer editor name from the redirect URI scheme for friendlier UX.
	editorName := "your editor"
	switch scheme {
	case "cursor":
		editorName = "Cursor"
	case "vscode", "vscode-insiders":
		editorName = "VS Code"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Authentication successful</title>
<style>
body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f9fafb}
.card{background:#fff;border-radius:12px;padding:40px 48px;box-shadow:0 1px 3px rgba(0,0,0,.1);text-align:center;max-width:400px}
.icon{color:#16a34a;font-size:40px;margin-bottom:16px}
h1{font-size:20px;font-weight:600;color:#111;margin:0 0 8px}
p{color:#6b7280;font-size:14px;margin:0 0 24px}
.btn{display:inline-block;padding:11px 28px;background:#111;color:#fff;border-radius:8px;text-decoration:none;font-size:14px;font-weight:500;letter-spacing:.01em}
</style>
</head>
<body>
<div class="card">
  <div class="icon">&#10003;</div>
  <h1>Authentication successful</h1>
  <p>Click below to finish connecting in %s.</p>
  <a class="btn" href=%q>Open in %s</a>
</div>
</body>
</html>`, editorName, finalURL, editorName)
}

// TokenHandler handles POST /token - exchanges code for tokens.
func (s *Server) TokenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		s.tokenError(w, "invalid_request", "Failed to parse request")
		return
	}

	grantType := r.FormValue("grant_type")

	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		s.tokenError(w, "unsupported_grant_type", "Unsupported grant type")
	}
}

func (s *Server) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	codeVerifier := r.FormValue("code_verifier")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")

	if code == "" {
		s.tokenError(w, "invalid_request", "Missing code")
		return
	}

	// Atomically consume the code state (single-use)
	codeState, err := s.store.ConsumeState(code)
	if err != nil {
		s.logger.Error("failed to get code state", "error", err)
		s.tokenError(w, "invalid_grant", "Invalid or expired code")
		return
	}

	// Validate client_id and redirect_uri match the original authorization request
	if clientID != "" && clientID != codeState.ClientID {
		s.logger.Error("client_id mismatch", "expected", codeState.ClientID, "got", clientID)
		s.tokenError(w, "invalid_grant", "client_id does not match")
		return
	}
	if redirectURI != "" && redirectURI != codeState.RedirectURI {
		s.logger.Error("redirect_uri mismatch", "expected", codeState.RedirectURI, "got", redirectURI)
		s.tokenError(w, "invalid_grant", "redirect_uri does not match")
		return
	}

	// Verify PKCE
	if codeVerifier == "" {
		s.tokenError(w, "invalid_request", "Missing code_verifier")
		return
	}

	// Verify: SHA256(code_verifier) == code_challenge
	h := sha256.Sum256([]byte(codeVerifier))
	calculatedChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	if calculatedChallenge != codeState.CodeVerifier {
		s.logger.Error("PKCE verification failed", "client_id", codeState.ClientID)
		s.tokenError(w, "invalid_grant", "PKCE verification failed")
		return
	}

	// Get the temporary token with Google credentials
	tempToken, err := s.store.GetTokenByAccess(code)
	if err != nil {
		s.logger.Error("failed to get temp token", "error", err)
		s.tokenError(w, "invalid_grant", "Invalid or expired code")
		return
	}

	// Clean up temporary token
	_ = s.store.DeleteToken(code)

	// Generate real tokens
	accessToken, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	refreshToken, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	// Create and store the real token
	tokenInfo := &TokenInfo{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresAt:        time.Now().Add(s.accessTokenTTL),
		RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		GoogleToken:      tempToken.GoogleToken,
		ClientID:         codeState.ClientID,
		CreatedAt:        time.Now(),
	}

	if err := s.store.StoreToken(tokenInfo); err != nil {
		s.logger.Error("failed to store token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	s.logger.Info("issued access token", "client_id", codeState.ClientID)

	// Return token response
	s.tokenResponse(w, accessToken, refreshToken, int(s.accessTokenTTL.Seconds()))
}

func (s *Server) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshToken := r.FormValue("refresh_token")

	if refreshToken == "" {
		s.tokenError(w, "invalid_request", "Missing refresh_token")
		return
	}

	// Get existing token info
	tokenInfo, err := s.store.GetTokenByRefresh(refreshToken)
	if err != nil {
		s.logger.Error("failed to get token by refresh", "error", err)
		s.tokenError(w, "invalid_grant", "Invalid refresh token")
		return
	}

	// Refresh the Google token if needed
	if tokenInfo.GoogleToken == nil {
		s.tokenError(w, "invalid_grant", "Token has no upstream credentials")
		return
	}
	if tokenInfo.GoogleToken.Expiry.Before(time.Now()) {
		newGoogleToken, err := s.google.RefreshToken(r.Context(), tokenInfo.GoogleToken.RefreshToken)
		if err != nil {
			s.logger.Error("failed to refresh Google token", "error", err)
			s.tokenError(w, "invalid_grant", "Failed to refresh upstream token")
			return
		}
		tokenInfo.GoogleToken = newGoogleToken
	}

	// Generate new access token and new refresh token (rotation)
	newAccessToken, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate access token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	newRefreshToken, err := GenerateToken(32)
	if err != nil {
		s.logger.Error("failed to generate refresh token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	// Delete old token (invalidates the old refresh token)
	_ = s.store.DeleteToken(tokenInfo.AccessToken)

	// Store new token with rotated refresh token
	newTokenInfo := &TokenInfo{
		AccessToken:      newAccessToken,
		RefreshToken:     newRefreshToken,
		ExpiresAt:        time.Now().Add(s.accessTokenTTL),
		RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		GoogleToken:      tokenInfo.GoogleToken,
		ClientID:         tokenInfo.ClientID,
		CreatedAt:        time.Now(),
	}

	if err := s.store.StoreToken(newTokenInfo); err != nil {
		s.logger.Error("failed to store new token", "error", err)
		s.tokenError(w, "server_error", "Internal server error")
		return
	}

	s.logger.Info("refreshed access token", "client_id", tokenInfo.ClientID)

	// Return token response with new refresh token
	s.tokenResponse(w, newAccessToken, newRefreshToken, int(s.accessTokenTTL.Seconds()))
}

func (s *Server) tokenResponse(w http.ResponseWriter, accessToken, refreshToken string, expiresIn int) {
	resp := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refreshToken,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) tokenError(w http.ResponseWriter, errCode, errDesc string) {
	resp := map[string]string{
		"error":             errCode,
		"error_description": errDesc,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) errorResponse(w http.ResponseWriter, errCode, errDesc string) {
	http.Error(w, fmt.Sprintf("%s: %s", errCode, errDesc), http.StatusBadRequest)
}

func isLoopbackHost(h string) bool {
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// redirectURIAllowed reports whether candidate matches one of the registered
// URIs. Per RFC 8252 section 7.3, http loopback redirects match on scheme,
// host and path only, since native clients bind an ephemeral port at request
// time. All other URIs require an exact match.
func redirectURIAllowed(registered []string, candidate string) bool {
	if slices.Contains(registered, candidate) {
		return true
	}
	u, err := url.Parse(candidate)
	if err != nil || u.Scheme != "http" || !isLoopbackHost(u.Hostname()) {
		return false
	}
	for _, r := range registered {
		ru, err := url.Parse(r)
		if err != nil || ru.Scheme != "http" || !isLoopbackHost(ru.Hostname()) {
			continue
		}
		if ru.Hostname() == u.Hostname() && ru.Path == u.Path {
			return true
		}
	}
	return false
}

// isValidRedirectURI validates redirect URIs for non-DCR clients.
// This is permissive because PKCE (required) binds the authorization code to the
// client's code_verifier, making authorization code interception attacks infeasible
// even with an attacker-controlled redirect URI. See RFC 7636 and RFC 8252.
//
// Blocked: javascript/data URIs (XSS), plaintext http to non-localhost (code leakage).
func isValidRedirectURI(uri string) bool {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme == "" {
		return false
	}

	scheme := strings.ToLower(parsed.Scheme)

	// Block dangerous schemes
	if scheme == "javascript" || scheme == "data" {
		return false
	}

	// For http, only allow localhost to prevent plaintext code leakage
	if scheme == "http" {
		hostname := parsed.Hostname()
		return hostname == "localhost" || hostname == "127.0.0.1"
	}

	// Allow https, and custom schemes (e.g. cursor://, vscode://) per RFC 8252
	return true
}
