-- name: GetTeam :one
SELECT * FROM teams
WHERE id = $1;

-- name: GetTeamBySlug :one
SELECT * FROM teams
WHERE slug = $1;

-- name: ListTeams :many
SELECT * FROM teams
ORDER BY name;

-- name: CreateTeam :one
INSERT INTO teams (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM teams
WHERE id = $1;
