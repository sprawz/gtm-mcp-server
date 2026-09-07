package config

import (
	"os"
	"testing"
	"time"
)

// TestAutoRefreshMaxAge_DefaultsToSevenDays pins the bound on the renewal chain.
// Auto-refresh renews a bearer without rotating it, so without an absolute cap a
// leaked bearer stays useful for the whole refresh window (issue #79).
func TestAutoRefreshMaxAge_DefaultsToSevenDays(t *testing.T) {
	os.Unsetenv("AUTH_AUTO_REFRESH_MAX_AGE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if want := 7 * 24 * time.Hour; cfg.AutoRefreshMaxAge != want {
		t.Errorf("AutoRefreshMaxAge = %v, want %v", cfg.AutoRefreshMaxAge, want)
	}
}

func TestAutoRefreshMaxAge_HonoursEnv(t *testing.T) {
	t.Setenv("AUTH_AUTO_REFRESH_MAX_AGE", "48h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if want := 48 * time.Hour; cfg.AutoRefreshMaxAge != want {
		t.Errorf("AutoRefreshMaxAge = %v, want %v", cfg.AutoRefreshMaxAge, want)
	}
}

func TestDashboardOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("DASHBOARD_ENABLED", value)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.DashboardEnabled != (value == "true") {
				t.Fatalf("DASHBOARD_ENABLED=%q: %v", value, cfg.DashboardEnabled)
			}
		})
	}
}
