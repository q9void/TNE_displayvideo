#!/usr/bin/env bash
# =============================================================================
# Catalyst GAM Setup Script
# =============================================================================
# Creates GAM orders, line items, and creatives for Catalyst S2S header bidding
# using Prebid's line-item-manager tool.
#
# Prerequisites:
#   1. pip install line-item-manager
#   2. GAM API credentials JSON file (Service Account key from Google API Console)
#   3. GAM API access enabled in your GAM network (Admin > Global Settings)
#   4. Service account added as GAM user with Administrator role
#
# Usage:
#   ./scripts/gam/setup_gam.sh --network-code 12345678 --network-name "My Network"
#   ./scripts/gam/setup_gam.sh --network-code 12345678 --network-name "My Network" --dry-run
#   ./scripts/gam/setup_gam.sh --network-code 12345678 --network-name "My Network" --test-run
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/catalyst_gam_config.yml"
CREDS_FILE="${GAM_CREDS_FILE:-${SCRIPT_DIR}/gam_creds.json}"

# Defaults
NETWORK_CODE=""
NETWORK_NAME=""
DRY_RUN=""
TEST_RUN=""
VERBOSE=""

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Required:
  --network-code CODE    GAM network code (e.g., 21775744923)
  --network-name NAME    GAM network name (must match exactly what GAM returns)

Optional:
  --creds-file PATH      Path to GAM credentials JSON (default: scripts/gam/gam_creds.json)
                         Can also set GAM_CREDS_FILE env var
  --dry-run              Preview what would be created without making changes
  --test-run             Create only 2 line items for visual inspection
  --verbose              Enable verbose logging
  -h, --help             Show this help

Environment:
  GAM_CREDS_FILE         Alternative path to GAM credentials JSON

Examples:
  # Dry run to preview
  $(basename "$0") --network-code 21775744923 --network-name "My Publisher" --dry-run

  # Test run (creates 2 line items for inspection)
  $(basename "$0") --network-code 21775744923 --network-name "My Publisher" --test-run

  # Full creation
  $(basename "$0") --network-code 21775744923 --network-name "My Publisher"
EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --network-code) NETWORK_CODE="$2"; shift 2;;
        --network-name) NETWORK_NAME="$2"; shift 2;;
        --creds-file)   CREDS_FILE="$2"; shift 2;;
        --dry-run)      DRY_RUN="--dry-run"; shift;;
        --test-run)     TEST_RUN="--test-run"; shift;;
        --verbose)      VERBOSE="-v"; shift;;
        -h|--help)      usage;;
        *) echo "Unknown option: $1"; usage;;
    esac
done

# Validate required args
if [[ -z "$NETWORK_CODE" ]]; then
    echo "Error: --network-code is required"
    echo "  Find it in GAM: Admin > Global Settings > Network code"
    exit 1
fi

if [[ -z "$NETWORK_NAME" ]]; then
    echo "Error: --network-name is required"
    echo "  Must match exactly what GAM returns for your network"
    exit 1
fi

# Check prerequisites
if ! command -v line_item_manager &> /dev/null; then
    echo "Error: line_item_manager not found"
    echo "  Install it: pip install line-item-manager"
    exit 1
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: Config file not found: $CONFIG_FILE"
    exit 1
fi

if [[ ! -f "$CREDS_FILE" ]]; then
    echo "Error: GAM credentials file not found: $CREDS_FILE"
    echo ""
    echo "To create credentials:"
    echo "  1. Go to Google API Console > Create Service Account"
    echo "  2. Generate a JSON key and save it as: $CREDS_FILE"
    echo "  3. In GAM: Admin > Global Settings > Enable API access"
    echo "  4. Add the service account email as a GAM user (Administrator role)"
    exit 1
fi

echo "============================================="
echo "  Catalyst GAM Setup"
echo "============================================="
echo "  Network Code: $NETWORK_CODE"
echo "  Network Name: $NETWORK_NAME"
echo "  Config:       $CONFIG_FILE"
echo "  Credentials:  $CREDS_FILE"
echo "  Mode:         ${DRY_RUN:-${TEST_RUN:-LIVE}}"
echo "============================================="
echo ""

if [[ -z "$DRY_RUN" && -z "$TEST_RUN" ]]; then
    echo "WARNING: This will create GAM orders, line items, and creatives."
    echo "         Run with --dry-run first to preview changes."
    echo ""
    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

# Build the command
CMD=(
    line_item_manager create "$CONFIG_FILE"
    --private-key-file "$CREDS_FILE"
    --network-code "$NETWORK_CODE"
    --network-name "$NETWORK_NAME"
    --single-order
)

[[ -n "$DRY_RUN" ]]  && CMD+=("$DRY_RUN")
[[ -n "$TEST_RUN" ]] && CMD+=("$TEST_RUN")
[[ -n "$VERBOSE" ]]  && CMD+=("$VERBOSE")

echo "Running: ${CMD[*]}"
echo ""

"${CMD[@]}"

STATUS=$?

echo ""
if [[ $STATUS -eq 0 ]]; then
    echo "============================================="
    echo "  GAM setup completed successfully!"
    echo "============================================="
    echo ""
    echo "Next steps:"
    echo "  1. Go to GAM and verify the orders/line items were created"
    echo "  2. Approve the orders to make them eligible to serve"
    echo "  3. Test with: ./scripts/gam/setup_gam.sh --network-code $NETWORK_CODE --network-name \"$NETWORK_NAME\" --test-run"
    echo ""
    echo "GAM Key-Values created:"
    echo "  hb_pb_catalyst       - CPM price bucket (targeting)"
    echo "  hb_bidder_catalyst   - Always 'thenexusengine'"
    echo "  hb_source_catalyst   - Always 's2s'"
    echo "  hb_size_catalyst     - Ad size (e.g., 300x250)"
    echo "  hb_adid_catalyst     - Bid ID"
    echo "  hb_creative_catalyst - Creative ID"
    echo "  hb_deal_catalyst     - Deal ID (PMP)"
    echo "  hb_adomain_catalyst  - Advertiser domain"
    echo "  hb_format_catalyst   - Ad format (banner)"
    echo "  hb_partner           - Partner alias"
else
    echo "============================================="
    echo "  GAM setup FAILED (exit code: $STATUS)"
    echo "============================================="
    echo ""
    echo "Common issues:"
    echo "  - Credentials expired: Regenerate JSON key in Google API Console"
    echo "  - Network name mismatch: Must match exactly (case-sensitive)"
    echo "  - API not enabled: GAM Admin > Global Settings > Enable API access"
    echo "  - Service account not added: Add as GAM user with Admin role"
    exit $STATUS
fi
