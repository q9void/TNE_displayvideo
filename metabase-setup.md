# Metabase Analytics Setup (Simplest Option)

## Why Metabase?

- **Easiest to use** - No SQL required for basic queries
- **Fast setup** - 5 minutes to first dashboard
- **Beautiful** - Great looking dashboards out of the box
- **Self-service** - Non-technical people can create reports

## Quick Setup

### 1. Run Metabase (Docker - Easiest)

```bash
docker run -d -p 3001:3000 \
  --name metabase \
  -e MB_DB_FILE=/metabase-data/metabase.db \
  -v ~/metabase-data:/metabase-data \
  metabase/metabase:latest
```

**Wait 2-3 minutes** for Metabase to start up.

### 2. Access Metabase

Open http://localhost:3001

**First-time setup**:
1. Create admin account (email/password)
2. Skip "Add your data" for now
3. Click "I'll add my data later"

### 3. Add Database Connection

1. Go to **Settings** (gear icon) → **Admin** → **Databases**
2. Click **Add database**
3. Select **PostgreSQL**
4. Fill in:
   - **Name**: `Analytics`
   - **Host**: `host.docker.internal` (if using Docker) or `localhost`
   - **Port**: `5432`
   - **Database name**: `idr_db`
   - **Username**: `your_db_user`
   - **Password**: `your_db_password`
5. Click **Save**

### 4. Auto-Generated Questions

Metabase will automatically suggest questions like:
- "How many auction events are there?"
- "What's the average total revenue?"
- "Show me win events grouped by bidder code"

**Just click them** to see instant results!

### 5. Create Custom Questions (No SQL!)

#### Question 1: Revenue by Bidder
1. Click **New** → **Question**
2. Select **Analytics** database → **Win Events** table
3. **Summarize** by `Sum of platform_cut`
4. **Group by** `bidder_code`
5. **Filter** by `created_at` is in the `last 24 hours`
6. **Visualize** as **Bar chart**
7. Save as "Revenue by Bidder (24h)"

#### Question 2: Auction Volume Over Time
1. **New Question** → **Win Events**
2. **Count** of rows
3. **Group by** `created_at` → by **Hour**
4. **Filter** `created_at` in the `last 7 days`
5. **Visualize** as **Line chart**
6. Save as "Auction Volume"

#### Question 3: Bidder Performance
1. **New Question** → **Bidder Events**
2. **Count** of rows
3. **Group by** `bidder_code`
4. **Add** `Sum of latency_ms`
5. **Filter** `created_at` in the `last 1 hour`
6. **Visualize** as **Table**
7. Save as "Bidder Performance"

### 6. Create Dashboard

1. Click **New** → **Dashboard**
2. Name it "Analytics Dashboard"
3. Click **Add question**
4. Select your saved questions
5. Arrange them nicely
6. Click **Save**

### 7. Auto-Refresh

1. Open your dashboard
2. Click **⋯** (three dots) → **Auto-refresh**
3. Set to **1 minute** or **5 minutes**

---

## Pre-Built Questions (With SQL)

If you want more advanced queries, use **Native Query** (SQL):

### Top Publishers by Revenue
```sql
SELECT
  publisher_id,
  COUNT(*) as auctions,
  ROUND(SUM(total_revenue), 2) as revenue
FROM auction_events
WHERE timestamp >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY publisher_id
ORDER BY revenue DESC
LIMIT 10
```

### Bidder Win Rate
```sql
SELECT
  b.bidder_code,
  COUNT(*) as requests,
  COUNT(w.bid_id) as wins,
  ROUND(100.0 * COUNT(w.bid_id) / COUNT(*), 2) as win_rate
FROM bidder_events b
LEFT JOIN win_events w ON b.auction_id = w.auction_id AND b.bidder_code = w.bidder_code
WHERE b.created_at >= NOW() - INTERVAL '24 hours'
GROUP BY b.bidder_code
ORDER BY requests DESC
```

### Hourly Revenue Trend
```sql
SELECT
  DATE_TRUNC('hour', created_at) as hour,
  ROUND(SUM(platform_cut), 2) as revenue
FROM win_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour
```

---

## Metabase Features

### 1. Email Reports
- Set up daily/weekly email reports
- Automatically send dashboards to team

### 2. Slack Integration
- Post reports to Slack channels
- Get alerts on metrics

### 3. Drill-Down
- Click any chart to drill into details
- Explore data interactively

### 4. Filtering
- Add dashboard filters
- Let users filter by date, publisher, bidder, etc.

### 5. Mobile App
- View dashboards on iPhone/Android
- Monitor analytics on the go

---

## Docker Compose (Production)

```yaml
version: '3.8'
services:
  metabase:
    image: metabase/metabase:latest
    ports:
      - "3001:3000"
    environment:
      MB_DB_TYPE: postgres
      MB_DB_DBNAME: metabase
      MB_DB_PORT: 5432
      MB_DB_USER: metabase
      MB_DB_PASS: metabase_password
      MB_DB_HOST: postgres
    depends_on:
      - postgres
    restart: unless-stopped

  postgres:
    image: postgres:14
    environment:
      POSTGRES_USER: metabase
      POSTGRES_DB: metabase
      POSTGRES_PASSWORD: metabase_password
    volumes:
      - metabase-data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  metabase-data:
```

---

## Comparison: Metabase vs Grafana

| Feature | Metabase | Grafana |
|---------|----------|---------|
| Ease of Use | ⭐⭐⭐⭐⭐ Easiest | ⭐⭐⭐ Moderate |
| No SQL Required | ✅ Yes | ❌ No |
| Real-time | ⭐⭐⭐ Good | ⭐⭐⭐⭐⭐ Excellent |
| Alerting | ⭐⭐⭐ Basic | ⭐⭐⭐⭐⭐ Advanced |
| Visual Appeal | ⭐⭐⭐⭐⭐ Beautiful | ⭐⭐⭐⭐ Professional |
| Best For | Business users | DevOps/Technical users |

**Recommendation**: Start with **Metabase** for simplicity, add **Grafana** later for real-time monitoring.
