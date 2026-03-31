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
- **LLM:** Anthropic Claude API (official Go SDK: `github.com/anthropic-ai/anthropic-sdk-go`)
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
    queries_memories.go      # Memories queries
    queries_schedule.go      # Schedules + one-shot reminders queries
    queries_conversations.go # Conversation persistence + summaries
    queries_watches.go       # Watch + watch result queries
/internal/llm/
    client.go                # LLMClient interface
    provider.go              # Provider factory (NewClient)
    anthropic.go             # Anthropic implementation
    openai.go                # OpenAI implementation
    tools.go                 # Tool definitions (provider-agnostic)
    prompt.go                # System prompt
/internal/agent/
    agent.go                 # Core agent loop + timezone helpers
    conversation.go          # RunWithConversation, Summarize (persistent history)
/internal/discord/
    bot.go                   # Discord bot setup
    handlers.go              # Message handlers
/internal/scheduler/
    scheduler.go             # Cron for check-ins, watch scheduling, data pruning
/internal/watch/
    fetch.go                 # URL fetching + HTML-to-text extraction
    runner.go                # Watch execution: fetch → LLM extract → dedup → store
/config/
    config.go                # Environment/config loading
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
    completed_at TEXT
);

CREATE TABLE notes (                  -- Internal config only (timezone, discord_user_id). Not exposed as LLM tools.
    id INTEGER PRIMARY KEY,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE memories (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'observation',  -- observation, decision, blocker, preference, event, reflection, habit
    tags TEXT,                         -- JSON array
    thing_id INTEGER REFERENCES things(id),
    source TEXT NOT NULL DEFAULT 'agent',
    expires_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- FTS5 full-text search index (content-sync'd with memories table via triggers)
CREATE VIRTUAL TABLE memories_fts USING fts5(content, content_rowid='id', content='memories');

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

CREATE TABLE conversations (
    id INTEGER PRIMARY KEY,
    user_id TEXT UNIQUE NOT NULL,      -- discord user ID or "cli"
    messages TEXT NOT NULL DEFAULT '[]', -- JSON array of llm.Message
    last_message_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE conversation_summaries (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    summary TEXT NOT NULL,
    message_count INTEGER,
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
```

## LLM Tools (19 total)

The agent has exactly these tools - no more, no less. Current time is injected into the system prompt, not exposed as a tool.

### Thing Tools (4)
- `list_things` - List things, optionally filtered by status, priority, tag. Also returns overdue items.
- `create_thing` - Create a new thing (title required; notes, priority, due_date, tags optional)
- `update_thing` - Update a thing by id (any field except id and created_at)
- `complete_thing` - Mark a thing as done

### Memory Tools (5)
- `save_memory` - Save a timestamped memory (events, decisions, blockers, habits)
- `search_memories` - Search past memories by text (FTS5), category, tag, thing, or date
- `list_recent_memories` - List most recent memories
- `update_memory` - Update a memory by ID (content, category, tags, expires_at)
- `delete_memory` - Delete a memory by ID

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

### Context (injected, not a tool)
- Current time and timezone are embedded in the system prompt on each request

## System Prompt Guidelines

The agent should:
- Be helpful but concise - no unnecessary chatter
- Proactively use tools to check state before answering questions about things
- Everything is a "thing" — use tags for categorization, status and priority for state
- Remember context across conversations using memories
- During check-ins: summarize open things, mention overdue items, ask about priorities
- Not be annoying - check-ins should be useful, not nagging
- Admit when it doesn't know something rather than making things up

## Environment Variables

```
# LLM Provider (choose one)
LLM_PROVIDER=anthropic         # anthropic or openai
ANTHROPIC_API_KEY=sk-ant-...   # If using Anthropic
OPENAI_API_KEY=sk-...          # If using OpenAI

DISCORD_BOT_TOKEN=...
DISCORD_WEBHOOK_URL=...        # For outbound notifications
DATABASE_PATH=./data.db        # SQLite file location
CHECK_IN_CRON="0 9 * * *"      # Daily at 9am (optional)
MAX_CONTEXT_TOKENS=180000      # Token budget for LLM context (default: 180000)

# Eval-specific (optional, fall back to LLM_PROVIDER/LLM_MODEL)
LLM_EVAL_PROVIDER=anthropic    # Provider for the LLM-as-judge
LLM_EVAL_MODEL=claude-sonnet-4-5-20250514  # Model for the LLM-as-judge
```

## Multi-Provider LLM Support

The agent should support both Anthropic and OpenAI APIs, selectable at runtime via config/environment. This allows using OAuth tokens from either provider.

### Implementation Approach

Define an `LLMClient` interface that abstracts the provider:

```go
// internal/llm/client.go

type Message struct {
    Role    string // "user", "assistant", "system"
    Content string
}

type ToolCall struct {
    Name   string
    Params map[string]any
}

type Response struct {
    Content   string
    ToolCalls []ToolCall
}

type LLMClient interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
}
```

Then implement for each provider:

```go
// internal/llm/anthropic.go
type AnthropicClient struct { ... }

// internal/llm/openai.go  
type OpenAIClient struct { ... }
```

### Provider Selection

```go
// internal/llm/provider.go

func NewClient(provider string, apiKey string) (LLMClient, error) {
    switch provider {
    case "anthropic":
        return NewAnthropicClient(apiKey)
    case "openai":
        return NewOpenAIClient(apiKey)
    default:
        return nil, fmt.Errorf("unknown provider: %s", provider)
    }
}
```

### Model Configuration

```
# Optional: specify model (defaults to claude-sonnet-4-20250514 or gpt-4o)
LLM_MODEL=claude-sonnet-4-20250514
```

### SDKs

- **Anthropic:** `github.com/anthropic-ai/anthropic-sdk-go`
- **OpenAI:** `github.com/openai/openai-go`

Both support tool calling. The interface abstraction means the agent core doesn't care which provider is in use - it just calls `client.Chat()` with messages and tools.

### Tool Schema Translation

Anthropic and OpenAI have slightly different tool/function calling formats. The provider implementations should translate from the internal `Tool` definition to the provider-specific format. Keep the internal representation simple and do the translation at the edges.

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

### Phase 4: Memory Improvements (PLAN2.md Phase 2)
- [x] FTS5 full-text search for memories (virtual table, triggers, backfill)
- [x] Memory management tools (update_memory, delete_memory, resolve_memory)
- [x] Persistent conversation history (conversations + conversation_summaries tables)
- [x] Auto-summarization on conversation gaps (>10 min)
- [x] Scheduler + reminders wired into conversation persistence

### Phase 5: Simplification
- [x] Removed skills (5 tools, 1 table)
- [x] Removed habits (3 tools, 1 table) — use memories with category='habit'
- [x] Removed check_ins (dead table)
- [x] Merged reminders into schedules (3 tools removed)
- [x] Hid notes from LLM (2 tools removed, table kept for internal config)
- [ ] Prune old conversation summaries (PruneOldSummaries exists, needs wiring into pruneOldData())
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
