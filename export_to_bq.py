#!/usr/bin/env python3
"""
Export analytics tables from Postgres to BigQuery.

Intended to run every 15 minutes via cron:
    */15 * * * * GOOGLE_APPLICATION_CREDENTIALS=/opt/tne/bq-key.json /opt/tne/export_to_bq.py

State is tracked in LAST_EXPORTED_FILE (a plain text file containing an ISO timestamp).
Only rows with created_at > last_exported_at are exported, preventing duplicates.

Required env vars:
    GOOGLE_APPLICATION_CREDENTIALS  - path to GCP service account JSON key
    POSTGRES_DSN                    - e.g. host=localhost port=5432 user=postgres dbname=catalyst
    BQ_PROJECT                      - GCP project ID
    BQ_DATASET                      - BigQuery dataset name (default: catalyst_analytics)

Dependencies:
    pip install google-cloud-bigquery psycopg2-binary
"""

import os
import sys
import logging
from datetime import datetime, timezone
from pathlib import Path

import psycopg2
import psycopg2.extras
from google.cloud import bigquery

# ── Configuration ────────────────────────────────────────────────────────────

POSTGRES_DSN = os.environ.get("POSTGRES_DSN", "host=localhost port=5432 user=postgres dbname=catalyst sslmode=disable")
BQ_PROJECT   = os.environ.get("BQ_PROJECT", "")
BQ_DATASET   = os.environ.get("BQ_DATASET", "catalyst_analytics")

LAST_EXPORTED_FILE = Path(os.environ.get("LAST_EXPORTED_FILE", "/opt/tne/last_exported_at.txt"))

TABLES = ["auction_events", "bidder_events", "win_events", "identity_events", "request_events"]

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
)
log = logging.getLogger(__name__)

# ── State helpers ─────────────────────────────────────────────────────────────

def load_last_exported() -> datetime:
    """Return the last export timestamp, or epoch if no prior run."""
    if LAST_EXPORTED_FILE.exists():
        raw = LAST_EXPORTED_FILE.read_text().strip()
        try:
            return datetime.fromisoformat(raw)
        except ValueError:
            log.warning("Could not parse last_exported_at '%s', starting from epoch", raw)
    return datetime(1970, 1, 1, tzinfo=timezone.utc)


def save_last_exported(ts: datetime) -> None:
    LAST_EXPORTED_FILE.parent.mkdir(parents=True, exist_ok=True)
    LAST_EXPORTED_FILE.write_text(ts.isoformat())

# ── Core export ───────────────────────────────────────────────────────────────

def export_table(
    pg_cursor: psycopg2.extensions.cursor,
    bq_client: bigquery.Client,
    table: str,
    since: datetime,
) -> int:
    """Export new rows from one Postgres table to BigQuery. Returns row count inserted."""
    pg_cursor.execute(
        f"SELECT * FROM {table} WHERE created_at > %s ORDER BY created_at",
        (since,),
    )
    rows = pg_cursor.fetchall()
    if not rows:
        log.info("  %s: no new rows since %s", table, since.isoformat())
        return 0

    records = [dict(row) for row in rows]

    # BigQuery cannot handle Python Decimal or datetime without timezone — normalize
    for record in records:
        for key, val in record.items():
            if hasattr(val, "__float__") and not isinstance(val, (int, float, bool)):
                record[key] = float(val)
            elif hasattr(val, "isoformat"):
                record[key] = val.isoformat()

    bq_table_ref = f"{BQ_PROJECT}.{BQ_DATASET}.{table}"

    # Use autodetect only for new tables; existing tables use their stored schema
    # (autodetect re-infers types and conflicts with NULL-able float columns)
    table_is_new = False
    try:
        bq_client.get_table(bq_table_ref)
    except Exception:
        table_is_new = True

    job_config = bigquery.LoadJobConfig(
        autodetect=table_is_new,
        write_disposition=bigquery.WriteDisposition.WRITE_APPEND,
        source_format=bigquery.SourceFormat.NEWLINE_DELIMITED_JSON,
        schema_update_options=[bigquery.SchemaUpdateOption.ALLOW_FIELD_ADDITION],
    )
    job = bq_client.load_table_from_json(records, bq_table_ref, job_config=job_config)
    job.result()  # wait for completion
    if job.errors:
        log.error("  %s: BigQuery load errors: %s", table, job.errors)
        raise RuntimeError(f"BigQuery load failed for {table}: {job.errors}")

    log.info("  %s: loaded %d rows", table, len(records))
    return len(records)


def run_export() -> None:
    if not BQ_PROJECT:
        log.error("BQ_PROJECT env var is required")
        sys.exit(1)

    since = load_last_exported()
    log.info("Exporting rows created after %s", since.isoformat())

    export_start = datetime.now(tz=timezone.utc)
    total_rows = 0

    pg_conn = psycopg2.connect(POSTGRES_DSN)
    bq_client = bigquery.Client(project=BQ_PROJECT)

    try:
        with pg_conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            for table in TABLES:
                total_rows += export_table(cur, bq_client, table, since)
    finally:
        pg_conn.close()

    save_last_exported(export_start)
    log.info("Export complete: %d total rows across %d tables", total_rows, len(TABLES))


if __name__ == "__main__":
    run_export()
