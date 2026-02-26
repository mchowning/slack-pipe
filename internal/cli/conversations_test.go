package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConversationsListDefaultsToJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/conversations.list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"channels": []map[string]interface{}{
				{"id": "C1", "name": "general", "is_channel": true},
			},
			"response_metadata": map[string]string{"next_cursor": ""},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer srv.Close()

	svc := newTestAuthService(srv.URL)
	cmd := newConversationsListCmd(svc)

	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--workspace", "T1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := strings.TrimSpace(out.String())
	if !json.Valid([]byte(output)) {
		t.Fatalf("expected valid JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"count": 1`) {
		t.Fatalf("expected count field, got: %s", output)
	}
}

func TestConversationsListTextFlagUsesHumanOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/conversations.list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"channels": []map[string]interface{}{
				{"id": "C1", "name": "general", "is_channel": true},
			},
			"response_metadata": map[string]string{"next_cursor": ""},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer srv.Close()

	svc := newTestAuthService(srv.URL)
	cmd := newConversationsListCmd(svc)

	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--workspace", "T1", "--text"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := out.String()
	if !strings.Contains(output, "📋 Conversations (1)") {
		t.Fatalf("expected human output header, got: %s", output)
	}
	if strings.Contains(output, `"channels"`) {
		t.Fatalf("expected text output, got JSON-like output: %s", output)
	}
}
