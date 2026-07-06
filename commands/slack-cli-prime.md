---
description: 'Prime the assistant on the slack-cli binary — commands, auth flow, and when to use each subcommand'
---
# Slack CLI Prime

Use this to bootstrap context on the local `slack-cli` binary before invoking any Slack-related workflow.

## What slack-cli does

Encrypted-credential Slack CLI + MCP server backend. Run everything via Bash against the installed binary. No HTTP, no DB — session file at `~/.slack/session`, credentials at `~/.slack/workspace_credentials.json`.

## Command map

| Command | Purpose |
|---|---|
| `slack-cli login` | Unlock the session (one-time per machine, prompts for master passphrase) |
| `slack-cli logout` | Clear the session file |
| `slack-cli list-workspaces` | List authenticated workspaces |
| `slack-cli auth-start <workspace>` | Open browser, print token/cookie extraction instructions |
| `slack-cli auth-complete <workspace> --token <xoxc-...> --cookie <xoxd-...>` | Store encrypted credentials |
| `slack-cli remove-creds <workspace>` | Drop credentials for a workspace |
| `slack-cli search-channels <workspace> <pattern> [--system-events] [--bot-messages]` | Find channels by name substring, returns JSON with recent messages |
| `slack-cli search <workspace> <query> [--count N] [--start-from YYYY-MM-DD]` | Full-text search, supports `in:#chan`, `from:@user` modifiers |
| `slack-cli load-context <workspace> <channel_id> <thread_ts> [--channel-name X] [--permalink URL] [--search-query Q]` | Load thread as markdown |

## Auth flow (whenever a command reports `no active session` or `unauthorized`)

1. `slack-cli login` if session missing (interactive passphrase prompt).
2. `slack-cli auth-start $WORKSPACE` → browser opens with extraction steps.
3. User supplies `xoxc-` token + `xoxd-` cookie.
4. `slack-cli auth-complete $WORKSPACE --token <token> --cookie <cookie>`.

On expired creds: `slack-cli remove-creds $WORKSPACE`, then re-auth.

## When to use which flow

- Want to browse a channel and pick a thread → `/slack-cli-channels`.
- Want full-text search across messages → `/slack-cli-search`.
- Have a permalink, ticket ID, or vague query and want the thread as markdown → `/slack-cli-load-thread`.

## Error cheat sheet

| Error | Fix |
|---|---|
| `no active session; run: slack-cli login` | `slack-cli login` |
| `unauthorized` / token expired | `remove-creds` → re-auth |
| `no channel access` | Skip that channel, note it |
| `slack-cli: command not found` | `make install` from the repo root |
