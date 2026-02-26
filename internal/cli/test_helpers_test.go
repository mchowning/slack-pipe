package cli

import (
	"os"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/config"
	"github.com/mchowning/slack-pipe/internal/keyring"
)

type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{files: make(map[string][]byte), dirs: make(map[string]bool)}
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

func newTestAuthService(workspaceURL string) *auth.Service {
	kr := keyring.NewMemory()
	cs := config.NewStore(newMemFS(), "/test/config")
	svc := auth.NewService(kr, cs, nil)

	_ = cs.AddWorkspace(config.Workspace{ID: "T1", Name: "test", URL: workspaceURL})
	_ = kr.Set("T1", keyring.KeyToken, "xoxc-test-token")
	_ = kr.Set("T1", keyring.KeyCookie, "xoxd-test-cookie")

	return svc
}
