package cli

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/format"
	slackpkg "github.com/mchowning/slack-pipe/internal/slack"
	"github.com/spf13/cobra"
)

const (
	sentDateLayout       = "2006-01-02"
	defaultSearchPerPage = 100
)

type sentDateWindow struct {
	Start        time.Time
	EndExclusive time.Time
}

func newMessagesCmd(authService *auth.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Search messages",
	}

	cmd.AddCommand(newMessagesSentCmd(authService))

	return cmd
}

func newMessagesSentCmd(svc *auth.Service) *cobra.Command {
	var (
		date      string
		startDate string
		endDate   string
		workspace string
		limit     int
		textOut   bool
	)

	cmd := &cobra.Command{
		Use:   "sent",
		Short: "List messages sent by the authenticated user for a date or date range",
		RunE: func(cmd *cobra.Command, args []string) error {
			window, err := parseSentDateWindow(date, startDate, endDate, time.Local)
			if err != nil {
				return err
			}

			token, cookie, wsURL, err := svc.GetCredentials(workspace)
			if err != nil {
				return err
			}

			client := slackpkg.NewClient(&http.Client{Timeout: httpTimeout}, token, cookie, wsURL)

			sent, err := collectSentMessages(client, window, limit)
			if err != nil {
				return err
			}

			if textOut {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), format.FormatSentMessages(sent)); err != nil {
					return err
				}
				return nil
			}

			out, err := format.SentMessagesJSON(sent)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), out); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Single date in YYYY-MM-DD (local timezone)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date in YYYY-MM-DD (local timezone)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date in YYYY-MM-DD (local timezone)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace ID or name")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum messages to return (0 for no cap)")
	cmd.Flags().BoolVar(&textOut, "text", false, "Human-readable output (JSON is default)")

	return cmd
}

func parseSentDateWindow(date, startDate, endDate string, loc *time.Location) (sentDateWindow, error) {
	if date != "" && (startDate != "" || endDate != "") {
		return sentDateWindow{}, fmt.Errorf("invalid flags: use either --date or --start-date/--end-date, not both")
	}

	if date == "" && startDate == "" && endDate == "" {
		return sentDateWindow{}, fmt.Errorf("missing date input: provide --date or --start-date with --end-date")
	}

	if date != "" {
		day, err := parseLocalDate(date, loc)
		if err != nil {
			return sentDateWindow{}, fmt.Errorf("invalid --date: %w", err)
		}
		return sentDateWindow{Start: day, EndExclusive: day.AddDate(0, 0, 1)}, nil
	}

	if startDate == "" || endDate == "" {
		return sentDateWindow{}, fmt.Errorf("invalid range: --start-date and --end-date must be provided together")
	}

	start, err := parseLocalDate(startDate, loc)
	if err != nil {
		return sentDateWindow{}, fmt.Errorf("invalid --start-date: %w", err)
	}
	end, err := parseLocalDate(endDate, loc)
	if err != nil {
		return sentDateWindow{}, fmt.Errorf("invalid --end-date: %w", err)
	}
	if end.Before(start) {
		return sentDateWindow{}, fmt.Errorf("invalid range: --end-date must be on or after --start-date")
	}

	return sentDateWindow{Start: start, EndExclusive: end.AddDate(0, 0, 1)}, nil
}

func parseLocalDate(v string, loc *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation(sentDateLayout, v, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD, got %q", v)
	}
	return parsed, nil
}

func collectSentMessages(client *slackpkg.Client, window sentDateWindow, limit int) ([]format.SentMessage, error) {
	var sent []format.SentMessage

	for page := 1; ; page++ {
		pageSize := defaultSearchPerPage
		if limit > 0 {
			remaining := limit - len(sent)
			if remaining <= 0 {
				break
			}
			if remaining < pageSize {
				pageSize = remaining
			}
		}

		resp, err := client.SearchMessages("from:me", pageSize, page, "timestamp", "desc")
		if err != nil {
			return nil, err
		}
		if len(resp.Messages.Matches) == 0 {
			break
		}

		oldestBeforeStart := false
		for _, match := range resp.Messages.Matches {
			msgTime, err := parseSlackTimestamp(match.Ts)
			if err != nil {
				continue
			}

			if msgTime.Before(window.Start) {
				oldestBeforeStart = true
				continue
			}
			if !msgTime.Before(window.EndExclusive) {
				continue
			}

			channelName := match.Channel.Name
			if channelName == "" {
				channelName = match.Channel.ID
			}

			sent = append(sent, format.SentMessage{
				Ts:        match.Ts,
				Time:      msgTime.Format("2006-01-02 15:04:05"),
				ChannelID: match.Channel.ID,
				Channel:   channelName,
				Text:      match.Text,
				Permalink: match.Permalink,
			})

			if limit > 0 && len(sent) >= limit {
				break
			}
		}

		if limit > 0 && len(sent) >= limit {
			break
		}
		if oldestBeforeStart {
			break
		}

		if resp.Messages.Paging.Pages > 0 && resp.Messages.Paging.Page >= resp.Messages.Paging.Pages {
			break
		}
		if len(resp.Messages.Matches) < pageSize {
			break
		}
	}

	return sent, nil
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	parts := strings.SplitN(ts, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	micro := int64(0)
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 6 {
			frac = frac[:6]
		}
		if len(frac) < 6 {
			frac = frac + strings.Repeat("0", 6-len(frac))
		}
		micro, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}

	return time.Unix(sec, micro*1000).In(time.Local), nil
}
