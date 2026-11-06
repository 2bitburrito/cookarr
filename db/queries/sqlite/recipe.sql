-- name: CreateRecipe :one
INSERT INTO recipe (
    title, description, source_url, site_name, hero_url, data, stripped_html, captured_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetRecipe :one
SELECT data, captured_at FROM recipe WHERE id = ?;

-- name: CountRecipes :one
SELECT COUNT(*) FROM recipe;

-- name: ListRecipes :many
SELECT id, title, description, source_url, site_name, hero_url, captured_at
FROM recipe ORDER BY captured_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: DeleteRecipe :execrows
DELETE FROM recipe WHERE id = ?;
