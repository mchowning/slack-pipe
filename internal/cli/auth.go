package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/curl"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAuthCmd(authService *auth.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage workspace authentication",
	}

	cmd.AddCommand(
		newAuthLoginCmd(authService),
		newAuthParseCurlCmd(authService),
		newAuthListCmd(authService),
		newAuthSetDefaultCmd(authService),
		newAuthRemoveCmd(authService),
		newAuthLogoutCmd(authService),
	)

	return cmd
}

func newAuthLoginCmd(svc *auth.Service) *cobra.Command {
	var workspaceURL string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login with session tokens (interactive, no echo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read token from TTY without echoing
			fmt.Fprint(os.Stderr, "Enter xoxc token: ")
			tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return fmt.Errorf("read token: %w", err)
			}

			fmt.Fprint(os.Stderr, "Enter d cookie (xoxd-...): ")
			cookieBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return fmt.Errorf("read cookie: %w", err)
			}

			ws, err := svc.Login(
				strings.TrimSpace(string(tokenBytes)),
				strings.TrimSpace(string(cookieBytes)),
				workspaceURL,
			)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "✅ Authenticated: %s (%s)\n", ws.Name, ws.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceURL, "workspace-url", "", "Workspace URL (e.g., https://myteam.slack.com)")
	_ = cmd.MarkFlagRequired("workspace-url")

	return cmd
}

func newAuthParseCurlCmd(svc *auth.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "parse-curl",
		Short: "Extract tokens from a pasted cURL command",
		Long: `Extract tokens from a pasted cURL command.

Reads from stdin. Usage:
  pbpaste | slack-pipe auth parse-curl
  slack-pipe auth parse-curl < curl.txt
  slack-pipe auth parse-curl   (paste, then Ctrl+D to finish)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintln(os.Stderr, "Paste your cURL command, then press Ctrl+D when done:")
			}

			input, readErr := io.ReadAll(os.Stdin)
			if readErr != nil {
				return fmt.Errorf("read stdin: %w", readErr)
			}
			if strings.TrimSpace(string(input)) == "" {
				return fmt.Errorf("no input provided")
			}

			parsed, err := curl.Parse(string(input))
			if err != nil {
				return fmt.Errorf("parse failed: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Parsed workspace: %s (%s)\n", parsed.WorkspaceName, parsed.WorkspaceURL)
			fmt.Fprintf(os.Stderr, "Authenticating...\n")

			ws, err := svc.Login(parsed.XoxcToken, parsed.DCookie, parsed.WorkspaceURL)
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "✅ Authenticated: %s (%s)\n", ws.Name, ws.ID)
			return nil
		},
	}
}

func newAuthListCmd(svc *auth.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List authenticated workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaces, defaultID, err := svc.ListWorkspaces()
			if err != nil {
				return err
			}
			if len(workspaces) == 0 {
				fmt.Fprintln(os.Stderr, "No authenticated workspaces. Run 'slack-pipe auth login' to authenticate.")
				return nil
			}
			fmt.Fprintf(os.Stderr, "📋 Authenticated Workspaces (%d):\n\n", len(workspaces))
			for i, ws := range workspaces {
				badge := ""
				if ws.ID == defaultID {
					badge = " (default)"
				}
				fmt.Fprintf(os.Stderr, "  %d. %s%s\n     ID: %s\n     URL: %s\n\n", i+1, ws.Name, badge, ws.ID, ws.URL)
			}
			return nil
		},
	}
}

func newAuthSetDefaultCmd(svc *auth.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <workspace-id>",
		Short: "Set default workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.SetDefault(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✅ Set %s as default workspace\n", args[0])
			return nil
		},
	}
}

func newAuthRemoveCmd(svc *auth.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <workspace-id>",
		Short: "Remove a workspace (deletes keyring entries)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✅ Removed workspace %s\n", args[0])
			return nil
		},
	}
}

func newAuthLogoutCmd(svc *auth.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Logout from all workspaces (clears keyring)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := svc.Logout(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "✅ Logged out from all workspaces")
			return nil
		},
	}
}
