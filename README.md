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

The bot responds to DMs and @mentions. Each message is an independent exchange —
the agent is stateless and keeps no conversation history; durable state lives in
the SQLite tables (things, schedules, watches, habits).

### Web UI

Jot ships with an embedded web interface — a kanban board for your things and a
calendar grid for your habits. It's served from the same binary and reads/writes
the same `data.db`; there's no separate service, no Node, and no build step.

Set `WEB_PORT` (and optionally `WEB_ADDR`) and run as usual:

```bash
# .env
WEB_PORT=8080
```

```bash
./jot
# web UI listening on http://127.0.0.1:8080
```

The web server starts alongside the CLI or Discord bot whenever **`WEB_ADDR` or
`WEB_PORT`** is set, and is skipped entirely when neither is. Open the URL in a
browser:

- **Board** (`/`) — a three-column kanban over your things: **Ideas** (`open`),
  **Active** (`active`), and **Done** (`done`). Archived (`dropped`) things are
  off the board. Drag a card to reorder it within a column or move it to another;
  the new position is saved, and dropping a card into **Done** stamps its
  completion time (dragging back out clears it).
- **Habits** (`/habits`) — a read-only calendar grid covering the last 30 days.
  A cell is filled on each day a habit was logged. Logging still happens through
  the agent (e.g. "log meditation" via CLI or Discord); the grid is for looking,
  not editing.

#### Exposure

The bind address is split across two variables. `WEB_ADDR` is the host to bind;
`WEB_PORT` is the port. Each has a default, so you can set just one:

| `WEB_ADDR` | `WEB_PORT` | Binds to | Use for |
|------------|-----------|----------|---------|
| _(unset)_ | `8080` | `127.0.0.1:8080` (loopback only) | local access |
| _(unset)_ | _(unset)_ | server off | — |
| `100.x.y.z` | _(unset)_ | `100.x.y.z:8080` | remote access (e.g. Tailscale) |
| `100.x.y.z` | `9090` | `100.x.y.z:9090` | remote, custom port |

To reach it over **Tailscale**, set `WEB_ADDR` to the node's tailnet IP (find it
with `tailscale ip -4`):

```bash
# .env
WEB_ADDR=100.x.y.z
WEB_PORT=8080
```

A bare `WEB_PORT` binds **loopback only** (`127.0.0.1`), so it works on the host
but not over the tailnet — set `WEB_ADDR` for that. There is **no
application-level login**; access control is delegated to the network layer. Bind
to loopback or a Tailscale interface and rely on Tailscale ACLs; do not bind to a
public interface (e.g. `WEB_ADDR=0.0.0.0`) without putting protection in front of it.

## What it can do

- **Things** — track anything with status, priority, tags, and due dates
- **Schedules** — recurring tasks via cron (e.g., daily check-ins, weekly reviews). Agent-manageable.
- **Reminders** — one-shot notifications via schedules ("remind me at 3pm"). Timezone-aware.
- **Watches** — monitor web pages on a schedule, extract structured info via LLM, notify on new items
- **Habits** — record whether a habit happened on a given day (record-only; no streaks or targets)
- **Web UI** — kanban board for things and a calendar grid for habits, served from the same binary (`WEB_PORT`)

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
