CREATE TABLE IF NOT EXISTS things (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    notes TEXT,
    status TEXT DEFAULT 'open',
    priority TEXT DEFAULT 'normal',
    tags TEXT,
    due_date TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY,
    key TEXT UNIQUE NOT NULL,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS schedules (
	id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  cron_expr TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  enabled INTEGER DEFAULT 1,
  last_run TEXT,
  fire_at TEXT,
  fired INTEGER DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS watches (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    prompt TEXT NOT NULL,
    urls TEXT NOT NULL DEFAULT '[]',
    cron_expr TEXT NOT NULL DEFAULT '',
    enabled INTEGER DEFAULT 1,
    last_run TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS watch_results (
    id INTEGER PRIMARY KEY,
    watch_id INTEGER NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
    content_hash TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    source_url TEXT,
    first_seen TEXT DEFAULT (datetime('now')),
    notified INTEGER DEFAULT 0,
    UNIQUE(watch_id, content_hash)
);
