package config_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mchowning/slack-pipe/internal/config"
)

// memFS is an in-memory filesystem for testing.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{
		files: make(map[string][]byte),
		dirs:  make(map[string]bool),
	}
}

func (m *memFS) ReadFile(path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}

func (m *memFS) MkdirAll(path string, _ os.FileMode) error {
	m.dirs[path] = true
	return nil
}

func (m *memFS) Exists(path string) bool {
	_, ok := m.files[path]
	return ok
}

func mustAdd(t *testing.T, store *config.Store, ws config.Workspace) {
	t.Helper()
	if err := store.AddWorkspace(ws); err != nil {
		t.Fatalf("AddWorkspace: %v", err)
	}
}

func TestAddAndGetWorkspace(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")

	ws := config.Workspace{ID: "T123", Name: "myteam", URL: "https://myteam.slack.com"}
	mustAdd(t, store, ws)

	// First workspace becomes default
	got, err := store.GetWorkspace("")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "T123" {
		t.Fatalf("expected T123, got %s", got.ID)
	}

	// Lookup by ID
	got, err = store.GetWorkspace("T123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "myteam" {
		t.Fatalf("expected myteam, got %s", got.Name)
	}

	// Lookup by name
	got, err = store.GetWorkspace("myteam")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "T123" {
		t.Fatalf("expected T123, got %s", got.ID)
	}
}

func TestRemoveWorkspace(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")

	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})
	mustAdd(t, store, config.Workspace{ID: "T2", Name: "team2", URL: "https://team2.slack.com"})

	if err := store.RemoveWorkspace("T1"); err != nil {
		t.Fatal(err)
	}

	// T2 should become default
	got, err := store.GetWorkspace("")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "T2" {
		t.Fatalf("expected T2 as default, got %s", got.ID)
	}

	// T1 is gone
	_, err = store.GetWorkspace("T1")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestRemoveNonexistent(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	err := store.RemoveWorkspace("TNOTEXIST")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestSetDefault(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})
	mustAdd(t, store, config.Workspace{ID: "T2", Name: "team2", URL: "https://team2.slack.com"})

	if err := store.SetDefault("T2"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetWorkspace("")
	if got.ID != "T2" {
		t.Fatalf("expected T2 default, got %s", got.ID)
	}
}

func TestListWorkspaces(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})
	mustAdd(t, store, config.Workspace{ID: "T2", Name: "team2", URL: "https://team2.slack.com"})

	list, defaultID, err := store.ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(list))
	}
	if defaultID != "T1" {
		t.Fatalf("expected T1 default, got %s", defaultID)
	}
}

func TestClearAll(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})
	if err := store.ClearAll(); err != nil {
		t.Fatal(err)
	}
	list, _, _ := store.ListWorkspaces()
	if len(list) != 0 {
		t.Fatalf("expected 0 workspaces, got %d", len(list))
	}
}

func TestEmptyConfigOnFirstLoad(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")

	_, err := store.GetWorkspace("")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound on empty config, got %v", err)
	}
}

func TestSetDefaultNonexistent(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	err := store.SetDefault("TNOTEXIST")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestGetWorkspaceNotFound(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})

	_, err := store.GetWorkspace("TNOTEXIST")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}

func TestConfigDirIsCreated(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	mustAdd(t, store, config.Workspace{ID: "T1", Name: "team1", URL: "https://team1.slack.com"})

	// Verify directory was created via MkdirAll
	if !fs.dirs["/test/config"] {
		t.Fatal("expected config directory to be created via MkdirAll")
	}
}

// errFS is a filesystem that returns errors for all operations.
type errFS struct {
	mkdirErr  error
	readErr   error
	writeErr  error
	existsVal bool
}

func (e *errFS) ReadFile(_ string) ([]byte, error)                    { return nil, e.readErr }
func (e *errFS) WriteFile(_ string, _ []byte, _ os.FileMode) error    { return e.writeErr }
func (e *errFS) MkdirAll(_ string, _ os.FileMode) error               { return e.mkdirErr }
func (e *errFS) Exists(_ string) bool                                  { return e.existsVal }

func TestLoadMkdirError(t *testing.T) {
	fs := &errFS{mkdirErr: fmt.Errorf("permission denied")}
	store := config.NewStore(fs, "/test/config")
	_, err := store.GetWorkspace("")
	if err == nil {
		t.Fatal("expected error from MkdirAll failure")
	}
}

func TestLoadReadError(t *testing.T) {
	fs := &errFS{existsVal: true, readErr: fmt.Errorf("disk error")}
	store := config.NewStore(fs, "/test/config")
	_, err := store.GetWorkspace("")
	if err == nil {
		t.Fatal("expected error from ReadFile failure")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	// Write invalid JSON directly
	fs.files["/test/config/workspaces.json"] = []byte("{invalid json")
	_, err := store.GetWorkspace("")
	if err == nil {
		t.Fatal("expected error from invalid JSON")
	}
}

func TestSaveMkdirError(t *testing.T) {
	fs := &errFS{mkdirErr: fmt.Errorf("permission denied")}
	store := config.NewStore(fs, "/test/config")
	err := store.ClearAll()
	if err == nil {
		t.Fatal("expected error from MkdirAll failure on save")
	}
}

func TestSaveWriteError(t *testing.T) {
	fs := &errFS{writeErr: fmt.Errorf("disk full")}
	store := config.NewStore(fs, "/test/config")
	err := store.ClearAll()
	if err == nil {
		t.Fatal("expected error from WriteFile failure")
	}
}

func TestGetDefaultWhenDefaultPointsToMissing(t *testing.T) {
	fs := newMemFS()
	store := config.NewStore(fs, "/test/config")
	// Manually write config with default pointing to non-existent workspace
	fs.files["/test/config/workspaces.json"] = []byte(`{"default_workspace":"TGONE","workspaces":{}}`)
	_, err := store.GetWorkspace("")
	if err != config.ErrWorkspaceNotFound {
		t.Fatalf("expected ErrWorkspaceNotFound, got %v", err)
	}
}
