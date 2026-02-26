package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSentDateWindowValidation(t *testing.T) {
	_, err := parseSentDateWindow("", "", "", time.Local)
	if err == nil || !strings.Contains(err.Error(), "missing date input") {
		t.Fatalf("expected missing input error, got: %v", err)
	}

	_, err = parseSentDateWindow("2026-02-20", "2026-02-01", "2026-02-10", time.Local)
	if err == nil || !strings.Contains(err.Error(), "either --date or --start-date/--end-date") {
		t.Fatalf("expected mixed mode error, got: %v", err)
	}

	_, err = parseSentDateWindow("", "2026-02-01", "", time.Local)
	if err == nil || !strings.Contains(err.Error(), "must be provided together") {
		t.Fatalf("expected paired range flag error, got: %v", err)
	}

	_, err = parseSentDateWindow("", "2026-02-10", "2026-02-01", time.Local)
	if err == nil || !strings.Contains(err.Error(), "on or after") {
		t.Fatalf("expected end-before-start error, got: %v", err)
	}
}

func TestParseSentDateWindowModes(t *testing.T) {
	window, err := parseSentDateWindow("2026-02-20", "", "", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if got := window.Start.Format("2006-01-02 15:04:05"); got != "2026-02-20 00:00:00" {
		t.Fatalf("unexpected single-day start: %s", got)
	}
	if got := window.EndExclusive.Format("2006-01-02 15:04:05"); got != "2026-02-21 00:00:00" {
		t.Fatalf("unexpected single-day end: %s", got)
	}

	window, err = parseSentDateWindow("", "2026-02-20", "2026-02-22", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if got := window.EndExclusive.Format("2006-01-02 15:04:05"); got != "2026-02-23 00:00:00" {
		t.Fatalf("unexpected range end: %s", got)
	}
}

func TestMessagesSentCommandValidation(t *testing.T) {
	cmd := newMessagesSentCmd(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--start-date", "2026-02-10", "--end-date", "2026-02-01"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "on or after") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMessagesSentFiltersByLocalDateAndStopsPaging(t *testing.T) {
	targetDay := time.Date(2026, 2, 20, 0, 0, 0, 0, time.Local)
	inRange := targetDay.Add(10 * time.Hour)
	newerThanRange := targetDay.AddDate(0, 0, 1).Add(2 * time.Hour)
	olderThanRange := targetDay.Add(-1 * time.Hour)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search.messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		calls++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("query") != "from:me" {
			t.Fatalf("unexpected query: %s", r.FormValue("query"))
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
			"messages": map[string]interface{}{
				"total":  3,
				"paging": map[string]interface{}{"count": 100, "total": 3, "page": 1, "pages": 5},
				"matches": []map[string]interface{}{
					{
						"type":      "message",
						"ts":        slackTS(newerThanRange),
						"text":      "too new",
						"permalink": "https://example/new",
						"channel":   map[string]interface{}{"id": "C1", "name": "general"},
					},
					{
						"type":      "message",
						"ts":        slackTS(inRange),
						"text":      "in range",
						"permalink": "https://example/in",
						"channel":   map[string]interface{}{"id": "C1", "name": "general"},
					},
					{
						"type":      "message",
						"ts":        slackTS(olderThanRange),
						"text":      "too old",
						"permalink": "https://example/old",
						"channel":   map[string]interface{}{"id": "C1", "name": "general"},
					},
				},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}))
	defer srv.Close()

	svc := newTestAuthService(srv.URL)
	cmd := newMessagesSentCmd(svc)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--workspace", "T1", "--date", targetDay.Format(sentDateLayout)})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	output := strings.TrimSpace(out.String())
	if !json.Valid([]byte(output)) {
		t.Fatalf("expected JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"count": 1`) {
		t.Fatalf("expected exactly one in-range message, got: %s", output)
	}
	if !strings.Contains(output, `"text": "in range"`) {
		t.Fatalf("expected in-range message in output, got: %s", output)
	}
	if calls != 1 {
		t.Fatalf("expected paging to stop after first page, got %d calls", calls)
	}
}

func slackTS(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000)
}
