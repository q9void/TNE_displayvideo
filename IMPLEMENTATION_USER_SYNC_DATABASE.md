# Database-Backed User Sync Implementation

## Summary

Implemented persistent storage for cookie sync UIDs in PostgreSQL database instead of relying solely on HTTP cookies. This provides:

- **Persistent UIDs** - survive browser restarts and cookie clearing
- **Reliable sync tracking** - single source of truth for all bidder UIDs
- **Better targeting** - UIDs are consistently passed to bidders in OpenRTB requests
- **Trackable** - can monitor sync rates, expiration, and usage

## Changes Made

### 1. Database Migration (010_user_syncs.sql)
Created new `user_syncs` table:
- Stores FPID → bidder_code → bidder_uid mappings
- Tracks sync timestamps, expiration, last usage
- Supports UID refresh when bidders rotate IDs

### 2. Storage Layer (internal/storage/usersync.go)
New `UserSyncStore` with methods:
- `UpsertSync()` - create/update sync record (called from /cookie_sync)
- `UpdateUID()` - update bidder UID (called from /setuid callback)
- `GetSyncsForUser()` - retrieve all UIDs for a given FPID (called from /v1/bid)
- `MarkUsed()` - track when UIDs are used in bids
- `DeleteExpired()` - cleanup expired syncs
- `DeleteStale()` - cleanup old unused syncs

### 3. Cookie Sync Handler (internal/endpoints/cookie_sync.go)
- Stores sync records in database when FPID is available
- Creates records with NULL UID (waiting for bidder callback)
- Sets 90-day expiration by default

### 4. SetUID Handler (internal/endpoints/setuid.go)
- Updates database with bidder UID when callback fires
- Handles UID refresh if bidder returns new UID
- Validates GDPR consent before storing

### 5. Catalyst Bid Handler (internal/endpoints/catalyst_bid_handler.go)
**NEW UID Loading Priority:**
1. **Database** (most reliable) - loaded via FPID
2. **Request body** (SDK-provided) - overrides database
3. **HTTP cookie** (fallback) - least reliable

UIDs are now included in `user.ext.eids` array in OpenRTB requests sent to bidders.

### 6. Server Initialization (cmd/server/server.go)
- Creates `UserSyncStore` instance
- Passes it to all relevant handlers

## Deployment Steps

### 1. Run Migration on Server
```bash
ssh catalyst
cd ~/catalyst
psql -h catalyst-postgres -U catalyst_prod -d catalyst_production < deployment/migrations/010_user_syncs.sql
```

### 2. Recompile and Deploy
```bash
# Local machine
cd ~/tnevideo
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-linux ./cmd/server
scp server-linux catalyst:~/server-test

# On server
ssh catalyst
pkill -f server-test
cd ~/catalyst
LOG_LEVEL=debug CORS_ALLOWED_ORIGINS="*" CORS_ENABLED=true \
  DB_HOST=catalyst-postgres DB_PORT=5432 \
  DB_USER=catalyst_prod DB_PASSWORD=ttlJRsJK7myCehgKyswnZP82v6L57xT5 \
  DB_NAME=catalyst_production IDR_ENABLED=false PBS_PORT=8001 \
  ~/server-test > /tmp/catalyst-test.log 2>&1 &
```

### 3. Verify

After cookie sync:
```sql
SELECT * FROM user_syncs WHERE fpid = 'fpi_1234...';
```

Should show records like:
```
| fpid           | bidder_code | bidder_uid      | synced_at           | updated_at          |
|----------------|-------------|-----------------|---------------------|---------------------|
| fpi_1234...    | rubicon     | ABC123          | 2026-02-14 04:00:00 | 2026-02-14 04:00:05 |
| fpi_1234...    | kargo       | XYZ789          | 2026-02-14 04:00:00 | 2026-02-14 04:00:06 |
```

Check bid request logs:
```bash
tail -f /tmp/catalyst-test.log | grep "Loaded user syncs from database"
```

Should see: `"syncs_loaded": 5, "bidders": ["rubicon", "kargo", "sovrn", ...]`

## How It Works

### Cookie Sync Flow
1. Browser calls `/cookie_sync` with FPID
2. Server creates records in `user_syncs` table (UID = NULL)
3. Returns sync URLs to browser
4. Browser loads sync pixels
5. Bidders redirect to `/setuid?bidder=X&uid=Y`
6. Server updates `user_syncs` with actual UID

### Bid Request Flow
1. SDK sends `/v1/bid` with FPID
2. Server queries `user_syncs` WHERE fpid = ?
3. Loads all bidder UIDs from database
4. Includes UIDs in `user.ext.eids` array
5. Sends OpenRTB request to each bidder with their UID

### UID Refresh
If a bidder returns a different UID on subsequent sync:
- Database record is UPDATED with new UID
- Old UID is replaced (not appended)
- Expiration is reset to 90 days

## Maintenance

### Cleanup Expired Syncs
Run periodically (e.g., daily cron):
```sql
DELETE FROM user_syncs WHERE expires_at < NOW();
```

Or use the store method:
```go
deleted, err := userSyncStore.DeleteExpired(ctx)
```

### Cleanup Stale Syncs
Remove syncs not used in 180 days:
```go
deleted, err := userSyncStore.DeleteStale(ctx, 180*24*time.Hour)
```

## Benefits Over Cookie-Only Approach

| Feature | HTTP Cookies | Database |
|---------|-------------|----------|
| Persistence | Cleared easily | Permanent |
| Cross-device | No | Yes (via FPID) |
| Trackability | Limited | Full audit trail |
| Reliability | Subject to browser restrictions | Always available |
| Expiration | Browser-dependent | Server-controlled |
| Debugging | Difficult | SQL queries |

## Files Modified

- `deployment/migrations/010_user_syncs.sql` (NEW)
- `internal/storage/usersync.go` (NEW)
- `internal/endpoints/cookie_sync.go`
- `internal/endpoints/setuid.go`
- `internal/endpoints/catalyst_bid_handler.go`
- `cmd/server/server.go`
