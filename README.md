# Jot

A personal assistant for tracking todos, projects, and ideas. Uses Claude/GPT/Ollama for natural language understanding, stores everything in SQLite. Runs as a CLI REPL or Discord bot.

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
jot> add a todo to buy groceries
jot> what are my active projects?
jot> create a project called "home renovation"
jot> exit
```

### CLI (pipe)

```bash
echo "list my todos" | ./jot
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

### Scheduled check-ins

Set both `DISCORD_WEBHOOK_URL` and `CHECK_IN_CRON` in `.env`. The bot will post a daily summary to the webhook channel. Default is 9am daily (`0 9 * * *`).

## What it can do

- **Todos** — create, list, update, complete (with priority and due dates)
- **Projects** — create, list, update status (active/paused/completed/archived)
- **Ideas** — capture with tags, list and filter
- **Notes** — key-value scratchpad for persistent memory
- **Memories** — contextual memory with categories, tags, and optional expiry
- **Summaries** — overview of active projects, pending/overdue todos

## Testing

```bash
go test ./...
```

Tests cover the database layer: CRUD operations, filtering, memory expiry/pruning, and input validation.

## Data

Everything lives in `data.db` (SQLite). Inspect it directly:

```bash
sqlite3 data.db "SELECT * FROM todos"
sqlite3 data.db "SELECT * FROM projects"
```
