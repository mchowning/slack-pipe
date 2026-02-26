package slack

import (
	"net/http"
	"time"

	"github.com/mchowning/slack-pipe/internal/auth"
)

// AuthAdapter implements auth.AuthTester using the Slack HTTP client.
type AuthAdapter struct{}

func NewAuthAdapter() *AuthAdapter { return &AuthAdapter{} }

func (a *AuthAdapter) TestAuth(token, cookie, workspaceURL string) (*auth.AuthTestResult, error) {
	client := NewClient(&http.Client{Timeout: 15 * time.Second}, token, cookie, workspaceURL)
	resp, err := client.AuthTest()
	if err != nil {
		return nil, err
	}
	return &auth.AuthTestResult{
		OK:     resp.OK,
		TeamID: resp.TeamID,
		Team:   resp.Team,
		UserID: resp.UserID,
		User:   resp.User,
		URL:    resp.URL,
		Error:  resp.Error,
	}, nil
}
