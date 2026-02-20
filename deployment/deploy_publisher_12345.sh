#!/bin/bash
# Deploy Publisher "12345" Configuration to Production
# This script safely adds publisher configuration without disrupting service

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Deploying Publisher 12345 Configuration${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo ""

# Check if Docker is running
if ! docker ps &> /dev/null; then
    echo -e "${RED}Error: Docker is not running${NC}"
    echo "Start Docker with: docker compose up -d"
    exit 1
fi

# Check if PostgreSQL container is running
if ! docker ps | grep -q catalyst-postgres; then
    echo -e "${RED}Error: PostgreSQL container is not running${NC}"
    echo "Start with: docker compose -f deployment/docker-compose.yml up -d"
    exit 1
fi

echo -e "${YELLOW}Step 1: Checking if publisher already exists...${NC}"
EXISTING=$(docker exec catalyst-postgres psql -U catalyst -d catalyst -tAc \
    "SELECT COUNT(*) FROM publishers WHERE publisher_id = '12345';")

if [ "$EXISTING" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Publisher 12345 already exists${NC}"
    echo ""
    echo "Current configuration:"
    docker exec catalyst-postgres psql -U catalyst -d catalyst -c \
        "SELECT publisher_id, name, status, allowed_domains FROM publishers WHERE publisher_id = '12345';"
    echo ""
    read -p "Do you want to UPDATE the existing configuration? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Skipping update${NC}"
        exit 0
    fi

    echo -e "${YELLOW}Step 2: Updating publisher configuration...${NC}"
    docker exec -i catalyst-postgres psql -U catalyst -d catalyst <<'EOF'
UPDATE publishers
SET
    name = 'Total Pro Sports',
    allowed_domains = 'totalprosports.com,dev.totalprosports.com,*.totalprosports.com',
    bidder_params = '{
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
    }'::jsonb,
    status = 'active',
    updated_at = CURRENT_TIMESTAMP
WHERE publisher_id = '12345';
EOF
    echo -e "${GREEN}✓ Publisher updated successfully${NC}"
else
    echo -e "${YELLOW}Step 2: Adding new publisher...${NC}"
    docker exec -i catalyst-postgres psql -U catalyst -d catalyst <<'EOF'
INSERT INTO publishers (
    id,
    publisher_id,
    name,
    allowed_domains,
    bidder_params,
    bid_multiplier,
    status,
    created_at,
    updated_at,
    notes
) VALUES (
    gen_random_uuid(),
    '12345',
    'Total Pro Sports',
    'totalprosports.com,dev.totalprosports.com,*.totalprosports.com',
    '{
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
    }'::jsonb,
    1.0,
    'active',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    'TotalProSports publisher using accountId 12345 (intentional, will update when going live)'
);
EOF
    echo -e "${GREEN}✓ Publisher added successfully${NC}"
fi

echo ""
echo -e "${YELLOW}Step 3: Verifying configuration...${NC}"
docker exec catalyst-postgres psql -U catalyst -d catalyst <<'EOF'
SELECT
    publisher_id,
    name,
    status,
    allowed_domains,
    jsonb_pretty(bidder_params) as bidder_configuration
FROM publishers
WHERE publisher_id = '12345';
EOF

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ Deployment Complete!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo "Publisher '12345' is now configured with:"
echo "  • Rubicon (accountId: 26298)"
echo "  • Kargo, Sovrn, PubMatic, TripleLift"
echo "  • Domains: totalprosports.com, dev.totalprosports.com"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Wait 30 seconds for PublisherAuth cache to expire"
echo "2. Test a bid request from the client"
echo "3. Check logs: docker logs -f catalyst"
echo ""
echo -e "${YELLOW}Expected Result:${NC}"
echo "  [Catalyst] Received 5 bids (instead of 0)"
echo "  No more HTTP 400 errors"
echo ""
