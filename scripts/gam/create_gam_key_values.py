#!/usr/bin/env python3
"""
Catalyst GAM Key-Value Setup Script
====================================
Creates all custom targeting keys and predefined values in Google Ad Manager
for Catalyst server-side header bidding.

Keys created:
  hb_pb_catalyst       - CPM price buckets (PREDEFINED, dense granularity)
  hb_bidder_catalyst   - Demand partner name (PREDEFINED)
  hb_size_catalyst     - Ad sizes (PREDEFINED)
  hb_adid_catalyst     - Bid ID (FREEFORM - dynamic values)
  hb_creative_catalyst - Creative ID (FREEFORM - dynamic values)
  hb_source_catalyst   - Bid source (PREDEFINED: "s2s")
  hb_format_catalyst   - Ad format (PREDEFINED: "banner", "video", "native")
  hb_deal_catalyst     - Deal ID for PMP (FREEFORM - dynamic values)
  hb_adomain_catalyst  - Advertiser domain (FREEFORM - dynamic values)
  hb_partner           - Partner alias (PREDEFINED)

Prerequisites:
  pip install googleads

Usage:
  python scripts/gam/create_gam_key_values.py \\
    --network-code 12345678 \\
    --key-file scripts/gam/gam_creds.json

  # Dry run (preview only):
  python scripts/gam/create_gam_key_values.py \\
    --network-code 12345678 \\
    --key-file scripts/gam/gam_creds.json \\
    --dry-run
"""

import argparse
import logging
import sys
from decimal import Decimal, ROUND_HALF_UP

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger("catalyst_gam_keys")

# ---------------------------------------------------------------------------
# Price bucket generation (dense granularity matching exchange.go)
# ---------------------------------------------------------------------------

def generate_dense_price_buckets():
    """Generate price bucket values using dense granularity.

    Dense granularity (matches Prebid spec):
      $0.01 increments from $0.01 to $3.00
      $0.05 increments from $3.05 to $8.00
      $0.50 increments from $8.50 to $20.00
    """
    buckets = []

    # $0.01 increments: $0.01 - $3.00
    price = Decimal("0.01")
    while price <= Decimal("3.00"):
        buckets.append(str(price.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)))
        price += Decimal("0.01")

    # $0.05 increments: $3.05 - $8.00
    price = Decimal("3.05")
    while price <= Decimal("8.00"):
        buckets.append(str(price.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)))
        price += Decimal("0.05")

    # $0.50 increments: $8.50 - $20.00
    price = Decimal("8.50")
    while price <= Decimal("20.00"):
        buckets.append(str(price.quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)))
        price += Decimal("0.50")

    return buckets


# ---------------------------------------------------------------------------
# Key-value definitions for Catalyst
# ---------------------------------------------------------------------------

# Each entry: (key_name, key_type, reportable, values_or_none)
#   key_type: "PREDEFINED" = all values known upfront, "FREEFORM" = dynamic
#   values_or_none: list of predefined values, or None for freeform keys
CATALYST_KEYS = [
    (
        "hb_pb_catalyst",
        "PREDEFINED",
        "ON",
        generate_dense_price_buckets,  # callable, ~423 values
    ),
    (
        "hb_bidder_catalyst",
        "PREDEFINED",
        "ON",
        lambda: ["thenexusengine"],
    ),
    (
        "hb_size_catalyst",
        "PREDEFINED",
        "ON",
        lambda: [
            "300x250", "728x90", "970x250", "320x50",
            "300x600", "160x600", "970x90", "336x280",
            "300x50", "320x100", "250x250", "200x200",
            "468x60", "120x600", "300x1050", "320x480",
            "640x480", "1x1",
        ],
    ),
    (
        "hb_adid_catalyst",
        "FREEFORM",
        "OFF",
        None,  # Dynamic bid IDs
    ),
    (
        "hb_creative_catalyst",
        "FREEFORM",
        "OFF",
        None,  # Dynamic creative IDs
    ),
    (
        "hb_source_catalyst",
        "PREDEFINED",
        "ON",
        lambda: ["s2s"],
    ),
    (
        "hb_format_catalyst",
        "PREDEFINED",
        "ON",
        lambda: ["banner", "video", "native"],
    ),
    (
        "hb_deal_catalyst",
        "FREEFORM",
        "ON",
        None,  # Dynamic deal IDs
    ),
    (
        "hb_adomain_catalyst",
        "FREEFORM",
        "OFF",
        None,  # Dynamic advertiser domains
    ),
    (
        "hb_partner",
        "PREDEFINED",
        "ON",
        lambda: ["thenexusengine"],
    ),
]

# GAM API limits: max values per batch create call
GAM_BATCH_SIZE = 200


# ---------------------------------------------------------------------------
# GAM API helpers
# ---------------------------------------------------------------------------

def get_gam_client(key_file, network_code, app_name="Catalyst-GAM-Setup"):
    """Create an authenticated GAM API client."""
    from googleads import ad_manager, oauth2

    oauth2_client = oauth2.GoogleServiceAccountClient(
        key_file, oauth2.GetAPIScope("ad_manager")
    )
    client = ad_manager.AdManagerClient(
        oauth2_client, app_name, network_code=network_code
    )
    return client


def find_key_by_name(targeting_service, key_name):
    """Look up a custom targeting key by name. Returns key dict or None."""
    from googleads import ad_manager

    statement = (
        ad_manager.StatementBuilder(version="v202408")
        .Where("name = :name")
        .WithBindVariable("name", key_name)
    )

    response = targeting_service.getCustomTargetingKeysByStatement(
        statement.ToStatement()
    )

    results = getattr(response, "results", None) or response.get("results", [])
    if results and len(results) > 0:
        return results[0]
    return None


def get_existing_values(targeting_service, key_id):
    """Get all existing value names for a given key ID."""
    from googleads import ad_manager

    existing = set()
    offset = 0
    page_size = 500

    while True:
        statement = (
            ad_manager.StatementBuilder(version="v202408")
            .Where("customTargetingKeyId = :keyId AND status = 'ACTIVE'")
            .WithBindVariable("keyId", key_id)
            .Limit(page_size)
            .Offset(offset)
        )

        response = targeting_service.getCustomTargetingValuesByStatement(
            statement.ToStatement()
        )

        results = getattr(response, "results", None) or response.get("results", [])
        if not results:
            break

        for val in results:
            name = val.get("name", "") if isinstance(val, dict) else getattr(val, "name", "")
            if name:
                existing.add(name)

        if len(results) < page_size:
            break
        offset += page_size

    return existing


def create_key(targeting_service, key_name, key_type, reportable_type):
    """Create a custom targeting key. Returns the key dict."""
    keys = targeting_service.createCustomTargetingKeys([
        {
            "name": key_name,
            "displayName": key_name,
            "type": key_type,
            "reportableType": reportable_type,
        }
    ])
    return keys[0]


def create_values_batch(targeting_service, key_id, value_names):
    """Create targeting values in batches. Returns count of values created."""
    created_count = 0

    for i in range(0, len(value_names), GAM_BATCH_SIZE):
        batch = value_names[i : i + GAM_BATCH_SIZE]
        value_configs = [
            {
                "customTargetingKeyId": key_id,
                "name": str(v),
                "displayName": str(v),
                "matchType": "EXACT",
            }
            for v in batch
        ]

        result = targeting_service.createCustomTargetingValues(value_configs)
        batch_created = len(result) if result else 0
        created_count += batch_created

        log.info(
            "  Created batch %d-%d (%d values)",
            i + 1,
            min(i + GAM_BATCH_SIZE, len(value_names)),
            batch_created,
        )

    return created_count


# ---------------------------------------------------------------------------
# Main logic
# ---------------------------------------------------------------------------

def create_all_keys(targeting_service, dry_run=False):
    """Create all Catalyst targeting keys and their values."""
    summary = []

    for key_name, key_type, reportable, values_fn in CATALYST_KEYS:
        log.info("=" * 60)
        log.info("Key: %s (%s)", key_name, key_type)

        # Check if key already exists
        existing_key = find_key_by_name(targeting_service, key_name)

        if existing_key:
            key_id = existing_key.get("id", None) if isinstance(existing_key, dict) else getattr(existing_key, "id", None)
            log.info("  Already exists (id=%s) - reusing", key_id)
        else:
            if dry_run:
                log.info("  [DRY RUN] Would create key: %s (%s)", key_name, key_type)
                key_id = None
            else:
                created = create_key(targeting_service, key_name, key_type, reportable)
                key_id = created.get("id", None) if isinstance(created, dict) else getattr(created, "id", None)
                log.info("  Created key (id=%s)", key_id)

        # Handle values
        if values_fn is None:
            log.info("  FREEFORM key - no predefined values needed")
            summary.append((key_name, key_type, 0, "freeform"))
            continue

        desired_values = values_fn()
        log.info("  Target values: %d", len(desired_values))

        if dry_run:
            log.info("  [DRY RUN] Would create up to %d values", len(desired_values))
            if len(desired_values) <= 10:
                log.info("  Values: %s", desired_values)
            else:
                log.info("  Sample: %s ... %s", desired_values[:3], desired_values[-3:])
            summary.append((key_name, key_type, len(desired_values), "dry-run"))
            continue

        if key_id is None:
            log.warning("  Skipping values - no key ID available")
            summary.append((key_name, key_type, 0, "skipped"))
            continue

        # Find existing values to avoid duplicates
        existing_values = get_existing_values(targeting_service, key_id)
        new_values = [v for v in desired_values if v not in existing_values]

        log.info("  Existing: %d, New to create: %d", len(existing_values), len(new_values))

        if not new_values:
            log.info("  All values already exist - nothing to do")
            summary.append((key_name, key_type, 0, "up-to-date"))
            continue

        created_count = create_values_batch(targeting_service, key_id, new_values)
        log.info("  Created %d values", created_count)
        summary.append((key_name, key_type, created_count, "created"))

    return summary


def print_summary(summary, dry_run=False):
    """Print a summary table of all operations."""
    print()
    print("=" * 65)
    print("  Catalyst GAM Key-Value Setup Summary" + (" (DRY RUN)" if dry_run else ""))
    print("=" * 65)
    print(f"  {'Key':<25} {'Type':<12} {'Values':<8} {'Status'}")
    print(f"  {'-'*24} {'-'*11} {'-'*7} {'-'*12}")
    for key_name, key_type, count, status in summary:
        print(f"  {key_name:<25} {key_type:<12} {count:<8} {status}")
    print("=" * 65)
    print()


def main():
    parser = argparse.ArgumentParser(
        description="Create Catalyst GAM custom targeting keys and values"
    )
    parser.add_argument(
        "--network-code",
        required=True,
        help="GAM network code (e.g., 21775744923)",
    )
    parser.add_argument(
        "--key-file",
        default="scripts/gam/gam_creds.json",
        help="Path to GAM service account JSON key (default: scripts/gam/gam_creds.json)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Preview what would be created without making changes",
    )
    parser.add_argument(
        "--verbose",
        action="store_true",
        help="Enable verbose/debug logging",
    )
    args = parser.parse_args()

    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)

    mode = "DRY RUN" if args.dry_run else "LIVE"
    log.info("Catalyst GAM Key-Value Setup (%s)", mode)
    log.info("Network code: %s", args.network_code)
    log.info("Credentials:  %s", args.key_file)
    print()

    try:
        client = get_gam_client(args.key_file, args.network_code)
        targeting_service = client.GetService(
            "CustomTargetingService", version="v202408"
        )
    except ImportError:
        log.error("Missing dependency: pip install googleads")
        sys.exit(1)
    except Exception as e:
        log.error("Failed to connect to GAM API: %s", e)
        log.error("")
        log.error("Checklist:")
        log.error("  1. Is the key file valid? %s", args.key_file)
        log.error("  2. Is GAM API access enabled? (Admin > Global Settings)")
        log.error("  3. Is the service account added as a GAM user?")
        sys.exit(1)

    try:
        summary = create_all_keys(targeting_service, dry_run=args.dry_run)
        print_summary(summary, dry_run=args.dry_run)

        if args.dry_run:
            log.info("Dry run complete. Run without --dry-run to create for real.")
        else:
            log.info("All Catalyst GAM key-values are set up!")
            log.info("")
            log.info("Next: Run the line item setup:")
            log.info("  ./scripts/gam/setup_gam.sh --network-code %s --network-name '<YOUR_NETWORK>'", args.network_code)

    except Exception as e:
        log.error("Failed: %s", e)
        log.error("", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
