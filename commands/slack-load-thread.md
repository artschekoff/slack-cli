---
description: 'Find a Slack thread by keyword or channel name and load it as markdown context using slack-cli'
---
# Slack Load Thread (CLI)

Uses the local `slack-cli` binary. All commands run via Bash.

## Step 1: Resolve workspace

```bash
slack-cli list-workspaces
```

- Has workspaces → AskQuestion: pick one (+ `Other — enter manually`) → store as `WORKSPACE`.
- Empty output → **Auth flow**.

**Auth flow:**
1. Ask for workspace name → store as `WORKSPACE`.
2. `slack-cli auth-start $WORKSPACE` (opens browser, prints extraction instructions).
3. Ask user to provide `xoxc-` token and `xoxd-` cookie.
4. `slack-cli auth-complete $WORKSPACE --token <token> --cookie <cookie>`.
5. On error → report and let user retry.

## Step 2: Get search input

Ask the user (plain text, NOT AskQuestion):

> **Slack search:** enter a keyword, ticket number, channel name pattern, or Slack permalink URL.
> Examples: `JH-568`, `deployment failed`, `#epic-970`, `https://acme.slack.com/archives/C.../p...`

### Parse input

- **Slack permalink** — matches `https?://.*/archives/(C[A-Z0-9]+)/p(\d{10})(\d{6})` → extract `CHANNEL_ID`, reconstruct `THREAD_TS` as `\2.\3` (insert dot after position 10) → skip to **Step 4**.
- **Channel pattern** (starts with `#` or matches `^[a-z0-9_-]+$` and looks like a channel slug) → **Step 3b**.
- **Anything else** → **Step 3a** (full-text search).

## Step 3a: Full-text search

```bash
slack-cli search "$WORKSPACE" "$QUERY" --count 20
```

Output format: groups by channel, each message has `thread_ts` and permalink. Zero results → report, repeat Step 2.

Present results via AskQuestion. Label each option:
`#channel-name | Author | Date — message snippet (thread_ts: …)`

Final option: `Refine search` → repeat Step 2.

Extract `CHANNEL_ID` (from permalink in output), `THREAD_TS` (from `thread_ts:` line), `PERMALINK`, `CHANNEL_NAME` from selected result.

→ **Step 4**.

## Step 3b: Channel search

```bash
slack-cli search-channels "$WORKSPACE" "$PATTERN"
```

Output: JSON array. Parse it to list matching channels.

- No channels → report, repeat Step 2.
- One channel → auto-select.
- Multiple → AskQuestion: `#channel-name (ID: CXXXXXXXX) — N messages`.

From selected channel, list top messages (user, timestamp, text snippet). AskQuestion to pick a thread, label: `Author | Date — snippet`. Extract `CHANNEL_ID`, `THREAD_TS` (from `rawTs`), `CHANNEL_NAME`.

→ **Step 4**.

## Step 4: Load thread as markdown

```bash
slack-cli load-context "$WORKSPACE" "$CHANNEL_ID" "$THREAD_TS" \
  --channel-name "$CHANNEL_NAME" \
  ${PERMALINK:+--permalink "$PERMALINK"} \
  ${QUERY:+--search-query "$QUERY"}
```

Display the returned markdown verbatim to the user.

If `truncated` warning appears → note it to the user.

## Error Handling

| Error | Action |
|---|---|
| `unauthorized` / `token expired` | `slack-cli remove-creds $WORKSPACE` → restart Auth flow |
| `no channel access` | Report "Bot/account has no access to this channel" |
| `slack-cli: command not found` | "slack-cli is not installed or not in PATH. Run `make install` from the repo root." |
| Any other error | Print error message, stop |
