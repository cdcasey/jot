# web-ui Specification

## Purpose

Defines the durable contract for Jot's web interface: a server-rendered layer
compiled into the existing single binary that reads and writes the same SQLite
database as the agent, scheduler, and watch runner. The web UI surfaces two
views — a drag-and-drop kanban board over `things` and a read-only habit
calendar grid over `habit_logs`. It introduces a manual `position` ordering for
things and relies on network-level access control rather than application-level
authentication for the MVP.

## Requirements

### Requirement: Embedded web layer served from the single binary

The system SHALL provide a server-rendered web interface from an `internal/web`
package compiled into the existing binary. The web server SHALL be started only
when a `WEB_PORT` environment variable is set, and SHALL read and write the same
SQLite database used by the agent, scheduler, and watch runner. No separate
service, Node toolchain, or front-end build step SHALL be required, and the
pure-Go `modernc.org/sqlite` driver (no CGO) SHALL be retained.

#### Scenario: Web server starts when WEB_PORT is set

- **WHEN** the binary starts with `WEB_PORT=8080` set
- **THEN** an HTTP server listens on port 8080 alongside the existing Discord bot and scheduler
- **AND** it serves the kanban board and habit grid from the same SQLite DB

#### Scenario: Web server is absent when WEB_PORT is unset

- **WHEN** the binary starts with no `WEB_PORT` set
- **THEN** no web server is started and existing CLI/Discord/scheduler behavior is unchanged

#### Scenario: No external access beyond SQLite

- **WHEN** the web layer handles any request
- **THEN** it reads and writes only the SQLite database and performs no filesystem, shell, or new outbound network access

### Requirement: Web app exposure relies on network-level access control

The system SHALL expose the web app over a loopback or Tailscale interface and
SHALL rely on network-level access control (e.g. Tailscale ACLs) for the MVP. No
application-level authentication SHALL be required in the MVP. The web app SHALL
NOT assume it is safe to bind to a public interface without external protection.

#### Scenario: Reachable over the tailnet without app login

- **WHEN** an authorized device on the tailnet requests the board
- **THEN** the board is served with no application-level login step

#### Scenario: No app-level auth gate in the MVP

- **WHEN** the web app is enumerated for authentication middleware
- **THEN** no application-level auth gate is present, and access control is delegated to the network layer

### Requirement: Things carry a manual position for ordering

The system SHALL add a `position` column to the `things` table that determines
the manual sort order of a thing within its status column. Existing rows SHALL
be backfilled with positions on migration so that current ordering is stable and
deterministic. The migration SHALL be idempotent for existing databases.

#### Scenario: Migration adds and backfills position

- **WHEN** an existing database without a `position` column is migrated
- **THEN** the `things` table gains a `position` column
- **AND** every existing row receives a deterministic position value

#### Scenario: Migration is idempotent

- **WHEN** migration runs again on a database that already has `position`
- **THEN** it completes without error and does not duplicate or reset positions

### Requirement: Kanban board renders things grouped by status and ordered by position

The system SHALL render a three-column kanban over `things`, mapping `open` to
"Ideas", `active` to "Active", and `done` to "Done". Things with status
`dropped` SHALL be treated as archived and SHALL NOT appear as a board column.
Within each column, things SHALL be ordered by `position` ascending.

#### Scenario: Columns map to statuses

- **WHEN** the board is rendered
- **THEN** the "Ideas", "Active", and "Done" columns contain the things whose status is `open`, `active`, and `done` respectively

#### Scenario: Dropped items are off the board

- **WHEN** a thing has status `dropped`
- **THEN** it does not appear in any of the three columns

#### Scenario: Cards ordered by position within a column

- **WHEN** a column contains multiple things
- **THEN** they are rendered in ascending `position` order

### Requirement: Dragging within a column persists order

The system SHALL persist a new manual order when a card is reordered within its
column, using fractional/sparse positions so that only the moved card's row is
updated in the common case. When no fractional value fits between the new
neighbors, the system SHALL renumber the affected column to restore spacing and
then place the moved card.

#### Scenario: Reorder updates only the moved card

- **WHEN** a card is dragged to a new slot between two cards in the same column
- **THEN** the moved card's `position` is set to a value between its new neighbors
- **AND** no other card's `position` is changed

#### Scenario: Renumber fallback when no gap remains

- **WHEN** a card is dropped between two neighbors whose positions leave no representable value between them
- **THEN** the system renumbers that column to restore spacing
- **AND** the moved card ends up in the dropped slot

### Requirement: Dragging across columns persists status and order together

The system SHALL, when a card is dragged into a different column, write the new
`status` for that thing, set its `position` for the new column, and bump
`updated_at`, in a single persisted operation. When a card is dropped into the
Done column, the system SHALL stamp `completed_at`. Moving a card out of Done
SHALL clear `completed_at`.

#### Scenario: Cross-column move updates status and position

- **WHEN** a card is dragged from "Ideas" to "Active"
- **THEN** the thing's `status` becomes `active`
- **AND** its `position` is set for its slot in the Active column
- **AND** `updated_at` is bumped

#### Scenario: Dropping into Done stamps completion

- **WHEN** a card is dragged into the "Done" column
- **THEN** the thing's `status` becomes `done`
- **AND** `completed_at` is stamped

#### Scenario: Moving out of Done clears completion

- **WHEN** a card is dragged from "Done" back to "Active"
- **THEN** the thing's `status` becomes `active`
- **AND** `completed_at` is cleared

### Requirement: Habit grid renders a read-only calendar from habit logs

The system SHALL render a per-habit calendar grid whose cells are derived solely
from `habit_logs`: a cell SHALL be filled where a `habit_logs` row exists for
that `(habit, date)` and empty otherwise. In the MVP the grid SHALL be
read-only and SHALL NOT create, update, or delete any `habit_logs` row; logging
remains the responsibility of the `log_habit` tool defined by the
`habit-tracking` capability.

#### Scenario: Filled cell reflects an existing log

- **WHEN** a `habit_logs` row exists for habit "meditate" on "2026-06-16"
- **THEN** the grid cell for that habit and date is rendered filled

#### Scenario: Empty cell reflects no log

- **WHEN** no `habit_logs` row exists for habit "read" on "2026-06-16"
- **THEN** the grid cell for that habit and date is rendered empty

#### Scenario: Grid never writes in the MVP

- **WHEN** the grid is viewed or interacted with in the MVP
- **THEN** no `habit_logs` row is created, updated, or deleted by the web layer
