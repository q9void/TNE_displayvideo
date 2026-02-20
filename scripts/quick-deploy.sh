#!/bin/bash
# Quick deployment script for code updates
# Applies migrations and rebuilds services

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}        TNE Catalyst Quick Deploy${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Deployment directory
DEPLOY_DIR="/opt/catalyst"
cd "$DEPLOY_DIR" || { echo "Error: Could not find $DEPLOY_DIR"; exit 1; }

# Step 1: Apply database migrations
echo -e "${CYAN}[1/4] Applying database migrations...${NC}"

# Check if new migrations exist
NEW_MIGRATIONS=$(find migrations/ -name "00[5-6]*.sql" 2>/dev/null || true)

if [ -n "$NEW_MIGRATIONS" ]; then
    echo "Found new migrations to apply:"
    echo "$NEW_MIGRATIONS"
    echo ""

    # Get database credentials from .env
    source .env

    # Apply each migration
    for migration in $(ls migrations/005*.sql migrations/006*.sql 2>/dev/null); do
        echo "Applying: $migration"
        docker exec catalyst-postgres psql -U "${DB_USER:-catalyst}" -d "${DB_NAME:-catalyst}" -f "/tmp/migration.sql" < "$migration" || {
            echo "Using alternate method..."
            cat "$migration" | docker exec -i catalyst-postgres psql -U "${DB_USER:-catalyst}" -d "${DB_NAME:-catalyst}"
        }
    done

    echo -e "${GREEN}✅ Migrations applied${NC}"
else
    echo -e "${YELLOW}⚠️  No new migrations found in migrations/00[5-6]*.sql${NC}"
    echo "Skipping migration step..."
fi

echo ""

# Step 2: Rebuild catalyst service
echo -e "${CYAN}[2/4] Rebuilding catalyst service (pulling latest code from GitHub)...${NC}"
docker-compose build --no-cache catalyst
echo -e "${GREEN}✅ Service rebuilt${NC}"
echo ""

# Step 3: Restart services
echo -e "${CYAN}[3/4] Restarting services...${NC}"
docker-compose up -d catalyst
echo -e "${GREEN}✅ Services restarted${NC}"
echo ""

# Step 4: Health check
echo -e "${CYAN}[4/4] Checking service health...${NC}"
sleep 5

if docker-compose ps | grep -q "catalyst.*Up"; then
    echo -e "${GREEN}✅ Catalyst service is running${NC}"

    # Test health endpoint
    if curl -f http://localhost:8000/health > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Health check passed${NC}"
    else
        echo -e "${YELLOW}⚠️  Health check failed (service may still be starting)${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Catalyst service not running${NC}"
    echo "Check logs with: docker-compose logs catalyst"
fi

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Deployment Complete!${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "View logs: docker-compose logs -f catalyst"
echo "Check status: docker-compose ps"
echo ""
