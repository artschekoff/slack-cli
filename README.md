<div align="center">

<img src="assets/cover.jpg" alt="slack-cli — Slack for your AI assistant" width="100%" />

# slack-cli

**Slack for your AI assistant.** A command-line tool — and the engine behind the `slack-mcp` Model Context Protocol server — that lets Claude, Cursor, and any MCP client search messages, browse channels, load threads, and resolve users straight from chat.

[![Go Reference](https://img.shields.io/badge/go-reference-00ADD8?logo=go&logoColor=white)](https://pkg.go.dev/github.com/artschekoff/slack-cli)
[![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![MCP](https://img.shields.io/badge/protocol-MCP-7C3AED)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

</div>

---

## ✨ Why slack-cli

- 🔍 **Search across a workspace** — full-text message search by query, count, and start date, results grouped by channel with permalinks and `thread_ts`.
- 🧵 **Load whole threads** — pull every reply, reaction, and file attachment in a thread, as plain text or AI-ready markdown.
- 📇 **Browse channels & DMs** — list channels matching a pattern or recent direct-message conversations as structured JSON.
- 🙋 **Resolve users** — turn opaque Slack user IDs into display names.
- 🔐 **Credentials encrypted at rest** — AES-256-GCM with an argon2id-derived key; no plaintext tokens on disk.
- ⚡ **Single static binary** — pure Go, drives the `slack-mcp` server over stdio and drops into any MCP client in seconds.

## Install

```bash
go install github.com/artschekoff/slack-cli/cmd/slack-cli@latest
# or
git clone https://github.com/artschekoff/slack-cli.git && cd slack-cli && make install
```

Requires Go 1.26+.

## Prerequisites

A Slack workspace you can sign in to. `slack-cli` authenticates with a browser-session token (`xoxc-…`) + cookie (`xoxd-…`) — no Slack app or admin approval required. Run `slack-cli auth <workspace>` to extract and store them interactively, or `slack-cli auth-start <workspace>` to print the DevTools extraction steps.

## Login

The master passphrase that decrypts your workspace credentials is stored,
encrypted, at `~/.slack/session`.

```bash
slack-cli login              # interactive prompt
echo "$PASS" | slack-cli login --stdin   # for scripts / CI
slack-cli logout             # delete the stored passphrase
```

After `login`, every other subcommand — and any MCP host that launches
`slack-cli` — reads the passphrase from the session file automatically.

**Security note.** The session file is AES-256-GCM encrypted with a key
derived from your hostname and home directory. This defends against casual
disk inspection and cloud-backup exposure. It does **not** protect against
local malware running as your user account; anything with your shell
privileges can call `slack-cli login`-derived keys the same way this
process does. If you need stronger guarantees, use full-disk encryption
and lock your machine.

## MCP server setup

`slack-cli` powers the `slack-mcp` server. Add it to your MCP client config. No env var is required — the MCP host inherits the passphrase from your `~/.slack/session` file (run `slack-cli login` once before starting the MCP host).

**Cursor** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "slack": {
      "type": "stdio",
      "command": "slack-mcp"
    }
  }
}
```

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-mcp"
    }
  }
}
```

To save credentials, run `slack-cli login` once before starting the MCP host:

```bash
slack-cli login                                    # interactive prompt for passphrase
slack-cli auth-complete acme --token xoxc-... --cookie xoxd-...  # save credentials for a workspace
```

## 🛠️ Tools

The `slack-mcp` server exposes these operations; each maps to a `slack-cli` command.

| Command | Parameters | Description |
|---|---|---|
| `search` | `<workspace> <query>` · `--count` `--start-from` | Search messages; results grouped by channel with author, timestamp, text, permalink, `thread_ts`. |
| `search-channels` | `<workspace> <pattern>` | List channels whose names contain the pattern → JSON array of `{id, name, messages}`. |
| `list-dms` | `<workspace>` · `--start-from` | List direct-message conversations → JSON array of `{id, userId, userName, name, isIm}`. |
| `load-thread` | `<workspace> <channel-id> <ts>` | Load every message in a thread (text, reactions, file attachments). |
| `load-context` | `<workspace> <channel-id> <ts>` · `--permalink` `--channel-name` `--search-query` `--start-from` | Load a thread as AI-ready markdown with a channel/date/permalink header. |
| `get-user` | `<workspace> <user-id>` | Resolve a Slack user ID to a display name. |

### Credential management

| Command | Parameters | Description |
|---|---|---|
| `auth` | `[workspace]` | Interactive auth — opens browser, prompts for token + cookie. |
| `auth-start` | `[workspace]` | Print DevTools extraction instructions without saving. |
| `auth-complete` | `<workspace> --token --cookie` | Save credentials non-interactively. |
| `list-workspaces` | — | List all saved workspace names. |
| `get-credentials` | `<workspace>` | Show whether token + cookie are present. |
| `test-creds` | `[workspace]` | Validate stored credentials against Slack `auth.test`. |
| `remove-creds` | `[workspace]` | Delete stored credentials for a workspace. |
| `show-creds` | — | Print the path to the credentials file. |

## 💬 Try it

Once wired up, just ask your assistant:

> "Search the acme workspace for messages about the deployment that failed yesterday."
> "Load the thread behind this Slack permalink and summarize what was decided."
> "Which acme channels are about onboarding, and who posts in them most?"

## 🚀 CLI

```bash
slack-cli auth acme                                   # authenticate a workspace
slack-cli search acme "deployment failed" --count 20  # search messages
slack-cli search-channels acme deploy                 # list matching channels (JSON)
slack-cli list-dms acme --start-from 2024-01-01       # recent DMs (JSON)
slack-cli load-thread acme C01234567 1700000000.123456
```

Run `slack-cli --help` for the full command list.

### Credentials & encryption

Credentials live in `~/.slack/workspace_credentials.json`, each workspace entry encrypted independently with **AES-256-GCM** and a key derived from your passphrase via **argon2id** (`time=1`, `memory=64 MiB`, `threads=4`). A fresh 16-byte salt and 12-byte nonce are generated per write, and the argon2id parameters are stored alongside the ciphertext so future upgrades stay backward-compatible. If you see `failed to read credentials for workspace '<name>'`, re-authenticate with the correct passphrase.

## 📦 Releasing

```bash
make release    # prompts for major/minor/patch, tags, builds all platforms, publishes a GitHub release
```

## 📄 License

[MIT](LICENSE)
