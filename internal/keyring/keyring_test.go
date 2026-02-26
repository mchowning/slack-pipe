package keyring_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mchowning/slack-pipe/internal/keyring"
)

// storeTest runs the full interface contract test against any Store implementation.
func storeTest(t *testing.T, store keyring.Store) {
	t.Helper()
	const wsID = "T12345"

	// Get on empty returns ErrNotFound
	_, err := store.Get(wsID, keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set then Get
	if err := store.Set(wsID, keyring.KeyToken, "xoxc-test"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := store.Get(wsID, keyring.KeyToken)
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if val != "xoxc-test" {
		t.Fatalf("expected xoxc-test, got %s", val)
	}

	// Delete then Get returns ErrNotFound
	if err := store.Delete(wsID, keyring.KeyToken); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = store.Get(wsID, keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Separate workspaces are isolated
	if err := store.Set("W1", keyring.KeyToken, "tok1"); err != nil {
		t.Fatalf("Set W1: %v", err)
	}
	if err := store.Set("W2", keyring.KeyToken, "tok2"); err != nil {
		t.Fatalf("Set W2: %v", err)
	}
	v1, _ := store.Get("W1", keyring.KeyToken)
	v2, _ := store.Get("W2", keyring.KeyToken)
	if v1 != "tok1" || v2 != "tok2" {
		t.Fatalf("workspace isolation failed: W1=%s W2=%s", v1, v2)
	}

	// Separate keys are isolated
	if err := store.Set(wsID, keyring.KeyToken, "tok"); err != nil {
		t.Fatalf("Set token: %v", err)
	}
	if err := store.Set(wsID, keyring.KeyCookie, "cookie"); err != nil {
		t.Fatalf("Set cookie: %v", err)
	}
	vt, _ := store.Get(wsID, keyring.KeyToken)
	vc, _ := store.Get(wsID, keyring.KeyCookie)
	if vt != "tok" || vc != "cookie" {
		t.Fatalf("key isolation failed: token=%s cookie=%s", vt, vc)
	}
}

func TestMemoryStore(t *testing.T) {
	storeTest(t, keyring.NewMemory())
}

func TestEnvStore(t *testing.T) {
	store := keyring.NewEnv()

	t.Setenv("SLACK_TOKEN", "xoxc-from-env")
	t.Setenv("SLACK_COOKIE", "xoxd-from-env")

	tok, err := store.Get("any-workspace", keyring.KeyToken)
	if err != nil || tok != "xoxc-from-env" {
		t.Fatalf("expected xoxc-from-env, got %s (err=%v)", tok, err)
	}

	cookie, err := store.Get("any-workspace", keyring.KeyCookie)
	if err != nil || cookie != "xoxd-from-env" {
		t.Fatalf("expected xoxd-from-env, got %s (err=%v)", cookie, err)
	}

	// Unset returns ErrNotFound
	t.Setenv("SLACK_TOKEN", "")
	os.Unsetenv("SLACK_TOKEN") //nolint:errcheck // test cleanup
	_, err = store.Get("any", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set and Delete return errors (read-only)
	if err := store.Set("ws", keyring.KeyToken, "val"); err == nil {
		t.Fatal("expected error on Set")
	}
	if err := store.Delete("ws", keyring.KeyToken); err == nil {
		t.Fatal("expected error on Delete")
	}
}

func TestFallbackGetFromPrimary(t *testing.T) {
	primary := keyring.NewMemory()
	secondary := keyring.NewMemory()
	fb := keyring.NewWithFallback(primary, secondary)

	if err := primary.Set("W1", keyring.KeyToken, "from-primary"); err != nil {
		t.Fatal(err)
	}
	if err := secondary.Set("W1", keyring.KeyToken, "from-secondary"); err != nil {
		t.Fatal(err)
	}

	val, err := fb.Get("W1", keyring.KeyToken)
	if err != nil {
		t.Fatal(err)
	}
	if val != "from-primary" {
		t.Fatalf("expected from-primary, got %s", val)
	}
}

func TestFallbackGetFromSecondary(t *testing.T) {
	primary := keyring.NewMemory()
	secondary := keyring.NewMemory()
	fb := keyring.NewWithFallback(primary, secondary)

	if err := secondary.Set("W1", keyring.KeyToken, "from-secondary"); err != nil {
		t.Fatal(err)
	}

	val, err := fb.Get("W1", keyring.KeyToken)
	if err != nil {
		t.Fatal(err)
	}
	if val != "from-secondary" {
		t.Fatalf("expected from-secondary, got %s", val)
	}
}

func TestFallbackGetNotFound(t *testing.T) {
	primary := keyring.NewMemory()
	secondary := keyring.NewMemory()
	fb := keyring.NewWithFallback(primary, secondary)

	_, err := fb.Get("W1", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFallbackSetWritesToPrimary(t *testing.T) {
	primary := keyring.NewMemory()
	secondary := keyring.NewMemory()
	fb := keyring.NewWithFallback(primary, secondary)

	if err := fb.Set("W1", keyring.KeyToken, "val"); err != nil {
		t.Fatal(err)
	}

	val, err := primary.Get("W1", keyring.KeyToken)
	if err != nil || val != "val" {
		t.Fatalf("expected val in primary, got %s (err=%v)", val, err)
	}

	_, err = secondary.Get("W1", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatal("expected secondary to be empty")
	}
}

func TestFallbackDeleteFromPrimary(t *testing.T) {
	primary := keyring.NewMemory()
	secondary := keyring.NewMemory()
	fb := keyring.NewWithFallback(primary, secondary)

	if err := primary.Set("W1", keyring.KeyToken, "val"); err != nil {
		t.Fatal(err)
	}
	if err := fb.Delete("W1", keyring.KeyToken); err != nil {
		t.Fatal(err)
	}

	_, err := primary.Get("W1", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatal("expected deleted from primary")
	}
}

// errorStore returns a non-ErrNotFound error to test fallback propagation.
type errorStore struct{}

func (e *errorStore) Get(_, _ string) (string, error) { return "", fmt.Errorf("keychain locked") }
func (e *errorStore) Set(_, _, _ string) error         { return fmt.Errorf("keychain locked") }
func (e *errorStore) Delete(_, _ string) error         { return fmt.Errorf("keychain locked") }

func TestFallbackPropagatesNonNotFoundError(t *testing.T) {
	primary := &errorStore{}
	secondary := keyring.NewMemory()
	if err := secondary.Set("W1", keyring.KeyToken, "from-secondary"); err != nil {
		t.Fatal(err)
	}
	fb := keyring.NewWithFallback(primary, secondary)

	// Should NOT fall back — should propagate the "keychain locked" error
	_, err := fb.Get("W1", keyring.KeyToken)
	if err == nil {
		t.Fatal("expected error")
	}
	if err == keyring.ErrNotFound {
		t.Fatal("should not be ErrNotFound — should be the original error")
	}
}

func TestEnvStoreUnknownKey(t *testing.T) {
	store := keyring.NewEnv()
	_, err := store.Get("ws", "unknown-key")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}
