# AGENTS.md

## Reference Projects

Prominent Slack CLI and terminal projects worth studying for inspiration. Ordered by relevance to `slack-pipe`.

### Tier 1: Direct Inspiration (Session Token Auth, Read-Oriented)

#### [wee-slack](https://github.com/wee-slack/wee-slack) — ⭐ 2,598 · Python · MIT · Active

The gold standard for non-bot Slack clients. A WeeChat native plugin that connects via the Slack API and maintains a persistent WebSocket for real-time events. Supports session tokens (xoxc + xoxd cookies), threads, reactions, read markers, typing notifications, and multi-workspace.

**Why study it:**
- Session token auth implementation (`slack/slack_api.py`, `slack/util.py`) — sends xoxc as `Authorization: Bearer` + d cookie in `Cookie` header
- Also supports a `d-s` cookie alongside the `d` cookie
- `extract_token_from_browser.py` — reads Chrome/Firefox cookie DBs directly (future work for us)
- Rate limiting with `Retry-After` + exponential backoff (`slack/http.py`)
- Comprehensive block/attachment rendering (`slack/slack_message.py`)
- Well-structured test suite (~7k lines of Python)
- Multi-workspace architecture (`slack/slack_workspace.py`)

**Last release:** v2.11.0 (Jan 2026) · Actively maintained since 2014

---

#### [slackcli](https://github.com/shaharia-lab/slackcli) — ⭐ 26 · TypeScript · MIT · Active

A newer CLI built with Bun, explicitly designed for AI agent integration. Supports both bot tokens (xoxb/xoxp) and browser session tokens (xoxd/xoxc). Includes cURL parsing, multi-workspace management, and structured output (JSON, table, text).

**Why study it:**
- cURL parser (`src/lib/curl-parser.ts`) — direct parallel to our `internal/curl/parser.go`
- Session token auth sends xoxc as POST form field + d cookie in `Cookie` header (`src/lib/slack-client.ts`)
- Workspace management (`src/lib/workspaces.ts`) — stores tokens in plaintext JSON (our Keychain approach improves on this)
- Write operations (send messages, reactions) — reference for future phases
- Clean command structure (`src/commands/`) maps closely to our Cobra commands

**Last release:** v0.2.3 (Feb 2026) · Small but actively developed

---

### Tier 2: Terminal Clients (UI/UX Patterns)

#### [slack-term](https://github.com/jpbruinsslot/slack-term) — ⭐ 6,584 · Go · MIT · Unmaintained

Full TUI Slack client written in Go. The most popular terminal Slack project by star count. Uses bot tokens (xoxp), not session tokens.

**Why study it:**
- Go project structure and Slack API usage patterns
- Message formatting and display logic
- Channel navigation and selection UX
- Shows what a full TUI client looks like (we're staying pipe-oriented, but useful for understanding user expectations)

**Last commit:** Apr 2024 · No longer actively maintained

---

#### [sclack](https://github.com/haskellcamargo/sclack) — ⭐ 2,479 · Python · GPL-3.0 · Unmaintained

"The best CLI client for Slack, because everything is terrible." A rich TUI with keybindings, sidebar, threads, and emoji rendering. Uses bot tokens.

**Why study it:**
- Polished terminal rendering of Slack messages
- Thread display UX
- Emoji and reaction rendering patterns

**Last commit:** Dec 2022 · Unmaintained

---

### Tier 3: Pipe/Scripting Tools (Unix Philosophy)

#### [slackcat](https://github.com/bcicen/slackcat) — ⭐ 1,219 · Go · MIT · Low Activity

CLI utility to post files and command output to Slack. Pure Unix pipe philosophy — `cat file | slackcat --channel general`.

**Why study it:**
- Go + Slack API patterns
- Pipe-oriented I/O design (most relevant to our `slack-pipe` philosophy)
- Simple, focused scope — does one thing well

**Last commit:** Jul 2024

---

#### [slack-cli](https://github.com/rockymadden/slack-cli) — ⭐ 1,125 · Shell · No License · Unmaintained

Pure bash Slack CLI. Rich messaging, file uploads, piping support. Demonstrates how much can be done with shell scripting and `curl`.

**Why study it:**
- Shows the full Slack Web API surface from a scripting perspective
- Pipe-oriented design patterns
- Demonstrates the DX people expect from a Slack CLI tool

**Last commit:** Feb 2023 · Unmaintained

---

#### [slacktee](https://github.com/coursehero/slacktee) — ⭐ 828 · Shell · Apache-2.0 · Unmaintained

Works like `tee` but posts stdin to Slack. Simple and elegant.

**Why study it:**
- Minimal, `tee`-like interface — closest to the Unix pipe philosophy
- Clean stdin handling patterns

**Last commit:** Mar 2023 · Unmaintained

---

### Tier 4: SDK/Libraries (API Reference)

These aren't CLIs but are the canonical Slack API implementations. Useful for understanding API surface, error handling, and rate limiting.

| Project | Stars | Language | Notes |
|---------|-------|----------|-------|
| [slack-go/slack](https://github.com/slack-go/slack) | 4,899 | Go | **Our language.** The de-facto Go Slack library. Conversations API, pagination, rate limiting. We don't depend on it (we use raw HTTP) but it's the reference for API semantics. |
| [python-slack-sdk](https://github.com/slackapi/python-slack-sdk) | 3,997 | Python | Slack's official Python SDK. Most complete API coverage. |
| [node-slack-sdk](https://github.com/slackapi/node-slack-sdk) | 3,358 | TypeScript | Slack's official Node SDK. Web API, Events API, Socket Mode. |
| [bolt-js](https://github.com/slackapi/bolt-js) | 2,889 | TypeScript | Slack's app framework. Higher-level patterns for handling events and commands. |

---

### Key Patterns Across Projects

| Pattern | wee-slack | slackcli | slack-term | slackcat |
|---------|-----------|----------|------------|----------|
| Session token auth (xoxc+xoxd) | ✅ | ✅ | ❌ | ❌ |
| Bot token auth (xoxb/xoxp) | ❌ | ✅ | ✅ | ✅ |
| Multi-workspace | ✅ | ✅ | ❌ | ❌ |
| Rate limiting / retry | ✅ | ❌ | ❌ | ❌ |
| Pipe-oriented I/O | ❌ | ✅ | ❌ | ✅ |
| Thread support | ✅ | ✅ | ❌ | ❌ |
| Secure token storage | ❌ | ❌ | ❌ | ✅ (OAuth) |
| JSON output | ❌ | ✅ | ❌ | ❌ |
