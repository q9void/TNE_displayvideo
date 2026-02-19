# Grafana Analytics Dashboard Setup

## Quick Setup (30 minutes)

### 1. Install Grafana

```bash
# macOS
brew install grafana

# Or with Docker
docker run -d \
  --name=grafana \
  -p 3000:3000 \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  grafana/grafana:latest
```

### 2. Start Grafana

```bash
# macOS
brew services start grafana

# Or with Docker (already running from above)
```

### 3. Access Grafana

Open http://localhost:3000
- Username: `admin`
- Password: `admin` (change on first login)

### 4. Add PostgreSQL Data Source

1. **Click** "Data Sources" → "Add data source"
2. **Select** "PostgreSQL"
3. **Configure**:
   - Name: `IDR Analytics`
   - Host: `localhost:5432` (or your DB host)
   - Database: `idr_db`
   - User: `your_db_user`
   - Password: `your_db_password`
   - TLS/SSL Mode: `disable` (or configure as needed)
4. **Click** "Save & Test"

### 5. Import Dashboard

Create a new dashboard with these panels:

#### Panel 1: Total Revenue (Big Number)
```sql
SELECT
  ROUND(SUM(platform_cut)::numeric, 2) as value
FROM win_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
```
**Visualization**: Stat
**Title**: Revenue (24h)

#### Panel 2: Revenue by Bidder (Bar Chart)
```sql
SELECT
  created_at as time,
  bidder_code as metric,
  platform_cut as value
FROM win_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
ORDER BY created_at
```
**Visualization**: Time series / Bar chart
**Title**: Revenue by Bidder

#### Panel 3: Auction Fill Rate (Graph)
```sql
SELECT
  DATE_TRUNC('minute', timestamp) as time,
  ROUND(100.0 * SUM(CASE WHEN winning_bids > 0 THEN 1 ELSE 0 END) / COUNT(*), 2) as fill_rate
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '6 hours'
GROUP BY DATE_TRUNC('minute', timestamp)
ORDER BY time
```
**Visualization**: Time series
**Title**: Fill Rate %

#### Panel 4: Bidder Performance Table
```sql
SELECT
  bidder_code as "Bidder",
  COUNT(*) as "Requests",
  SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) as "Bids",
  ROUND(100.0 * SUM(CASE WHEN had_bid THEN 1 ELSE 0 END) / COUNT(*), 1) as "Bid Rate %",
  ROUND(AVG(latency_ms)::numeric, 0) as "Avg Latency (ms)"
FROM bidder_events
WHERE created_at >= NOW() - INTERVAL '1 hour'
GROUP BY bidder_code
ORDER BY "Requests" DESC
```
**Visualization**: Table
**Title**: Bidder Performance (1h)

#### Panel 5: Auction Volume (Graph)
```sql
SELECT
  DATE_TRUNC('minute', timestamp) as time,
  COUNT(*) as auctions
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '6 hours'
GROUP BY DATE_TRUNC('minute', timestamp)
ORDER BY time
```
**Visualization**: Time series
**Title**: Auctions per Minute

#### Panel 6: Geographic Distribution (Pie Chart)
```sql
SELECT
  device_country as country,
  COUNT(*) as auctions
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '24 hours'
  AND device_country IS NOT NULL
GROUP BY device_country
ORDER BY auctions DESC
LIMIT 10
```
**Visualization**: Pie chart
**Title**: Top Countries (24h)

#### Panel 7: Average CPM by Device (Bar)
```sql
SELECT
  a.device_type as device,
  ROUND(AVG(w.adjusted_cpm)::numeric, 2) as avg_cpm
FROM auction_events a
JOIN win_events w ON a.auction_id = w.auction_id
WHERE a.timestamp >= NOW() - INTERVAL '24 hours'
GROUP BY a.device_type
```
**Visualization**: Bar gauge
**Title**: Avg CPM by Device

#### Panel 8: Real-time Event Feed (Logs)
```sql
SELECT
  timestamp,
  auction_id,
  publisher_id,
  bidders_selected as bidders,
  total_bids as bids,
  winning_bids as wins,
  ROUND(total_revenue::numeric, 2) as revenue
FROM auction_events
WHERE timestamp >= NOW() - INTERVAL '5 minutes'
ORDER BY timestamp DESC
LIMIT 50
```
**Visualization**: Table
**Title**: Recent Auctions

### 6. Set Refresh Interval

- Click dashboard settings (gear icon)
- Set "Auto refresh": `5s`, `10s`, or `30s`
- Save dashboard

### 7. Create Alerts (Optional)

Set up alerts for:
- Revenue drops below threshold
- Fill rate drops below 50%
- Bidder latency exceeds 200ms
- Error rate increases

---

## Pre-built Dashboard JSON

Want to skip manual setup? Import this JSON:

```json
{
  "dashboard": {
    "title": "TNE Analytics Dashboard",
    "panels": [
      {
        "title": "Revenue (24h)",
        "type": "stat",
        "datasource": "IDR Analytics",
        "targets": [
          {
            "rawSql": "SELECT ROUND(SUM(platform_cut)::numeric, 2) as value FROM win_events WHERE created_at >= NOW() - INTERVAL '24 hours'"
          }
        ]
      }
    ],
    "refresh": "30s",
    "time": {
      "from": "now-6h",
      "to": "now"
    }
  }
}
```

Save this to `grafana-dashboard.json` and import via Grafana UI.

---

## Docker Compose (Easy Setup)

```yaml
version: '3.8'
services:
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_INSTALL_PLUGINS=grafana-piechart-panel
    volumes:
      - grafana-data:/var/lib/grafana
    restart: unless-stopped

volumes:
  grafana-data:
```

Run: `docker-compose up -d`

---

## Benefits of Grafana

✅ **Real-time** - Updates every few seconds
✅ **Beautiful** - Professional dashboards
✅ **Alerting** - Get notified of issues
✅ **Shareable** - Share dashboards with team
✅ **Free** - Open source
