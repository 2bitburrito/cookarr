-- +goose Up
CREATE TABLE owner (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE session (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE recipe (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL,
    site_name TEXT NOT NULL DEFAULT '',
    hero_url TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL,
    stripped_html BYTEA NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX recipe_captured_at ON recipe (captured_at DESC, id DESC);

-- +goose Down
DROP TABLE recipe;
DROP TABLE session;
DROP TABLE owner;
