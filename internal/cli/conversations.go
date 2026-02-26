package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/format"
	slackpkg "github.com/mchowning/slack-pipe/internal/slack"
	"github.com/spf13/cobra"
)

const httpTimeout = 30 * time.Second

func newConversationsCmd(authService *auth.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversations",
		Short: "List and read Slack conversations",
	}

	cmd.AddCommand(
		newConversationsListCmd(authService),
		newConversationsReadCmd(authService),
	)

	return cmd
}

func newConversationsListCmd(svc *auth.Service) *cobra.Command {
	var (
		types           string
		limit           int
		excludeArchived bool
		workspace       string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, cookie, wsURL, err := svc.GetCredentials(workspace)
			if err != nil {
				return err
			}

			client := slackpkg.NewClient(&http.Client{Timeout: httpTimeout}, token, cookie, wsURL)

			channels, err := client.ConversationsList(types, limit, excludeArchived)
			if err != nil {
				return err
			}

			// Resolve DM user names
			var userIDs []string
			for _, ch := range channels {
				if ch.IsIM && ch.User != "" {
					userIDs = append(userIDs, ch.User)
				}
			}
			users, _ := client.UsersInfoBatch(userIDs)

			fmt.Println(format.FormatChannelList(channels, users))
			return nil
		},
	}

	cmd.Flags().StringVar(&types, "types", "public_channel,private_channel,mpim,im", "Conversation types")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max conversations")
	cmd.Flags().BoolVar(&excludeArchived, "exclude-archived", false, "Exclude archived")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace ID or name")

	return cmd
}

func newConversationsReadCmd(svc *auth.Service) *cobra.Command {
	var (
		threadTs  string
		limit     int
		oldest    string
		latest    string
		jsonOut   bool
		workspace string
	)

	cmd := &cobra.Command{
		Use:   "read <channel-id>",
		Short: "Read conversation history or thread",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelID := args[0]

			token, cookie, wsURL, err := svc.GetCredentials(workspace)
			if err != nil {
				return err
			}

			client := slackpkg.NewClient(&http.Client{Timeout: httpTimeout}, token, cookie, wsURL)

			var messages []slackpkg.Message
			if threadTs != "" {
				// conversations.replies returns chronological order — no reversal needed
				messages, err = client.ConversationsReplies(channelID, threadTs, limit)
			} else {
				// conversations.history returns newest-first — reverse to chronological
				messages, err = client.ConversationsHistory(channelID, limit, oldest, latest)
				if err == nil {
					for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
						messages[i], messages[j] = messages[j], messages[i]
					}
				}
			}
			if err != nil {
				return err
			}

			// Resolve user names
			userIDSet := make(map[string]bool)
			for _, m := range messages {
				if m.User != "" {
					userIDSet[m.User] = true
				}
			}
			var userIDs []string
			for id := range userIDSet {
				userIDs = append(userIDs, id)
			}
			users, _ := client.UsersInfoBatch(userIDs)

			if jsonOut {
				output, err := format.MessagesJSON(channelID, messages, users)
				if err != nil {
					return err
				}
				fmt.Println(output)
			} else {
				fmt.Println(format.FormatMessages(channelID, messages, users))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&threadTs, "thread-ts", "", "Thread timestamp")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max messages")
	cmd.Flags().StringVar(&oldest, "oldest", "", "Start of time range")
	cmd.Flags().StringVar(&latest, "latest", "", "End of time range")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace ID or name")

	return cmd
}
