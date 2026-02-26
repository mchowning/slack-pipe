package format

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mchowning/slack-pipe/internal/slack"
)

// FormatTimestamp converts a Slack ts string to human-readable time.
func FormatTimestamp(ts string) string {
	var sec, nsec int64
	fmt.Sscanf(ts, "%d.%d", &sec, &nsec) //nolint:gosec,errcheck // best-effort parse
	t := time.Unix(sec, nsec*1000)        // Slack nsec is microseconds
	return t.Format("2006-01-02 15:04:05")
}

// FormatChannelList formats channels for human-readable output.
func FormatChannelList(channels []slack.Channel, users map[string]*slack.UserInfo) string {
	var public, private, dms, groups []slack.Channel

	for _, ch := range channels {
		switch {
		case ch.IsIM:
			dms = append(dms, ch)
		case ch.IsMPIM:
			groups = append(groups, ch)
		case ch.IsPrivate:
			private = append(private, ch)
		default:
			public = append(public, ch)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📋 Conversations (%d)\n", len(channels))

	writeSection := func(title string, chs []slack.Channel) {
		if len(chs) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n%s:\n", title)
		for i, ch := range chs {
			if ch.IsIM && ch.User != "" {
				name := ch.User
				if u, ok := users[ch.User]; ok {
					name = displayName(u)
				}
				fmt.Fprintf(&b, "  %d. 👤 @%s (%s)\n", i+1, name, ch.ID)
			} else {
				prefix := "#"
				if ch.IsPrivate {
					prefix = "🔒"
				}
				if ch.IsMPIM {
					prefix = "👥"
				}
				archived := ""
				if ch.IsArchived {
					archived = " [archived]"
				}
				fmt.Fprintf(&b, "  %d. %s %s (%s)%s\n", i+1, prefix, ch.Name, ch.ID, archived)
				if ch.Topic.Value != "" {
					fmt.Fprintf(&b, "     %s\n", ch.Topic.Value)
				}
			}
		}
	}

	writeSection("Public Channels", public)
	writeSection("Private Channels", private)
	writeSection("Group Messages", groups)
	writeSection("Direct Messages", dms)

	return b.String()
}

// FormatMessages formats messages for human-readable output.
func FormatMessages(channelID string, messages []slack.Message, users map[string]*slack.UserInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "💬 #%s (%d messages)\n\n", channelID, len(messages))

	for _, msg := range messages {
		userName := msg.User
		if u, ok := users[msg.User]; ok {
			userName = displayName(u)
		}
		if msg.BotID != "" && userName == "" {
			userName = fmt.Sprintf("bot:%s", msg.BotID)
		}

		ts := FormatTimestamp(msg.Ts)
		fmt.Fprintf(&b, "[%s] @%s\n", ts, userName)

		for _, line := range strings.Split(msg.Text, "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}

		fmt.Fprintf(&b, "  ts: %s", msg.Ts)
		if msg.ThreadTs != "" && msg.ThreadTs != msg.Ts {
			fmt.Fprintf(&b, " | thread_ts: %s", msg.ThreadTs)
		}
		fmt.Fprintln(&b)

		if len(msg.Reactions) > 0 {
			var parts []string
			for _, r := range msg.Reactions {
				parts = append(parts, fmt.Sprintf(":%s: %d", r.Name, r.Count))
			}
			fmt.Fprintf(&b, "  %s\n", strings.Join(parts, "  "))
		}

		if msg.ReplyCount > 0 && (msg.ThreadTs == "" || msg.ThreadTs == msg.Ts) {
			fmt.Fprintf(&b, "  💬 %d replies\n", msg.ReplyCount)
		}

		fmt.Fprintln(&b)
	}

	return b.String()
}

// MessagesJSON returns messages in JSON format with user info.
func MessagesJSON(channelID string, messages []slack.Message, users map[string]*slack.UserInfo) (string, error) {
	type jsonUser struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		RealName string `json:"real_name"`
		Email    string `json:"email,omitempty"`
	}

	type jsonMessage struct {
		Ts         string `json:"ts"`
		ThreadTs   string `json:"thread_ts,omitempty"`
		User       string `json:"user,omitempty"`
		Text       string `json:"text"`
		Type       string `json:"type"`
		ReplyCount int    `json:"reply_count,omitempty"`
		BotID      string `json:"bot_id,omitempty"`
	}

	output := struct {
		ChannelID    string        `json:"channel_id"`
		MessageCount int           `json:"message_count"`
		Messages     []jsonMessage `json:"messages"`
		Users        []jsonUser    `json:"users"`
	}{
		ChannelID:    channelID,
		MessageCount: len(messages),
	}

	for _, m := range messages {
		output.Messages = append(output.Messages, jsonMessage{
			Ts: m.Ts, ThreadTs: m.ThreadTs, User: m.User, Text: m.Text,
			Type: m.Type, ReplyCount: m.ReplyCount, BotID: m.BotID,
		})
	}

	// Collect and sort users
	var userList []jsonUser
	for _, u := range users {
		userList = append(userList, jsonUser{
			ID: u.ID, Name: u.Name, RealName: u.RealName, Email: u.Profile.Email,
		})
	}
	sort.Slice(userList, func(i, j int) bool { return userList[i].ID < userList[j].ID })
	output.Users = userList

	raw, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func displayName(u *slack.UserInfo) string {
	if u.Profile.DisplayName != "" {
		return u.Profile.DisplayName
	}
	if u.RealName != "" {
		return u.RealName
	}
	return u.Name
}
