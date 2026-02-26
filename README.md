# slack-pipe

A read-only CLI for Slack that authenticates using browser session tokens. Tokens are stored in macOS Keychain — never written to disk.

## Why?

Slack doesn't offer a simple way to read workspace messages from the command line without creating a bot app and going through OAuth. `slack-pipe` sidesteps that by reusing the same session tokens your browser already has. Copy a cURL command from DevTools, pipe it in, and you're authenticated.

Output goes to stdout, status messages to stderr, making it easy to pipe into `jq`, `grep`, or other tools.

## Install

### Nix (recommended)

```bash
nix build
./result/bin/slack-pipe --version
```

Or run directly without installing:

```bash
nix run . -- conversations list
```

### From source

Requires Go 1.24+:

```bash
go build -o slack-pipe ./cmd/slack-pipe
```

## Getting Started

### Step 1: Authenticate

You need a session token and cookie from your browser. The easiest way is to copy a cURL command from DevTools.

1. Open Slack in your **browser** (not the desktop app)
2. Open DevTools (`Cmd+Option+I` on macOS)
3. Go to the **Network** tab
4. Do anything in Slack (switch channels, scroll) to trigger API requests
5. Find any request to `*.slack.com/api/*`
6. Right-click → **Copy as cURL**
7. Authenticate:

```bash
# Pipe from clipboard (macOS)
pbpaste | slack-pipe auth parse-curl

# Or from a file
slack-pipe auth parse-curl < curl.txt

# Or paste interactively (Ctrl+D when done)
slack-pipe auth parse-curl
```

On success you'll see:

```
Parsed workspace: myteam (https://myteam.slack.com)
Authenticating...
✅ Authenticated: myteam (T0ABC1234)
```

#### Manual token entry

If you'd rather enter the token and cookie separately:

```bash
slack-pipe auth login --workspace-url=https://myteam.slack.com
```

You'll be prompted for the xoxc token and d cookie. Input is not echoed to the terminal.

### Step 2: List conversations

```bash
slack-pipe conversations list
```

This lists all your channels, DMs, and group messages. Filter by type:

```bash
slack-pipe conversations list --types=public_channel
slack-pipe conversations list --types=im                    # DMs only
slack-pipe conversations list --types=public_channel,im     # combine types
slack-pipe conversations list --exclude-archived
```

### Step 3: Read messages

Use a channel ID from the list output:

```bash
slack-pipe conversations read C0ABC1234
```

Messages are printed in chronological order with timestamps and usernames.

Read a thread:

```bash
slack-pipe conversations read C0ABC1234 --thread-ts=1700000000.123456
```

Limit the number of messages:

```bash
slack-pipe conversations read C0ABC1234 --limit=10
```

Get JSON output for scripting:

```bash
slack-pipe conversations read C0ABC1234 --json
slack-pipe conversations read C0ABC1234 --json | jq '.messages[] | .text'
```

## Commands

### `auth`

Manage workspace authentication.

| Command | Description |
|---|---|
| `auth parse-curl` | Authenticate by parsing a cURL command from stdin |
| `auth login` | Authenticate by entering token and cookie interactively |
| `auth list` | Show authenticated workspaces |
| `auth set-default <id>` | Switch the default workspace |
| `auth remove <id>` | Remove a workspace and delete its keyring entries |
| `auth logout` | Remove all workspaces and keyring entries |

### `conversations`

List and read Slack conversations.

#### `conversations list`

```
slack-pipe conversations list [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--types` | `public_channel,private_channel,mpim,im` | Conversation types to include |
| `--limit` | `100` | Maximum number of conversations |
| `--exclude-archived` | `false` | Hide archived channels |
| `--workspace` | *(default workspace)* | Workspace ID or name |

#### `conversations read <channel-id>`

```
slack-pipe conversations read <channel-id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--limit` | `100` | Maximum number of messages |
| `--thread-ts` | | Thread timestamp — read replies instead of channel history |
| `--oldest` | | Only messages after this Slack timestamp |
| `--latest` | | Only messages before this Slack timestamp |
| `--json` | `false` | Output as JSON |
| `--workspace` | *(default workspace)* | Workspace ID or name |

## Multiple Workspaces

You can authenticate with multiple Slack workspaces. The first one you add becomes the default.

```bash
# Authenticate with a second workspace
pbpaste | slack-pipe auth parse-curl

# See all workspaces
slack-pipe auth list

# Use a specific workspace for a command
slack-pipe conversations list --workspace=T0OTHER99

# Change the default
slack-pipe auth set-default T0OTHER99
```

## Environment Variables

For CI, scripts, or systems without a Keychain:

```bash
export SLACK_TOKEN=xoxc-...
export SLACK_COOKIE=xoxd-...
slack-pipe conversations list
```

The CLI tries macOS Keychain first, then falls back to these environment variables.

## Security

- **Tokens never touch disk.** They're stored in macOS Keychain or read from environment variables.
- **Interactive input is not echoed.** The `auth login` command uses TTY password prompts.
- **Tokens are never passed as CLI flags.** No risk of leaking into shell history or `ps` output.
- **Config file contains no secrets.** `~/.config/slack-pipe/workspaces.json` stores only workspace ID, name, and URL.
- **Read-only by design.** No commands to send messages, react, or modify anything.

## Development

```bash
nix develop                      # Dev shell with Go, golangci-lint, gopls
go test ./...                    # Run tests
go test ./... -cover             # With coverage
golangci-lint run ./...          # Lint
nix build                        # Reproducible build
```

## How It Works

`slack-pipe` authenticates the same way the Slack web client does:

- The **xoxc token** is sent as a form field in POST requests
- The **d cookie** (xoxd) is sent in the `Cookie` header

This is the browser session auth method — no OAuth app or bot token required. The same approach is used by [slackcli](https://github.com/nickstenning/slackcli) and [wee-slack](https://github.com/wee-slack/wee-slack).

Rate limiting (HTTP 429) is handled automatically with retries using the `Retry-After` header.
