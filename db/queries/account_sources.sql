-- name: GetAccountSource :one
SELECT * FROM account_sources WHERE id = $1;

-- name: ListAccountSources :many
SELECT * FROM account_sources ORDER BY priority ASC, created_at DESC;

-- name: ListHealthySourcesByType :many
SELECT * FROM account_sources WHERE source_type = $1 AND status = 'active' AND health_state NOT IN ('dead', 'circuit_open') ORDER BY priority ASC;

-- name: CreateAccountSource :one
INSERT INTO account_sources (name, source_type, provider_id, endpoint, credential_ciphertext, credential_fingerprint, model_policy_json, priority, weight, max_concurrency, daily_budget_micro_usd, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateSourceHealth :exec
UPDATE account_sources SET health_state = $2, last_validated_at = $3, updated_at = NOW() WHERE id = $1;

-- name: DisableSource :exec
UPDATE account_sources SET status = 'disabled', updated_at = NOW() WHERE id = $1;
