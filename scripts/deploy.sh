#!/bin/bash
# Production deployment script for TNE PBS
# Usage: ./scripts/deploy.sh [staging|production]

set -e

ENVIRONMENT=${1:-staging}
VERSION=$(git describe --tags --always --dirty)
DEPLOY_USER="${DEPLOY_USER:-ubuntu}"
SSH_KEY="${SSH_KEY:-~/.ssh/id_rsa}"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Load environment-specific configuration
if [ "$ENVIRONMENT" = "staging" ]; then
    TARGET_HOST="${STAGING_HOST:-staging.tne-pbs.com}"
    DEPLOY_PATH="/opt/tne-pbs-staging"
elif [ "$ENVIRONMENT" = "production" ]; then
    TARGET_HOST="${PRODUCTION_HOST:-pbs1.tne.com,pbs2.tne.com}"
    DEPLOY_PATH="/opt/tne-pbs"
else
    error "Unknown environment: $ENVIRONMENT. Use 'staging' or 'production'"
fi

info "Deploying version $VERSION to $ENVIRONMENT ($TARGET_HOST)"

# Step 1: Verify binary exists
if [ ! -f "server-production" ]; then
    error "Production binary not found. Run: make build-production"
fi

# Step 2: Create backup of current binary
info "Creating backup on remote servers"
for host in ${TARGET_HOST//,/ }; do
    ssh -i "$SSH_KEY" "$DEPLOY_USER@$host" \
        "sudo cp $DEPLOY_PATH/server $DEPLOY_PATH/server.backup.$(date +%Y%m%d-%H%M%S) 2>/dev/null || true"
done

# Step 3: Upload new binary
info "Uploading new binary"
for host in ${TARGET_HOST//,/ }; do
    scp -i "$SSH_KEY" server-production "$DEPLOY_USER@$host:/tmp/server-new"
    ssh -i "$SSH_KEY" "$DEPLOY_USER@$host" \
        "sudo mv /tmp/server-new $DEPLOY_PATH/server && sudo chmod +x $DEPLOY_PATH/server"
done

# Step 4: Restart service
info "Restarting TNE PBS service"
for host in ${TARGET_HOST//,/ }; do
    ssh -i "$SSH_KEY" "$DEPLOY_USER@$host" \
        "sudo systemctl restart tne-pbs && sudo systemctl status tne-pbs --no-pager"
done

# Step 5: Health check
info "Running health checks"
sleep 5  # Wait for service to start

for host in ${TARGET_HOST//,/ }; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "https://$host/health" || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        info "Health check passed for $host"
    else
        error "Health check failed for $host (HTTP $HTTP_CODE)"
    fi
done

# Step 6: Run smoke tests (staging only)
if [ "$ENVIRONMENT" = "staging" ]; then
    info "Running smoke tests"
    export PBS_URL="https://$TARGET_HOST"
    ./test/hooks_validation_test.sh || warn "Some smoke tests failed - review logs"
fi

info "Deployment completed successfully!"
info "Monitor logs: ssh $DEPLOY_USER@$TARGET_HOST 'sudo journalctl -u tne-pbs -f'"
info ""
info "Rollback command if needed:"
info "  ssh $DEPLOY_USER@$TARGET_HOST 'sudo systemctl stop tne-pbs && sudo cp $DEPLOY_PATH/server.backup.* $DEPLOY_PATH/server && sudo systemctl start tne-pbs'"
