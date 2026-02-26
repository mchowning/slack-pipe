package slack

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func newInternalTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := NewClient(srv.Client(), "xoxc-test-token", "xoxd-test-cookie", srv.URL)
	return srv, client
}

func parseUsersParam(param string) []string {
	if param == "" {
		return nil
	}
	return strings.Split(param, ",")
}

func TestUsersInfoBatchUsesUsersParamAndParsesArray(t *testing.T) {
	calls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if got := r.FormValue("users"); got != "U1,U2" {
			t.Fatalf("expected users=U1,U2, got %q", got)
		}
		if got := r.FormValue("user"); got != "" {
			t.Fatalf("did not expect singular user param, got %q", got)
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

	users, err := client.UsersInfoBatch([]string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 batch request, got %d", calls)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users["U1"] == nil || users["U2"] == nil {
		t.Fatal("missing expected users")
	}
}

func TestUsersInfoBatchAllInvalidChunkReturnsEmptyMap(t *testing.T) {
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("users") == "" {
			t.Fatal("expected users param")
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    false,
			"error": "user_not_found",
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	users, err := client.UsersInfoBatch([]string{"U_BAD1", "U_BAD2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty map for all-invalid chunk, got %d users", len(users))
	}
}

func TestUsersInfoBatchTooManyUsersHalvesAndRetries(t *testing.T) {
	var chunkSizes []int
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		ids := parseUsersParam(r.FormValue("users"))
		chunkSizes = append(chunkSizes, len(ids))

		if len(ids) > 2 {
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "too_many_users",
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		users := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			users = append(users, map[string]string{"id": id, "name": "user_" + id})
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": users}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()
	client.maxBatchSize = 4

	users, err := client.UsersInfoBatch([]string{"U1", "U2", "U3", "U4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 4 {
		t.Fatalf("expected 4 users, got %d", len(users))
	}
	if !reflect.DeepEqual(chunkSizes, []int{4, 2, 2}) {
		t.Fatalf("expected chunk sizes [4 2 2], got %v", chunkSizes)
	}
}

func TestUsersInfoBatchHTTP500HalvesAndRetries(t *testing.T) {
	var chunkSizes []int
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		ids := parseUsersParam(r.FormValue("users"))
		chunkSizes = append(chunkSizes, len(ids))

		if len(ids) > 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal server error"))
			return
		}

		users := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			users = append(users, map[string]string{"id": id, "name": "user_" + id})
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": users}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()
	client.maxBatchSize = 4

	users, err := client.UsersInfoBatch([]string{"U1", "U2", "U3", "U4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 4 {
		t.Fatalf("expected 4 users, got %d", len(users))
	}
	if !reflect.DeepEqual(chunkSizes, []int{4, 2, 2}) {
		t.Fatalf("expected chunk sizes [4 2 2], got %v", chunkSizes)
	}
}

func TestUsersInfoBatchHTTP400FallsBackSequentialWithoutHalving(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if usersParam := r.FormValue("users"); usersParam != "" {
			batchCalls++
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("bad request"))
			return
		}

		if userID := r.FormValue("user"); userID != "" {
			sequentialCalls++
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":   true,
				"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		t.Fatal("expected either users or user param")
	})
	defer srv.Close()
	client.maxBatchSize = 4

	users, err := client.UsersInfoBatch([]string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected sequential fallback to return 2 users, got %d", len(users))
	}
	if batchCalls != 1 {
		t.Fatalf("expected exactly 1 batch attempt, got %d", batchCalls)
	}
	if sequentialCalls != 2 {
		t.Fatalf("expected 2 sequential calls after fallback, got %d", sequentialCalls)
	}
	if client.batchSupported {
		t.Fatal("expected batchSupported=false after HTTP 400 fallback")
	}
}

func TestUsersInfoBatchSizeOneStillFailingFallsBackSequential(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if r.FormValue("users") != "" {
			batchCalls++
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "too_many_users",
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		userID := r.FormValue("user")
		if userID == "" {
			t.Fatal("expected user param for sequential fallback")
		}
		sequentialCalls++
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()
	client.maxBatchSize = 1

	users, err := client.UsersInfoBatch([]string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users from sequential fallback, got %d", len(users))
	}
	if batchCalls != 1 {
		t.Fatalf("expected 1 batch attempt at size 1, got %d", batchCalls)
	}
	if sequentialCalls != 2 {
		t.Fatalf("expected 2 sequential calls, got %d", sequentialCalls)
	}
	if client.batchSupported {
		t.Fatal("expected batchSupported=false after size-1 failure")
	}
}

func TestUsersInfoBatchInvalidArgumentsDisablesBatchAndFallsBackSequential(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if r.FormValue("users") != "" {
			batchCalls++
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "invalid_arguments",
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		userID := r.FormValue("user")
		if userID == "" {
			t.Fatal("expected user param for sequential fallback")
		}
		sequentialCalls++
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	users, err := client.UsersInfoBatch([]string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users from sequential fallback, got %d", len(users))
	}
	if batchCalls != 1 {
		t.Fatalf("expected 1 batch attempt, got %d", batchCalls)
	}
	if sequentialCalls != 2 {
		t.Fatalf("expected 2 sequential calls, got %d", sequentialCalls)
	}
	if client.batchSupported {
		t.Fatal("expected batchSupported=false after invalid_arguments")
	}
}

func TestUsersInfoBatchTransientErrorSkipsChunkButKeepsBatchEnabled(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if usersParam := r.FormValue("users"); usersParam != "" {
			batchCalls++
			ids := parseUsersParam(usersParam)
			if batchCalls == 1 {
				if err := json.NewEncoder(w).Encode(map[string]interface{}{
					"ok":    false,
					"error": "internal_error",
				}); err != nil {
					t.Fatal(err)
				}
				return
			}

			users := make([]map[string]string, 0, len(ids))
			for _, id := range ids {
				users = append(users, map[string]string{"id": id, "name": "user_" + id})
			}
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": users}); err != nil {
				t.Fatal(err)
			}
			return
		}

		if r.FormValue("user") != "" {
			sequentialCalls++
		}
		t.Fatal("did not expect sequential request")
	})
	defer srv.Close()
	client.maxBatchSize = 2

	first, err := client.UsersInfoBatch([]string{"U1", "U2", "U3", "U4"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("expected 2 users from second chunk, got %d", len(first))
	}
	if first["U3"] == nil || first["U4"] == nil {
		t.Fatalf("expected users from second chunk only, got keys: %v", mapKeys(first))
	}
	if !client.batchSupported {
		t.Fatal("expected batch to remain supported after transient error")
	}
	if sequentialCalls != 0 {
		t.Fatalf("expected no sequential calls, got %d", sequentialCalls)
	}

	second, err := client.UsersInfoBatch([]string{"U9"})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second["U9"] == nil {
		t.Fatalf("expected second batch call to succeed, got %v", mapKeys(second))
	}
	if batchCalls < 3 {
		t.Fatalf("expected at least 3 batch calls total, got %d", batchCalls)
	}
}

func TestUsersInfoBatchNonAPIErrorFallsBackSequentialButKeepsBatchEnabled(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if usersParam := r.FormValue("users"); usersParam != "" {
			batchCalls++
			ids := parseUsersParam(usersParam)
			if batchCalls == 1 {
				_, _ = w.Write([]byte("this-is-not-json"))
				return
			}

			users := make([]map[string]string, 0, len(ids))
			for _, id := range ids {
				users = append(users, map[string]string{"id": id, "name": "user_" + id})
			}
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": users}); err != nil {
				t.Fatal(err)
			}
			return
		}

		userID := r.FormValue("user")
		if userID == "" {
			t.Fatal("expected user param")
		}
		sequentialCalls++
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	first, err := client.UsersInfoBatch([]string{"U1", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 {
		t.Fatalf("expected 2 users from sequential fallback, got %d", len(first))
	}
	if !client.batchSupported {
		t.Fatal("expected batch support to remain enabled after non-API error")
	}
	if batchCalls != 1 {
		t.Fatalf("expected one failed batch call, got %d", batchCalls)
	}
	if sequentialCalls != 2 {
		t.Fatalf("expected 2 sequential calls after fallback, got %d", sequentialCalls)
	}

	second, err := client.UsersInfoBatch([]string{"U3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second["U3"] == nil {
		t.Fatalf("expected second call to use batch and return U3, got %v", mapKeys(second))
	}
	if batchCalls != 2 {
		t.Fatalf("expected second call to attempt batch again, got %d batch calls", batchCalls)
	}
}

func TestUsersInfoBatchSecondCallAfterFallbackGoesDirectlySequential(t *testing.T) {
	batchCalls := 0
	sequentialCalls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if usersParam := r.FormValue("users"); usersParam != "" {
			batchCalls++
			if batchCalls > 1 {
				t.Fatalf("expected no batch attempts after fallback, saw %d", batchCalls)
			}
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "invalid_arguments",
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		userID := r.FormValue("user")
		if userID == "" {
			t.Fatal("expected user param")
		}
		sequentialCalls++
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()

	first, err := client.UsersInfoBatch([]string{"U1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first["U1"] == nil {
		t.Fatalf("expected first call fallback result for U1, got %v", mapKeys(first))
	}

	second, err := client.UsersInfoBatch([]string{"U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second["U2"] == nil {
		t.Fatalf("expected second sequential result for U2, got %v", mapKeys(second))
	}

	if batchCalls != 1 {
		t.Fatalf("expected exactly one batch attempt across both calls, got %d", batchCalls)
	}
	if sequentialCalls != 2 {
		t.Fatalf("expected two sequential calls across both calls, got %d", sequentialCalls)
	}
}

func TestUsersInfoBatchSequentialFallbackSkipsFailuresAndReturnsPartial(t *testing.T) {
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}

		if r.FormValue("users") != "" {
			t.Fatal("did not expect batch call when batchSupported=false")
		}

		userID := r.FormValue("user")
		if userID == "U_BAD" {
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "user_not_found",
			}); err != nil {
				t.Fatal(err)
			}
			return
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":   true,
			"user": map[string]interface{}{"id": userID, "name": "user_" + userID},
		}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()
	client.batchSupported = false

	users, err := client.UsersInfoBatch([]string{"U1", "U_BAD", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users with one skipped failure, got %d", len(users))
	}
	if users["U1"] == nil || users["U2"] == nil {
		t.Fatalf("missing expected users, got %v", mapKeys(users))
	}
}

func TestUsersInfoBatchDedupSendsSingleBatchRequest(t *testing.T) {
	calls := 0
	var gotUsersParam string
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotUsersParam = r.FormValue("users")
		ids := parseUsersParam(gotUsersParam)
		users := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			users = append(users, map[string]string{"id": id, "name": "user_" + id})
		}
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "users": users}); err != nil {
			t.Fatal(err)
		}
	})
	defer srv.Close()
	client.maxBatchSize = 10

	users, err := client.UsersInfoBatch([]string{"U1", "", "U1", "U2", "U2"})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one batch request after dedup, got %d", calls)
	}
	if gotUsersParam != "U1,U2" {
		t.Fatalf("expected deduped users param U1,U2, got %q", gotUsersParam)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 deduped users, got %d", len(users))
	}
}

func TestUsersInfoBatchEmptyInputReturnsImmediately(t *testing.T) {
	calls := 0
	srv, client := newInternalTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		t.Fatal("should not call server for empty input")
	})
	defer srv.Close()

	users, err := client.UsersInfoBatch([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty map, got %d users", len(users))
	}
	users, err = client.UsersInfoBatch([]string{"", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty map for blank IDs, got %d users", len(users))
	}
	if calls != 0 {
		t.Fatalf("expected 0 server calls, got %d", calls)
	}
}

func TestIsAPIError(t *testing.T) {
	baseErr := &APIError{Method: "users.info", Code: "too_many_users", Status: 200}
	wrappedErr := fmt.Errorf("wrapped: %w", baseErr)

	if !isAPIError(baseErr, "too_many_users") {
		t.Fatal("expected direct APIError to match code")
	}
	if !isAPIError(wrappedErr, "too_many_users") {
		t.Fatal("expected wrapped APIError to match code")
	}
	if isAPIError(baseErr, "user_not_found") {
		t.Fatal("expected non-matching code to return false")
	}
	if isAPIError(errors.New("boom"), "too_many_users") {
		t.Fatal("expected non-API error to return false")
	}
	if isAPIError(nil, "too_many_users") {
		t.Fatal("expected nil error to return false")
	}
}

func TestIsHTTPServerError(t *testing.T) {
	if !isHTTPServerError(&APIError{Method: "users.info", Code: "http_error", Status: 500}) {
		t.Fatal("expected 500 http_error to be server error")
	}
	if !isHTTPServerError(fmt.Errorf("wrapped: %w", &APIError{Method: "users.info", Code: "http_error", Status: 503})) {
		t.Fatal("expected wrapped 503 http_error to be server error")
	}
	if isHTTPServerError(&APIError{Method: "users.info", Code: "http_error", Status: 400}) {
		t.Fatal("expected 400 http_error to not be server error")
	}
	if isHTTPServerError(&APIError{Method: "users.info", Code: "too_many_users", Status: 200}) {
		t.Fatal("expected non-http_error API code to return false")
	}
	if isHTTPServerError(errors.New("boom")) {
		t.Fatal("expected non-API error to return false")
	}
	if isHTTPServerError(nil) {
		t.Fatal("expected nil error to return false")
	}
}

func mapKeys(m map[string]*UserInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
