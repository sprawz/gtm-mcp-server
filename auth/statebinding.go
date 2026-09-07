package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

// Binding the Google federation leg to the browser that started it.
//
// Without this, `state` is only a server-side lookup key that anyone can mint.
// An attacker registers a client (registration is open, as MCP requires), calls
// /authorize to get a state of their choosing, and lures a victim into
// completing Google consent with that state. Google returns the victim's code
// to our callback; we exchange it, see the *victim's* identity — which passes
// the domain allowlist, because the victim is legitimately allowed — and hand
// the resulting authorization code to the *attacker's* registered redirect_uri.
// The attacker then completes the token exchange with the PKCE verifier they
// chose, and holds an access token backed by the victim's Google credentials.
//
// At /authorize we set an opaque, HttpOnly cookie and store only its SHA-256
// next to the state row. At the callback we require a cookie whose hash
// matches. The victim's browser never visited the attacker's /authorize, so it
// carries no such cookie and the flow is refused.
//
// Cookies have no origin or scheme integrity, so the binding alone would still
// be plantable: an attacker holding a sibling subdomain, or able to intercept
// plain http to any host under the parent domain, could set a Domain-scoped
// cookie the victim's browser would send here. The `__Host-` prefix closes
// that — a browser accepts such a cookie only for the exact host that set it,
// with no Domain attribute — at the cost of mandating Path=/ and Secure.
//
// A plain-http run on a non-localhost origin cannot set a Secure cookie at all,
// so the name is chosen from the issuer scheme: prefixed over https, plain
// otherwise. The callback accepts only the name that matches its own flow's
// issuer. Accepting either would give the whole prefix away, since the
// unprefixed name is exactly the one an attacker can plant.
const (
	bindingCookiePlainName = "gtm_fed_binding"
	bindingCookieHostName  = "__Host-" + bindingCookiePlainName
	bindingCookieMaxAge    = 600
)

// issuerIsHTTPS parses rather than prefix-matches, so an issuer that differs
// only in case or leading space cannot silently select the weaker regime.
func issuerIsHTTPS(issuer string) bool {
	u, err := url.Parse(strings.TrimSpace(issuer))
	return err == nil && u.Scheme == "https"
}

// bindingRegimeIsHTTPS decides which cookie regime a flow uses, taking the
// stronger of the configured base URL and the issuer resolved for the flow.
//
// The resolved issuer alone is not safe to trust here. `URLResolver.Resolve`
// derives the scheme from `X-Forwarded-Proto`, which — unlike `X-Forwarded-For`
// in the rate limiter — carries no TrustProxy gate. The attacker is the party
// who calls /authorize in this attack, so they own that header: suppressing it
// on an https deployment would otherwise hand them the unprefixed, non-Secure
// cookie, which is exactly the one a sibling subdomain can plant, reopening the
// injection path this whole mechanism exists to close.
//
// Taking the stronger of the two means the regime can only ever be raised by
// the request, never lowered below what the operator configured.
func (s *Server) bindingRegimeIsHTTPS(stateIssuer string) bool {
	return issuerIsHTTPS(s.baseURL) || issuerIsHTTPS(stateIssuer)
}

// bindingCookieNameFor picks the name a flow uses. It must give the same answer
// at /authorize and at the callback, so both sides derive it the same way from
// the issuer recorded on the state row.
func bindingCookieNameFor(httpsRegime bool) string {
	if httpsRegime {
		return bindingCookieHostName
	}
	return bindingCookiePlainName
}

func hashBinding(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// bindingMatches compares in constant time and treats either value being
// absent as a mismatch, so a state row without a recorded binding can never be
// satisfied by a request without a cookie.
func bindingMatches(cookieValue, expectedHash string) bool {
	if cookieValue == "" || expectedHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashBinding(cookieValue)), []byte(expectedHash)) == 1
}

// setBindingCookie issues the binding to the browser starting the flow.
//
// SameSite=Lax is deliberate: Google's redirect back to us is a cross-site
// top-level GET navigation, which Lax permits and Strict would drop. The
// cookie is marked Secure whenever the issuer we resolved for this request is
// https, so a plain-http local run still works.
func setBindingCookie(w http.ResponseWriter, value string, httpsRegime bool) {
	// The prefix is only honoured with Path=/ and Secure and no Domain, so the
	// https path cannot keep the tighter callback scoping. The cookie is opaque,
	// HttpOnly and short-lived, and grants nothing without the matching state
	// row, so sending it on every request to this host is a fair trade for
	// closing the injection path.
	path := "/oauth/callback"
	if httpsRegime {
		path = "/"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     bindingCookieNameFor(httpsRegime),
		Value:    value,
		Path:     path,
		MaxAge:   bindingCookieMaxAge,
		HttpOnly: true,
		Secure:   httpsRegime,
		SameSite: http.SameSiteLaxMode,
	})
}

func bindingFromRequest(r *http.Request, httpsRegime bool) string {
	c, err := r.Cookie(bindingCookieNameFor(httpsRegime))
	if err != nil {
		return ""
	}
	return c.Value
}
