---
description: 'Full-text search across Slack messages, then optionally load the selected thread as markdown context'
---
# Slack Search (CLI)

Uses the local `slack-cli` binary. All commands run via Bash.

## Step 1: Resolve workspace

```bash
slack-cli list-workspaces
```

- Has workspaces → AskQuestion: pick one (+ `Other — enter manually`) → store as `WORKSPACE`.
- Empty → **Auth flow**.

**Auth flow:** If any command below returns `no active session; run: slack-cli login`, run `slack-cli login` first (interactive prompt for the master passphrase — one-time per machine). Then: ask for workspace name → `slack-cli auth-start $WORKSPACE` (opens browser) → ask user for `xoxc-` token and `xoxd-` cookie → `slack-cli auth-complete $WORKSPACE --token <token> --cookie <cookie>`.

## Step 2: Get search query

Ask the user (plain text):

> **Search query:** keyword, phrase, or Slack search modifier (e.g. `deploy failed`, `in:#general JH-568`, `from:@alice`).
> Optional: `--count N` (default 20, max 100), `--start-from YYYY-MM-DD`.

Store as `QUERY`. Extract any `--count` / `--start-from` overrides from user input.

## Step 3: Run search

```bash
slack-cli search "$WORKSPACE" "$QUERY" [--count N] [--start-from YYYY-MM-DD]
```

Output format (plain text):
```
#channel-name (CXXXXXXXX) — N messages:
  1. Username | 2024-01-15 10:30 | Message text
     Permalink: https://...
     thread_ts: 1705312200.000000
```

- Zero results → report "No results for «$QUERY»", offer to refine (repeat Step 2).
- Error → report and stop.

## Step 4: Present results

AskQuestion with one option per message:
`#channel | Author | Date — snippet`

Final options:
- `Refine search` → repeat Step 2.
- `Done — just show results` → display formatted results and stop.

Extract from selected result: `CHANNEL_ID`, `THREAD_TS`, `PERMALINK`, `CHANNEL_NAME`.

## Step 5: Load thread (optional)

AskQuestion: **Load full thread?**
- `Yes — load as markdown context` → run `slack-load-thread` flow from Step 4 using extracted values.
- `No — results are enough` → stop.

## Error Handling

| Error | Action |
|---|---|
| `unauthorized` / expired | `slack-cli remove-creds $WORKSPACE` → Auth flow |
| No results | Offer to refine query |
| `slack-cli: command not found` | "Run `make install` from the repo root." |
