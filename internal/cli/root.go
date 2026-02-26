package cli

import (
	"fmt"
	"os"

	"github.com/mchowning/slack-pipe/internal/auth"
	"github.com/mchowning/slack-pipe/internal/config"
	"github.com/mchowning/slack-pipe/internal/keyring"
	"github.com/mchowning/slack-pipe/internal/logging"
	slackpkg "github.com/mchowning/slack-pipe/internal/slack"
	"github.com/spf13/cobra"
)

// OSFileSystem implements config.FileSystem using real OS calls.
type OSFileSystem struct{}

func (OSFileSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) } //nolint:gosec // path comes from our own config, not user input
func (OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFileSystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func NewRootCmd(version, commit string) *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:     "slack-pipe",
		Short:   "Secure CLI for reading Slack via session tokens",
		Version: fmt.Sprintf("%s (commit: %s)", version, commit),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cleanup, err := logging.Setup(verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not set up logging: %v\n", err)
				return nil // non-fatal — don't block the command
			}
			// Register cleanup to run after the command completes
			cmd.PostRunE = func(cmd *cobra.Command, args []string) error {
				cleanup()
				return nil
			}
			return nil
		},
	}

	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")

	// Build dependency graph
	kr := buildKeyring()
	fs := OSFileSystem{}
	configStore := config.NewStore(fs, config.DefaultDirPath())
	authTester := slackpkg.NewAuthAdapter()
	authSvc := auth.NewService(kr, configStore, authTester)

	cmd.AddCommand(newAuthCmd(authSvc))
	cmd.AddCommand(newConversationsCmd(authSvc))
	cmd.AddCommand(newMessagesCmd(authSvc))

	return cmd
}

// buildKeyring returns macOS Keychain, falling back to env vars.
func buildKeyring() keyring.Store {
	return keyring.NewWithFallback(keyring.NewMacOS(), keyring.NewEnv())
}
