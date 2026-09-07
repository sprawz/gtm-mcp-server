package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetupStates(t *testing.T) {
	for _, tt := range []struct {
		name      string
		oauth, sa bool
		want      []string
	}{
		{"unconfigured", false, false, []string{"Setup needed", "No authentication configured."}},
		{"oauth", true, false, []string{"OAuth configured", "choose its sign-in option"}},
		{"service account", false, true, []string{"API key configured", "Setup needed"}},
		{"both", true, true, []string{"OAuth configured", "API key configured"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler(Status{Endpoint: "https://mcp.example.test", OAuthConfigured: tt.oauth, ServiceAccountConfigured: tt.sa}).ServeHTTP(w, httptest.NewRequest("GET", "/dashboard/", nil))
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
			for _, want := range tt.want {
				if !strings.Contains(w.Body.String(), want) {
					t.Errorf("missing %q", want)
				}
			}
			if (tt.oauth || tt.sa) && strings.Contains(w.Body.String(), "No authentication configured.") {
				t.Error("authenticated mode described as open")
			}
			if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Error("missing response protections")
			}
		})
	}
}

func TestSetupEscapesAndRedacts(t *testing.T) {
	w := httptest.NewRecorder()
	NewHandler(Status{Endpoint: "https://user:secret@example.test/?token=hidden#fragment", Version: `<script>alert(1)</script>`}).ServeHTTP(w, httptest.NewRequest("GET", "/dashboard/", nil))
	for _, forbidden := range []string{"user:secret", "token=hidden", "#fragment", "<script>"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Errorf("rendered %q", forbidden)
		}
	}
	if !strings.Contains(w.Body.String(), "https://example.test/") {
		t.Error("missing safe endpoint")
	}
}

func TestSetupRoutes(t *testing.T) {
	handler := NewHandler(Status{})
	for _, tt := range []struct {
		method, path string
		code         int
	}{
		{"GET", "/dashboard/style.css", 200}, {"GET", "/dashboard/tokens", 404},
		{"POST", "/dashboard/", 405}, {"HEAD", "/dashboard/", 200},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
		if w.Code != tt.code {
			t.Errorf("%s %s: %d", tt.method, tt.path, w.Code)
		}
		if tt.method == "HEAD" && w.Body.Len() != 0 {
			t.Error("HEAD returned a body")
		}
	}
}

var _ http.Handler = NewHandler(Status{})
