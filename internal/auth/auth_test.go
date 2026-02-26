package auth_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/config"
	"github.com/mchowning/slack-pipe/internal/keyring"
)

// fakeAuthTester implements auth.AuthTester for tests.
type fakeAuthTester struct {
	result *auth.AuthTestResult
	err    error
}

func (f *fakeAuthTester) TestAuth(_, _, _ string) (*auth.AuthTestResult, error) {
	return f.result, f.err
}

// memFS implements config.FileSystem for tests.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{files: make(map[string][]byte), dirs: make(map[string]bool)}
}
func (m *memFS) ReadFile(path string) ([]byte, error) {
	d, ok := m.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}
func (m *memFS) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = data
	return nil
}
func (m *memFS) MkdirAll(path string, _ os.FileMode) error { m.dirs[path] = true; return nil }
func (m *memFS) Exists(path string) bool                    { _, ok := m.files[path]; return ok }

func newTestService(tester auth.AuthTester) (*auth.Service, *keyring.Memory) {
	kr := keyring.NewMemory()
	cs := config.NewStore(newMemFS(), "/test/config")
	return auth.NewService(kr, cs, tester), kr
}

func mustLogin(t *testing.T, svc *auth.Service, token, cookie, url string) *config.Workspace {
	t.Helper()
	ws, err := svc.Login(token, cookie, url)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return ws
}

func TestLoginSuccess(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{
			OK: true, TeamID: "T123", Team: "myteam", UserID: "U1", User: "me", URL: "https://myteam.slack.com",
		},
	}
	svc, kr := newTestService(tester)

	ws := mustLogin(t, svc, "xoxc-test-token", "xoxd-test-cookie", "https://myteam.slack.com")
	if ws.ID != "T123" || ws.Name != "myteam" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}

	// Verify keyring has the secrets
	tok, _ := kr.Get("T123", keyring.KeyToken)
	if tok != "xoxc-test-token" {
		t.Fatalf("expected xoxc-test-token in keyring, got %s", tok)
	}
	cookie, _ := kr.Get("T123", keyring.KeyCookie)
	if cookie != "xoxd-test-cookie" {
		t.Fatalf("expected xoxd-test-cookie in keyring, got %s", cookie)
	}
}

func TestLoginInvalidTokenFormat(t *testing.T) {
	tester := &fakeAuthTester{}
	svc, _ := newTestService(tester)

	_, err := svc.Login("bad-token", "xoxd-cookie", "https://team.slack.com")
	if err == nil {
		t.Fatal("expected error for bad token format")
	}

	_, err = svc.Login("xoxc-token", "bad-cookie", "https://team.slack.com")
	if err == nil {
		t.Fatal("expected error for bad cookie format")
	}
}

func TestLoginAuthTestFailure(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: false, Error: "invalid_auth"},
	}
	svc, _ := newTestService(tester)

	_, err := svc.Login("xoxc-token", "xoxd-cookie", "https://team.slack.com")
	if err == nil {
		t.Fatal("expected error for failed auth test")
	}
}

func TestLoginNetworkError(t *testing.T) {
	tester := &fakeAuthTester{err: fmt.Errorf("connection refused")}
	svc, _ := newTestService(tester)

	_, err := svc.Login("xoxc-token", "xoxd-cookie", "https://team.slack.com")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestGetCredentials(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T123", Team: "myteam"},
	}
	svc, _ := newTestService(tester)
	mustLogin(t, svc, "xoxc-secret", "xoxd-secret", "https://myteam.slack.com")

	tok, cookie, url, err := svc.GetCredentials("T123")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "xoxc-secret" || cookie != "xoxd-secret" || url != "https://myteam.slack.com" {
		t.Fatalf("unexpected creds: tok=%s cookie=%s url=%s", tok, cookie, url)
	}
}

func TestGetCredentialsByName(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T123", Team: "myteam"},
	}
	svc, _ := newTestService(tester)
	mustLogin(t, svc, "xoxc-secret", "xoxd-secret", "https://myteam.slack.com")

	tok, _, _, err := svc.GetCredentials("myteam")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "xoxc-secret" {
		t.Fatalf("expected xoxc-secret, got %s", tok)
	}
}

func TestGetCredentialsDefault(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T123", Team: "myteam"},
	}
	svc, _ := newTestService(tester)
	mustLogin(t, svc, "xoxc-secret", "xoxd-secret", "https://myteam.slack.com")

	tok, _, _, err := svc.GetCredentials("")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "xoxc-secret" {
		t.Fatalf("expected xoxc-secret from default, got %s", tok)
	}
}

func TestRemove(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T123", Team: "myteam"},
	}
	svc, kr := newTestService(tester)
	mustLogin(t, svc, "xoxc-tok", "xoxd-cook", "https://myteam.slack.com")

	if err := svc.Remove("T123"); err != nil {
		t.Fatal(err)
	}

	// Keyring entries should be gone
	_, err := kr.Get("T123", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatal("expected token deleted from keyring")
	}

	// Config should be gone
	_, _, _, err = svc.GetCredentials("T123")
	if err == nil {
		t.Fatal("expected error after removal")
	}
}

func TestListWorkspaces(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T1", Team: "team1"},
	}
	svc, _ := newTestService(tester)
	mustLogin(t, svc, "xoxc-t1", "xoxd-c1", "https://team1.slack.com")
	tester.result = &auth.AuthTestResult{OK: true, TeamID: "T2", Team: "team2"}
	mustLogin(t, svc, "xoxc-t2", "xoxd-c2", "https://team2.slack.com")

	list, defaultID, err := svc.ListWorkspaces()
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

func TestSetDefault(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T1", Team: "team1"},
	}
	svc, _ := newTestService(tester)
	mustLogin(t, svc, "xoxc-t1", "xoxd-c1", "https://team1.slack.com")
	tester.result = &auth.AuthTestResult{OK: true, TeamID: "T2", Team: "team2"}
	mustLogin(t, svc, "xoxc-t2", "xoxd-c2", "https://team2.slack.com")

	if err := svc.SetDefault("T2"); err != nil {
		t.Fatal(err)
	}

	// Verify default changed
	_, defaultID, _ := svc.ListWorkspaces()
	if defaultID != "T2" {
		t.Fatalf("expected T2 default, got %s", defaultID)
	}
}

func TestLogout(t *testing.T) {
	tester := &fakeAuthTester{
		result: &auth.AuthTestResult{OK: true, TeamID: "T1", Team: "team1"},
	}
	svc, kr := newTestService(tester)
	mustLogin(t, svc, "xoxc-t1", "xoxd-c1", "https://team1.slack.com")
	tester.result = &auth.AuthTestResult{OK: true, TeamID: "T2", Team: "team2"}
	mustLogin(t, svc, "xoxc-t2", "xoxd-c2", "https://team2.slack.com")

	if err := svc.Logout(); err != nil {
		t.Fatal(err)
	}

	_, err := kr.Get("T1", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatal("expected T1 token deleted")
	}
	_, err = kr.Get("T2", keyring.KeyToken)
	if err != keyring.ErrNotFound {
		t.Fatal("expected T2 token deleted")
	}
}
