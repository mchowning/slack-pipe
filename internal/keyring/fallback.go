package keyring

import "errors"

// Fallback tries a primary store, falling back to secondary only on ErrNotFound.
// Other errors (e.g., Keychain locked, permission denied) are returned as-is
// to avoid silently degrading security.
type Fallback struct {
	primary   Store
	secondary Store
}

func NewWithFallback(primary, secondary Store) *Fallback {
	return &Fallback{primary: primary, secondary: secondary}
}

func (f *Fallback) Get(workspaceID, key string) (string, error) {
	val, err := f.primary.Get(workspaceID, key)
	if err == nil {
		return val, nil
	}
	// Only fall back to secondary on ErrNotFound (primary doesn't have it).
	// Other errors (locked keychain, permission denied) should propagate.
	if errors.Is(err, ErrNotFound) {
		return f.secondary.Get(workspaceID, key)
	}
	return "", err
}

func (f *Fallback) Set(workspaceID, key, value string) error {
	// Always write to primary. Never silently fall back for writes —
	// if the keychain is unavailable, the user should know.
	return f.primary.Set(workspaceID, key, value)
}

func (f *Fallback) Delete(workspaceID, key string) error {
	// Always delete from primary. Same rationale as Set.
	return f.primary.Delete(workspaceID, key)
}
