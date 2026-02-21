# Jot Enhancement Plan

This plan adds four features to Jot, inspired by Picobot's design but adapted to Jot's sandboxed security model. Everything stays within the SQLite boundary — no filesystem, no exec.

Work through each phase in order. Each phase is self-contained and testable before moving on.

---

## Phase 1: Skills System + Time Awareness ✓ COMPLETE

### 1.0 `get_time` Tool

The agent currently has no way to know the current date or time. This is needed for reasoning about due dates, scheduling, and basic temporal awareness.

Add to `internal/llm/tools.go`:

- `get_time` — no params. Returns current UTC time in RFC3339 format plus the local date in YYYY-MM-DD.

Implementation in `internal/agent/agent.go` `executeTool()`:

```go
case "get_time":
    now := time.Now().UTC()
    result = map[string]any{
        "utc":  now.Format(time.RFC3339),
        "date": now.Format("2006-01-02"),
        "day":  now.Weekday().String(),
    }
```

Add to system prompt:

```
Time:
- Use get_time when you need to know the current date or time.
- Use it to calculate how many days until a due date, or to set sensible expiry times on memories.
```

No schema changes. No tests needed beyond a manual check. Do this first — it's one tool, a few lines, and immediately useful.

### 1.1 Skills

Skills are reusable knowledge packages stored in SQLite. They let the agent build up procedural knowledge over time — "how to do X" instructions that get loaded into context when relevant. Think of them as modular, agent-managed extensions to the system prompt.

### 1.2 Schema

Add to `internal/db/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS skills (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT, -- JSON array, same pattern as ideas
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
```

`name` is a short slug (e.g. "weekly-review", "project-summary-format"). `description` is a one-liner the agent uses to decide relevance. `content` is the actual skill body — instructions, templates, examples. `tags` follows the existing JSON array pattern from ideas/memories.

### 1.3 Database Queries

Add to `internal/db/queries.go`:

- `CreateSkill(name, description, content string, tags []string) (int64, error)`
- `GetSkill(name string) (*Skill, error)` — lookup by name
- `ListSkills(tag string) ([]Skill, error)` — list all, optionally filter by tag
- `UpdateSkill(name string, fields map[string]any) error` — update content/description/tags
- `DeleteSkill(name string) error`

Add `"skills"` to the `allowedColumns` map with: `name`, `description`, `content`, `tags`.

Define the `Skill` struct following the same patterns as `Project`, `Todo`, etc.

### 1.4 Tool Definitions

Add to `internal/llm/tools.go`:

- `create_skill` — name (required), description (required), content (required), tags (optional)
- `get_skill` — name (required)
- `list_skills` — tag (optional)
- `update_skill` — name (required), plus optional description, content, tags
- `delete_skill` — name (required)

### 1.5 Agent Wiring

Add cases for all five skill tools in `internal/agent/agent.go` `executeTool()`.

### 1.6 System Prompt Update

Add to `internal/llm/prompt.go` under the Memory section:

```
Skills:
- Skills are reusable procedures and knowledge you've learned. Use them to remember HOW to do things.
- Before performing a complex or repeated task, check list_skills for an existing skill.
- If you figure out a good approach for something, save it as a skill for next time.
- Skills are different from memories: memories record WHAT happened, skills record HOW to do things.
- Keep skills focused and actionable. A good skill has clear steps or a template.
```

### 1.7 Context Loading

Update `internal/agent/context.go` `BuildCheckInPrompt()` to load skills tagged with "check-in" and include them in the check-in prompt context.

Consider: should the agent auto-load relevant skills at conversation start? One approach is to have the agent call `list_skills` proactively (the system prompt already tells it to check state before answering). Another is to load skill names/descriptions into every system prompt so the agent knows what's available. Start with the first approach (simpler, no extra tokens on every call) and revisit if the agent doesn't use skills enough.

### 1.8 Tests

Add to `internal/db/queries_test.go`:

- `TestCreateAndListSkills` — basic CRUD
- `TestGetSkillByName` — lookup, including missing skill
- `TestListSkillsFilterByTag` — tag filtering
- `TestUpdateSkill` — partial updates
- `TestDeleteSkill` — delete and verify gone
- `TestCreateSkillDuplicateName` — should error on UNIQUE constraint

---

## Phase 2: Memory Improvements

Three changes: add FTS5 for better search, add memory management tools, and persist conversation summaries.

### 2.1 FTS5 Full-Text Search

SQLite FTS5 enables proper full-text search so the agent doesn't have to guess exact keywords.

Add to schema (after the `memories` table):

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    content,
    content_rowid='id',
    content='memories'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete', old.id, old.content);
    INSERT INTO memories_fts(rowid, content) VALUES (new.id, new.content);
END;
```

**Important**: `modernc.org/sqlite` (the pure-Go driver Jot uses) does support FTS5. Verify this works with a test before building on it. If it doesn't, fall back to FTS4 or keep LIKE search as a fallback.

Add to `internal/db/queries.go`:

- `SearchMemoriesFTS(query string, limit int) ([]Memory, error)` — uses `memories_fts MATCH ?` joined back to `memories` for full rows, ordered by rank.

Update the `search_memories` tool handler in `agent.go`: if the query param is provided, use FTS search instead of LIKE. Keep the existing category/tag/project filters as additional WHERE clauses on the join.

### 2.2 Memory Management Tools

Add to `internal/db/queries.go`:

- `UpdateMemory(id int64, fields map[string]any) error` — allow updating content, category, tags, expires_at
- `DeleteMemory(id int64) error`
- `ResolveMemory(id int64, resolution string) error` — marks a memory (especially blockers) as resolved: sets `category` to `'resolved'` and appends resolution text to content

Add `"memories"` to `allowedColumns`: `content`, `category`, `tags`, `expires_at`.

Add to `internal/llm/tools.go`:

- `update_memory` — id (required), content, category, tags, expires_at (all optional)
- `delete_memory` — id (required)

Update system prompt to mention these tools and when to use them (e.g., "When a blocker is resolved, update or delete the memory").

### 2.3 Conversation Summaries

Add a table:

```sql
CREATE TABLE IF NOT EXISTS conversation_summaries (
    id INTEGER PRIMARY KEY,
    channel TEXT NOT NULL, -- discord channel ID, "cli", etc.
    summary TEXT NOT NULL,
    message_count INTEGER,
    created_at TEXT DEFAULT (datetime('now'))
);
```

Add to `internal/db/queries.go`:

- `SaveConversationSummary(channel, summary string, msgCount int) (int64, error)`
- `GetRecentConversationSummaries(channel string, limit int) ([]ConversationSummary, error)`

The idea: when a Discord conversation goes quiet (e.g., no messages for 10+ minutes) or hits a length threshold, the agent summarizes the conversation and stores it. On the next message, recent summaries for that channel get loaded into context.

Implementation in `internal/discord/handlers.go`:

- Track last message timestamp per channel
- Before processing a new message, if it's been >10 minutes since the last one, summarize the old history using the LLM, save it, then start fresh
- Load the last 2-3 conversation summaries into the prompt as context

This replaces the current in-memory `histories` map as the primary continuity mechanism. The map still holds the current conversation, but summaries survive restarts.

### 2.4 Tests

- `TestSearchMemoriesFTS` — verify FTS returns results for semantic-adjacent terms
- `TestFTSSyncOnInsert` — insert a memory, verify FTS finds it
- `TestFTSSyncOnDelete` — delete a memory, verify FTS no longer finds it
- `TestUpdateMemory` — partial field updates
- `TestDeleteMemory` — delete and verify gone
- `TestResolveMemory` — verify category changes and content is appended
- `TestSaveAndGetConversationSummaries` — basic CRUD, channel filtering

---

## Phase 3: Multi-Schedule Cron + Reminders ✓ COMPLETE

Replace the single `CHECK_IN_CRON` with a schedule system the agent can manage, and add one-shot reminders for "remind me in 5 minutes" style requests.

### 3.0 Design Note: Schedules vs. Reminders

These are two different patterns:
- **Schedules** are recurring (cron-based): "check in every morning at 9am"
- **Reminders** are one-shot (timestamp-based): "remind me about this in 5 minutes" or "nudge me about the PR at 3pm"

Both are fired by the scheduler, but stored separately. The agent uses `get_time` (from Phase 1) to calculate `fire_at` timestamps for reminders.

### 3.1 Schema

```sql
CREATE TABLE IF NOT EXISTS schedules (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    cron_expr TEXT NOT NULL,
    prompt TEXT NOT NULL, -- what to tell the agent when this fires
    enabled INTEGER DEFAULT 1,
    last_run TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
```

Seed a default schedule on first run (in the migration or in the scheduler init):

```sql
INSERT OR IGNORE INTO schedules (name, cron_expr, prompt) 
VALUES ('morning-checkin', '0 9 * * *', 'Perform a morning check-in. Summarize pending work, mention overdue items, suggest priorities for the day.');
```

### 3.2 Database Queries

Add to `internal/db/queries.go`:

- `ListSchedules(enabledOnly bool) ([]Schedule, error)`
- `CreateSchedule(name, cronExpr, prompt string) (int64, error)`
- `UpdateSchedule(id int64, fields map[string]any) error`
- `DeleteSchedule(name string) error`
- `RecordScheduleRun(id int64) error` — sets `last_run` to now

### 3.3 Scheduler Rewrite

Rewrite `internal/scheduler/scheduler.go`:

- On startup, load all enabled schedules from the DB
- Register each as a cron job
- Each job: builds context (reuse `BuildCheckInPrompt` or a generalized version), runs the agent with the schedule's prompt, posts result to Discord webhook, records the run
- Periodically (every 5 min) reload schedules from DB to pick up changes made through conversation

The `CHECK_IN_CRON` env var becomes a fallback — if the schedules table is empty, seed it with this value. If it's populated, ignore the env var.

### 3.4 Tool Definitions

Add to `internal/llm/tools.go`:

- `list_schedules` — no params
- `create_schedule` — name (required), cron_expr (required), prompt (required)
- `update_schedule` — name (required), cron_expr, prompt, enabled (optional)
- `delete_schedule` — name (required)

### 3.5 System Prompt Update

```
Schedules:
- You can create and manage recurring scheduled tasks.
- Each schedule has a cron expression and a prompt that gets run when it fires.
- Use this for daily check-ins, weekly reviews, periodic reminders, etc.
- Common cron patterns: "0 9 * * *" (daily 9am), "0 9 * * 1" (Monday 9am), "0 */4 * * *" (every 4 hours).
```

### 3.6 Generalize Context Building

Rename/refactor `BuildCheckInPrompt` in `internal/agent/context.go` to `BuildScheduledPrompt(database *db.DB, scheduleName string) (string, error)`. It should:

1. Prune expired memories
2. Get summary
3. Get last check-in
4. Get recent memories (last 7 days)
5. Load skills tagged with the schedule name (if any)
6. Return the assembled context

The schedule's `prompt` field gets appended to this context when the cron fires.

### 3.7 Reminders Schema

```sql
CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY,
    prompt TEXT NOT NULL,       -- what to tell the agent when this fires
    fire_at TEXT NOT NULL,      -- UTC datetime: "2025-02-07 14:30:00"
    fired INTEGER DEFAULT 0,   -- 0 = pending, 1 = fired
    created_at TEXT DEFAULT (datetime('now'))
);
```

### 3.8 Reminder Queries

Add to `internal/db/queries.go`:

- `CreateReminder(prompt, fireAt string) (int64, error)`
- `ListPendingReminders() ([]Reminder, error)` — where `fired = 0` and `fire_at <= datetime('now')`, ordered by `fire_at`
- `MarkReminderFired(id int64) error` — sets `fired = 1`
- `ListUpcomingReminders() ([]Reminder, error)` — where `fired = 0` and `fire_at > datetime('now')`, ordered by `fire_at`. So the agent can answer "what reminders do I have?"

### 3.9 Reminder Tools

Add to `internal/llm/tools.go`:

- `create_reminder` — prompt (required), fire_at (required, UTC datetime string). The agent should use `get_time` first to calculate the correct `fire_at`.
- `list_reminders` — no params. Returns upcoming unfired reminders.
- `cancel_reminder` — id (required). Deletes or marks as fired.

Add to system prompt:

```
Reminders:
- Use create_reminder for one-shot future tasks: "remind me in 5 minutes", "nudge me at 3pm".
- Always call get_time first to calculate the correct fire_at datetime.
- fire_at must be in UTC format: "YYYY-MM-DD HH:MM:SS".
- Reminders are different from schedules: reminders fire once, schedules recur.
```

### 3.10 Scheduler: Reminder Polling

Update the scheduler to check for due reminders on a short interval. Add a ticker (every 60 seconds) that:

1. Calls `ListPendingReminders()` to get all reminders where `fire_at <= now` and `fired = 0`
2. For each due reminder: runs the agent with the reminder's prompt (wrapped in context like "A reminder you set: {prompt}"), posts the result to the Discord webhook, marks the reminder as fired
3. Optionally: clean up old fired reminders (older than 7 days) during this sweep

This runs independently of the cron-based schedules. The ticker should be lightweight — if there are no due reminders, it's just a single cheap SQL query.

### 3.11 Tests

Schedules:
- `TestCreateAndListSchedules` — basic CRUD
- `TestCreateScheduleDuplicateName` — UNIQUE constraint
- `TestUpdateScheduleEnable/Disable` — toggle enabled
- `TestRecordScheduleRun` — verify last_run updates
- `TestDeleteSchedule` — delete and verify gone

Reminders:
- `TestCreateAndListReminders` — basic CRUD
- `TestListPendingReminders` — only returns due, unfired reminders
- `TestListUpcomingReminders` — only returns future, unfired reminders
- `TestMarkReminderFired` — verify fired flag updates
- `TestPendingRemindersExcludesFired` — fired reminders don't appear in pending list

---

## Phase 4: Polish and Integration

Final cleanup once Phases 1-3 are working.

### 4.1 Update `BuildScheduledPrompt` to Include Skills

When building context for any scheduled run or check-in, include a brief listing of available skill names and descriptions so the agent knows what tools/procedures it has available.

### 4.2 Prune Old Conversation Summaries ⚠️ BLOCKED on Phase 2

Add a cleanup job: delete conversation summaries older than 30 days. Run this as part of `PruneExpiredMemories` or alongside it.

### 4.3 Update CLAUDE.md

Add the new tables to the schema section. Add the new tools to the tools list. Update the development phases checklist. Document the skills, schedules, and FTS features.

### 4.4 Update README.md

Add sections for skills, memory improvements, and multi-schedule cron. Update the "What it can do" list.

### 4.5 Migration Safety

Since the schema uses `CREATE TABLE IF NOT EXISTS` and `CREATE TRIGGER IF NOT EXISTS`, existing databases will get the new tables on next startup without losing data. Verify this works by running against a database that already has data in the existing tables.

For FTS5 specifically ⚠️ BLOCKED on Phase 2: if the `memories` table already has rows when the FTS table is first created, the FTS index will be empty. Add a one-time backfill in `db.go` `Open()`:

```go
// Backfill FTS if needed
d.conn.Exec(`INSERT OR IGNORE INTO memories_fts(rowid, content) SELECT id, content FROM memories`)
```

### 4.6 Reminder DM Delivery

Reminders currently fire to the Discord webhook (a channel), not back to the user as a DM. For time-sensitive reminders this feels wrong.

**Implementation:**

1. Add `SendDM(userID, content string) error` to `internal/discord/bot.go`, using the existing `discordgo.Session` to open a DM channel and send a message.

2. Store the user's Discord ID as a note in the DB (`discord_user_id`). The agent can set this via `set_note` on first interaction, or the bot can capture it from the first DM it receives and store it automatically.

3. Update `fireReminders()` in `scheduler.go` to send a DM if `discord_user_id` is set, falling back to the webhook if not.

The scheduler needs a reference to the bot to call `SendDM`. Pass it in via `scheduler.New()` or add a `SetBot(*discord.Bot)` method — whichever is cleaner given circular import constraints (check: scheduler imports discord, discord imports nothing from scheduler, so direct injection is fine).

**Note on future abstraction:** when/if a second notification target is added (e.g. SMS, push), extract a `Notifier` interface at that point. Don't do it now.

### 4.7 Final Test Pass

Run `go test ./...` and `go vet ./...`. Ensure all new and existing tests pass. Build and do a manual smoke test through the CLI.

---

## Implementation Order Summary

1. **Phase 1** (Skills + Time) — `get_time` tool first (trivial, immediately useful), then skills table, 5 skill tools, tests. No changes to existing code except additive.
2. **Phase 2** (Memory) — FTS5, management tools, conversation summaries. Touches existing memory queries and Discord handler.
3. **Phase 3** (Schedules + Reminders) — schedules table, reminders table, scheduler rewrite with reminder polling, 7 new tools. Replaces single-cron approach.
4. **Phase 4** (Polish) — integration, docs, migration safety.

Each phase should end with `go test ./...` passing and a manual test via CLI.
