-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: GetAPIKeyByPrefix :many
SELECT * FROM api_keys WHERE key_prefix = $1 AND revoked_at IS NULL LIMIT 10;

-- name: CreateAPIKey :one
INSERT INTO api_keys (organization_id, project_id, name, environment, key_prefix, key_hash, scopes_json, model_policy_json, ip_allowlist_json, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: RevokeAPIKey :one
UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL RETURNING *;

-- name: ListAPIKeysByProject :many
SELECT id, name, environment, key_prefix, scopes_json, model_policy_json, expires_at, last_used_at, created_at
FROM api_keys WHERE project_id = $1 AND revoked_at IS NULL ORDER BY created_at DESC;
