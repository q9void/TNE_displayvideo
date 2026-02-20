#!/bin/bash
# Script to check publisher_id='12345' configuration
# This script queries the database for publisher-level bidder params

POSTGRES_CONTAINER="catalyst-postgres"
DB_NAME="${DB_NAME:-catalyst}"
DB_USER="${DB_USER:-catalyst}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Checking Publisher ID: 12345${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo ""

# Check if PostgreSQL is running
if ! docker ps | grep -q $POSTGRES_CONTAINER; then
    echo -e "${RED}Error: PostgreSQL container '$POSTGRES_CONTAINER' not running${NC}"
    echo -e "${YELLOW}Start with: docker compose up -d${NC}"
    echo ""
    echo -e "${YELLOW}Alternative: Check production database${NC}"
    echo "  PGPASSWORD='ttlJRsJK7myCehgKyswnZP82v6L57xT5' psql -h <host> -U catalyst_prod -d catalyst_production"
    exit 1
fi

echo "Querying database for publisher_id='12345'..."
echo ""

# Query for the publisher
QUERY="SELECT publisher_id, name, allowed_domains, bidder_params, bid_multiplier, status, created_at 
       FROM publishers 
       WHERE publisher_id = '12345';"

RESULT=$(docker exec $POSTGRES_CONTAINER psql -U $DB_USER -d $DB_NAME -t -A -c "$QUERY" 2>/dev/null)

if [ -z "$RESULT" ]; then
    echo -e "${YELLOW}Publisher ID '12345' NOT FOUND in database${NC}"
    echo ""
    echo -e "${RED}Status: Publisher-level bidder params DO NOT exist${NC}"
    echo ""
    echo -e "${BLUE}Action Required:${NC}"
    echo "  Add publisher with bidder params using:"
    echo ""
    echo -e "${GREEN}  ./deployment/manage-publishers.sh add '12345' 'Publisher Name' 'allowed-domain.com' '{\"rubicon\":{\"accountId\":26298,\"siteId\":556630},\"appnexus\":{\"placementId\":12345}}'${NC}"
    echo ""
else
    IFS='|' read -r pub_id name domains bidder_params multiplier status created <<< "$RESULT"
    
    echo -e "${GREEN}✓ Publisher Found!${NC}"
    echo ""
    echo -e "  Publisher ID:   ${BLUE}$pub_id${NC}"
    echo -e "  Name:           ${BLUE}$name${NC}"
    echo -e "  Status:         ${GREEN}$status${NC}"
    echo -e "  Domains:        ${BLUE}$domains${NC}"
    echo -e "  Bid Multiplier: ${BLUE}$multiplier${NC}"
    echo -e "  Created:        ${BLUE}$created${NC}"
    echo ""
    echo -e "${GREEN}Bidder Parameters (JSONB):${NC}"
    echo "$bidder_params" | python3 -m json.tool 2>/dev/null || echo "$bidder_params"
    echo ""
    
    # Check if bidder_params is empty
    if [ "$bidder_params" = "{}" ]; then
        echo -e "${RED}⚠ Warning: bidder_params is EMPTY!${NC}"
        echo -e "${YELLOW}Publisher-level bidder params need to be added.${NC}"
        echo ""
        echo "Update with:"
        echo -e "${GREEN}  ./deployment/manage-publishers.sh update '12345' bidder_params '{\"rubicon\":{\"accountId\":26298,\"siteId\":556630}}'${NC}"
    else
        echo -e "${GREEN}✓ Publisher-level bidder params EXIST${NC}"
        
        # Parse and display bidder count
        BIDDER_COUNT=$(echo "$bidder_params" | python3 -c "import sys, json; print(len(json.load(sys.stdin)))" 2>/dev/null)
        if [ -n "$BIDDER_COUNT" ]; then
            echo -e "  ${BLUE}$BIDDER_COUNT bidder(s) configured${NC}"
        fi
    fi
fi

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
