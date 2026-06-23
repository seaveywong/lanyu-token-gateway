# Channel Failure Runbook

## Detection

Alert triggers when:
- All sources in a channel return errors for > 2 minutes
- Channel health check fails 3 consecutive times
- Pool available ratio drops below 30%

## Triage

```bash
# 1. Check channel health
curl http://localhost:8081/admin-api/channels/health

# 2. Check specific source status
curl http://localhost:8081/admin-api/account-sources/<source_id>

# 3. Check recent errors for a source
# (via admin console or database)
SELECT event_type, metadata_json, created_at 
FROM account_source_events 
WHERE source_id = '<source_id>' 
ORDER BY created_at DESC LIMIT 20;
```

## Common Scenarios

### Upstream 401/403 (Credentials revoked)

1. Mark the affected accounts as `dead`.
2. Notify operator to provide new tokens.
3. If all accounts dead → source auto-circuit-breaks.
4. Traffic falls back to next priority source.

### Upstream 429 (Rate limited)

1. Accounts enter automatic cooldown (5 min default).
2. Traffic automatically shifts to other accounts in pool.
3. If all accounts rate-limited → pool temporary unavailable.
4. Alert if rate-limit state persists > 30 minutes.

### Provider outage

1. Source health state transitions to `unhealthy`.
2. Circuit breaker opens after `failure_threshold` consecutive failures.
3. Traffic automatically routes to next source/channel.
4. Half-open probe requests after `cooldown_seconds`.
5. Manual override: force-disable or force-enable via admin API.

## Recovery

```bash
# Force re-enable a circuit-broken source
curl -X POST http://localhost:8081/admin-api/account-sources/<id>/reset-circuit

# Trigger immediate health check
curl -X POST http://localhost:8081/admin-api/account-sources/<id>/validate

# Force token refresh for all pool accounts
curl -X POST http://localhost:8081/admin-api/account-sources/<id>/refresh-all
```

## Escalation

| Severity | Condition | Action |
|----------|-----------|--------|
| P3 | Single account dead | Slack notification |
| P2 | Pool >50% accounts dead | PagerDuty + Slack |
| P1 | All sources for a model exhausted | PagerDuty (wake) + Phone call |
| P0 | All channels unavailable | Incident declared, all hands |
