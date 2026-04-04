# Jot

A personal assistant for tracking open loops — anything on your mind. Uses Claude/GPT/Gemini/Ollama for natural language understanding, stores everything in SQLite. Runs as a CLI REPL or Discord bot.

## Setup

```bash
go build -o jot ./cmd/agent
```

Copy the example config and set your model:

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` to define models and set `active_model`. API keys go in `.env`:

```bash
# .env
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
GEMINI_API_KEY=...
```

Keys are resolved by provider name: `anthropic` reads `ANTHROPIC_API_KEY`, `openai` reads `OPENAI_API_KEY`, etc. Ollama needs no key.

If no `config.yaml` exists, the app falls back to `LLM_PROVIDER` and `LLM_MODEL` env vars for backward compatibility.

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

### Switching models

Edit `active_model` in `config.yaml`:

```yaml
active_model: ollama-local
```

### Discord bot

Set `DISCORD_BOT_TOKEN` in `.env`, then run:

```bash
./jot
```

The bot responds to DMs and @mentions. Conversation history is maintained per channel.

## What it can do

- **Things** — track anything with status, priority, tags, and due dates
- **Memories** — contextual memory with full-text search (FTS5), categories, tags, and optional expiry
- **Schedules** — recurring tasks via cron (e.g., daily check-ins, weekly reviews). Agent-manageable.
- **Reminders** — one-shot notifications via schedules ("remind me at 3pm"). Timezone-aware.
- **Watches** — monitor web pages on a schedule, extract structured info via LLM, notify on new items
- **Summaries** — overview of open things, overdue items, recent activity

## Scheduling

Schedules and reminders are stored in SQLite and managed by the agent through conversation. The scheduler delivers via Discord DM (preferred) or webhook fallback.

Set `DISCORD_WEBHOOK_URL` in `.env` for webhook delivery. `CHECK_IN_CRON` seeds a default morning check-in if the schedules table is empty.

## Watches

Web watches monitor URLs on a schedule, extract structured information using the LLM, and notify you when new items appear.

```
jot> watch https://austintheatre.org/auditions for new auditions every Monday
jot> what has my auditions watch found?
```

Each watch has an extraction prompt (what to look for), a list of URLs, and an optional cron expression. Watches without a cron are manual-only — trigger them with "run my watch." Results are deduplicated across runs so you only get notified about new items. Old results are pruned after 6 months.

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
sqlite3 data.db "SELECT name, cron_expr, enabled FROM watches"
sqlite3 data.db "SELECT title, body, first_seen FROM watch_results ORDER BY first_seen DESC LIMIT 10"
```
