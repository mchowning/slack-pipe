package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPDoer abstracts http.Client for testability.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client interacts with Slack's API using session tokens.
type Client struct {
	httpClient     HTTPDoer
	token          string // xoxc token
	cookie         string // d cookie (xoxd)
	workspaceURL   string // e.g., https://myteam.slack.com
	maxRetries     int
	maxBatchSize   int  // max user IDs per batch request, halves on too_many_users/5xx
	batchSupported bool // false if undocumented users param is unavailable
}

func NewClient(httpClient HTTPDoer, token, cookie, workspaceURL string) *Client {
	return &Client{
		httpClient:     httpClient,
		token:          token,
		cookie:         cookie,
		workspaceURL:   strings.TrimRight(workspaceURL, "/"),
		maxRetries:     3,
		maxBatchSize:   256,
		batchSupported: true,
	}
}

// APIError represents a Slack API error response.
type APIError struct {
	Method string
	Code   string
	Status int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("slack API error: method=%s, error=%s, status=%d", e.Method, e.Code, e.Status)
}

// request makes a POST request to Slack's API with session token auth.
func (c *Client) request(method string, params map[string]string) (json.RawMessage, error) {
	retriesLeft := c.maxRetries

	for {
		apiURL := fmt.Sprintf("%s/api/%s", c.workspaceURL, method)

		// Build form body with token
		form := url.Values{}
		form.Set("token", c.token)
		for k, v := range params {
			form.Set(k, v)
		}

		slog.Debug("slack API request", "method", method, "params", params)
		reqStart := time.Now()

		req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		// URL-encode the cookie value for the Cookie header
		encodedCookie := url.QueryEscape(c.cookie)
		req.Header.Set("Cookie", fmt.Sprintf("d=%s", encodedCookie))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}

		elapsed := time.Since(reqStart)
		slog.Debug("slack API response", "method", method, "status", resp.StatusCode, "elapsed", elapsed.Round(time.Millisecond))

		// Handle rate limiting — close body before sleeping to avoid connection leak
		if resp.StatusCode == 429 && retriesLeft > 0 {
			retryAfter := 1
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if parsed, err := strconv.Atoi(ra); err == nil {
					retryAfter = parsed
				}
			}
			_ = resp.Body.Close()
			slog.Warn("rate limited by Slack API, retrying", "method", method, "retry_after_secs", retryAfter, "retries_left", retriesLeft)
			time.Sleep(time.Duration(retryAfter) * time.Second)
			retriesLeft--
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode >= 400 {
			return nil, &APIError{Method: method, Code: "http_error", Status: resp.StatusCode}
		}

		// Parse envelope to check for Slack-level errors
		var envelope struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		if !envelope.OK {
			return nil, &APIError{Method: method, Code: envelope.Error, Status: resp.StatusCode}
		}

		return body, nil
	}
}

// --- API methods ---

// AuthTestResponse represents the response from auth.test.
type AuthTestResponse struct {
	OK     bool   `json:"ok"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
	Error  string `json:"error"`
}

func (c *Client) AuthTest() (*AuthTestResponse, error) {
	body, err := c.request("auth.test", nil)
	if err != nil {
		return nil, err
	}
	var resp AuthTestResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse auth.test: %w", err)
	}
	return &resp, nil
}

// Channel represents a Slack conversation.
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsChannel  bool   `json:"is_channel"`
	IsGroup    bool   `json:"is_group"`
	IsIM       bool   `json:"is_im"`
	IsMPIM     bool   `json:"is_mpim"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	NumMembers int    `json:"num_members"`
	User       string `json:"user"` // For DMs
	Topic      struct {
		Value string `json:"value"`
	} `json:"topic"`
	Purpose struct {
		Value string `json:"value"`
	} `json:"purpose"`
}

// ConversationsListResponse represents the response from conversations.list.
type ConversationsListResponse struct {
	OK               bool      `json:"ok"`
	Channels         []Channel `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) ConversationsList(types string, limit int, excludeArchived bool) ([]Channel, error) {
	var allChannels []Channel
	remaining := limit
	cursor := ""
	page := 0

	for {
		pageSize := remaining
		if pageSize > 200 {
			pageSize = 200 // Slack max per page
		}

		params := map[string]string{
			"types":            types,
			"limit":            strconv.Itoa(pageSize),
			"exclude_archived": strconv.FormatBool(excludeArchived),
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		body, err := c.request("conversations.list", params)
		if err != nil {
			return nil, err
		}

		var resp ConversationsListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse conversations.list: %w", err)
		}

		allChannels = append(allChannels, resp.Channels...)
		remaining -= len(resp.Channels)
		page++
		slog.Debug("conversations.list page fetched", "page", page, "fetched", len(resp.Channels), "total_so_far", len(allChannels), "remaining", remaining)

		cursor = resp.ResponseMetadata.NextCursor
		if cursor == "" || remaining <= 0 {
			break
		}
	}

	return allChannels, nil
}

// Message represents a Slack message.
type Message struct {
	Type       string `json:"type"`
	User       string `json:"user"`
	BotID      string `json:"bot_id"`
	Text       string `json:"text"`
	Ts         string `json:"ts"`
	ThreadTs   string `json:"thread_ts"`
	ReplyCount int    `json:"reply_count"`
	Reactions  []struct {
		Name  string   `json:"name"`
		Count int      `json:"count"`
		Users []string `json:"users"`
	} `json:"reactions"`
}

// ConversationsHistoryResponse represents the response from conversations.history.
type ConversationsHistoryResponse struct {
	OK               bool      `json:"ok"`
	Messages         []Message `json:"messages"`
	HasMore          bool      `json:"has_more"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (c *Client) ConversationsHistory(channelID string, limit int, oldest, latest string) ([]Message, error) {
	var allMessages []Message
	remaining := limit
	cursor := ""

	for {
		pageSize := remaining
		if pageSize > 200 {
			pageSize = 200 // Slack max per page
		}

		params := map[string]string{
			"channel": channelID,
			"limit":   strconv.Itoa(pageSize),
		}
		if oldest != "" {
			params["oldest"] = oldest
		}
		if latest != "" {
			params["latest"] = latest
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		body, err := c.request("conversations.history", params)
		if err != nil {
			return nil, err
		}

		var resp ConversationsHistoryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse conversations.history: %w", err)
		}

		allMessages = append(allMessages, resp.Messages...)
		remaining -= len(resp.Messages)

		cursor = resp.ResponseMetadata.NextCursor
		if cursor == "" || !resp.HasMore || remaining <= 0 {
			break
		}
	}

	return allMessages, nil
}

func (c *Client) ConversationsReplies(channelID, threadTs string, limit int) ([]Message, error) {
	params := map[string]string{
		"channel": channelID,
		"ts":      threadTs,
		"limit":   strconv.Itoa(limit),
	}

	body, err := c.request("conversations.replies", params)
	if err != nil {
		return nil, err
	}

	var resp ConversationsHistoryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse conversations.replies: %w", err)
	}

	return resp.Messages, nil
}

// SearchMessagesResponse represents the response from search.messages.
type SearchMessagesResponse struct {
	OK       bool                  `json:"ok"`
	Query    string                `json:"query"`
	Messages SearchMessagesPayload `json:"messages"`
}

// SearchMessagesPayload represents the messages section of search.messages.
type SearchMessagesPayload struct {
	Total   int                  `json:"total"`
	Paging  SearchMessagesPaging `json:"paging"`
	Matches []SearchMessageMatch `json:"matches"`
}

// SearchMessagesPaging contains pagination metadata.
type SearchMessagesPaging struct {
	Count int `json:"count"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

// SearchMessageMatch represents one message match in search.messages.
type SearchMessageMatch struct {
	Type      string `json:"type"`
	User      string `json:"user"`
	Username  string `json:"username"`
	Text      string `json:"text"`
	Ts        string `json:"ts"`
	Permalink string `json:"permalink"`
	Channel   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
}

// SearchMessages searches messages using Slack's search.messages endpoint.
func (c *Client) SearchMessages(query string, count, page int, sort, sortDir string) (*SearchMessagesResponse, error) {
	params := map[string]string{
		"query": query,
		"count": strconv.Itoa(count),
		"page":  strconv.Itoa(page),
	}
	if sort != "" {
		params["sort"] = sort
	}
	if sortDir != "" {
		params["sort_dir"] = sortDir
	}

	body, err := c.request("search.messages", params)
	if err != nil {
		return nil, err
	}

	var resp SearchMessagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse search.messages: %w", err)
	}

	return &resp, nil
}

// UserInfo represents a Slack user.
type UserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RealName string `json:"real_name"`
	Profile  struct {
		DisplayName string `json:"display_name"`
		RealName    string `json:"real_name"`
		Email       string `json:"email"`
	} `json:"profile"`
}

// UsersInfoResponse represents the response from users.info.
type UsersInfoResponse struct {
	OK   bool     `json:"ok"`
	User UserInfo `json:"user"`
}

func (c *Client) UsersInfo(userID string) (*UserInfo, error) {
	body, err := c.request("users.info", map[string]string{"user": userID})
	if err != nil {
		return nil, err
	}

	var resp UsersInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse users.info: %w", err)
	}

	return &resp.User, nil
}

// UsersInfoBulkResponse represents the response from users.info with multiple users.
type UsersInfoBulkResponse struct {
	OK    bool       `json:"ok"`
	Users []UserInfo `json:"users"`
}

// usersInfoBulk fetches multiple users in a single API call using the
// comma-separated "users" parameter (undocumented but used by wee-slack).
func (c *Client) usersInfoBulk(userIDs []string) ([]UserInfo, error) {
	slog.Debug("bulk users.info request", "count", len(userIDs))
	body, err := c.request("users.info", map[string]string{
		"users": strings.Join(userIDs, ","),
	})
	if err != nil {
		return nil, err
	}

	var resp UsersInfoBulkResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse users.info bulk: %w", err)
	}

	return resp.Users, nil
}

// isAPIError checks whether err is an *APIError with the given code.
func isAPIError(err error, code string) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == code
	}
	return false
}

// isHTTPServerError checks whether err is an *APIError from an HTTP 5xx response.
func isHTTPServerError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == "http_error" && apiErr.Status >= 500
	}
	return false
}

// UsersInfoBatch fetches multiple users, returning a map of ID -> UserInfo.
// Uses batch API when available, falling back to sequential calls.
// Errors on individual users are silently skipped.
func (c *Client) UsersInfoBatch(userIDs []string) (map[string]*UserInfo, error) {
	seen := make(map[string]bool)
	unique := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, id)
	}

	if len(unique) == 0 {
		return make(map[string]*UserInfo), nil
	}

	slog.Debug("fetching user info", "count", len(unique), "batch_supported", c.batchSupported)

	if c.batchSupported {
		users, err := c.usersInfoBatchChunked(unique)
		if err == nil {
			return users, nil
		}
	}

	return c.usersInfoSequential(unique), nil
}

// usersInfoBatchChunked fetches users in batches with adaptive chunk sizing.
// Returns a non-nil error only if batch mode should be abandoned (triggering sequential fallback).
func (c *Client) usersInfoBatchChunked(userIDs []string) (map[string]*UserInfo, error) {
	users := make(map[string]*UserInfo)

	for start := 0; start < len(userIDs); {
		if c.maxBatchSize < 1 {
			c.maxBatchSize = 1
		}

		end := start + c.maxBatchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		chunk := userIDs[start:end]

		slog.Debug("batch users.info request", "chunk_size", len(chunk), "max_batch_size", c.maxBatchSize, "offset", start, "total", len(userIDs))

		fetched, err := c.usersInfoBulk(chunk)
		if err != nil {
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				// Non-API errors (network/parse) are treated as transient:
				// return an error so caller can fall back to sequential without disabling batch globally.
				return nil, err
			}

			// user_not_found means all IDs in chunk were invalid — skip chunk, continue
			if isAPIError(err, "user_not_found") {
				slog.Debug("all users in chunk not found, skipping", "chunk_size", len(chunk))
				start = end
				continue
			}

			// too_many_users or HTTP 5xx — halve batch size and retry same chunk
			if isAPIError(err, "too_many_users") || isHTTPServerError(err) {
				if c.maxBatchSize > 1 {
					c.maxBatchSize /= 2
					slog.Warn("batch too large or server error, halving batch size", "new_max_batch_size", c.maxBatchSize, "error", err)
					continue
				}
				slog.Warn("batch users.info failing at size 1, falling back to sequential", "error", err)
				c.batchSupported = false
				return nil, err
			}

			// HTTP 4xx from batch requests are treated as non-retryable client errors.
			if apiErr.Code == "http_error" && apiErr.Status >= 400 && apiErr.Status < 500 {
				slog.Warn("batch users.info HTTP client error, falling back to sequential", "status", apiErr.Status, "error", err)
				c.batchSupported = false
				return nil, err
			}

			// Errors indicating the batch param itself is unsupported.
			if isAPIError(err, "invalid_arguments") || isAPIError(err, "invalid_form_data") {
				slog.Warn("batch users.info param not supported, falling back to sequential", "error", err)
				c.batchSupported = false
				return nil, err
			}

			// Any other Slack API error — log and skip this chunk, don't disable batch.
			slog.Warn("batch users.info chunk failed, skipping chunk", "error", err, "chunk_size", len(chunk))
			start = end
			continue
		}

		for i := range fetched {
			users[fetched[i].ID] = &fetched[i]
		}
		start = end
	}

	return users, nil
}

// usersInfoSequential fetches users one at a time. Used as fallback when batch API is unavailable.
// Relies on existing 429 + Retry-After handling in request() for rate-limit pacing.
func (c *Client) usersInfoSequential(userIDs []string) map[string]*UserInfo {
	slog.Warn("using sequential user lookups — this may be slow", "count", len(userIDs))
	users := make(map[string]*UserInfo)
	for i, id := range userIDs {
		slog.Debug("fetching user sequentially", "index", i+1, "total", len(userIDs), "user_id", id)
		user, err := c.UsersInfo(id)
		if err != nil {
			slog.Debug("failed to fetch user, skipping", "user_id", id, "error", err)
			continue
		}
		users[id] = user
	}
	return users
}
