package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gtm-mcp-server/auth"
	"gtm-mcp-server/config"
	"gtm-mcp-server/gtm"
	"gtm-mcp-server/middleware"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "gtm-mcp-server"
	serverVersion = "1.4.0"
)

func main() {
	// Set up structured logging to stderr (stdout is reserved for MCP in stdio mode)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Adjust log level
	if cfg.LogLevel == "debug" {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		slog.SetDefault(logger)
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)

	// Add logging middleware
	server.AddReceivingMiddleware(middleware.NewLoggingMiddleware(logger))

	// Create HTTP handler for MCP
	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Health check endpoint (no auth required)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": serverName,
			"version": serverVersion,
		})
	})

	// URL resolver for dynamic base URL resolution in Docker-to-Docker contexts.
	// Only resolves dynamically for hosts in the allowlist; falls back to cfg.BaseURL.
	var urlResolver *auth.URLResolver
	if len(cfg.AllowedHosts) > 0 {
		urlResolver = auth.NewURLResolver(cfg.BaseURL, cfg.AllowedHosts)
		logger.Info("dynamic URL resolution enabled", "allowed_hosts", cfg.AllowedHosts)
	}

	// OAuth metadata endpoints (always served, no auth required)
	// RFC 9728: Protected Resource Metadata - tells clients where to find the authorization server
	mux.HandleFunc("GET /.well-known/oauth-protected-resource",
		auth.ProtectedResourceMetadataHandler(cfg.BaseURL, cfg.BaseURL, urlResolver))

	// RFC 8414: Authorization Server Metadata - tells clients about OAuth endpoints
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", auth.MetadataHandler(cfg.BaseURL, urlResolver))

	// Check if OAuth is configured
	var authServer *auth.Server
	var tokenStore auth.TokenStore
	oauthConfigured := cfg.ValidateAuth() == nil

	// Rate limiters for public endpoints
	oauthLimiter := middleware.NewRateLimiter(10, 20)   // 10 req/s, burst 20
	registerLimiter := middleware.NewRateLimiter(2, 5)   // 2 req/s, burst 5

	var tokenProvider auth.TokenProvider

	if cfg.AuthMode == "adc" {
		logger.Info("using server-side Application Default Credentials (ADC) for GTM authentication")
		tokenProvider = auth.NewServerADCProvider(auth.GoogleScopes)
		
		// In ADC mode, we don't need interactive OAuth middleware
		mux.Handle("/", maxBytesHandler(5<<20, mcpHandler))
	} else if oauthConfigured {
		logger.Info("using interactive user OAuth flow for GTM authentication")
		
		tokenStore = auth.NewMemoryTokenStore()
		googleProvider := auth.NewGoogleProvider(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.BaseURL+"/oauth/callback",
		)
		authServer = auth.NewServer(cfg.BaseURL, googleProvider, tokenStore, logger, cfg.AccessTokenTTL)

		tokenProvider = auth.NewUserOAuthProvider()

		// OAuth endpoints with rate limiting and body size limits
		mux.HandleFunc("GET /authorize", oauthLimiter.MiddlewareFunc(authServer.AuthorizeHandler))
		mux.HandleFunc("GET /oauth/callback", oauthLimiter.MiddlewareFunc(authServer.CallbackHandler))
		mux.HandleFunc("POST /token", oauthLimiter.MiddlewareFunc(middleware.MaxBytesMiddleware(1<<20, authServer.TokenHandler)))
		mux.HandleFunc("POST /register", registerLimiter.MiddlewareFunc(middleware.MaxBytesMiddleware(1<<20, authServer.RegistrationHandler)))

		// MCP endpoint with REQUIRED auth middleware and body size limit
		// Returns 401 if no valid Bearer token - triggers Claude's OAuth flow
		authMiddleware := auth.Middleware(tokenStore, googleProvider, logger, cfg.BaseURL, cfg.AccessTokenTTL, urlResolver)
		mux.Handle("/", authMiddleware(maxBytesHandler(5<<20, mcpHandler)))

		logger.Info("OAuth configured",
			"authorize_endpoint", cfg.BaseURL+"/authorize",
			"token_endpoint", cfg.BaseURL+"/token",
			"callback_endpoint", cfg.BaseURL+"/oauth/callback",
			"register_endpoint", cfg.BaseURL+"/register",
			"protected_resource_metadata", cfg.BaseURL+"/.well-known/oauth-protected-resource",
			"authorization_server_metadata", cfg.BaseURL+"/.well-known/oauth-authorization-server",
		)
	} else {
		logger.Warn("OAuth not configured, running without authentication", "error", cfg.ValidateAuth())

		// Fallback to UserOAuthProvider which will return errors when tools are called without tokens
		tokenProvider = auth.NewUserOAuthProvider()

		// Register OAuth endpoints that return proper errors
		oauthNotConfiguredHandler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "server_error",
				"error_description": "OAuth is not configured on this server.",
			})
		}
		mux.HandleFunc("GET /authorize", oauthLimiter.MiddlewareFunc(oauthNotConfiguredHandler))
		mux.HandleFunc("GET /oauth/callback", oauthLimiter.MiddlewareFunc(oauthNotConfiguredHandler))
		mux.HandleFunc("POST /token", oauthLimiter.MiddlewareFunc(oauthNotConfiguredHandler))
		mux.HandleFunc("POST /register", registerLimiter.MiddlewareFunc(oauthNotConfiguredHandler))

		// MCP endpoint without auth (still apply body size limit)
		mux.Handle("/", maxBytesHandler(5<<20, mcpHandler))
	}

	// Register tools
	registerTools(server, tokenProvider)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0, // Disabled for SSE streams
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server
	go func() {
		logger.Info("starting GTM MCP server",
			"port", cfg.Port,
			"base_url", cfg.BaseURL,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutting down server")

	// Give outstanding requests 10 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}

// registerTools adds MCP tools to the server.
func registerTools(server *mcp.Server, tokenProvider auth.TokenProvider) {
	registerUtilityTools(server)
	gtm.RegisterTools(server, tokenProvider)
}

// maxBytesHandler wraps an http.Handler with a request body size limit.
func maxBytesHandler(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

// registerUtilityTools adds ping and auth_status tools.
func registerUtilityTools(server *mcp.Server) {
	// Ping tool for testing connectivity
	type PingInput struct {
		Message string `json:"message,omitempty" jsonschema:"Optional message to echo back"`
	}
	type PingOutput struct {
		Reply     string `json:"reply"`
		Timestamp string `json:"timestamp"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ping",
		Description: "Test connectivity to the GTM MCP server",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input PingInput) (*mcp.CallToolResult, PingOutput, error) {
		reply := "pong"
		if input.Message != "" {
			reply = fmt.Sprintf("pong: %s", input.Message)
		}
		return nil, PingOutput{Reply: reply, Timestamp: time.Now().UTC().Format(time.RFC3339)}, nil
	})

	// Auth status tool
	type AuthStatusInput struct{}
	type AuthStatusOutput struct {
		Authenticated bool   `json:"authenticated"`
		Message       string `json:"message"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "auth_status",
		Description: "Check authentication status with Google Tag Manager",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input AuthStatusInput) (*mcp.CallToolResult, AuthStatusOutput, error) {
		tokenInfo := auth.GetTokenInfo(ctx)
		output := AuthStatusOutput{Authenticated: tokenInfo != nil}
		if tokenInfo != nil {
			output.Message = "You are authenticated and can access GTM data"
		} else {
			output.Message = "Not authenticated. GTM tools will require authentication."
		}
		return nil, output, nil
	})
}