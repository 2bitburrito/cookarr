-- name: ListLayoutPresets :many
SELECT name, device_class, layout_json, updated_at FROM layout_preset
WHERE device_class = ? ORDER BY name;

-- name: GetLayoutPreset :one
SELECT name, device_class, layout_json, updated_at FROM layout_preset
WHERE name = ? AND device_class = ?;

-- name: UpsertLayoutPreset :exec
INSERT INTO layout_preset (name, device_class, layout_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (name, device_class) DO UPDATE SET
    layout_json = excluded.layout_json,
    updated_at = excluded.updated_at;

-- name: DeleteLayoutPreset :execrows
DELETE FROM layout_preset WHERE name = ? AND device_class = ?;

-- name: GetRecipeLayout :one
SELECT layout_json, updated_at FROM recipe_layout
WHERE recipe_id = ? AND device_class = ?;

-- name: UpsertRecipeLayout :exec
INSERT INTO recipe_layout (recipe_id, device_class, layout_json, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (recipe_id, device_class) DO UPDATE SET
    layout_json = excluded.layout_json,
    updated_at = excluded.updated_at;

-- name: DeleteRecipeLayout :execrows
DELETE FROM recipe_layout WHERE recipe_id = ? AND device_class = ?;

-- name: GetPref :one
SELECT value FROM pref WHERE key = ?;

-- name: SetPref :exec
INSERT INTO pref (key, value, updated_at)
VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at;
