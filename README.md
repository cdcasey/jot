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
go test ./...      # Unit tests (no API calls)
make eval          # LLM eval suite (hits real API)
```

Unit tests cover the database layer, LLM token management, agent param helpers, and Discord utilities.

### Eval Suite

The eval runner (`eval/`) tests the agent end-to-end against a real LLM. Each case gets a fresh in-memory DB seeded with test data — nothing touches `data.db`.

Three eval categories:
- **Tool reliability** (pass/fail) — did the agent call the right tools?
- **Context integration** (1-5) — did it synthesize seeded data correctly?
- **Reasoning** (1-5) — did it engage meaningfully with tradeoffs?

Scored cases use LLM-as-judge. Edit `eval/cases.json` to add or modify cases without touching Go.

```bash
# Test with a specific model
LLM_MODEL=claude-haiku-3-5-20241022 make eval

# Use a different judge model
LLM_MODEL=claude-haiku-3-5-20241022 LLM_EVAL_MODEL=claude-sonnet-4-5-20250514 make eval
```

## Data

Everything lives in `data.db` (SQLite). Inspect it directly:

```bash
sqlite3 data.db ".tables"
sqlite3 data.db "SELECT * FROM things"
sqlite3 data.db "SELECT name, cron_expr, prompt FROM schedules"
```
