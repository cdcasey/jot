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

### 2.3 Conversation Summaries ✓ COMPLETE

Two tables: `conversations` (persistent message history keyed by user ID) and `conversation_summaries` (LLM-generated summaries of past conversations).

Key design decisions:
- **Keyed by user ID, not channel ID.** Scheduler creates DM channels on the fly and never exposes the channel ID. User ID is stable across Discord DMs, CLI, and any future frontend.
- **`agent.RunWithConversation(ctx, userID, message)`** is the single entry point for all conversation-aware callers (Discord handler, scheduler, CLI REPL, pipe mode). Handles: load → gap-detect → summarize → prepend context → run → trim → save.
- **`agent.Summarize(ctx, messages)`** calls the LLM with a summarization system prompt and no tools.
- **Gap detection (>10 min):** if enough time has passed since the last message, the old conversation is summarized, the summary is saved, and raw messages are cleared. On failure, raw messages are kept (no data loss).
- **Recent summaries (up to 3)** are prepended as a synthetic user/assistant context pair that gets stripped before saving.
- **Discord handlers simplified:** removed in-memory `histories` map and mutex entirely.
- **Scheduler wired in:** `runSchedule` and `fireReminders` use `RunWithConversation` when `discord_user_id` note exists, falling back to `agent.Run` otherwise.
- **CLI:** both pipe mode and interactive REPL use `RunWithConversation("cli", ...)` for cross-restart persistence.

Files added:
- `internal/db/queries_conversations.go` — LoadConversation, SaveConversation, ClearConversation, SaveConversationSummary, GetRecentSummaries, PruneOldSummaries
- `internal/agent/conversation.go` — RunWithConversation, Summarize
- `internal/db/queries_conversations_test.go` — 7 test cases

### 2.4 Tests

- `TestSearchMemoriesFTS` — verify FTS returns results for semantic-adjacent terms
- `TestFTSSyncOnInsert` — insert a memory, verify FTS finds it
- `TestFTSSyncOnDelete` — delete a memory, verify FTS no longer finds it
- `TestUpdateMemory` — partial field updates
- `TestDeleteMemory` — delete and verify gone
- `TestResolveMemory` — verify category changes and content is appended
- `TestSaveAndLoadConversation` — round-trip JSON serialization
- `TestLoadConversationMissing` — returns empty slice + zero time
- `TestSaveConversationUpsert` — overwrite works
- `TestClearConversation` — reset to empty
- `TestSaveAndGetSummaries` — basic CRUD, ordering
- `TestGetRecentSummariesLimit` — respects limit param
- `TestGetRecentSummariesDifferentUsers` — user isolation
- `TestPruneOldSummaries` — deletes old, keeps recent

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

### 3.6 Generalize Context Building ✓ SUPERSEDED

~~Rename/refactor `BuildCheckInPrompt`...~~

**Decision (2026-02-27):** Removed `BuildCheckInPrompt` and `context.go` entirely. Schedules now send their `prompt` directly to the agent with no pre-built context. The LLM self-serves via tools (`get_summary`, `list_things`, `search_memories`) when the prompt asks for it. This fixed a bug where all schedules (including simple ones like "ask about the gym") got the full check-in treatment with 12 open items, overdue reviews, etc.

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

- `create_reminder` — prompt (required), fire_at (required, local datetime string). The agent should use `get_time` first to determine current time.
- `list_reminders` — no params. Returns upcoming unfired reminders.
- `cancel_reminder` — id (required). Deletes or marks as fired.

**Timezone handling (added 2026-02-27):** `fire_at` is now accepted in the user's local time. `agent.go` has a `localToUTC()` helper that converts using the `timezone` note (IANA name, e.g. "America/New_York"), falling back to server local. `get_time` also uses `userLocation()` so the LLM sees consistent local time. The user can set their timezone by telling the agent (e.g., "I'm in Eastern") which persists it via `set_note("timezone", "America/New_York")`.

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

### 4.1 ~~Update `BuildScheduledPrompt` to Include Skills~~ ✓ SUPERSEDED

No longer applicable — `BuildCheckInPrompt`/`BuildScheduledPrompt` was removed. The LLM can call `list_skills` on its own when relevant.

### 4.2 Prune Old Conversation Summaries

Add a cleanup job: delete conversation summaries older than 30 days. `PruneOldSummaries(olderThanDays int)` already exists in `queries_conversations.go` — just needs to be wired into a periodic cron or startup hook.

### 4.3 Update CLAUDE.md

Add the new tables to the schema section. Add the new tools to the tools list. Update the development phases checklist. Document the skills, schedules, and FTS features.

### 4.4 Update README.md

Add sections for skills, memory improvements, and multi-schedule cron. Update the "What it can do" list.

### 4.5 Migration Safety

Since the schema uses `CREATE TABLE IF NOT EXISTS` and `CREATE TRIGGER IF NOT EXISTS`, existing databases will get the new tables on next startup without losing data. Verify this works by running against a database that already has data in the existing tables.

For FTS5 specifically: if the `memories` table already has rows when the FTS table is first created, the FTS index will be empty. Add a one-time backfill in `db.go` `Open()`:

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

## Phase 5: Habit Tracking ✓ COMPLETE

Structured habit logging with streak computation in Go. One table (`habit_logs`), three tools (`log_habit`, `get_habit_stats`, `list_habits`). Logs are immutable — no `updateRow`, no `allowedColumns` entry. Streaks computed in Go and handed to the LLM as clean numbers.

Files added/modified:
- `internal/db/schema.sql` — `habit_logs` table + compound index
- `internal/db/queries.go` — `HabitLog`, `HabitStats`, `HabitSummary` structs
- `internal/db/queries_habits.go` — new file with `LogHabit`, `ListHabits`, `GetHabitStats` + streak helpers
- `internal/llm/tools.go` — 3 tool definitions
- `internal/agent/agent.go` — 3 `executeTool` cases
- `internal/llm/prompt.go` — Habits section in system prompt
- `internal/db/queries_test.go` — 6 test cases

---

## Phase 6: Web Watches ✓ COMPLETE

URL monitoring with LLM-powered extraction. Watches fetch web pages on a schedule, extract structured items via a dedicated LLM call, deduplicate against stored results, and notify via Discord DM/webhook.

### Architecture

```
Scheduler cron / run_watch tool
    → watch.Runner.RunWatch(ctx, watch)
        → Fetch(ctx, urls)           # HTTP GET, HTML→text, 2MB cap, 30s timeout
        → LLM extraction             # Separate system prompt, JSON array output
        → Dedup via SHA-256(title + source_url)
        → Store new results in watch_results
    → Scheduler formats + delivers via DM/webhook
    → Mark results notified
```

### Key Design Decisions

- **Two-phase LLM pipeline**: main agent loop for tool orchestration, separate extraction LLM call with constrained JSON-only system prompt and no tools.
- **Dedup on title + source_url**: hash includes source URL so same-named items from different sites don't collide. Extraction system prompt tells the LLM to produce unique titles (e.g., "Hamlet - Austin Playhouse" not just "Hamlet").
- **Context propagation**: `context.Context` flows through `Fetch` → `fetchOne` → `http.NewRequestWithContext` for proper cancellation.
- **Age-based pruning**: `PruneOldWatchResults(180)` runs daily via scheduler's `pruneOldData()` tick.
- **`UpdateWatch` uses shared `updateRow`**: `watches` registered in `allowedColumns`, gets `updated_at` auto-stamp and not-found checks for free.

### Files

- `internal/watch/fetch.go` — URL fetching + HTML-to-text (golang.org/x/net/html tokenizer)
- `internal/watch/runner.go` — Extraction orchestration, JSON parsing, dedup, storage
- `internal/db/queries_watches.go` — CRUD for watches + watch_results, pruning
- `internal/db/schema.sql` — watches + watch_results tables
- `internal/agent/agent.go` — 6 tool cases (list/create/update/delete/run/list_results)
- `internal/llm/tools.go` — 6 tool definitions
- `internal/llm/prompt.go` — Watches section in system prompt
- `internal/scheduler/scheduler.go` — loadWatches, runWatch, pruneOldData, formatWatchResults
- `eval/eval.go` — SeedWatch + SeedWatchResult support
- `eval/cases.json` — 2 eval cases (tool_create_watch, tool_list_watch_results)

### Tests

- `internal/db/queries_watched_test.go` — 12 tests (CRUD, dedup, cascade, enabled filter, prune)
- `internal/watch/fetch_test.go` — 8 tests (HTML parsing, script stripping, errors, block structure)
- `internal/watch/runner_test.go` — 9 tests (new items, dedup, error cases, markdown fences, hash)

---

## Implementation Order Summary

1. **Phase 1** (Skills + Time) — ✓ COMPLETE
2. **Phase 2** (Memory) — ✓ COMPLETE (FTS5, Memory Mgmt, Conversation Summaries)
3. **Phase 3** (Schedules + Reminders) — ✓ COMPLETE
4. **Phase 4** (Polish) — 4.2 PruneOldSummaries needs wiring into pruneOldData(), 4.3/4.4 done, 4.6 Reminder DM Delivery done, 4.7 done
5. **Phase 5** (Habit Tracking) — ✓ COMPLETE (later simplified: habits removed, use memories with category='habit')
6. **Phase 6** (Web Watches) — ✓ COMPLETE

Each phase should end with `go test ./...` passing and a manual test via CLI.
