#!/bin/bash
# Test script for database-backed user sync flow

set -e

echo "=== Testing Database-Backed User Sync Flow ==="
echo ""

# Test FPID
FPID="fpi_test_$(date +%s)"

echo "1. Testing cookie sync with FPID: $FPID"
curl -X POST https://ads.thenexusengine.com/cookie_sync \
  -H 'Content-Type: application/json' \
  -d "{\"fpid\": \"$FPID\", \"bidders\": [\"rubicon\", \"pubmatic\"], \"limit\": 5}" \
  -s | jq .

echo ""
echo "2. Checking database for sync records..."
ssh catalyst "docker exec -i catalyst-postgres psql -U catalyst_prod -d catalyst_production -c \"SELECT fpid, bidder_code, bidder_uid, synced_at FROM user_syncs WHERE fpid = '$FPID';\""

echo ""
echo "3. Simulating setuid callback for rubicon..."
curl "https://ads.thenexusengine.com/setuid?bidder=rubicon&uid=RUBICON_UID_12345&gdpr=0" -s > /dev/null

echo ""
echo "4. Checking if UID was stored in database..."
ssh catalyst "docker exec -i catalyst-postgres psql -U catalyst_prod -d catalyst_production -c \"SELECT fpid, bidder_code, bidder_uid, updated_at FROM user_syncs WHERE fpid = '$FPID';\""

echo ""
echo "5. Testing bid request with this FPID (should load UIDs from database)..."
echo "   Watch server logs for: 'Loaded user syncs from database'"
ssh catalyst "tail -f /tmp/catalyst-test.log | grep --line-buffered 'Loaded user syncs from database' &"
TAIL_PID=$!

curl -X POST https://ads.thenexusengine.com/v1/bid \
  -H 'Content-Type: application/json' \
  -d "{
    \"accountId\": \"icisic-media\",
    \"timeout\": 2800,
    \"slots\": [{
      \"divId\": \"test-ad\",
      \"sizes\": [[300,250]],
      \"enabled_bidders\": [\"catalyst\"]
    }],
    \"page\": {
      \"url\": \"https://test.com\",
      \"domain\": \"test.com\"
    },
    \"device\": {
      \"width\": 1920,
      \"height\": 1080,
      \"deviceType\": \"desktop\"
    },
    \"user\": {
      \"fpid\": \"$FPID\"
    }
  }" -s | jq .

kill $TAIL_PID 2>/dev/null || true

echo ""
echo "=== Test Complete ==="
echo "The UID flow is working if you saw:"
echo "  ✓ Sync records created in database"
echo "  ✓ UID updated after setuid callback"
echo "  ✓ 'Loaded user syncs from database' in logs"
