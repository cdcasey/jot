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
```

### Tech Stack

- **Language:** Go
- **Database:** SQLite (via `modernc.org/sqlite` - pure Go, no CGO)
- **LLM:** Anthropic Claude API (official Go SDK: `github.com/anthropic-ai/anthropic-sdk-go`)
- **Discord:** `github.com/bwmarrin/discordgo`
- **Config:** `~/.jot/config` (dotenv format), `.env` override, or environment variables

## Project Structure

```
/cmd/agent/main.go           # Entry point
/internal/db/
    schema.sql               # SQLite schema
    db.go                    # Connection, migrations
    queries.go               # Struct type definitions
    queries_helpers.go       # Shared helpers (updateRow, nullStr, allowedColumns)
    queries_things.go        # Things + Summary queries
    queries_notes.go         # Notes queries
    queries_memories.go      # Memories + check-in queries
    queries_skills.go        # Skills queries
    queries_schedule.go      # Schedules queries
    queries_reminders.go     # Reminders queries
/internal/llm/
    client.go                # LLMClient interface
    provider.go              # Provider factory (NewClient)
    anthropic.go             # Anthropic implementation
    openai.go                # OpenAI implementation
    tools.go                 # Tool definitions (provider-agnostic)
    prompt.go                # System prompt
/internal/agent/
    agent.go                 # Core agent loop
    context.go               # Context building for LLM
/internal/discord/
    bot.go                   # Discord bot setup
    handlers.go              # Message handlers
/internal/scheduler/
    scheduler.go             # Cron for check-ins
/internal/service/
    service.go               # launchd service management (install/start/stop)
/config/
    config.go                # Environment/config loading
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

CREATE TABLE check_ins (
    id INTEGER PRIMARY KEY,
    summary TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE notes (
    id INTEGER PRIMARY KEY,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE memories (
    id INTEGER PRIMARY KEY,
    content TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'observation',
    tags TEXT,                         -- JSON array
    thing_id INTEGER REFERENCES things(id),
    source TEXT NOT NULL DEFAULT 'agent',
    expires_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- FTS5 full-text search index (content-sync'd with memories table via triggers)
CREATE VIRTUAL TABLE memories_fts USING fts5(content, content_rowid='id', content='memories');

CREATE TABLE skills (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,                         -- JSON array
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE schedules (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    cron_expr TEXT NOT NULL,
    prompt TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    last_run TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE reminders (
    id INTEGER PRIMARY KEY,
    prompt TEXT NOT NULL,
    fire_at TEXT NOT NULL,             -- UTC datetime: "YYYY-MM-DD HH:MM:SS"
    fired INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);
```

## LLM Tools

The agent has exactly these tools - no more, no less:

### Thing Tools
- `list_things` - List things, optionally filtered by status, priority, tag
- `create_thing` - Create a new thing (title required; notes, priority, due_date, tags optional)
- `update_thing` - Update a thing by id (any field except id and created_at)
- `complete_thing` - Mark a thing as done
- `get_summary` - Returns open things count, overdue things, recent activity

### Notes & Memory Tools
- `get_note` - Retrieve a stored note by key
- `set_note` - Store or update a note (agent's scratchpad memory)
- `save_memory` - Save a timestamped memory (events, decisions, blockers)
- `search_memories` - Search past memories by text (FTS5), category, tag, thing, or date
- `list_recent_memories` - List most recent memories
- `update_memory` - Update a memory by ID (content, category, tags, expires_at)
- `delete_memory` - Delete a memory by ID

### Utility Tools
- `get_time` - Get the current system time
- `create_skill` / `get_skill` / `list_skills` / `update_skill` / `delete_skill` - Manage reusable skills

### Schedule Tools
- `list_schedules` - List all recurring scheduled tasks
- `create_schedule` - Create a recurring task (name, cron_expr, prompt required)
- `update_schedule` - Update cron_expr, prompt, or enabled flag by name
- `delete_schedule` - Delete a schedule by name

### Reminder Tools
- `create_reminder` - Create a one-shot reminder (prompt, fire_at required; always call get_time first)
- `list_reminders` - List upcoming unfired reminders
- `cancel_reminder` - Cancel a reminder by id

## System Prompt Guidelines

The agent should:
- Be helpful but concise - no unnecessary chatter
- Proactively use tools to check state before answering questions about things
- Everything is a "thing" — use tags for categorization, status and priority for state
- Remember context across conversations using notes and memories
- During check-ins: summarize open things, mention overdue items, ask about priorities
- Not be annoying - check-ins should be useful, not nagging
- Admit when it doesn't know something rather than making things up

## Configuration

Config is loaded in priority order: environment variables > `.env` (local dev) > `~/.jot/config` (installed service).

All files use dotenv format (`KEY=value`). The `~/.jot/config` file is seeded from `.env` on first `make install`.

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
make build

# Run in foreground
make run
# or: ./jotd
# or: ./jotd run

# Install as login service (copies to /usr/local/bin, sets up launchd)
make install

# Service management (after install)
jotd status
jotd stop
jotd start
jotd restart
jotd logs

# Uninstall
make uninstall
```

## Development Phases

### Phase 1: Core (MVP)
- [x] SQLite schema and migrations
- [x] Database query functions
- [x] Anthropic client with tool calling
- [x] Core tools (5 thing tools + notes + memory + skills + time)
- [x] CLI mode for testing: `echo "track a thing: buy milk" | ./agent`

### Phase 2: Discord
- [x] Discord bot setup (listen for DMs)
- [x] Message handling (pipe through agent)
- [x] Webhook for outbound messages

### Phase 3: Scheduling
- [x] Internal cron scheduler
- [x] Check-in logic (build context, send to LLM, post to Discord)
- [x] DB-driven multi-schedule cron (schedules table, agent-manageable)
- [x] One-shot reminders with 60s polling ticker
- [x] CHECK_IN_CRON demoted to seed fallback

### Phase 4: Memory Improvements (PLAN2.md Phase 2)
- [x] FTS5 full-text search for memories (virtual table, triggers, backfill)
- [x] Memory management tools (update_memory, delete_memory, resolve_memory)
- [ ] Conversation summaries (table, Discord handler rework, auto-summarize)

### Phase 5: Polish
- [x] Remaining tools (ideas, notes)
- [x] Better context window management
- [x] Conversation history (how many messages to include)
- [ ] Markdown export command for human review
- [x] Start on startup or login (launchd service via `make install`)

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
- Discord bot should only respond to DMs from authorized user(s)
- Store secrets in environment variables, never in code

## Testing

```bash
go test ./...
```

For integration tests with the LLM, use a test flag or separate build tag to avoid API calls in CI.

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
