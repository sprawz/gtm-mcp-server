// Package dashboard serves an opt-in connection setup page with no session data.
package dashboard

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
)

//go:embed page.html style.css
var assets embed.FS
var page = template.Must(template.ParseFS(assets, "page.html"))

// Status deliberately contains no credentials, tokens, or user/container data.
type Status struct {
	Endpoint                 string
	Version                  string
	OAuthConfigured          bool
	ServiceAccountConfigured bool
}

func NewHandler(status Status) http.Handler {
	// Display the canonical endpoint without any URL credentials or query secrets.
	endpoint, err := url.Parse(status.Endpoint)
	if err == nil && (endpoint.Scheme == "http" || endpoint.Scheme == "https") && endpoint.Host != "" {
		endpoint.User = nil
		endpoint.RawQuery = ""
		endpoint.Fragment = ""
		status.Endpoint = endpoint.String()
	} else {
		status.Endpoint = "Check the server's BASE_URL setting"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/dashboard/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			_ = page.Execute(w, status)
		case "/dashboard/style.css":
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			if r.Method == http.MethodHead {
				return
			}
			css, _ := assets.ReadFile("style.css")
			_, _ = w.Write(css)
		default:
			http.NotFound(w, r)
		}
	})
}
