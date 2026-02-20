# Database Data Persistence & Recovery Guide

## ✅ Your Data is Safe - Here's How

### Docker Volume Configuration

Your PostgreSQL database is configured for **full data persistence** using Docker named volumes:

```yaml
# From docker-compose.yml lines 52-53
volumes:
  - postgres-data:/var/lib/postgresql/data

# Volume declaration (lines 204-205)
volumes:
  postgres-data:
    driver: local
```

## What This Means

### ✅ Data Survives:
- ✅ Container restarts (`docker restart catalyst-postgres`)
- ✅ Container crashes/failures
- ✅ `docker-compose restart`
- ✅ `docker-compose down` (without `-v` flag)
- ✅ Server reboots
- ✅ Docker daemon restarts
- ✅ Container upgrades/rebuilds

### ❌ Data is ONLY Lost If:
- ❌ You explicitly delete the volume: `docker volume rm postgres-data`
- ❌ You use `docker-compose down -v` (the `-v` flag removes volumes)
- ❌ Physical disk failure (but you have backups!)

## Data Recovery Scenarios

### Scenario 1: Container Crashes or is Deleted

**What happens:**
```bash
# Container dies or is deleted
docker rm -f catalyst-postgres
```

**Recovery:**
```bash
# Just recreate the container - data is preserved!
docker-compose up -d postgres

# The volume automatically remounts with all your data intact
```

**Result:** ✅ All data preserved, zero data loss

---

### Scenario 2: Complete Docker Compose Down

**What happens:**
```bash
# Stop all services
docker-compose down
```

**Recovery:**
```bash
# Restart services
docker-compose up -d

# Volumes are preserved and remounted automatically
```

**Result:** ✅ All data preserved, zero data loss

---

### Scenario 3: Server Reboot

**What happens:**
- Server loses power or is rebooted
- All containers stop

**Recovery:**
```bash
# After reboot, restart Docker Compose
docker-compose up -d

# Services restart with all data intact
```

**Result:** ✅ All data preserved, zero data loss

---

### Scenario 4: Accidental Volume Deletion

**What happens:**
```bash
# Worst case: someone deletes the volume
docker volume rm postgres-data
```

**Recovery:**
```bash
# Restore from automated backup
./deployment/restore-postgres.sh <backup-file>

# Or restore from S3 backup
./deployment/restore-postgres.sh s3
```

**Result:** ✅ Data restored from backup (may lose up to 24 hours depending on backup schedule)

---

## Backup System

You have **three layers of backup protection**:

### Layer 1: Docker Volume (Primary Storage)
- Location: `/var/lib/docker/volumes/postgres-data/_data`
- Persistence: Survives container restarts/crashes
- Access: `docker volume inspect postgres-data`

### Layer 2: Local Automated Backups
- Location: Docker volume `backup-data`
- Schedule: Daily at 2 AM (configurable via `BACKUP_CRON`)
- Retention:
  - Daily: 7 days
  - Weekly: 4 weeks
  - Monthly: 3 months
- Container: `catalyst-backup`

### Layer 3: S3 Remote Backups (Optional)
- Location: AWS S3 bucket (configured via `.env`)
- Schedule: Same as local backups
- Retention: Same as local backups
- Encryption: At-rest encryption enabled

## Verifying Data Persistence

### Check Volume Exists
```bash
docker volume ls | grep postgres-data
```

Expected output:
```
local     postgres-data
```

### Check Volume Location
```bash
docker volume inspect postgres-data | grep Mountpoint
```

Expected output:
```
"Mountpoint": "/var/lib/docker/volumes/postgres-data/_data"
```

### Check Data Size
```bash
docker exec catalyst-postgres du -sh /var/lib/postgresql/data
```

### Verify Database Contents After Restart
```bash
# Stop container
docker-compose stop postgres

# Start container
docker-compose start postgres

# Verify data is still there
docker exec catalyst-postgres psql -U catalyst_prod -d catalyst_production -c "SELECT COUNT(*) FROM accounts;"
```

Expected: Should return the same count as before restart

## Manual Backup Commands

### Create Manual Backup
```bash
# Full database backup
docker exec catalyst-postgres pg_dump -U catalyst_prod -d catalyst_production > backup-$(date +%Y%m%d-%H%M%S).sql

# Compressed backup
docker exec catalyst-postgres pg_dump -U catalyst_prod -d catalyst_production | gzip > backup-$(date +%Y%m%d-%H%M%S).sql.gz
```

### Restore from Backup
```bash
# Stop application (prevent writes during restore)
docker-compose stop catalyst

# Restore database
cat backup-20260214-120000.sql | docker exec -i catalyst-postgres psql -U catalyst_prod -d catalyst_production

# Or from compressed backup
gunzip -c backup-20260214-120000.sql.gz | docker exec -i catalyst-postgres psql -U catalyst_prod -d catalyst_production

# Restart application
docker-compose start catalyst
```

## Migration Data Safety

When running the new schema migration (`009_create_new_schema.sql`):

### ✅ Safe Operations:
- `CREATE TABLE IF NOT EXISTS` - Won't affect existing tables
- `CREATE INDEX` - Adds indexes, doesn't delete data
- No `DROP TABLE` or `DELETE` statements
- Old tables (`publishers`, `bidders`) remain untouched

### Migration Rollback Plan:
If you need to rollback the migration:

1. Old schema still works (tables not dropped)
2. Code falls back to legacy methods if new tables don't exist
3. Can restore from backup taken before migration

## Best Practices

### Before Major Changes:
```bash
# 1. Create manual backup
./deployment/backup-postgres.sh

# 2. Verify backup exists
ls -lh /var/lib/docker/volumes/backup-data/_data/

# 3. Make changes
# 4. Test thoroughly
# 5. Keep backup for 7+ days
```

### Regular Monitoring:
```bash
# Check backup container is running
docker ps | grep catalyst-backup

# View backup logs
docker logs catalyst-backup

# List recent backups
docker exec catalyst-backup ls -lh /backups/daily/
```

### Disaster Recovery Checklist:
1. ✅ Docker volume exists: `docker volume ls | grep postgres-data`
2. ✅ Backup container running: `docker ps | grep backup`
3. ✅ Recent backups available: `docker exec catalyst-backup ls /backups/daily/`
4. ✅ S3 backups configured (optional): Check `.env` for S3 settings
5. ✅ Database accessible: `docker exec catalyst-postgres pg_isready`

## Emergency Contacts

If you experience data loss:
1. **DO NOT** run `docker volume rm` or `docker-compose down -v`
2. **DO NOT** restart services until you assess the situation
3. Check Docker volume: `docker volume inspect postgres-data`
4. Check backups: `docker exec catalyst-backup ls /backups/`
5. Contact support with Docker logs: `docker logs catalyst-postgres`

## Summary

**Your database configuration is enterprise-grade and production-ready:**

✅ Docker named volumes provide persistent storage
✅ Data survives all normal container operations
✅ Automated daily backups to local storage
✅ Optional S3 remote backups for disaster recovery
✅ 3-layer backup protection (volume + local + S3)
✅ No data loss from container crashes or restarts
✅ Easy recovery from backups if needed

**Your data is safe! 🛡️**
