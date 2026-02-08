# Personal Assistant Agent

A sandboxed personal assistant for tracking todos, ideas, and projects. Interacts via Discord, stores data in SQLite, uses Claude for natural language understanding.

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
- **Config:** Environment variables or `.env` file

## Project Structure

```
/cmd/agent/main.go           # Entry point
/internal/db/
    schema.sql               # SQLite schema
    db.go                    # Connection, migrations
    queries.go               # All database operations
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
/config/
    config.go                # Environment/config loading
```

## Database Schema

```sql
CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT DEFAULT 'active',  -- active, paused, completed, archived
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE todos (
    id INTEGER PRIMARY KEY,
    project_id INTEGER REFERENCES projects(id),
    title TEXT NOT NULL,
    notes TEXT,
    status TEXT DEFAULT 'pending',  -- pending, in_progress, done, cancelled
    priority TEXT DEFAULT 'normal', -- low, normal, high, urgent
    due_date TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE TABLE ideas (
    id INTEGER PRIMARY KEY,
    project_id INTEGER REFERENCES projects(id),
    title TEXT NOT NULL,
    content TEXT,
    tags TEXT,  -- JSON array: ["tag1", "tag2"]
    created_at TEXT DEFAULT (datetime('now'))
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
```

## LLM Tools

The agent has exactly these tools - no more, no less:

### Core Tools (Phase 1)
- `list_todos` - List todos, optionally filtered by project_id, status, priority
- `create_todo` - Create a new todo (title required; project_id, notes, priority, due_date optional)
- `update_todo` - Update a todo by id (any field except id and created_at)
- `complete_todo` - Mark a todo as done (shorthand for update_todo with status=done)
- `list_projects` - List all projects, optionally filtered by status
- `create_project` - Create a new project (name required, description optional)
- `get_summary` - Returns active projects count, pending todos count, overdue todos, recent activity

### Extended Tools (Phase 2)
- `update_project` - Update project details or status
- `list_ideas` - List ideas, optionally filtered by project_id or tags
- `create_idea` - Capture a new idea
- `get_note` - Retrieve a stored note by key
- `set_note` - Store or update a note (agent's scratchpad memory)

## System Prompt Guidelines

The agent should:
- Be helpful but concise - no unnecessary chatter
- Proactively use tools to check state before answering questions about todos/projects
- Remember context across the conversation using the notes table for persistent memory
- During check-ins: summarize what's pending, gently nudge about overdue items, ask about priorities
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
```

## Development Phases

### Phase 1: Core (MVP)
- [ ] SQLite schema and migrations
- [ ] Database query functions
- [ ] Anthropic client with tool calling
- [ ] Core tools (6 tools listed above)
- [ ] CLI mode for testing: `echo "add a todo to buy milk" | ./agent`

### Phase 2: Discord
- [ ] Discord bot setup (listen for DMs)
- [ ] Message handling (pipe through agent)
- [ ] Webhook for outbound messages

### Phase 3: Scheduling
- [ ] Internal cron scheduler
- [ ] Check-in logic (build context, send to LLM, post to Discord)

### Phase 4: Polish
- [ ] Remaining tools (ideas, notes)
- [ ] Better context window management
- [ ] Conversation history (how many messages to include)
- [ ] Markdown export command for human review

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
sqlite3 data.db "SELECT * FROM todos"

# Test agent locally
echo "what are my active projects?" | go run ./cmd/agent

# Check Discord bot token is valid
# (bot should come online in your server)
```
