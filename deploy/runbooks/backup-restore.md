# Backup & Restore Runbook

## PostgreSQL Backup

### Automated (daily full backup)

The scheduler service runs daily backups at 02:00 UTC:

```bash
# Manual full backup
pg_dump -h $PGHOST -U $PGUSER -d token_gateway \
  -F custom -f backup_$(date +%Y%m%d_%H%M%S).dump \
  --no-owner --no-acl

# Verify backup
pg_restore --list backup_*.dump | head -20
```

### WAL Archiving

- `archive_command` configured to write WAL segments to object storage.
- Retention: 30 days.
- Test restore from WAL monthly.

### Recovery Steps

```bash
# 1. Stop application traffic (drain SSE connections first)
# 2. Restore base backup
pg_restore -h $PGHOST -U $PGUSER -d token_gateway \
  --clean --if-exists backup_YYYYMMDD_HHMMSS.dump

# 3. Apply WAL segments
# (handled by restore_command in recovery.conf)

# 4. Verify critical data
psql -h $PGHOST -U $PGUSER -d token_gateway \
  -c "SELECT COUNT(*) FROM ledger_entries;"
psql -h $PGHOST -U $PGUSER -d token_gateway \
  -c "SELECT SUM(amount_micro_usd) FROM ledger_entries;"

# 5. Resume traffic
```

### RPO/RTO Targets

| System | RPO | RTO |
|--------|-----|-----|
| Ledger & Payments | 5 min | 4 hours |
| Control Plane | 15 min | 4 hours |
| Data Plane | Stateless | 1 hour |
| Usage Logs | 15 min | 24 hours |
