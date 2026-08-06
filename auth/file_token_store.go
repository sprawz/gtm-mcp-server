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
	f.persist()
	return nil
}

// DeleteToken persists after delegating to the in-memory store.
func (f *FileTokenStore) DeleteToken(accessToken string) error {
	if err := f.MemoryTokenStore.DeleteToken(accessToken); err != nil {
		return err
	}
	f.persist()
	return nil
}

// UpdateGoogleToken persists after delegating to the in-memory store.
func (f *FileTokenStore) UpdateGoogleToken(accessToken string, googleToken *oauth2.Token) error {
	if err := f.MemoryTokenStore.UpdateGoogleToken(accessToken, googleToken); err != nil {
		return err
	}
	f.persist()
	return nil
}

// ExtendTokenExpiry persists after delegating to the in-memory store.
func (f *FileTokenStore) ExtendTokenExpiry(accessToken string, newExpiry time.Time) error {
	if err := f.MemoryTokenStore.ExtendTokenExpiry(accessToken, newExpiry); err != nil {
		return err
	}
	f.persist()
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

// persist writes a snapshot of the current tokens to disk. It is best-effort:
// a write failure is logged but does not fail the originating request, since
// the in-memory state is still correct for the running process.
func (f *FileTokenStore) persist() {
	f.saveMu.Lock()
	defer f.saveMu.Unlock()

	state := persistedState{Tokens: f.snapshot()}

	data, err := json.Marshal(state)
	if err != nil {
		if f.logger != nil {
			f.logger.Warn("failed to marshal token store", "error", err)
		}
		return
	}

	// Atomic write: write to a temp file then rename, so a crash mid-write
	// cannot corrupt the existing file.
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		if f.logger != nil {
			f.logger.Warn("failed to write token store", "error", err, "path", tmp)
		}
		return
	}
	if err := os.Rename(tmp, f.path); err != nil {
		if f.logger != nil {
			f.logger.Warn("failed to rename token store", "error", err, "path", f.path)
		}
	}
}

// snapshot returns the current tokens under the read lock. Holding the lock for
// the duration of the copy prevents a concurrent-map-iteration panic against
// writers; marshalling happens after the lock is released in persist.
func (f *FileTokenStore) snapshot() []*TokenInfo {
	f.MemoryTokenStore.mu.RLock()
	defer f.MemoryTokenStore.mu.RUnlock()

	out := make([]*TokenInfo, 0, len(f.MemoryTokenStore.tokens))
	for _, info := range f.MemoryTokenStore.tokens {
		out = append(out, info)
	}
	return out
}
