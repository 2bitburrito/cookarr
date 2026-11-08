-- +goose Up
-- Named, reusable layouts per device class (built-ins live in the frontend;
-- these are user-saved/custom presets plus the built-in overrides).
CREATE TABLE IF NOT EXISTS layout_preset (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    device_class TEXT NOT NULL,
    layout_json  TEXT NOT NULL CHECK (json_valid(layout_json)),
    updated_at   TEXT NOT NULL,
    UNIQUE (name, device_class)
);

-- A per-recipe layout override, one per device class. Resolution order
-- (docs/PLAN.md): per-recipe override, then the user's chosen preset, then
-- the built-in default.
CREATE TABLE IF NOT EXISTS recipe_layout (
    recipe_id    INTEGER NOT NULL REFERENCES recipe (id) ON DELETE CASCADE,
    device_class TEXT NOT NULL,
    layout_json  TEXT NOT NULL CHECK (json_valid(layout_json)),
    updated_at   TEXT NOT NULL,
    PRIMARY KEY (recipe_id, device_class)
);

-- Small key/value store for prefs, e.g. which preset is "active" per device
-- class. Values are JSON so callers don't need a new column per pref.
CREATE TABLE IF NOT EXISTS pref (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL CHECK (json_valid(value)),
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE pref;
DROP TABLE recipe_layout;
DROP TABLE layout_preset;
