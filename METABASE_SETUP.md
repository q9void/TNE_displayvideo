# Metabase Setup Complete! 🎉

## Access Metabase

**URL**: http://localhost:3002

**Status**: Running (initializing - give it 1-2 minutes on first startup)

## First-Time Setup Steps

### 1. Create Admin Account
Open http://localhost:3002 and you'll see the setup wizard:
- **Email**: your@email.com
- **Password**: (choose a secure password)
- **Company/Team Name**: TNE Video

### 2. Connect to Analytics Database

Click **Add a database** and enter:
- **Database type**: PostgreSQL
- **Display name**: Analytics
- **Host**: `host.docker.internal` (or `localhost`)
- **Port**: `5432`
- **Database name**: `idr_db`
- **Database username**: `andrewstreets`
- **Database password**: (leave empty if no password)

Click **Save** and Metabase will test the connection.

### 3. Explore Your Data

Metabase will automatically detect your tables:
- ✅ `auction_events` - Auction-level analytics
- ✅ `bidder_events` - Per-bidder performance (for ML)
- ✅ `win_events` - Revenue tracking

### 4. Create Your First Dashboard

#### Quick Question 1: Revenue by Bidder (24h)
1. Click **New** → **Question**
2. Select **Analytics** → **Win Events**
3. **Summarize**: Sum of `platform_cut`
4. **Group by**: `bidder_code`
5. **Filter**: `created_at` is in the **last 24 hours**
6. **Visualize**: Bar chart
7. Save as "Revenue by Bidder (24h)"

#### Quick Question 2: Auction Volume Over Time
1. **New Question** → **Win Events**
2. **Count** of rows
3. **Group by**: `created_at` → by **Hour**
4. **Filter**: `created_at` in the **last 7 days**
5. **Visualize**: Line chart
6. Save as "Auction Volume"

#### Quick Question 3: Bidder Performance Table
1. **New Question** → **Bidder Events**
2. **Summarize**: Count, Average `latency_ms`
3. **Group by**: `bidder_code`
4. **Filter**: `created_at` in the **last 1 hour**
5. **Visualize**: Table
6. Save as "Bidder Performance"

### 5. Create a Dashboard

1. Click **New** → **Dashboard**
2. Name it "TNE Analytics Dashboard"
3. Click **Add question** and select your saved questions
4. Arrange them in a nice layout
5. **Save**

### 6. Enable Auto-Refresh

1. Open your dashboard
2. Click **⋯** (three dots) → **Auto-refresh**
3. Set to **1 minute** or **5 minutes**
4. Save

## Pre-Built SQL Queries

For more advanced queries, use **Native Query** (SQL mode):

### Top Publishers by Revenue
```sql
SELECT
  publisher_id,
  COUNT(*) as auctions,
  ROUND(SUM(total_revenue)::numeric, 2) as revenue
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
  ROUND(SUM(platform_cut)::numeric, 2) as revenue
FROM win_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour
```

## Managing Metabase

### Stop Metabase
```bash
docker stop metabase
```

### Start Metabase (after stopping)
```bash
docker start metabase
```

### View Logs
```bash
docker logs metabase -f
```

### Remove Metabase (complete cleanup)
```bash
docker stop metabase
docker rm metabase
rm -rf ~/metabase-data
```

## Troubleshooting

### Can't connect to database?
- Make sure PostgreSQL is running: `psql -U andrewstreets -d idr_db -c "SELECT 1"`
- Try host `localhost` instead of `host.docker.internal`
- Check if database user has a password

### Metabase won't load?
- Wait 2-3 minutes on first startup (it's setting up its own database)
- Check logs: `docker logs metabase --tail 50`
- Restart: `docker restart metabase`

### Port already in use?
The container is running on port 3002 (3001 was in use). If you need to change it:
```bash
docker stop metabase
docker rm metabase
docker run -d -p 3003:3000 --name metabase \
  -v ~/metabase-data:/metabase-data \
  metabase/metabase:latest
```

## What's Next?

1. ✅ **Metabase is running** - Access at http://localhost:3002
2. ⏳ **Complete setup wizard** (2 minutes)
3. ⏳ **Connect to analytics database**
4. ⏳ **Create your first dashboard** (10 minutes)
5. ⏳ **Set auto-refresh** for real-time updates

## Sample Data Available

Your database already has sample data to test with:
- 3 demo auctions
- 5 bidder events (Rubicon, AppNexus, Pubmatic, Kargo)
- 4 win events with revenue data

You can see this data immediately once you connect the database!

---

**Need help?** Check the full guide at `/Users/andrewstreets/tnevideo/metabase-setup.md`

**Pro Tip**: Once you have real auction data flowing through your PBS server (after IDR service is updated), all these dashboards will show live data automatically!
