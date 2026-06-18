# Personal Assistant Agent

A sandboxed personal assistant for tracking open loops — anything on your mind. Interacts via Discord, stores data in SQLite, uses Claude for natural language understanding.

## Project Overview

This is a Go application with intentionally limited capabilities. The agent can ONLY read/write to its own SQLite database - no filesystem access, no shell execution. This constraint is a feature, not a limitation.

### Architecture

```
Discord Bot <-> Agent Core <-> SQLite (data.db)
                   |
                   v
            Anthropic API
                   |
                   v
              Scheduler (check-ins via Discord webhook)
                   |
                   v
              Watch Runner (fetches URLs → LLM extraction → dedup → notify)
```

### Tech Stack

- **Language:** Go
- **Database:** SQLite (via `modernc.org/sqlite` - pure Go, no CGO)
- **LLM:** Anthropic, OpenAI, Gemini, Ollama (see Multi-Provider LLM Support)
- **Discord:** `github.com/bwmarrin/discordgo`
- **Config:** Environment variables or `.env` file

## Project Structure

```
/cmd/agent/main.go           # Entry point
/internal/db/
    schema.sql               # SQLite schema
    db.go                    # Connection, migrations
    queries.go               # Struct type definitions
    queries_helpers.go       # Shared helpers (updateRow, nullStr, allowedColumns)
    queries_things.go        # Things + Summary queries
    queries_notes.go         # Notes queries (internal config only, not exposed as LLM tools)
    queries_schedule.go      # Schedules + one-shot reminders queries
    queries_watches.go       # Watch + watch result queries
    queries_habits.go        # Habits + habit log queries (record-only)
/internal/llm/
    client.go                # LLMClient interface
    provider.go              # Provider factory (NewClient)
    anthropic.go             # Anthropic implementation
    openai.go                # OpenAI implementation
    tools.go                 # Tool definitions (provider-agnostic)
    prompt.go                # System prompt
/internal/agent/
    agent.go                 # Core agent loop + timezone helpers (stateless Run)
/internal/discord/
    bot.go                   # Discord bot setup
    handlers.go              # Message handlers
/internal/scheduler/
    scheduler.go             # Cron for check-ins, watch scheduling, data pruning
/internal/watch/
    fetch.go                 # URL fetching + HTML-to-text extraction
    runner.go                # Watch execution: fetch → LLM extract → dedup → store
/internal/web/
    web.go                   # Embedded web UI: Server, routes, view models (kanban + habit grid)
    templates/               # html/template files (layout, board, habits)
    static/                  # Vendored htmx + Sortable.js + app.css (embed.FS, no npm/build step)
/config.example.yaml             # YAML config template (checked in)
/config/
    config.go                # YAML + env config loading
/eval/
    eval.go                  # Eval runner, seeder, asserter, LLM-as-judge
    eval_test.go             # Test entry point (guarded by RUN_EVAL=1)
    cases.json               # Eval cases — edit without touching Go
```

## Database Schema

```sql
CREATE TABLE things (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    notes TEXT,
    status TEXT DEFAULT 'open',       -- open, active, done, dropped
    priority TEXT DEFAULT 'normal',   -- low, normal, high, urgent
    tags TEXT,                         -- JSON array: ["tag1", "tag2"]
    due_date TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT,
    position INTEGER NOT NULL DEFAULT 0 -- manual sort order within a status column (kanban)
);

CREATE TABLE notes (                  -- Internal config only (timezone, discord_user_id). Not exposed as LLM tools.
    id INTEGER PRIMARY KEY,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE schedules (              -- Unified: recurring (cron) + one-shot reminders (fire_at)
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    cron_expr TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    last_run TEXT,
    fire_at TEXT,                      -- For one-shot reminders: UTC datetime. NULL for recurring.
    fired INTEGER DEFAULT 0,          -- For one-shot: 1 when fired.
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE watches (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    prompt TEXT NOT NULL,              -- extraction instructions for the LLM
    urls TEXT NOT NULL DEFAULT '[]',   -- JSON array of URLs to fetch
    cron_expr TEXT NOT NULL DEFAULT '',-- cron schedule or empty for manual-only
    enabled INTEGER DEFAULT 1,
    last_run TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE watch_results (
    id INTEGER PRIMARY KEY,
    watch_id INTEGER NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    content_hash TEXT NOT NULL,        -- SHA-256 of normalized title+source_url for dedup
    title TEXT NOT NULL,
    body TEXT,
    source_url TEXT,
    first_seen TEXT DEFAULT (datetime('now')),
    notified INTEGER DEFAULT 0,       -- 0=new, 1=delivered
    UNIQUE(watch_id, content_hash)
);

CREATE TABLE habits (                 -- Habit definitions (record-only; no streaks/targets/cadence)
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    active INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE habit_logs (             -- One row = habit was done that local calendar day
    id INTEGER PRIMARY KEY,
    habit_id INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    date TEXT NOT NULL,                -- local calendar date YYYY-MM-DD
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(habit_id, date)            -- structural guarantee: one entry per habit per day
);
```

## LLM Tools (15 total)

The agent has exactly these tools - no more, no less. The agent is stateless: each
message is a single independent exchange with no persisted conversation history or
free-text memory. Current time is injected into the system prompt, not exposed as a tool.

### Thing Tools (4)
- `list_things` - List things, optionally filtered by status, priority, tag. Items past due date are marked `overdue: true`.
- `create_thing` - Create a new thing (title required; notes, priority, due_date, tags optional)
- `update_thing` - Update a thing by id (any field except id and created_at)
- `complete_thing` - Mark a thing as done

### Schedule Tools (4)
- `list_schedules` - List all schedules (recurring + one-shot reminders)
- `create_schedule` - Create a recurring schedule (cron_expr) or one-shot reminder (fire_at)
- `update_schedule` - Update cron_expr, prompt, or enabled flag by name
- `delete_schedule` - Delete a schedule by name

### Watch Tools (6)
- `list_watches` - List all web watches
- `create_watch` - Create a watch (name, extraction prompt, URLs, optional cron_expr)
- `update_watch` - Update prompt, URLs, cron_expr, or enabled flag by name
- `delete_watch` - Delete a watch by name (cascades to results)
- `run_watch` - Manually trigger a watch to fetch URLs and extract items now
- `list_watch_results` - List stored results for a watch (optionally unnotified only)

### Habit Tools (1)
- `log_habit` - Record whether a habit happened on a local calendar date. Logging marks the day done; re-logging the same habit+date is a harmless no-op (guaranteed by `UNIQUE(habit_id, date)`). `done=false` toggles the entry off. Unknown habit names are auto-created (`active=1`). Record-only: no streaks, targets, cadence, or success/failure. Date defaults to the user's current local date via `userLocation()`.

### Context (injected, not a tool)
- Current time and timezone are embedded in the system prompt on each request

## System Prompt Guidelines

The agent should:
- Be helpful but concise - no unnecessary chatter
- Proactively use tools to check state before answering questions about things
- Everything is a "thing" — use tags for categorization, status and priority for state
- Be stateless: each message is an independent exchange. There is no persisted conversation history or free-text memory; durable state lives in structured tables (things, schedules, watches)
- During check-ins: summarize open things, mention overdue items, ask about priorities
- Not be annoying - check-ins should be useful, not nagging
- Admit when it doesn't know something rather than making things up

## Configuration

LLM models are defined in `config.yaml` (not checked in). Copy from `config.example.yaml`:

```yaml
models:
  anthropic-sonnet:
    provider: anthropic
    model: claude-sonnet-4-20250514
    temperature: 0.7
  ollama-local:
    provider: ollama
    model: llama3.1
    base_url: http://localhost:11434/v1
    temperature: 0.7
active_model: anthropic-sonnet
```

API keys and secrets go in `.env`:

```
# API keys (resolved by provider name)
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...
GEMINI_API_KEY=...
ANTHROPIC_AUTH_TOKEN=...       # Optional: OAuth token (Bearer auth)

# App
DISCORD_BOT_TOKEN=...
DISCORD_WEBHOOK_URL=...        # For outbound notifications
DISCORD_USER_ID=...
DATABASE_PATH=./data.db        # SQLite file location
CHECK_IN_CRON="0 9 * * *"      # Daily at 9am (optional)
MAX_CONTEXT_TOKENS=180000      # Token budget for LLM context (default: 180000)
WEB_PORT=8080                  # Optional: enable embedded web UI. Bare port binds
                               #   loopback only; "host:port" exposes that interface
                               #   (e.g. Tailscale). No app-level auth — use network ACLs.

# Eval-specific (optional, fall back to active_model from YAML)
LLM_EVAL_PROVIDER=anthropic
LLM_EVAL_MODEL=claude-sonnet-4-5-20250514
```

If `config.yaml` is missing, the app falls back to `LLM_PROVIDER` and `LLM_MODEL` env vars.

## Multi-Provider LLM Support

Four providers are supported. API keys are resolved by provider name from `.env`:

| Provider | Default Model | API Key Env Var | Notes |
|----------|---------------|-----------------|-------|
| `anthropic` | `claude-sonnet-4-20250514` | `ANTHROPIC_API_KEY` | Also supports OAuth via `ANTHROPIC_AUTH_TOKEN` |
| `openai` | `gpt-4o` | `OPENAI_API_KEY` | |
| `gemini` | `gemini-2.5-flash` | `GEMINI_API_KEY` | Uses Gemini's OpenAI-compatible endpoint |
| `ollama` | `llama3.1` | (none) | Set `base_url` in YAML |

### Architecture

- `config/config.go` — Loads `config.yaml` (model definitions) + `.env` (secrets), merges into `Config`
- `internal/llm/anthropic.go` — Raw HTTP implementation (not the SDK) for Anthropic
- `internal/llm/openai.go` — OpenAI SDK implementation, reused by Gemini and Ollama with different base URLs
- `internal/llm/provider.go` — Factory that routes `ProviderConfig` to the correct implementation

Gemini and Ollama both reuse `OpenAIClient` with a custom base URL — no additional SDK dependencies.

### Temperature

Set `temperature` per model in `config.yaml`. When omitted, the provider's default is used (typically 1.0). Anthropic accepts 0.0-1.0; OpenAI/Gemini accept 0.0-2.0.

## Build & Run

```bash
# Development
go run ./cmd/agent

# Build
go build -o agent ./cmd/agent

# Run
./agent

# Run evals (requires LLM API key)
make eval

# Run evals with specific models
LLM_MODEL=claude-haiku-3-5-20241022 make eval
LLM_MODEL=claude-haiku-3-5-20241022 LLM_EVAL_MODEL=claude-sonnet-4-5-20250514 make eval
```

## Development Phases

### Phase 1: Core (MVP)
- [x] SQLite schema and migrations
- [x] Database query functions
- [x] Anthropic client with tool calling
- [x] Core tools (5 thing tools + memory + time)
- [x] CLI mode for testing: `echo "track a thing: buy milk" | ./agent`

### Phase 2: Discord
- [x] Discord bot setup (listen for DMs)
- [x] Message handling (pipe through agent)
- [x] Webhook for outbound messages

### Phase 3: Scheduling
- [x] Internal cron scheduler
- [x] Check-in logic (build context, send to LLM, post to Discord)
- [x] DB-driven multi-schedule cron (schedules table, agent-manageable)
- [x] One-shot reminders unified into schedules table (fire_at column)
- [x] CHECK_IN_CRON demoted to seed fallback
- [x] Schedules send prompt directly to agent (no forced check-in context)
- [x] Timezone-aware reminders (local→UTC conversion via `timezone` note)

### Phase 4: Memory Improvements (PLAN2.md Phase 2) — REMOVED in Phase 7
This layer was built and later stripped (see Phase 7). Listed here for history:
- ~~FTS5 full-text search for memories (virtual table, triggers, backfill)~~
- ~~Memory management tools (update_memory, delete_memory, resolve_memory)~~
- ~~Persistent conversation history (conversations + conversation_summaries tables)~~
- ~~Auto-summarization on conversation gaps (>10 min)~~
- ~~Scheduler + reminders wired into conversation persistence~~

### Phase 5: Simplification
- [x] Removed skills (5 tools, 1 table)
- [x] Removed habits (3 tools, 1 table) — use memories with category='habit'
- [x] Removed check_ins (dead table)
- [x] Merged reminders into schedules (3 tools removed)
- [x] Hid notes from LLM (2 tools removed, table kept for internal config)
- [x] ~~Prune old conversation summaries~~ — moot; conversation_summaries removed in Phase 7
- [ ] Migrate notes table to .env config
- [ ] Expose timezone updates to LLM (re-add set_note tool or a dedicated set_timezone tool). Currently userLocation() reads from notes table but LLM has no way to write it.

### Phase 6: Web Watches
- [x] URL fetching with HTML-to-text extraction (internal/watch/fetch.go)
- [x] LLM-powered item extraction with JSON schema (internal/watch/runner.go)
- [x] Deduplication via SHA-256 hash of title + source_url
- [x] watches + watch_results tables with cascade delete
- [x] 6 LLM tools: list/create/update/delete watches, run_watch, list_watch_results
- [x] Scheduler integration: cron-based watch runs with Discord DM/webhook delivery
- [x] Age-based pruning of watch results (180 days, runs daily via scheduler)
- [x] Context propagation (context.Context through fetch pipeline)
- [x] Eval cases for watch creation and result querying

### Phase 8: Web UI (openspec: add-web-kanban-habit-grid)
Web-primary interface in the single binary — no Node, no SPA, no build step.
- [x] `internal/web` package served behind `WEB_PORT` (off when unset), shares the SQLite handle
- [x] Three-column kanban over `things` (open=Ideas, active=Active, done=Done; dropped off-board)
- [x] `things.position` column (+ idempotent migrate/backfill) for manual ordering
- [x] Drag-and-drop via htmx + Sortable.js (vendored, embed.FS); fractional positions with renumber fallback
- [x] Cross-column move writes status+position+updated_at; stamps/clears completed_at for Done
- [x] Read-only per-habit calendar grid over `habit_logs` (rolling 30-day window)
- [x] Loopback/Tailscale exposure, no app-level auth (network ACLs)

### Phase 7: Strip Memory Layer (openspec: strip-memory-layer)
Reorienting Jot toward a web-primary, structured tool. The agent becomes a thin,
stateless command surface; everything durable lives in structured tables.
- [x] Dropped `conversations`, `conversation_summaries`, `memories` tables (+ `memories_fts` + triggers)
- [x] Migration drops them idempotently for existing DBs (triggers → FTS → base tables, in order)
- [x] Removed 5 memory LLM tools (19 → 14 tools)
- [x] Removed `RunWithConversation`/`Summarize`; all callers use stateless `Run`
- [x] Removed summary pruning and `resolveUserID` from the scheduler
- [x] Pared the system prompt of memory/summary/continuity language
- [x] Removed memory eval cases and seeding; KEPT schedules, watches, notes intact

## Code Style

- Use standard Go project layout
- Error handling: wrap errors with context (`fmt.Errorf("doing X: %w", err)`)
- No global state - pass dependencies explicitly
- Keep functions small and focused
- Write table-driven tests for database queries and tool execution

## Security Notes

- The agent has NO filesystem access beyond SQLite
- The agent has NO shell/exec capabilities
- The agent can ONLY call the defined tools
- Watches make outbound HTTP GET requests to user-specified URLs (read-only, 2MB cap, 30s timeout)
- Discord bot should only respond to DMs from authorized user(s)
- Store secrets in environment variables, never in code

## Testing

```bash
go test ./...      # Unit tests (no API calls)
make eval          # LLM eval suite (hits real API)
```

Unit tests use in-memory SQLite and run without network access. The eval suite (`eval/`) runs the agent against a real LLM with an in-memory DB per case, then scores responses via tool-call assertions and LLM-as-judge. Eval cases are defined in `eval/cases.json` — edit without touching Go code. Guarded by `RUN_EVAL=1` so `go test ./...` skips them.

## Useful Commands During Development

```bash
# View SQLite contents
sqlite3 data.db ".tables"
sqlite3 data.db "SELECT * FROM things"

# Test agent locally
echo "what things am I tracking?" | go run ./cmd/agent

# Check Discord bot token is valid
# (bot should come online in your server)
```
