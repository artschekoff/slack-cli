# slack-cli

A command-line tool and MCP server for managing Slack credentials and interacting with Slack workspaces — search messages, browse channels, load threads, and resolve users.

---

## Installation

```bash
go install ./cmd/slack-cli
```

---

## Quick Start

```bash
# 1. Authenticate a workspace
slack-cli auth acme

# 2. Search messages
slack-cli search acme "deployment failed"

# 3. List channels matching a pattern
slack-cli search-channels acme deploy

# 4. List recent DMs
slack-cli list-dms acme --start-from 2024-01-01

# 5. Load a thread
slack-cli load-thread acme C01234567 1700000000.123456
```

---

## Commands

### Credential management

| Command | Description | Output |
|---|---|---|
| `slack-cli auth [workspace]` | Interactive auth — opens browser, prompts for token + cookie | Plain text — success message with workspace/user info |
| `slack-cli auth-start [workspace]` | Print extraction instructions without saving | Plain text — step-by-step guide for DevTools extraction |
| `slack-cli auth-complete <workspace> --token xoxc-... --cookie xoxd-...` | Save credentials non-interactively | Plain text — success or validation error |
| `slack-cli list-workspaces` | List all saved workspace names | Plain text — one workspace name per line |
| `slack-cli get-credentials <workspace>` | Show whether token + cookie are present | Plain text — `present` or `missing` per field |
| `slack-cli test-creds [workspace]` | Validate stored credentials against Slack's `auth.test` | Plain text — valid/expired status with workspace info |
| `slack-cli remove-creds [workspace]` | Delete stored credentials for a workspace | Plain text — confirmation message |
| `slack-cli show-creds` | Print path to the credentials file | Plain text — file path |

### Slack operations

| Command | Description | Output |
|---|---|---|
| `slack-cli search <workspace> <query>` | Search messages (`--count`, `--start-from`) | Plain text — results grouped by channel; each entry shows author, timestamp, text, permalink, and `thread_ts` |
| `slack-cli search-channels <workspace> <pattern>` | List channels whose names contain pattern (case-insensitive; hyphens = spaces) | JSON array of `{id, name, messages}` objects |
| `slack-cli list-dms <workspace>` | List direct message conversations (`--start-from`) | JSON array of `{id, userId, userName, name, isIm}` objects |
| `slack-cli load-thread <workspace> <channel-id> <ts>` | Load all messages in a thread | Plain text — each message shows `**UserID** (timestamp): text`, reactions, and file attachments, separated by `---` |
| `slack-cli load-context <workspace> <channel-id> <ts>` | Load thread as AI-ready markdown (`--permalink`, `--channel-name`, `--search-query`, `--start-from`) | Markdown — header with channel/date/permalink, then each message as a `> **DisplayName** (timestamp): text` block with reactions and attachments |
| `slack-cli get-user <workspace> <user-id>` | Resolve a Slack user ID to a display name | Plain text — `User <id>: <display name>` |

---

## Credentials & Encryption

### Where credentials are stored

All credentials are saved to a single JSON file:

```
~/.slack/workspace_credentials.json
```

### Encryption mechanism

Credentials are encrypted at rest using **AES-256-GCM** with a key derived from your passphrase via **argon2id**. Each workspace entry is encrypted independently, and each encryption call generates a fresh random **salt** (16 bytes) and **nonce** (12 bytes), so ciphertexts are never reused.

**Key derivation (argon2id):**

```
key = argon2id(passphrase, salt, time=1, memory=64 MiB, threads=4) → 32 bytes
```

**Encryption (AES-256-GCM):**

```
ciphertext = AES-256-GCM.Seal(key, nonce, json(token + cookie))
```

The on-disk entry per workspace looks like this:

```json
{
  "acme": {
    "data":    "<base64 ciphertext + GCM authentication tag>",
    "salt":    "<base64 random 16-byte salt>",
    "nonce":   "<base64 random 12-byte nonce>",
    "memory":  65536,
    "time":    1,
    "threads": 4
  }
}
```

The argon2id parameters are stored alongside the ciphertext so future parameter upgrades remain backward-compatible.

### `SLACK_MCP_PASSPHRASE`

The passphrase is required to encrypt and decrypt credentials. It is read from two sources, in order:

1. **`SLACK_MCP_PASSPHRASE` environment variable** — used automatically by the MCP server (or any headless / non-interactive process).
2. **Interactive terminal prompt** — used when running CLI commands directly in a terminal; the passphrase is read with echo suppressed.

> **Critical:** The passphrase used when saving credentials must exactly match the one used when reading them. A mismatch produces a `"failed to read credentials"` error.

#### Setting up for the MCP server (Cursor)

Add `SLACK_MCP_PASSPHRASE` to your Cursor MCP config at `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-mcp",
      "env": {
        "SLACK_MCP_PASSPHRASE": "your-passphrase-here"
      }
    }
  }
}
```

#### Saving credentials from the CLI with a matching passphrase

To ensure the CLI and MCP server use the same passphrase, export the env var before running `auth`:

```bash
export SLACK_MCP_PASSPHRASE=your-passphrase-here
slack-cli auth-complete acme --token xoxc-... --cookie xoxd-...
```

Alternatively, run `slack-cli auth acme` interactively and type the **same value** that is set in `mcp.json` when prompted.

#### Fixing a passphrase mismatch

If you see `failed to read credentials for workspace '<name>'`, re-authenticate with the correct passphrase:

```bash
export SLACK_MCP_PASSPHRASE=your-passphrase-here
slack-cli auth-complete acme --token xoxc-... --cookie xoxd-...
```

This overwrites the stored entry, encrypting it with the new passphrase.

---

## Development

```bash
make build       # Build binary to bin/slack-cli
make test        # Run tests with race detector
make lint        # golangci-lint
make fmt         # gofumpt
make vet         # go vet
make validate    # fmt + vet + lint + test
```

---

## Project Structure

```
cmd/slack-cli/          # Entry point, passphrase provider
internal/cli/           # Cobra commands + business logic
internal/credentials/   # Encrypted credential store (AES-256-GCM + argon2id)
internal/slack/         # Slack API client
internal/browser/       # Browser opener utility
```
