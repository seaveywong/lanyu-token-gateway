# Deploy & Rollback Runbook

## Pre-Deploy Checklist

- [ ] All CI checks green
- [ ] Database migration tested on staging
- [ ] Backup created and verified
- [ ] Change ticket approved
- [ ] Rollback plan documented

## Deploy Steps

### 1. Database Migration

```bash
# Always backup first
pg_dump -h $PGHOST -U $PGUSER -d token_gateway \
  -F custom -f pre_deploy_backup.dump

# Run migrations
cd db && goose postgres "$DATABASE_URL" up

# Verify migration applied
goose postgres "$DATABASE_URL" status
```

### 2. Data Plane (zero-downtime)

```bash
# Build new image
docker compose -f deploy/compose/docker-compose.yml build data-plane

# Drain existing connections (stop accepting new, wait for active to finish)
# Signal data-plane to stop accepting new requests
curl -X POST http://localhost:8080/admin/drain

# Wait for active SSE connections to close (max 60s)
sleep 60

# Deploy new version
docker compose -f deploy/compose/docker-compose.yml up -d data-plane

# Health check
curl http://localhost:8080/health
```

### 3. Control Plane & Async Worker

```bash
docker compose up -d control-plane async-worker
curl http://localhost:8081/health
```

### 4. Edge Worker (Cloudflare)

```bash
cd apps/edge-gateway
wrangler deploy
```

## Rollback

### Immediate rollback (same release)

```bash
# Revert Docker image
docker compose up -d --force-recreate data-plane

# OR rollback Cloudflare Worker
cd apps/edge-gateway && wrangler rollback
```

### Database rollback

```bash
cd db && goose postgres "$DATABASE_URL" down
# WARNING: goose down may drop columns/tables. 
# If the migration is destructive, restore from backup instead.
```

### Full restore from backup

```bash
pg_restore -h $PGHOST -U $PGUSER -d token_gateway \
  --clean --if-exists pre_deploy_backup.dump
```

## Post-Deploy Verification

- [ ] `/health` returns 200 on all services
- [ ] `/v1/models` returns model list
- [ ] SSE connection works end-to-end
- [ ] Admin console loads
- [ ] Customer portal loads
- [ ] No alerts firing
- [ ] Error rate within SLO
