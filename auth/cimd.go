package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

// CIMD (Client ID Metadata Documents) support per
// draft-ietf-oauth-client-id-metadata-document-00, required by the MCP
// 2026-07-28 authorization spec. A client_id may be an HTTPS URL pointing
// to a JSON document describing the client.

const (
	cimdMaxBodyBytes = 64 << 10 // 64 KiB is ample for a metadata document
	cimdFetchTimeout = 10 * time.Second
	cimdDefaultTTL   = 5 * time.Minute
	cimdMaxTTL       = 24 * time.Hour
	cimdMaxRedirects = 5
	// cimdMaxCacheEntries bounds the cache. /authorize is unauthenticated and
	// entries are keyed by the caller-supplied client_id, so without a cap one
	// domain serving valid documents at unlimited paths grows it without limit.
	cimdMaxCacheEntries = 256
)

// IsClientIDMetadataURL reports whether clientID is a valid CIMD identifier:
// an https URL with a non-empty path and no fragment.
func IsClientIDMetadataURL(clientID string) bool {
	u, err := url.Parse(clientID)
	if err != nil {
		return false
	}
	return u.Scheme == "https" &&
		u.Host != "" &&
		u.Path != "" && u.Path != "/" &&
		u.Fragment == "" && u.RawFragment == ""
}

// clientIDMetadataDocument is the wire format of a CIMD document.
type clientIDMetadataDocument struct {
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

type cimdCacheEntry struct {
	client    *ClientInfo
	expiresAt time.Time
}

// CIMDFetcher fetches, validates, and caches Client ID Metadata Documents.
type CIMDFetcher struct {
	// HTTPClient may be overridden in tests. Defaults to a client with a
	// conservative timeout.
	HTTPClient *http.Client
	// AllowPrivateHosts disables the SSRF guard (loopback/private/link-local
	// address rejection). Only for tests.
	AllowPrivateHosts bool

	mu    sync.Mutex
	cache map[string]cimdCacheEntry
}

// NewCIMDFetcher creates a fetcher with safe defaults.
func NewCIMDFetcher() *CIMDFetcher {
	f := &CIMDFetcher{cache: make(map[string]cimdCacheEntry)}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Clone inherits ProxyFromEnvironment. Through a proxy the Control hook
	// below would only ever see the proxy's own address, never the target,
	// so the guard would be silently blind. CIMD documents are public
	// internet resources; fetch them directly.
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{
		Timeout:   cimdFetchTimeout,
		KeepAlive: 30 * time.Second,
		Control:   f.dialControl,
	}).DialContext
	f.HTTPClient = &http.Client{Timeout: cimdFetchTimeout, Transport: transport}
	return f
}

// dialControl runs after DNS resolution, on the address actually being
// dialed. That closes the window rejectPrivateHost cannot: a name that
// resolves public during the pre-flight check and private at connect time
// (DNS rebinding), and any redirect hop that never went through the check.
func (f *CIMDFetcher) dialControl(network, address string, c syscall.RawConn) error {
	if f.AllowPrivateHosts {
		return nil
	}
	return rejectPrivateAddr(network, address, c)
}

// checkRedirect vets every hop the client is asked to follow. Without it Go
// follows redirects with no further validation, letting a public host bounce
// the fetch to an internal one.
func (f *CIMDFetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= cimdMaxRedirects {
		return fmt.Errorf("client metadata fetch exceeded %d redirects", cimdMaxRedirects)
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("client metadata redirect to non-https URL rejected")
	}
	if f.AllowPrivateHosts {
		return nil
	}
	return rejectPrivateHost(req.Context(), req.URL.Hostname())
}

// clientForFetch returns HTTPClient with the redirect guard installed. It
// copies rather than mutates so that an injected client (tests) is still
// guarded without being modified.
func (f *CIMDFetcher) clientForFetch() *http.Client {
	c := *f.HTTPClient
	c.CheckRedirect = f.checkRedirect
	return &c
}

// Fetch retrieves and validates the metadata document at clientID.
// Results are cached honoring Cache-Control max-age (bounded, with a default).
func (f *CIMDFetcher) Fetch(ctx context.Context, clientID string) (*ClientInfo, error) {
	if !IsClientIDMetadataURL(clientID) {
		return nil, fmt.Errorf("client_id is not a valid metadata document URL")
	}

	f.mu.Lock()
	if entry, ok := f.cache[clientID]; ok && time.Now().Before(entry.expiresAt) {
		f.mu.Unlock()
		return entry.client, nil
	}
	f.mu.Unlock()

	u, _ := url.Parse(clientID)
	if !f.AllowPrivateHosts {
		if err := rejectPrivateHost(ctx, u.Hostname()); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid metadata URL: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := f.clientForFetch().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch client metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("client metadata fetch returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, cimdMaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read client metadata: %w", err)
	}
	if len(body) > cimdMaxBodyBytes {
		return nil, fmt.Errorf("client metadata document too large")
	}

	var doc clientIDMetadataDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("client metadata is not valid JSON: %w", err)
	}

	// The document's client_id MUST match the URL it was fetched from exactly.
	if doc.ClientID != clientID {
		return nil, fmt.Errorf("client metadata client_id %q does not match document URL", doc.ClientID)
	}
	if doc.ClientName == "" {
		return nil, fmt.Errorf("client metadata missing required field client_name")
	}
	if len(doc.RedirectURIs) == 0 {
		return nil, fmt.Errorf("client metadata missing required field redirect_uris")
	}

	for _, uri := range doc.RedirectURIs {
		if !isValidRedirectURI(uri) {
			return nil, fmt.Errorf("client metadata contains an invalid redirect_uri")
		}
	}

	client := &ClientInfo{
		ClientID:     doc.ClientID,
		ClientName:   doc.ClientName,
		RedirectURIs: doc.RedirectURIs,
		CreatedAt:    time.Now(),
	}

	ttl := cacheTTLFromResponse(resp)
	f.storeInCache(clientID, cimdCacheEntry{client: client, expiresAt: time.Now().Add(ttl)})

	return client, nil
}

// storeInCache inserts an entry, first dropping every expired entry (lookup
// only skips them, so otherwise nothing is ever reclaimed) and then evicting
// until there is room. The victim comes from Go's unspecified, runtime-
// randomized map iteration order rather than from expiry order: TTLs are
// caller-influenced via Cache-Control, so evicting the soonest-to-expire
// would let a flood of long-TTL entries preferentially push out legitimate
// short-TTL ones. The property relied on is that a caller cannot predict or
// bias the victim, not that the choice is uniform.
func (f *CIMDFetcher) storeInCache(clientID string, entry cimdCacheEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	for k, e := range f.cache {
		if !now.Before(e.expiresAt) {
			delete(f.cache, k)
		}
	}
	for k := range f.cache {
		if len(f.cache) < cimdMaxCacheEntries {
			break
		}
		delete(f.cache, k)
	}

	f.cache[clientID] = entry
}

// cacheTTLFromResponse derives a bounded cache TTL from Cache-Control max-age.
func cacheTTLFromResponse(resp *http.Response) time.Duration {
	cc := resp.Header.Get("Cache-Control")
	for _, part := range strings.Split(cc, ",") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "max-age="); ok {
			var secs int
			if _, err := fmt.Sscanf(v, "%d", &secs); err == nil && secs > 0 {
				ttl := time.Duration(secs) * time.Second
				if ttl > cimdMaxTTL {
					return cimdMaxTTL
				}
				return ttl
			}
		}
	}
	return cimdDefaultTTL
}

// rejectPrivateAddr fails for a host:port whose IP is not publicly routable.
// Intended as a net.Dialer.Control hook.
func rejectPrivateAddr(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("malformed dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("dial address %q is not a resolved IP", address)
	}
	if !isPublicIP(ip) {
		return fmt.Errorf("client metadata host resolves to a non-public address")
	}
	return nil
}

// nonPublicCIDRs covers ranges that are not publicly routable but that net.IP
// does not classify: IsPrivate only knows RFC1918 and fc00::/7. Without these,
// a CGNAT or NAT64 address reaches real infrastructure — 64:ff9b::a9fe:a9fe is
// the cloud metadata service on any NAT64 network.
var nonPublicCIDRs = parseCIDRs(
	"100.64.0.0/10", // RFC 6598 carrier-grade NAT
	"192.0.0.0/24",  // RFC 6890 IETF protocol assignments
	"198.18.0.0/15", // RFC 2544 benchmarking
	"240.0.0.0/4",   // class E, includes 255.255.255.255
	"64:ff9b::/96",  // RFC 6052 NAT64
	"64:ff9b:1::/48",
	"2002::/16",      // 6to4, reaches the embedded IPv4 address
	"100::/64",       // RFC 6666 discard-only
	"192.88.99.0/24", // RFC 7526 6to4 relay anycast
	"2001::/32",      // Teredo, embeds an IPv4 address
	"2001:10::/28",   // ORCHID
	"2001:20::/28",   // ORCHIDv2
	// IPv4-compatible IPv6 (::a.b.c.d). Deprecated by RFC 4291 and normally
	// unroutable, but To4() only normalizes the ::ffff: form, so ::7f00:1
	// would otherwise pass every net.IP check as public.
	"::/96",
)

func parseCIDRs(cidrs ...string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("auth: bad CIDR in nonPublicCIDRs: " + c)
		}
		nets = append(nets, n)
	}
	return nets
}

// isPublicIP reports whether ip is publicly routable.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, n := range nonPublicCIDRs {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

// rejectPrivateHost resolves host and fails if any address is loopback,
// private, link-local, or otherwise non-global (SSRF guard).
func rejectPrivateHost(ctx context.Context, host string) error {
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to resolve client metadata host: %w", err)
	}
	for _, ip := range ips {
		if !isPublicIP(ip.IP) {
			return fmt.Errorf("client metadata host resolves to a non-public address")
		}
	}
	return nil
}
