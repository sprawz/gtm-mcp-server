package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func securityServer(t *testing.T, store TokenStore) *Server {
	t.Helper()
	google, cleanup := newFakeGoogleProvider(t)
	t.Cleanup(cleanup)
	return NewServer("https://mcp.example.com", google, store,
		slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour)
}

func tokenRequest(server *Server, form url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	server.TokenHandler(w, r)
	return w
}

func bearerStatus(server *Server, bearer string) int {
	h := Middleware(server.store, server.google, server.logger, server.baseURL, time.Hour, nil, nil, "", 7*24*time.Hour)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if info := GetTokenInfo(r.Context()); info == nil || info.GoogleToken == nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+bearer)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestSecurityAuthorizationCodeNotBearer(t *testing.T) {
	for _, fileBacked := range []bool{false, true} {
		name := "memory"
		if fileBacked {
			name = "file"
		}
		t.Run(name, func(t *testing.T) {
			var store TokenStore
			var path string
			if fileBacked {
				f, p := newTestFileStore(t)
				t.Cleanup(func() { f.Close() })
				store, path = f, p
			} else {
				m := NewMemoryTokenStore()
				t.Cleanup(func() { m.Close() })
				store = m
			}
			s := securityServer(t, store)
			code := runAuthorizeCallbackFlow(t, s, "").Query().Get("code")
			if code == "" {
				t.Fatal("callback returned no code")
			}
			if got := bearerStatus(s, code); got != http.StatusUnauthorized {
				t.Errorf("authorization code authenticated as bearer: status %d", got)
			}
			if fileBacked {
				data, err := os.ReadFile(path)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if strings.Contains(string(data), code) || strings.Contains(string(data), "google-refresh") {
					t.Error("unfinished authorization code credentials persisted")
				}
			}
			form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {"verifier"}}
			w := tokenRequest(s, form)
			if w.Code != http.StatusOK {
				t.Fatalf("legitimate exchange: %d %s", w.Code, w.Body.String())
			}
			var result map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if got := bearerStatus(s, result["access_token"].(string)); got != http.StatusNoContent {
				t.Errorf("issued access token rejected: %d", got)
			}
			if got := tokenRequest(s, form).Code; got != http.StatusBadRequest {
				t.Errorf("code replay: %d", got)
			}
		})
	}
}

func TestSecurityLegacyAuthorizationCodeNotReloaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	code := sampleToken("legacy-code", "")
	code.RefreshExpiresAt = time.Time{}
	data, err := json.Marshal(map[string]any{"tokens": []*TokenInfo{code, sampleToken("access", "refresh")}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.GetTokenByAccess("legacy-code"); err != ErrTokenNotFound {
		t.Errorf("legacy code reloaded: %v", err)
	}
	if _, err := f.GetTokenByAccessIncludeExpired("legacy-code"); err != ErrTokenNotFound {
		t.Errorf("legacy code in expired lookup: %v", err)
	}
	if _, err := f.GetTokenByRefresh("refresh"); err != nil {
		t.Errorf("legitimate legacy session lost: %v", err)
	}
}

func TestSecurityAuthorizationCodeRejections(t *testing.T) {
	for _, scenario := range []string{"wrong-verifier", "expired", "client-mismatch", "redirect-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			m := NewMemoryTokenStore()
			defer m.Close()
			s := securityServer(t, m)
			code := runAuthorizeCallbackFlow(t, s, "").Query().Get("code")
			form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {"verifier"}}
			switch scenario {
			case "wrong-verifier":
				form.Set("code_verifier", "incorrect")
			case "client-mismatch":
				form.Set("client_id", "another-client")
			case "redirect-mismatch":
				form.Set("redirect_uri", "https://elsewhere.example/callback")
			case "expired":
				m.mu.Lock()
				m.codes[code].ExpiresAt = time.Now().Add(-time.Second)
				m.mu.Unlock()
			}
			if w := tokenRequest(s, form); w.Code != http.StatusBadRequest {
				t.Fatalf("invalid exchange: %d", w.Code)
			}
			if got := bearerStatus(s, code); got != http.StatusUnauthorized {
				t.Errorf("rejected code authenticates: %d", got)
			}
			// Optional auth must not attach upstream authority either.
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+code)
			OptionalMiddleware(m, s.logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if GetTokenInfo(r.Context()) != nil || GetGoogleToken(r.Context()) != nil {
					t.Error("optional auth accepted code")
				}
			})).ServeHTTP(httptest.NewRecorder(), r)
		})
	}
}

func TestSecurityAuthorizationCodeConcurrentExchange(t *testing.T) {
	m := NewMemoryTokenStore()
	defer m.Close()
	s := securityServer(t, m)
	code := runAuthorizeCallbackFlow(t, s, "").Query().Get("code")
	start := make(chan struct{})
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- tokenRequest(s, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {"verifier"}}).Code
		}()
	}
	close(start)
	a, b := <-results, <-results
	if !((a == 200 && b == 400) || (a == 400 && b == 200)) {
		t.Errorf("expected one exchange, got %d, %d", a, b)
	}
}

func TestSecurityAuthorizationCodeCleanup(t *testing.T) {
	m := NewMemoryTokenStore()
	defer m.Close()
	hash := sha256.Sum256([]byte("verifier"))
	entry := &AuthorizationCode{AuthState: AuthState{State: "expired", CodeVerifier: base64.RawURLEncoding.EncodeToString(hash[:])}, ExpiresAt: time.Now().Add(-time.Second)}
	if err := m.StoreAuthorizationCode(entry); err != nil {
		t.Fatal(err)
	}
	m.purgeExpired(time.Now())
	if _, ok := m.codes["expired"]; ok {
		t.Error("expired code credentials retained")
	}
}

func TestSecurityMarkedAccessOnlyTokenSurvivesRestart(t *testing.T) {
	f, path := newTestFileStore(t)
	defer f.Close()
	if err := f.StoreToken(sampleToken("access-only", "")); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.GetTokenByAccess("access-only"); err != nil {
		t.Errorf("new explicitly issued access-only credential lost: %v", err)
	}
}

func TestSecurityRedirectForms(t *testing.T) {
	for _, uri := range []string{"http://[::1]:3456/callback", "com.example.app:/oauth/callback", "cursor://anysphere.cursor-mcp/oauth/callback", "vscode-insiders://extension/callback"} {
		if !isValidRedirectURI(uri) {
			t.Errorf("legitimate native redirect rejected: %s", uri)
		}
	}
	for _, uri := range []string{"JaVaScRiPt://host/alert(1)", "vbscript:/alert(1)", "https:opaque", "https:///missing-host", "https://user@example.com/callback", "cursor://host/callback#fragment", "file:///tmp/code", "data:text/html,evil"} {
		if isValidRedirectURI(uri) || redirectURIAllowed([]string{uri}, uri) {
			t.Errorf("unsafe redirect accepted: %s", uri)
		}
	}
}

func TestSecurityCallbackHTML(t *testing.T) {
	for _, redirect := range []string{`cursor:"><img src=x onerror=alert(1)>`, `vscode:&#34; onclick=alert(1)`, "cursor://anysphere.cursor-mcp/oauth/callback", "vscode://extension/oauth/callback"} {
		t.Run(redirect, func(t *testing.T) {
			m := NewMemoryTokenStore()
			defer m.Close()
			s := securityServer(t, m)
			// Exercise the render boundary even if a stale/injected state bypasses authorize validation.
			if err := m.StoreState(&AuthState{State: "google|client", RedirectURI: redirect, BindingHash: hashBinding("test-browser-binding"), CreatedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/oauth/callback?code=google-code&state=google%7Cclient", nil)
			req.AddCookie(&http.Cookie{Name: bindingCookieNameFor(s.bindingRegimeIsHTTPS("")), Value: "test-browser-binding"})
			s.CallbackHandler(w, req)
			if w.Code == http.StatusBadRequest && strings.Contains(redirect, "://") == false {
				return
			}
			if w.Code != http.StatusOK {
				t.Fatalf("callback: %d %s", w.Code, w.Body.String())
			}
			doc, err := html.Parse(strings.NewReader(w.Body.String()))
			if err != nil {
				t.Fatal(err)
			}
			links := 0
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if n.Type == html.ElementNode {
					if n.Data == "img" || n.Data == "script" {
						t.Errorf("injected element: %s", n.Data)
					}
					for _, a := range n.Attr {
						if strings.HasPrefix(a.Key, "on") {
							t.Errorf("injected handler: %s", a.Key)
						}
						if n.Data == "a" && a.Key == "href" {
							links++
							u, err := url.Parse(a.Val)
							if err != nil || u.Query().Get("code") == "" || !strings.HasPrefix(a.Val, strings.SplitN(redirect, ":", 2)[0]+":") {
								t.Errorf("broken native link: %q", a.Val)
							}
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
			walk(doc)
			if links != 1 {
				t.Errorf("got %d callback links", links)
			}
		})
	}
}

func TestSecurityCIMDRedirectValidation(t *testing.T) {
	for _, redirect := range []string{"javascript:alert(1)", "data:text/html,evil", "http://remote.example/callback", `cursor:"><img src=x onerror=alert(1)>`} {
		t.Run(redirect, func(t *testing.T) {
			var metadataURL string
			f, u, cleanup := newCIMDTestServer(t, "/client.json", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"client_id": metadataURL, "client_name": "test", "redirect_uris": []string{redirect}})
			})
			defer cleanup()
			metadataURL = u
			if _, err := f.Fetch(context.Background(), u); err == nil {
				t.Error("unsafe redirect accepted from metadata")
			}
		})
	}
}

// Both requests must obtain the same predecessor before either can rotate it.
type refreshBarrierStore struct {
	TokenStore
	ready   chan struct{}
	release chan struct{}
}

func (s *refreshBarrierStore) GetTokenByRefresh(token string) (*TokenInfo, error) {
	info, err := s.TokenStore.GetTokenByRefresh(token)
	s.ready <- struct{}{}
	<-s.release
	return info, err
}

func TestSecurityRefreshSingleSuccessor(t *testing.T) {
	for _, fileBacked := range []bool{false, true} {
		name := "memory"
		if fileBacked {
			name = "file"
		}
		t.Run(name, func(t *testing.T) {
			var base TokenStore
			var path string
			if fileBacked {
				f, p := newTestFileStore(t)
				defer f.Close()
				base, path = f, p
			} else {
				m := NewMemoryTokenStore()
				defer m.Close()
				base = m
			}
			if err := base.StoreToken(sampleToken("old-access", "old-refresh")); err != nil {
				t.Fatal(err)
			}
			if err := base.StoreToken(sampleToken("other-access", "other-refresh")); err != nil {
				t.Fatal(err)
			}
			barrier := &refreshBarrierStore{TokenStore: base, ready: make(chan struct{}, 2), release: make(chan struct{})}
			s := securityServer(t, barrier)
			results := make(chan *httptest.ResponseRecorder, 2)
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					results <- tokenRequest(s, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"old-refresh"}})
				}()
			}
			<-barrier.ready
			<-barrier.ready
			close(barrier.release)
			wg.Wait()
			close(results)
			successes := 0
			var successor string
			for w := range results {
				if w.Code == http.StatusOK {
					successes++
					var result map[string]any
					if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					successor = result["refresh_token"].(string)
				} else if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_grant") {
					t.Errorf("unexpected failure: %d %s", w.Code, w.Body.String())
				}
			}
			if successes != 1 {
				t.Errorf("got %d successful rotations, want exactly one", successes)
			}
			if fileBacked {
				reopened, err := NewFileTokenStore(path, testLogger())
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				base = reopened
			}
			if _, err := base.GetTokenByRefresh("old-refresh"); err != ErrTokenNotFound {
				t.Errorf("predecessor still refreshable: %v", err)
			}
			if _, err := base.GetTokenByAccess("old-access"); err != ErrTokenNotFound {
				t.Errorf("predecessor bearer still valid: %v", err)
			}
			if _, err := base.GetTokenByRefresh(successor); err != nil {
				t.Errorf("successor missing: %v", err)
			}
			if _, err := base.GetTokenByRefresh("other-refresh"); err != nil {
				t.Errorf("unrelated session changed: %v", err)
			}
		})
	}
}

func TestSecurityRefreshPersistenceFailure(t *testing.T) {
	f, path := newTestFileStore(t)
	defer f.Close()
	if err := f.StoreToken(sampleToken("old-access", "old-refresh")); err != nil {
		t.Fatal(err)
	}
	// An existing directory makes atomic rename fail while preserving the real snapshot.
	f.path = t.TempDir()
	s := securityServer(t, f)
	w := tokenRequest(s, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"old-refresh"}})
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), `"access_token"`) {
		t.Errorf("uncommitted credentials acknowledged: %d %s", w.Code, w.Body.String())
	}
	if _, err := f.GetTokenByRefresh("old-refresh"); err != nil {
		t.Errorf("failed rotation consumed predecessor in memory: %v", err)
	}
	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.GetTokenByRefresh("old-refresh"); err != nil {
		t.Errorf("failed rotation changed disk: %v", err)
	}
}
