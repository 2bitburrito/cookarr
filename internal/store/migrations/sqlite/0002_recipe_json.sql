-- +goose Up
-- A separate migration also upgrades databases created by the initial runner.
CREATE TABLE recipe_json (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL,
    site_name TEXT NOT NULL DEFAULT '',
    hero_url TEXT NOT NULL DEFAULT '',
    data TEXT NOT NULL CHECK (json_valid(data)),
    stripped_html BLOB NOT NULL,
    captured_at TEXT NOT NULL
);

INSERT INTO recipe_json SELECT * FROM recipe;
DROP TABLE recipe;
ALTER TABLE recipe_json RENAME TO recipe;
CREATE INDEX recipe_captured_at ON recipe (captured_at DESC, id DESC);

-- +goose Down
CREATE TABLE recipe_unchecked (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL,
    site_name TEXT NOT NULL DEFAULT '',
    hero_url TEXT NOT NULL DEFAULT '',
    data TEXT NOT NULL,
    stripped_html BLOB NOT NULL,
    captured_at TEXT NOT NULL
);
INSERT INTO recipe_unchecked SELECT * FROM recipe;
DROP TABLE recipe;
ALTER TABLE recipe_unchecked RENAME TO recipe;
CREATE INDEX recipe_captured_at ON recipe (captured_at DESC, id DESC);
