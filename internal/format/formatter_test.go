package format_test

import (
	"strings"
	"testing"

	"github.com/mchowning/slack-pipe/internal/format"
	"github.com/mchowning/slack-pipe/internal/slack"
)

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		ts   string
		want string
	}{
		{"1700000000.000000", "2023-11-14"},
		{"1700100000.000000", "2023-11-15"}, // Second unambiguous timestamp
	}

	for _, tt := range tests {
		result := format.FormatTimestamp(tt.ts)
		if !strings.Contains(result, tt.want) {
			t.Errorf("FormatTimestamp(%q) = %q, want to contain %q", tt.ts, result, tt.want)
		}
	}
}

func TestFormatChannelListCategories(t *testing.T) {
	channels := []slack.Channel{
		{ID: "C1", Name: "general", IsChannel: true},
		{ID: "C2", Name: "secret", IsPrivate: true},
		{ID: "D1", IsIM: true, User: "U1"},
		{ID: "G1", Name: "group", IsMPIM: true},
	}
	users := map[string]*slack.UserInfo{
		"U1": {ID: "U1", Name: "alice", RealName: "Alice Smith"},
	}

	result := format.FormatChannelList(channels, users)

	if !strings.Contains(result, "Public Channels") {
		t.Error("missing Public Channels section")
	}
	if !strings.Contains(result, "Private Channels") {
		t.Error("missing Private Channels section")
	}
	if !strings.Contains(result, "Direct Messages") {
		t.Error("missing Direct Messages section")
	}
	if !strings.Contains(result, "Group Messages") {
		t.Error("missing Group Messages section")
	}
	if !strings.Contains(result, "Alice Smith") {
		t.Error("DM user should be resolved to display name")
	}
	if !strings.Contains(result, "Conversations (4)") {
		t.Error("missing total count")
	}
}

func TestFormatChannelListWithTopicAndArchived(t *testing.T) {
	ch := slack.Channel{ID: "C1", Name: "dev", IsChannel: true, IsArchived: true}
	ch.Topic.Value = "Development discussion"
	channels := []slack.Channel{ch}

	result := format.FormatChannelList(channels, nil)

	if !strings.Contains(result, "[archived]") {
		t.Error("missing archived indicator")
	}
	if !strings.Contains(result, "Development discussion") {
		t.Error("missing topic")
	}
}

func TestFormatMessagesThreadIndicator(t *testing.T) {
	msgs := []slack.Message{
		{Type: "message", User: "U1", Text: "Parent", Ts: "1234.0000", ReplyCount: 3},
		{Type: "message", User: "U2", Text: "Reply", Ts: "1234.0001", ThreadTs: "1234.0000"},
	}
	users := map[string]*slack.UserInfo{
		"U1": {ID: "U1", Name: "alice"},
		"U2": {ID: "U2", Name: "bob"},
	}

	result := format.FormatMessages("C1", msgs, users)

	if !strings.Contains(result, "3 replies") {
		t.Error("missing reply count indicator")
	}
	if !strings.Contains(result, "thread_ts: 1234.0000") {
		t.Error("missing thread_ts for reply")
	}
}

func TestFormatMessagesBotUser(t *testing.T) {
	msgs := []slack.Message{
		{Type: "message", BotID: "B123", Text: "Bot message", Ts: "1234.0000"},
	}

	result := format.FormatMessages("C1", msgs, nil)

	if !strings.Contains(result, "bot:B123") {
		t.Error("missing bot indicator")
	}
}

func TestFormatMessagesWithReactions(t *testing.T) {
	msgs := []slack.Message{
		{
			Type: "message", User: "U1", Text: "Great!", Ts: "1234.0000",
			Reactions: []struct {
				Name  string   `json:"name"`
				Count int      `json:"count"`
				Users []string `json:"users"`
			}{
				{Name: "thumbsup", Count: 3},
				{Name: "heart", Count: 1},
			},
		},
	}

	result := format.FormatMessages("C1", msgs, nil)

	if !strings.Contains(result, ":thumbsup: 3") {
		t.Error("missing thumbsup reaction")
	}
	if !strings.Contains(result, ":heart: 1") {
		t.Error("missing heart reaction")
	}
}

func TestFormatMessagesDisplayNamePriority(t *testing.T) {
	msgs := []slack.Message{
		{Type: "message", User: "U1", Text: "Hello", Ts: "1234.0000"},
	}
	users := map[string]*slack.UserInfo{
		"U1": {
			ID: "U1", Name: "alice", RealName: "Alice Smith",
			Profile: struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
				Email       string `json:"email"`
			}{DisplayName: "alice-display"},
		},
	}

	result := format.FormatMessages("C1", msgs, users)

	if !strings.Contains(result, "@alice-display") {
		t.Errorf("expected display_name priority, got: %s", result)
	}
}

func TestMessagesJSON(t *testing.T) {
	msgs := []slack.Message{
		{Type: "message", User: "U1", Text: "Hello", Ts: "1234.5678"},
	}
	users := map[string]*slack.UserInfo{
		"U1": {ID: "U1", Name: "alice", RealName: "Alice"},
	}

	result, err := format.MessagesJSON("C1", msgs, users)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, `"channel_id": "C1"`) {
		t.Error("missing channel_id")
	}
	if !strings.Contains(result, `"message_count": 1`) {
		t.Error("missing message_count")
	}
	if !strings.Contains(result, `"text": "Hello"`) {
		t.Error("missing message text")
	}
	if !strings.Contains(result, `"name": "alice"`) {
		t.Error("missing user in JSON")
	}
}

func TestFormatEmptyChannelList(t *testing.T) {
	result := format.FormatChannelList(nil, nil)
	if !strings.Contains(result, "Conversations (0)") {
		t.Errorf("expected 0 count, got: %s", result)
	}
}

func TestFormatEmptyMessages(t *testing.T) {
	result := format.FormatMessages("C1", nil, nil)
	if !strings.Contains(result, "0 messages") {
		t.Errorf("expected 0 messages, got: %s", result)
	}
}
