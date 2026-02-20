# Fix for Publisher "12345" - Zero Bids & HTTP 400 Errors

## Problem Summary

Publisher ID "12345" is being sent by TotalProSports client but is not properly configured in the database, causing:
- ✅ HTTP 200 responses but **0 bids** (no bidder params found)
- ❌ HTTP 400 errors after ~3 minutes (database lookup failures/timeouts)

## Root Cause

The client SDK sends `accountId: "12345"`, but:
1. Database doesn't have publisher_id='12345' configured
2. OR publisher exists but has empty `bidder_params: {}`
3. Static mapping file (`config/bizbudding-all-bidders-mapping.json`) has config under "icisic-media" instead

## Solution

Add publisher "12345" to the database with proper bidder configuration.

---

## Quick Fix (Copy-Paste Commands)

### Option 1: Using Docker (Local/Dev)

```bash
# 1. Start Docker if not running
docker compose -f deployment/docker-compose.yml up -d

# 2. Wait a few seconds for PostgreSQL to be ready
sleep 5

# 3. Run the SQL script
docker exec -i catalyst-postgres psql -U catalyst -d catalyst < deployment/add_publisher_12345.sql

# 4. Verify it worked
docker exec -i catalyst-postgres psql -U catalyst -d catalyst -c \
  "SELECT publisher_id, name, status, jsonb_pretty(bidder_params) FROM publishers WHERE publisher_id = '12345';"
```

### Option 2: Using Management Script

```bash
cd deployment

./manage-publishers.sh add '12345' 'Total Pro Sports' \
  'totalprosports.com,dev.totalprosports.com,*.totalprosports.com' \
  '{
    "rubicon": {"accountId": 26298, "siteId": 556630, "zoneId": 3767186},
    "kargo": {"placementId": "_o9n8eh8Lsw"},
    "sovrn": {"tagid": "1294952"},
    "pubmatic": {"publisherId": "166938", "adSlot": "7079290"},
    "triplelift": {"inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"}
  }'
```

### Option 3: Direct SQL (Production)

```bash
# Connect to production database and run the SQL
PGPASSWORD='your-prod-password' psql \
  -h your-prod-host \
  -U catalyst_prod \
  -d catalyst_production \
  -f deployment/add_publisher_12345.sql
```

---

## Verification

After adding the publisher, check the logs again. You should see:

### Before (Broken):
```
[Catalyst] Received 0 bids in 411ms
POST https://ads.thenexusengine.com/v1/bid 400 (Bad Request)
```

### After (Fixed):
```
[Catalyst] Received 5 bids in 411ms
[Catalyst] Bid response: rubicon $2.50, kargo $1.80, pubmatic $2.10...
```

### Manual Verification Query:

```sql
-- Check if publisher exists with bidder params
SELECT
    publisher_id,
    name,
    status,
    allowed_domains,
    jsonb_pretty(bidder_params) as bidder_config
FROM publishers
WHERE publisher_id = '12345';
```

**Expected result:**
```
 publisher_id |      name         | status | allowed_domains                              | bidder_config
--------------+-------------------+--------+----------------------------------------------+------------------
 12345        | Total Pro Sports  | active | totalprosports.com,dev.totalprosports.com,.. | {
              |                   |        |                                              |   "rubicon": {...},
              |                   |        |                                              |   "kargo": {...},
              |                   |        |                                              |   ...
              |                   |        |                                              | }
```

---

## What This Fixes

1. **Zero Bids Issue**:
   - ✅ Bidder params will now be loaded from database
   - ✅ Bidders receive proper accountId/siteId/placementId
   - ✅ Bidders can now bid on inventory

2. **HTTP 400 Errors**:
   - ✅ Database lookups succeed (publisher found)
   - ✅ No repeated timeout failures
   - ✅ PublisherAuth middleware passes requests through

3. **Performance**:
   - ✅ Faster bid responses (no fallback chain needed)
   - ✅ Reduced database load (cached config)

---

## Configuration Details

### Bidder Parameters for Publisher "12345":

```json
{
  "rubicon": {
    "accountId": 26298,
    "siteId": 556630,
    "zoneId": 3767186,
    "bidonmultiformat": false
  },
  "kargo": {
    "placementId": "_o9n8eh8Lsw"
  },
  "sovrn": {
    "tagid": "1294952"
  },
  "pubmatic": {
    "publisherId": "166938",
    "adSlot": "7079290"
  },
  "triplelift": {
    "inventoryCode": "BizBudding_RON_NativeFlex_pbc2s"
  }
}
```

### Allowed Domains:
- `totalprosports.com`
- `dev.totalprosports.com`
- `*.totalprosports.com` (wildcard)

---

## Future: When Going Live

When client is ready to use a proper publisher ID:

1. **Create new publisher** with proper ID:
   ```bash
   ./manage-publishers.sh add 'totalprosports-prod' 'Total Pro Sports' \
     'totalprosports.com' '{...same bidder params...}'
   ```

2. **Update client SDK** to use new ID:
   ```javascript
   catalyst.init({
     accountId: 'totalprosports-prod',  // Updated!
     serverUrl: 'https://ads.thenexusengine.com'
   });
   ```

3. **Keep "12345" active** temporarily during transition, then deactivate:
   ```bash
   ./manage-publishers.sh update '12345' status 'inactive'
   ```

---

## Troubleshooting

### Still Getting 0 Bids After Adding Publisher?

1. **Clear PublisherAuth cache** (30-second TTL, so wait 30 seconds)
2. **Check bidder-specific errors** in server logs
3. **Verify domain matches**: Client domain must match `allowed_domains`

### Still Getting HTTP 400 Errors?

1. **Check server logs** for actual error message:
   ```bash
   tail -f /var/log/catalyst/app.log | grep "400"
   ```

2. **Verify database connection**:
   ```bash
   docker exec catalyst-postgres psql -U catalyst -d catalyst -c "SELECT 1;"
   ```

3. **Check connection pool** isn't exhausted:
   ```sql
   SELECT count(*) FROM pg_stat_activity WHERE datname = 'catalyst';
   -- Should be < 100 (max_connections limit)
   ```

---

## Files Referenced

- SQL script: `/Users/andrewstreets/tnevideo/deployment/add_publisher_12345.sql`
- Management script: `/Users/andrewstreets/tnevideo/deployment/manage-publishers.sh`
- Static mapping: `/Users/andrewstreets/tnevideo/config/bizbudding-all-bidders-mapping.json`
- Migration schema: `/Users/andrewstreets/tnevideo/deployment/migrations/001_create_publishers_table.sql`

---

**Created:** 2026-02-12
**Status:** Ready to apply
**Priority:** HIGH (blocking production bids)
