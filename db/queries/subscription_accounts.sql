-- name: ListActiveAccountsBySource :many
SELECT * FROM subscription_accounts WHERE source_id = $1 AND status = 'active' AND quota_remaining > 0 AND (cooldown_until IS NULL OR cooldown_until < NOW()) ORDER BY quota_remaining DESC;

-- name: UpdateAccountQuota :exec
UPDATE subscription_accounts SET quota_remaining = $2, last_used_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: MarkAccountExhausted :exec
UPDATE subscription_accounts SET status = 'exhausted', updated_at = NOW() WHERE id = $1;

-- name: MarkAccountDead :exec
UPDATE subscription_accounts SET status = 'dead', last_error_code = $2, last_error_message = $3, updated_at = NOW() WHERE id = $1;

-- name: RefreshAccountToken :exec
UPDATE subscription_accounts SET credential_ciphertext = $2, token_expires_at = $3, last_refreshed_at = NOW(), refresh_count = refresh_count + 1, consecutive_failures = 0, updated_at = NOW() WHERE id = $1;
