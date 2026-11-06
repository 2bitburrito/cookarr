-- +goose Up
-- The CHECK pins the table to a single row: Cookarr has one owner, and two
-- concurrent setup requests can't both insert it.
CREATE TABLE IF NOT EXISTS owner (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS session (
    id         INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS recipe (
    id            INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    source_url    TEXT NOT NULL,
    site_name     TEXT NOT NULL DEFAULT '',
    hero_url      TEXT NOT NULL DEFAULT '',
    data          TEXT NOT NULL, -- capture.Recipe as JSON
    stripped_html BLOB NOT NULL,
    captured_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS recipe_captured_at ON recipe (captured_at DESC, id DESC);

-- +goose Down
DROP TABLE recipe;
DROP TABLE session;
DROP TABLE owner;
