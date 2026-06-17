# stateless-agent Specification

## Purpose

Defines the durable contract for Jot's stateless agent: each inbound message is processed as an independent exchange with no persisted conversation history, no summarization, a reduced tool set, and a system prompt free of memory references.

## Requirements

### Requirement: Stateless single-exchange processing

The agent SHALL process each inbound message as an independent exchange. It MUST NOT persist or load conversation history between messages, and MUST NOT read or write any conversation or summary store.

#### Scenario: Two consecutive messages do not share context

- **WHEN** a user sends "idea: redesign onboarding" and then a second message "what did I just say?"
- **THEN** the agent processes the second message with no knowledge of the first, because no conversation state was persisted

#### Scenario: Quick capture from Discord

- **WHEN** a Discord DM "idea: redesign onboarding" is received
- **THEN** the agent issues a single `create_thing` call with `status='open'` and no conversational state is persisted

### Requirement: No summarization

The agent SHALL NOT run gap-detection or auto-summarization. No code path may trigger summarization based on elapsed time since a previous message.

#### Scenario: Message after a long pause

- **WHEN** a message arrives more than 10 minutes after any previous message
- **THEN** no summarization runs and nothing is written to or read from a summary store

### Requirement: Reduced tool set

The agent SHALL expose exactly these 14 tools and no memory tools: the 4 thing tools (`list_things`, `create_thing`, `update_thing`, `complete_thing`), the 4 schedule tools (`list_schedules`, `create_schedule`, `update_schedule`, `delete_schedule`), and the 6 watch tools (`list_watches`, `create_watch`, `update_watch`, `delete_watch`, `run_watch`, `list_watch_results`). No `save_memory`, `search_memories`, `list_recent_memories`, `update_memory`, or `delete_memory` tool SHALL be present.

#### Scenario: Memory tools are absent

- **WHEN** the agent's tool definitions are enumerated
- **THEN** none of the five memory tools appear in the list

#### Scenario: Watch and schedule tools remain available

- **WHEN** the agent's tool definitions are enumerated
- **THEN** all 4 schedule tools and all 6 watch tools are present and callable

### Requirement: System prompt free of memory references

The system prompt SHALL contain no instructions referencing memories, conversation summaries, or cross-conversation continuity.

#### Scenario: Prompt inspection

- **WHEN** the system prompt is rendered
- **THEN** it contains no mention of saving/searching memories or remembering context across conversations
