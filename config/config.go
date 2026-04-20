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

	// Auth configuration
	// AuthMode determines how the server authenticates with Google Tag Manager.
	// Valid values: "oauth" (default, interactive user flow) or "adc" (server-side Application Default Credentials).
	AuthMode string

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

	// AllowedHosts lists additional trusted hostnames for dynamic base URL resolution.
	// Enables Docker-to-Docker contexts where the server is reached via internal aliases.
	AllowedHosts []string
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
		AuthMode:          getEnv("AUTH_MODE", "oauth"),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI: getEnv("GOOGLE_REDIRECT_URI", ""),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		AccessTokenTTL:    getEnvDuration("ACCESS_TOKEN_TTL", 8*time.Hour),
		AllowedHosts:      getEnvList("ALLOWED_HOSTS"),
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
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required for authentication")
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
