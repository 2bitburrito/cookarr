-- name: ListLayoutPresets :many
SELECT name, device_class, layout_json, updated_at FROM layout_preset
WHERE device_class = $1 ORDER BY name;

-- name: GetLayoutPreset :one
SELECT name, device_class, layout_json, updated_at FROM layout_preset
WHERE name = $1 AND device_class = $2;

-- name: UpsertLayoutPreset :exec
INSERT INTO layout_preset (name, device_class, layout_json, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name, device_class) DO UPDATE SET
    layout_json = excluded.layout_json,
    updated_at = excluded.updated_at;

-- name: DeleteLayoutPreset :execrows
DELETE FROM layout_preset WHERE name = $1 AND device_class = $2;

-- name: GetRecipeLayout :one
SELECT layout_json, updated_at FROM recipe_layout
WHERE recipe_id = $1 AND device_class = $2;

-- name: UpsertRecipeLayout :exec
INSERT INTO recipe_layout (recipe_id, device_class, layout_json, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (recipe_id, device_class) DO UPDATE SET
    layout_json = excluded.layout_json,
    updated_at = excluded.updated_at;

-- name: DeleteRecipeLayout :execrows
DELETE FROM recipe_layout WHERE recipe_id = $1 AND device_class = $2;

-- name: GetPref :one
SELECT value FROM pref WHERE key = $1;

-- name: SetPref :exec
INSERT INTO pref (key, value, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at;
