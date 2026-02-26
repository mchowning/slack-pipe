package auth

import (
	"fmt"

	"github.com/mchowning/slack-pipe/internal/config"
	"github.com/mchowning/slack-pipe/internal/curl"
	"github.com/mchowning/slack-pipe/internal/keyring"
)

// AuthTestResult represents the response from Slack's auth.test API.
type AuthTestResult struct {
	OK     bool
	TeamID string
	Team   string
	UserID string
	User   string
	URL    string
	Error  string
}

// AuthTester verifies tokens against Slack's auth.test endpoint.
// In production this is the Slack HTTP client; in tests it's a fake.
type AuthTester interface {
	TestAuth(token, cookie, workspaceURL string) (*AuthTestResult, error)
}

// Service orchestrates authentication operations.
type Service struct {
	keyring     keyring.Store
	configStore *config.Store
	tester      AuthTester
}

func NewService(kr keyring.Store, cs *config.Store, tester AuthTester) *Service {
	return &Service{keyring: kr, configStore: cs, tester: tester}
}

// Login validates the token+cookie, stores secrets in keyring, and saves workspace metadata.
func (s *Service) Login(token, cookie, workspaceURL string) (*config.Workspace, error) {
	// Validate token formats
	if err := curl.ValidateTokenFormat(token, "xoxc-"); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if err := curl.ValidateTokenFormat(cookie, "xoxd-"); err != nil {
		return nil, fmt.Errorf("invalid cookie: %w", err)
	}

	// Verify with Slack
	result, err := s.tester.TestAuth(token, cookie, workspaceURL)
	if err != nil {
		return nil, fmt.Errorf("auth test failed: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("authentication failed: %s", result.Error)
	}

	// Store secrets in keyring
	if err := s.keyring.Set(result.TeamID, keyring.KeyToken, token); err != nil {
		return nil, fmt.Errorf("store token in keyring: %w", err)
	}
	if err := s.keyring.Set(result.TeamID, keyring.KeyCookie, cookie); err != nil {
		return nil, fmt.Errorf("store cookie in keyring: %w", err)
	}

	// Save workspace metadata (no secrets!)
	ws := config.Workspace{
		ID:   result.TeamID,
		Name: result.Team,
		URL:  workspaceURL,
	}
	if err := s.configStore.AddWorkspace(ws); err != nil {
		return nil, fmt.Errorf("save workspace config: %w", err)
	}

	return &ws, nil
}

// Remove deletes keyring entries and workspace config.
func (s *Service) Remove(workspaceID string) error {
	// Delete from keyring (best-effort — might already be gone)
	_ = s.keyring.Delete(workspaceID, keyring.KeyToken)
	_ = s.keyring.Delete(workspaceID, keyring.KeyCookie)

	return s.configStore.RemoveWorkspace(workspaceID)
}

// Logout clears all workspaces and their keyring entries.
func (s *Service) Logout() error {
	workspaces, _, err := s.configStore.ListWorkspaces()
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		_ = s.keyring.Delete(ws.ID, keyring.KeyToken)
		_ = s.keyring.Delete(ws.ID, keyring.KeyCookie)
	}
	return s.configStore.ClearAll()
}

// ListWorkspaces returns all workspaces and the default workspace ID.
func (s *Service) ListWorkspaces() ([]config.Workspace, string, error) {
	return s.configStore.ListWorkspaces()
}

// SetDefault sets the default workspace.
func (s *Service) SetDefault(workspaceID string) error {
	return s.configStore.SetDefault(workspaceID)
}

// GetCredentials retrieves token and cookie for a workspace from the keyring.
func (s *Service) GetCredentials(workspaceIdentifier string) (token, cookie, workspaceURL string, err error) {
	ws, err := s.configStore.GetWorkspace(workspaceIdentifier)
	if err != nil {
		return "", "", "", fmt.Errorf("workspace lookup: %w", err)
	}

	token, err = s.keyring.Get(ws.ID, keyring.KeyToken)
	if err != nil {
		return "", "", "", fmt.Errorf("retrieve token from keyring: %w (try re-authenticating with 'slack-pipe auth login')", err)
	}
	cookie, err = s.keyring.Get(ws.ID, keyring.KeyCookie)
	if err != nil {
		return "", "", "", fmt.Errorf("retrieve cookie from keyring: %w (try re-authenticating with 'slack-pipe auth login')", err)
	}

	return token, cookie, ws.URL, nil
}
