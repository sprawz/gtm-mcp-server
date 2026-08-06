package auth

import (
	"testing"
	"time"
)

// A token whose access has expired but whose refresh token is still valid must
// survive cleanup: the refresh token can still mint new access tokens, so
// purging it forces a needless re-authentication. This is the root cause of the
// "re-authenticate several times a day" bug.
func TestMemoryTokenStore_Cleanup_KeepsTokenWithValidRefresh(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	now := time.Now()
	store.StoreToken(&TokenInfo{
		AccessToken:      "access-1",
		RefreshToken:     "refresh-1",
		ExpiresAt:        now.Add(-2 * time.Hour),      // access expired well past the +1h grace
		RefreshExpiresAt: now.Add(29 * 24 * time.Hour), // refresh still valid for ~29 days
		CreatedAt:        now.Add(-10 * time.Hour),
	})

	store.purgeExpired(now)

	if _, err := store.GetTokenByRefresh("refresh-1"); err != nil {
		t.Fatalf("token with valid refresh window was purged: %v", err)
	}
}

// Once the refresh window has lapsed the token is dead weight and must be removed.
func TestMemoryTokenStore_Cleanup_RemovesTokenWithExpiredRefresh(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	now := time.Now()
	store.StoreToken(&TokenInfo{
		AccessToken:      "access-2",
		RefreshToken:     "refresh-2",
		ExpiresAt:        now.Add(-31 * 24 * time.Hour),
		RefreshExpiresAt: now.Add(-1 * time.Hour), // refresh window lapsed
		CreatedAt:        now.Add(-31 * 24 * time.Hour),
	})

	store.purgeExpired(now)

	if _, err := store.GetTokenByRefresh("refresh-2"); err != ErrTokenNotFound {
		t.Errorf("expected expired-refresh token to be purged, got %v", err)
	}
}

// Short-lived auth-code tokens have no refresh token; they must still be purged
// once their (short) access expiry has lapsed, so they don't leak forever.
func TestMemoryTokenStore_Cleanup_RemovesExpiredAuthCodeToken(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	now := time.Now()
	store.StoreToken(&TokenInfo{
		AccessToken: "code-token",
		// no RefreshToken, no RefreshExpiresAt — like the temporary auth-code token
		ExpiresAt: now.Add(-2 * time.Hour),
		CreatedAt: now.Add(-2 * time.Hour),
	})

	store.purgeExpired(now)

	if _, err := store.GetTokenByAccessIncludeExpired("code-token"); err != ErrTokenNotFound {
		t.Errorf("expected expired auth-code token to be purged, got %v", err)
	}
}

// An entry carrying a refresh token but no recorded refresh window has no
// expiry to check against, so treating it as refreshable would keep it — and
// keep it resolvable through GetTokenByRefresh — for the process lifetime. No
// current caller stores that shape; the guard is for the next one.
func TestMemoryTokenStore_Cleanup_RemovesTokenWithNoRefreshWindow(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	now := time.Now()
	store.StoreToken(&TokenInfo{
		AccessToken:  "access-3",
		RefreshToken: "refresh-3",
		ExpiresAt:    now.Add(-2 * time.Hour),
		// RefreshExpiresAt deliberately left zero
		CreatedAt: now.Add(-10 * time.Hour),
	})

	store.purgeExpired(now)

	if _, err := store.GetTokenByRefresh("refresh-3"); err != ErrTokenNotFound {
		t.Errorf("expected token with no refresh window to be purged, got %v", err)
	}
}

// The access token stays usable until its own expiry, so an entry whose
// refresh window has lapsed must not be yanked out from under a live session.
func TestMemoryTokenStore_Cleanup_KeepsLiveAccessAfterRefreshLapses(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	now := time.Now()
	store.StoreToken(&TokenInfo{
		AccessToken:      "access-4",
		RefreshToken:     "refresh-4",
		ExpiresAt:        now.Add(1 * time.Hour),  // still valid
		RefreshExpiresAt: now.Add(-1 * time.Hour), // refresh window lapsed
		CreatedAt:        now.Add(-30 * 24 * time.Hour),
	})

	store.purgeExpired(now)

	if _, err := store.GetTokenByAccess("access-4"); err != nil {
		t.Errorf("live access token was purged with its lapsed refresh window: %v", err)
	}
}
