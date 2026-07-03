---
description: 'Find Slack channels by name pattern, browse their messages, and optionally load a thread as context'
---
# Slack Channels (CLI)

Uses the local `slack-cli` binary. All commands run via Bash.

## Step 1: Resolve workspace

```bash
slack-cli list-workspaces
```

- Has workspaces → AskQuestion: pick one (+ `Other — enter manually`) → store as `WORKSPACE`.
- Empty → **Auth flow**.

**Auth flow:** If any command below returns `no active session; run: slack-cli login`, run `slack-cli login` first (interactive prompt for the master passphrase — one-time per machine). Then: ask for workspace name → `slack-cli auth-start $WORKSPACE` (opens browser) → ask user for `xoxc-` token and `xoxd-` cookie → `slack-cli auth-complete $WORKSPACE --token <token> --cookie <cookie>`.

## Step 2: Get channel pattern

Ask the user (plain text):

> **Channel pattern:** substring to match against channel names (case-insensitive, hyphens = spaces).
> Examples: `epic-970`, `970`, `deploy`, `backend`.

Store as `PATTERN`.

Flags to offer: `--system-events` (include join/leave notices), `--bot-messages` (include bot posts).

## Step 3: Search channels

```bash
slack-cli search-channels "$WORKSPACE" "$PATTERN" [--system-events] [--bot-messages]
```

Output: JSON array. Parse it.

- Empty array `[]` → "No channels matching «$PATTERN»", repeat Step 2.
- Error → report and stop.

## Step 4: Pick a channel

AskQuestion with one option per channel:
`#channel-name (CXXXXXXXX) — N messages`

Final option: `Refine pattern` → repeat Step 2.

Store selected channel's `CHANNEL_ID`, `CHANNEL_NAME`, and its `messages` array.

## Step 5: Pick a message / thread

From the selected channel's messages list, present AskQuestion:
`Author | Date — message snippet [N replies]`

- Message has `replies` array → label it `[N replies]`.
- `rawTs` of the selected message = `THREAD_TS`.

Final option: `Back to channel list` → repeat Step 4.

## Step 6: Load thread as context

```bash
slack-cli load-context "$WORKSPACE" "$CHANNEL_ID" "$THREAD_TS" \
  --channel-name "$CHANNEL_NAME" \
  --search-query "$PATTERN"
```

Display the returned markdown verbatim.

If output says `truncated` → note it to the user.

## Error Handling

| Error | Action |
|---|---|
| `unauthorized` / expired | `slack-cli remove-creds $WORKSPACE` → Auth flow |
| `no channel access` | Skip that channel, note it, continue |
| `slack-cli: command not found` | "Run `make install` from the repo root." |
