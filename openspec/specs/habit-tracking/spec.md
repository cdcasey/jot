# habit-tracking Specification

## Purpose

Defines the durable contract for structured, record-only habit tracking in Jot:
habit definitions, a uniqueness-guaranteed daily log, and a single toggle tool
(`log_habit`) exposed to the LLM. The presence of a log entry is the entire data
model — recording whether a habit happened on a day. The system deliberately
does not judge: it computes no streaks, targets, cadence, or success. The raw
daily entries are the substrate a future visual grid reads.

## Requirements

### Requirement: Habit definitions are structured records

The system SHALL store habit definitions in a `habits` table with a unique
name, an `active` flag, and a creation timestamp. A habit definition SHALL be
independent of its log entries, so a habit can exist before it is ever logged
and can be renamed without altering any log rows.

#### Scenario: Habit exists before any log

- **WHEN** a habit named "meditate" is created with no log entries
- **THEN** the habit appears in the `habits` table with `active=1`
- **AND** querying its logs returns zero rows without error

#### Scenario: Renaming a habit preserves its logs

- **WHEN** a habit "meditate" has log entries and its name is changed to "meditation"
- **THEN** the log entries remain associated with the same habit id
- **AND** no log row is created, deleted, or altered by the rename

### Requirement: A habit has at most one log entry per local calendar date

The system SHALL enforce a uniqueness constraint of `UNIQUE(habit_id, date)`
on the `habit_logs` table. The presence of a row for `(habit, date)` SHALL be
the sole meaning of "the habit was done that day"; its absence SHALL mean "not
done". No second, contradictory row for the same `(habit, date)` can exist.

#### Scenario: Logging a habit records the day as done

- **WHEN** `log_habit` is called for habit "meditate" on date "2026-06-16"
- **THEN** exactly one row exists in `habit_logs` for that `(habit_id, date)`

#### Scenario: Re-logging the same habit and date is a harmless no-op

- **WHEN** `log_habit` is called twice for habit "meditate" on date "2026-06-16"
- **THEN** still exactly one row exists for that `(habit_id, date)`
- **AND** neither call returns an error

### Requirement: log_habit toggles an entry for a local calendar date

The system SHALL expose a single LLM tool, `log_habit`, that records or removes
a habit's entry for a date. When called to log, it SHALL insert a row
(no-op on conflict). When called to toggle off, it SHALL delete the row for that
`(habit, date)`. If the named habit does not exist, the tool SHALL auto-create
it with `active=1` and then log the entry in the same call. The date SHALL be a
local calendar date resolved through the existing `notes.timezone` /
`userLocation()` path; when no date is supplied, the tool SHALL use the user's
current local date.

#### Scenario: Logging an unknown habit auto-creates it

- **WHEN** `log_habit` is called for a habit name not present in `habits`
- **THEN** the habit is created with `active=1`
- **AND** a log row is inserted for the requested date

#### Scenario: Toggling off removes the entry

- **WHEN** `log_habit` is called with toggle-off for an existing `(habit, date)` entry
- **THEN** that log row is deleted
- **AND** the habit definition row is left untouched

#### Scenario: Omitted date uses the user's local date

- **WHEN** `log_habit` is called without a date
- **THEN** the entry is recorded against the user's current local calendar date as derived from the `timezone` note

### Requirement: Daily logs are readable over a date range

The system SHALL provide a query returning a habit's log entries within an
inclusive date range, ordered by date, to serve as the substrate for a future
visual grid. This capability provides the read query only; it computes no
streaks, targets, cadence, or success metrics.

#### Scenario: Range query returns only dates with entries

- **WHEN** logs for a habit are requested over a date range
- **THEN** the result contains one entry per date that has a row
- **AND** dates without a row are absent from the result
