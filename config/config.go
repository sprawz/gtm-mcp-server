package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the GTM MCP Server.
type Config struct {
	// Server configuration
	Port    int
	BaseURL string

	// Google OAuth configuration
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// JWT configuration
	JWTSecret string

	// Logging
	LogLevel string

	// Token configuration
	AccessTokenTTL time.Duration

	// TokenStorePath, when set, persists issued tokens to this JSON file so
	// sessions survive restarts. Empty means in-memory only (the default).
	TokenStorePath string

	// AllowedHosts lists additional trusted hostnames for dynamic base URL resolution.
	// Enables Docker-to-Docker contexts where the server is reached via internal aliases.
	AllowedHosts []string

	// Service account authentication (S2S mode)
	ServiceAccountAPIKey  string // SERVICE_ACCOUNT_API_KEY
	ServiceAccountKeyJSON string // GOOGLE_SERVICE_ACCOUNT_KEY_JSON

	// TrustProxy enables trusting X-Forwarded-For for rate limiting.
	// Set to true when behind a reverse proxy (e.g. Caddy).
	TrustProxy bool
}

// Load reads configuration from environment variables.
// It first attempts to load from .env file if present, then .env.local for overrides.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()
	// Load .env.local for local development overrides (takes precedence)
	_ = godotenv.Overload(".env.local")

	cfg := &Config{
		Port:              getEnvInt("PORT", 8080),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI: getEnv("GOOGLE_REDIRECT_URI", ""),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		AccessTokenTTL:    getEnvDuration("ACCESS_TOKEN_TTL", 8*time.Hour),
		TokenStorePath:    getEnv("TOKEN_STORE_PATH", ""),
		AllowedHosts:      getEnvList("ALLOWED_HOSTS"),
		ServiceAccountAPIKey:  getEnv("SERVICE_ACCOUNT_API_KEY", ""),
		ServiceAccountKeyJSON: getEnv("GOOGLE_SERVICE_ACCOUNT_KEY_JSON", ""),
		TrustProxy:            getEnvBool("TRUST_PROXY", false),
	}

	// Validation is deferred to when auth is actually needed
	// This allows the server to start and respond to initialize/ping
	// even without OAuth credentials configured

	return cfg, nil
}

// ValidateAuth checks if OAuth credentials are configured.
func (c *Config) ValidateAuth() error {
	if c.GoogleClientID == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required for authentication")
	}
	if c.GoogleClientSecret == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required for authentication")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvList(key string) []string {
	if value := os.Getenv(key); value != "" {
		var hosts []string
		for _, h := range strings.Split(value, ",") {
			if h = strings.TrimSpace(h); h != "" {
				hosts = append(hosts, h)
			}
		}
		return hosts
	}
	return nil
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
