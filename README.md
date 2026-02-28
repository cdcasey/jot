# Jot

A personal assistant for tracking open loops — anything on your mind. Uses Claude/GPT/Ollama for natural language understanding, stores everything in SQLite. Runs as a CLI REPL or Discord bot.

## Setup

```bash
go build -o jot ./cmd/agent
```

Copy `.env` and fill in your API key:

```bash
cp .env .env.local  # or just edit .env directly
```

Set `LLM_PROVIDER` to one of:
- `anthropic` — requires `ANTHROPIC_API_KEY`
- `openai` — requires `OPENAI_API_KEY`
- `ollama` — no key needed, runs locally

## Usage

### CLI (interactive)

```bash
./jot
jot> track a thing: buy groceries
jot> what am I tracking?
jot> remind me to call the dentist at 3pm
jot> exit
```

### CLI (pipe)

```bash
echo "list my open things" | ./jot
```

### Ollama (local)

```bash
LLM_PROVIDER=ollama LLM_MODEL=llama3.1 ./jot
```

### Discord bot

Set `DISCORD_BOT_TOKEN` in `.env`, then run:

```bash
./jot
```

The bot responds to DMs and @mentions. Conversation history is maintained per channel.

## What it can do

- **Things** — track anything with status, priority, tags, and due dates
- **Notes** — key-value scratchpad for persistent memory
- **Memories** — contextual memory with full-text search (FTS5), categories, tags, and optional expiry
- **Skills** — reusable knowledge packages the agent can create and reference
- **Schedules** — recurring tasks via cron (e.g., daily check-ins, weekly reviews). Agent-manageable.
- **Reminders** — one-shot notifications ("remind me in 5 minutes"). Timezone-aware.
- **Summaries** — overview of open things, overdue items, recent activity

## Scheduling

Schedules and reminders are stored in SQLite and managed by the agent through conversation. The scheduler delivers via Discord DM (preferred) or webhook fallback.

Set `DISCORD_WEBHOOK_URL` in `.env` for webhook delivery. `CHECK_IN_CRON` seeds a default morning check-in if the schedules table is empty.

## Testing

```bash
go test ./...
```

Tests cover the database layer (CRUD, filtering, FTS5 search, memory expiry), LLM token management and message trimming, agent param extraction helpers, and Discord message utilities.

## Data

Everything lives in `data.db` (SQLite). Inspect it directly:

```bash
sqlite3 data.db ".tables"
sqlite3 data.db "SELECT * FROM things"
sqlite3 data.db "SELECT name, cron_expr, prompt FROM schedules"
```
