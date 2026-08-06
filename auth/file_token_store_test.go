package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// newTestFileStore creates a FileTokenStore backed by a fresh temp file.
func newTestFileStore(t *testing.T) (*FileTokenStore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.json")
	store, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("NewFileTokenStore failed: %v", err)
	}
	return store, path
}

func sampleToken(access, refresh string) *TokenInfo {
	return &TokenInfo{
		AccessToken:      access,
		RefreshToken:     refresh,
		ExpiresAt:        time.Now().Add(1 * time.Hour),
		RefreshExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		GoogleToken: &oauth2.Token{
			AccessToken:  "google-access",
			RefreshToken: "google-refresh",
			Expiry:       time.Now().Add(1 * time.Hour),
		},
		ClientID:  "test-client",
		CreatedAt: time.Now(),
	}
}

// The core bug fix: a token stored before a "restart" must still be there
// after the process (and its in-memory map) is recreated from the same file.
func TestFileTokenStore_SurvivesRestart(t *testing.T) {
	store, path := newTestFileStore(t)
	if err := store.StoreToken(sampleToken("access-1", "refresh-1")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	store.Close()

	// Simulate a restart: brand new store, same file.
	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("reopen NewFileTokenStore failed: %v", err)
	}
	defer reopened.Close()

	retrieved, err := reopened.GetTokenByAccess("access-1")
	if err != nil {
		t.Fatalf("token lost across restart: %v", err)
	}
	if retrieved.GoogleToken == nil || retrieved.GoogleToken.RefreshToken != "google-refresh" {
		t.Errorf("Google refresh token not persisted, got %+v", retrieved.GoogleToken)
	}
}

// After a restart, refresh-token lookups must work, which requires the
// secondary refresh index to be rebuilt from the persisted file.
func TestFileTokenStore_RefreshIndexRebuiltAfterRestart(t *testing.T) {
	store, path := newTestFileStore(t)
	if err := store.StoreToken(sampleToken("access-2", "refresh-2")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	store.Close()

	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer reopened.Close()

	if _, err := reopened.GetTokenByRefresh("refresh-2"); err != nil {
		t.Fatalf("GetTokenByRefresh after restart failed: %v", err)
	}
}

func TestFileTokenStore_DeletePersists(t *testing.T) {
	store, path := newTestFileStore(t)
	if err := store.StoreToken(sampleToken("access-3", "refresh-3")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	if err := store.DeleteToken("access-3"); err != nil {
		t.Fatalf("DeleteToken failed: %v", err)
	}
	store.Close()

	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer reopened.Close()

	if _, err := reopened.GetTokenByAccess("access-3"); err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after deletion+restart, got %v", err)
	}
}

func TestFileTokenStore_UpdateGoogleTokenPersists(t *testing.T) {
	store, path := newTestFileStore(t)
	if err := store.StoreToken(sampleToken("access-4", "refresh-4")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	newGoogle := &oauth2.Token{AccessToken: "rotated", RefreshToken: "google-refresh", Expiry: time.Now().Add(time.Hour)}
	if err := store.UpdateGoogleToken("access-4", newGoogle); err != nil {
		t.Fatalf("UpdateGoogleToken failed: %v", err)
	}
	store.Close()

	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer reopened.Close()

	retrieved, err := reopened.GetTokenByAccess("access-4")
	if err != nil {
		t.Fatalf("GetTokenByAccess failed: %v", err)
	}
	if retrieved.GoogleToken.AccessToken != "rotated" {
		t.Errorf("expected rotated google token, got %q", retrieved.GoogleToken.AccessToken)
	}
}

// First run: no file on disk yet. The store must start empty, not error.
func TestFileTokenStore_FirstRunNoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "tokens.json")
	store, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("NewFileTokenStore on missing file should succeed, got %v", err)
	}
	defer store.Close()

	if _, err := store.GetTokenByAccess("anything"); err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound on empty store, got %v", err)
	}
}

// Tokens whose refresh window has already lapsed are dead weight; they should
// not be reloaded after a restart.
func TestFileTokenStore_PrunesExpiredRefreshOnLoad(t *testing.T) {
	store, path := newTestFileStore(t)
	dead := sampleToken("access-dead", "refresh-dead")
	dead.RefreshExpiresAt = time.Now().Add(-1 * time.Hour) // refresh window lapsed
	if err := store.StoreToken(dead); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	store.Close()

	reopened, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer reopened.Close()

	if _, err := reopened.GetTokenByAccessIncludeExpired("access-dead"); err != ErrTokenNotFound {
		t.Errorf("expected expired-refresh token to be pruned on load, got %v", err)
	}
}

// Persisting marshals a snapshot outside the store lock while other requests
// mutate the same entries in place, so the snapshot has to be a copy. Run
// under -race, this fails if the snapshot aliases the stored structs.
func TestFileTokenStore_ConcurrentPersistAndMutation(t *testing.T) {
	store, _ := newTestFileStore(t)
	defer store.Close()

	if err := store.StoreToken(sampleToken("access-1", "refresh-1")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 25; n++ {
				if err := store.ExtendTokenExpiry("access-1", time.Now().Add(time.Duration(n)*time.Minute)); err != nil {
					t.Errorf("ExtendTokenExpiry failed: %v", err)
					return
				}
				if err := store.StoreToken(sampleToken(fmt.Sprintf("access-%d-%d", i, n), "")); err != nil {
					t.Errorf("StoreToken failed: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

// os.WriteFile follows symlinks, so a fixed "<path>.tmp" would let anyone who
// can create files in the directory siphon every refresh token elsewhere.
func TestFileTokenStore_DoesNotWriteThroughPlantedTempSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	target := filepath.Join(dir, "attacker-readable")
	if err := os.WriteFile(target, []byte("untouched"), 0o644); err != nil {
		t.Fatalf("seeding target failed: %v", err)
	}
	if err := os.Symlink(target, path+".tmp"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	store, err := NewFileTokenStore(path, testLogger())
	if err != nil {
		t.Fatalf("NewFileTokenStore failed: %v", err)
	}
	defer store.Close()

	if err := store.StoreToken(sampleToken("access-1", "refresh-1")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target failed: %v", err)
	}
	if string(got) != "untouched" {
		t.Errorf("tokens were written through the planted symlink: %s", got)
	}
}

// A revocation that never reaches disk is valid again after a restart, so
// unlike the other paths DeleteToken must not swallow a persist failure.
func TestFileTokenStore_DeleteReportsPersistFailure(t *testing.T) {
	store, path := newTestFileStore(t)
	defer store.Close()

	if err := store.StoreToken(sampleToken("access-1", "refresh-1")); err != nil {
		t.Fatalf("StoreToken failed: %v", err)
	}
	// Remove the directory out from under the store so the write cannot land.
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatalf("removing store dir failed: %v", err)
	}

	if err := store.DeleteToken("access-1"); err == nil {
		t.Error("expected DeleteToken to report the persist failure")
	}
}
