package slack_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/mchowning/slack-pipe/internal/slack"
)

// newTestServer creates an httptest server that captures and validates requests.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *slack.Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := slack.NewClient(srv.Client(), "xoxc-test-token", "xoxd-test-cookie", srv.URL)
	return srv, client
}

func TestAuthTest(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Verify request construction
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/auth.test") {
			t.Errorf("expected /api/auth.test, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
		}

		// Verify token is in form body
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("token") != "xoxc-test-token" {
			t.Errorf("expected xoxc-test-token in body, got %s", r.FormValue("token"))
		}

		// Verify cookie header
		cookie := r.Header.Get("Cookie")
		if !strings.HasPrefix(cookie, "d=") {
			t.Errorf("expected Cookie header with d=, got %s", cookie)
		}
		// Decode and verify the cookie value
		cookieVal := strings.TrimPrefix(cookie, "d=")
		decoded, _ := url.QueryUnescape(cookieVal)
		if decoded != "xoxd-test-cookie" {
			t.Errorf("expected decoded cookie xoxd-test-cookie, got %s", decoded)
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"team":    "Test Team",
			"team_id": "T123",
			"user":    "testuser",
			"user_id": "U123",
			"url":     "https://testteam.slack.com/",
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	resp, err := client.AuthTest()
	if err != nil {
		t.Fatal(err)
	}
	if resp.TeamID != "T123" || resp.Team != "Test Team" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestConversationsList(t *testing.T) {
	callCount := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		callCount++

		var resp interface{}
		if callCount == 1 {
			resp = map[string]interface{}{
				"ok": true,
				"channels": []map[string]interface{}{
					{"id": "C1", "name": "general", "is_channel": true},
					{"id": "C2", "name": "random", "is_channel": true},
				},
				"response_metadata": map[string]string{"next_cursor": "page2"},
			}
		} else {
			resp = map[string]interface{}{
				"ok": true,
				"channels": []map[string]interface{}{
					{"id": "C3", "name": "dev", "is_channel": true},
				},
				"response_metadata": map[string]string{"next_cursor": ""},
			}
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	channels, err := client.ConversationsList("public_channel", 100, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels (paginated), got %d", len(channels))
	}
	if channels[0].Name != "general" || channels[2].Name != "dev" {
		t.Fatalf("unexpected channels: %+v", channels)
	}
}

func TestConversationsHistory(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("channel") != "C123" {
			t.Errorf("expected channel C123, got %s", r.FormValue("channel"))
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"messages": []map[string]interface{}{
				{"type": "message", "user": "U1", "text": "Hello", "ts": "1234.5678"},
				{"type": "message", "user": "U2", "text": "World", "ts": "1234.5679"},
			},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	msgs, err := client.ConversationsHistory("C123", 100, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "Hello" || msgs[1].Text != "World" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestConversationsHistoryWithTimeRange(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("oldest") != "1000.0" {
			t.Errorf("expected oldest=1000.0, got %s", r.FormValue("oldest"))
		}
		if r.FormValue("latest") != "2000.0" {
			t.Errorf("expected latest=2000.0, got %s", r.FormValue("latest"))
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":       true,
			"messages": []map[string]interface{}{},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	_, err := client.ConversationsHistory("C123", 100, "1000.0", "2000.0")
	if err != nil {
		t.Fatal(err)
	}
}

func TestConversationsReplies(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("channel") != "C123" || r.FormValue("ts") != "1234.5678" {
			t.Errorf("unexpected params: channel=%s ts=%s", r.FormValue("channel"), r.FormValue("ts"))
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"messages": []map[string]interface{}{
				{"type": "message", "user": "U1", "text": "Thread parent", "ts": "1234.5678", "thread_ts": "1234.5678"},
				{"type": "message", "user": "U2", "text": "Reply", "ts": "1234.5680", "thread_ts": "1234.5678"},
			},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	msgs, err := client.ConversationsReplies("C123", "1234.5678", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
}

func TestRateLimitRetry(t *testing.T) {
	callCount := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0") // 0 second for fast test
			w.WriteHeader(429)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "ratelimited"}); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true, "team": "Test", "team_id": "T1", "user": "u", "user_id": "U1",
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	resp, err := client.AuthTest()
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if resp.TeamID != "T1" {
		t.Fatalf("unexpected response after retry: %+v", resp)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", callCount)
	}
}

func TestAPIError(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "channel_not_found",
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	_, err := client.ConversationsHistory("CBAD", 10, "", "")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*slack.APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "channel_not_found" {
		t.Fatalf("expected channel_not_found, got %s", apiErr.Code)
	}
}

func TestHTTPError(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		if _, err := w.Write([]byte("internal server error")); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	_, err := client.AuthTest()
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestUsersInfo(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"user": map[string]interface{}{
				"id":        "U123",
				"name":      "jdoe",
				"real_name": "Jane Doe",
				"profile": map[string]interface{}{
					"display_name": "janedoe",
					"email":        "jane@example.com",
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	user, err := client.UsersInfo("U123")
	if err != nil {
		t.Fatal(err)
	}
	if user.RealName != "Jane Doe" || user.Profile.DisplayName != "janedoe" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestUsersInfoBatchSkipsErrors(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		usersParam := r.FormValue("users")
		if usersParam == "" {
			t.Fatal("expected 'users' param in batch request")
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"users": []map[string]interface{}{
				{"id": "U1", "name": "user_U1"},
				{"id": "U2", "name": "user_U2"},
			},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	users, err := client.UsersInfoBatch([]string{"U1", "U_BAD", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users (1 skipped by Slack), got %d", len(users))
	}
	if users["U1"] == nil || users["U2"] == nil {
		t.Fatal("missing expected users")
	}
}

func TestAPIErrorString(t *testing.T) {
	e := &slack.APIError{Method: "test.method", Code: "some_error", Status: 200}
	s := e.Error()
	if !strings.Contains(s, "test.method") || !strings.Contains(s, "some_error") {
		t.Fatalf("unexpected error string: %s", s)
	}
}
