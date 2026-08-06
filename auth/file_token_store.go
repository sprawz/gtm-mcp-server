package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// FileTokenStore is a TokenStore that persists issued tokens to a JSON file so
// that sessions survive process restarts (container redeploys, crashes, host
// reboots). It embeds MemoryTokenStore for all in-memory behaviour and writes a
// snapshot to disk after every token mutation.
//
// Only issued tokens are persisted. OAuth flow states (10-minute lifetime) and
// dynamically-registered clients are intentionally not persisted: they are
// short-lived or re-created by the client on reconnect, and a valid bearer
// token alone is enough to keep a user authenticated across a restart.
type FileTokenStore struct {
	*MemoryTokenStore

	path   string
	logger *slog.Logger
	saveMu sync.Mutex // serialises writes to the file
}

// persistedState is the on-disk shape of the store.
type persistedState struct {
	Tokens []*TokenInfo `json:"tokens"`
}

// NewFileTokenStore creates a file-backed token store, loading any existing
// tokens from path. A missing file is treated as an empty store (first run).
func NewFileTokenStore(path string, logger *slog.Logger) (*FileTokenStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create token store directory: %w", err)
	}

	f := &FileTokenStore{
		MemoryTokenStore: NewMemoryTokenStore(),
		path:             path,
		logger:           logger,
	}

	if err := f.load(); err != nil {
		return nil, fmt.Errorf("failed to load token store from %s: %w", path, err)
	}

	return f, nil
}

// StoreToken persists after delegating to the in-memory store.
func (f *FileTokenStore) StoreToken(info *TokenInfo) error {
	if err := f.MemoryTokenStore.StoreToken(info); err != nil {
		return err
	}
	f.persistBestEffort()
	return nil
}

// DeleteToken persists after delegating to the in-memory store.
func (f *FileTokenStore) DeleteToken(accessToken string) error {
	if err := f.MemoryTokenStore.DeleteToken(accessToken); err != nil {
		return err
	}
	// A revoked token that stays on disk is valid again after a restart, so
	// this is the one path where a persist failure must reach the caller.
	if err := f.persist(); err != nil {
		if f.logger != nil {
			f.logger.Error("failed to persist token revocation", "error", err, "path", f.path)
		}
		return err
	}
	return nil
}

// UpdateGoogleToken persists after delegating to the in-memory store.
func (f *FileTokenStore) UpdateGoogleToken(accessToken string, googleToken *oauth2.Token) error {
	if err := f.MemoryTokenStore.UpdateGoogleToken(accessToken, googleToken); err != nil {
		return err
	}
	f.persistBestEffort()
	return nil
}

// ExtendTokenExpiry persists after delegating to the in-memory store.
func (f *FileTokenStore) ExtendTokenExpiry(accessToken string, newExpiry time.Time) error {
	if err := f.MemoryTokenStore.ExtendTokenExpiry(accessToken, newExpiry); err != nil {
		return err
	}
	f.persistBestEffort()
	return nil
}

// load reads tokens from disk into the in-memory maps. Tokens whose refresh
// window has already lapsed are skipped — they cannot be refreshed anyway.
func (f *FileTokenStore) load() error {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first run, nothing to load
		}
		return err
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	now := time.Now()

	f.MemoryTokenStore.mu.Lock()
	defer f.MemoryTokenStore.mu.Unlock()

	loaded := 0
	for _, info := range state.Tokens {
		if info == nil || info.AccessToken == "" {
			continue
		}
		if !info.RefreshExpiresAt.IsZero() && now.After(info.RefreshExpiresAt) {
			continue
		}
		f.MemoryTokenStore.tokens[info.AccessToken] = info
		if info.RefreshToken != "" {
			f.MemoryTokenStore.refreshIndex[info.RefreshToken] = info.AccessToken
		}
		loaded++
	}

	if f.logger != nil {
		f.logger.Info("loaded persisted tokens", "count", loaded, "path", f.path)
	}
	return nil
}

// persist writes a snapshot of the current tokens to disk, returning the first
// error it hits. Callers that only add or update state treat it as best-effort
// — the in-memory state is still correct for the running process — but a
// revocation that is not on disk would come back to life on restart, so
// DeleteToken surfaces the failure.
func (f *FileTokenStore) persist() error {
	f.saveMu.Lock()
	defer f.saveMu.Unlock()

	state := persistedState{Tokens: f.snapshot()}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal token store: %w", err)
	}

	// Atomic write: write to a temp file then rename, so a crash mid-write
	// cannot corrupt the existing file. The temp file is created exclusively
	// with a random name rather than reusing a predictable path: os.WriteFile
	// follows symlinks, so a pre-planted <path>.tmp link would siphon every
	// refresh token to a location of the attacker's choosing.
	tmp, err := os.CreateTemp(filepath.Dir(f.path), filepath.Base(f.path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create token store temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename below succeeds

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to set token store permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write token store: %w", err)
	}
	// Flush before the rename, or a power loss can leave a truncated file at
	// the final path — the corruption the atomic write exists to prevent.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to flush token store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close token store: %w", err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("failed to rename token store: %w", err)
	}
	return nil
}

// persistBestEffort records a persist failure without failing the originating
// request.
func (f *FileTokenStore) persistBestEffort() {
	if err := f.persist(); err != nil && f.logger != nil {
		f.logger.Warn("failed to persist token store", "error", err, "path", f.path)
	}
}

// snapshot returns a copy of the current tokens under the read lock. The
// entries must be cloned, not aliased: marshalling happens after the lock is
// released, while ExtendTokenExpiry and UpdateGoogleToken mutate the stored
// structs in place, so sharing the pointers would race and could serialise a
// torn ExpiresAt as the authoritative expiry.
func (f *FileTokenStore) snapshot() []*TokenInfo {
	f.MemoryTokenStore.mu.RLock()
	defer f.MemoryTokenStore.mu.RUnlock()

	out := make([]*TokenInfo, 0, len(f.MemoryTokenStore.tokens))
	for _, info := range f.MemoryTokenStore.tokens {
		out = append(out, cloneTokenInfo(info))
	}
	return out
}
