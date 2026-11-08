-- +goose Up
CREATE TABLE layout_preset (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         TEXT NOT NULL,
    device_class TEXT NOT NULL,
    layout_json  JSONB NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    UNIQUE (name, device_class)
);

CREATE TABLE recipe_layout (
    recipe_id    BIGINT NOT NULL REFERENCES recipe (id) ON DELETE CASCADE,
    device_class TEXT NOT NULL,
    layout_json  JSONB NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (recipe_id, device_class)
);

CREATE TABLE pref (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE pref;
DROP TABLE recipe_layout;
DROP TABLE layout_preset;
