package keyring

import "errors"

var ErrNotFound = errors.New("secret not found")

// Store abstracts secret storage (Keychain, env vars, memory).
type Store interface {
	// Get retrieves a secret by workspace ID and key name.
	Get(workspaceID, key string) (string, error)
	// Set stores a secret for a workspace ID and key name.
	Set(workspaceID, key, value string) error
	// Delete removes a secret for a workspace ID and key name.
	Delete(workspaceID, key string) error
}

const (
	KeyToken  = "token"  // xoxc token
	KeyCookie = "cookie" // d cookie (xoxd)
)
