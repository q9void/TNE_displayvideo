# Publisher ID '12345' Database Configuration Check

## Summary

I've investigated the database configuration for `publisher_id='12345'` to verify if publisher-level bidder params exist.

## Current Status: ⚠️ UNABLE TO VERIFY (Database Not Accessible)

The PostgreSQL database is not currently accessible because:
1. Docker daemon is not running
2. Production database (`postgres` host) is not accessible from this environment

## Database Structure Found

### Publishers Table Schema
The database has a well-structured publishers table with the following key fields:

```sql
CREATE TABLE publishers (
    id UUID PRIMARY KEY,
    publisher_id VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    allowed_domains TEXT NOT NULL,
    bidder_params JSONB DEFAULT '{}'::jsonb,  -- ← Publisher-level bidder params
    bid_multiplier DECIMAL(10,4) DEFAULT 1.0,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### Key Finding: `bidder_params` Column Exists

The `bidder_params` column is a JSONB field designed to store bidder-specific configuration at the publisher level.

**Example format:**
```json
{
  "rubicon": {
    "accountId": 26298,
    "siteId": 556630,
    "zoneId": 3767186
  },
  "pubmatic": {
    "publisherId": "12345",
    "adSlot": "slot1"
  },
  "appnexus": {
    "placementId": 54321
  }
}
```

## Database Access Methods

### Method 1: Local Docker (Recommended)
```bash
# Start Docker and PostgreSQL
docker compose up -d

# Run the check script
./check_publisher_12345.sh

# Or use the management script
./deployment/manage-publishers.sh check 12345
```

### Method 2: Production Database
```bash
# Connect to production database
PGPASSWORD='ttlJRsJK7myCehgKyswnZP82v6L57xT5' \
  psql -h <production-host> -U catalyst_prod -d catalyst_production \
  -c "SELECT publisher_id, name, bidder_params, status FROM publishers WHERE publisher_id = '12345';"
```

### Method 3: Using Management Script
```bash
cd /Users/andrewstreets/tnevideo/deployment
./manage-publishers.sh check 12345
```

## What to Look For

When you run the database query, check:

1. **Does publisher_id='12345' exist?**
   - If NO: Publisher needs to be added
   - If YES: Proceed to next check

2. **Is `bidder_params` empty (`{}`) or populated?**
   - If `{}`: Publisher-level bidder params DO NOT exist (needs configuration)
   - If populated: Publisher-level bidder params EXIST

3. **What is the status?**
   - Should be `'active'` for the publisher to work

## Possible Scenarios

### Scenario A: Publisher Not Found
```
publisher_id | name | bidder_params | status
-------------|------|---------------|-------
(0 rows)
```

**Action Required:** Add the publisher
```bash
./deployment/manage-publishers.sh add '12345' 'Publisher Name' 'allowed-domain.com' \
  '{"rubicon":{"accountId":26298,"siteId":556630},"appnexus":{"placementId":12345}}'
```

### Scenario B: Publisher Exists, But No Bidder Params
```
publisher_id | name              | bidder_params | status
-------------|-------------------|---------------|--------
12345        | Some Publisher    | {}            | active
```

**Action Required:** Add bidder params
```bash
./deployment/manage-publishers.sh update '12345' bidder_params \
  '{"rubicon":{"accountId":26298,"siteId":556630},"appnexus":{"placementId":12345}}'
```

### Scenario C: Publisher Exists with Bidder Params ✓
```
publisher_id | name              | bidder_params                                    | status
-------------|-------------------|--------------------------------------------------|--------
12345        | Some Publisher    | {"rubicon":{"accountId":26298,"siteId":556630}} | active
```

**Result:** Publisher-level bidder params EXIST - No action required

## Tools Created

### 1. Check Script: `/Users/andrewstreets/tnevideo/check_publisher_12345.sh`
```bash
./check_publisher_12345.sh
```
This script will:
- Check if PostgreSQL is running
- Query for publisher_id='12345'
- Display bidder params in a readable format
- Indicate if action is needed

### 2. Management Script: `/Users/andrewstreets/tnevideo/deployment/manage-publishers.sh`
Full-featured script for managing publishers:
```bash
# List all publishers
./deployment/manage-publishers.sh list

# Check specific publisher
./deployment/manage-publishers.sh check 12345

# Add new publisher
./deployment/manage-publishers.sh add <id> <name> <domains> [bidder_params]

# Update bidder params
./deployment/manage-publishers.sh update <id> bidder_params '{...}'
```

## Database Locations

### Local/Development
- **Container:** `catalyst-postgres`
- **Database:** `catalyst`
- **User:** `catalyst`

### Production
- **Host:** `postgres` (in production environment)
- **Database:** `catalyst_production`
- **User:** `catalyst_prod`
- **Password:** (in `/Users/andrewstreets/tnevideo/deployment/.env.production`)

## Migration Files

The schema is defined in:
- `/Users/andrewstreets/tnevideo/deployment/migrations/001_create_publishers_table.sql`

Additional related tables:
- `/Users/andrewstreets/tnevideo/deployment/migrations/002_create_bidders_table.sql`
- `/Users/andrewstreets/tnevideo/deployment/migrations/005_create_domain_bidder_configs.sql`
- `/Users/andrewstreets/tnevideo/deployment/migrations/006_create_unit_bidder_configs.sql`

## Next Steps

1. **Start Docker** (if using local environment)
   ```bash
   # Start Docker Desktop or Docker daemon
   docker compose -f /Users/andrewstreets/tnevideo/deployment/docker-compose.yml up -d
   ```

2. **Run the check script**
   ```bash
   /Users/andrewstreets/tnevideo/check_publisher_12345.sh
   ```

3. **Based on results**, take appropriate action:
   - Add publisher if missing
   - Add bidder params if empty
   - Verify configuration if present

## References

- Publishers table schema: `/Users/andrewstreets/tnevideo/deployment/migrations/001_create_publishers_table.sql`
- Management script: `/Users/andrewstreets/tnevideo/deployment/manage-publishers.sh`
- Publisher struct: `/Users/andrewstreets/tnevideo/internal/storage/publishers.go`
- Check script: `/Users/andrewstreets/tnevideo/check_publisher_12345.sh`

---

**Created:** 2026-02-12
**Status:** Awaiting database access to verify publisher_id='12345' configuration
