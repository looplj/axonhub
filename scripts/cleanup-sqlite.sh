#!/usr/bin/env bash
# AxonHub SQLite Database Cleanup Script
# Issue: https://github.com/looplj/axonhub/issues/1905
#
# Usage:
#   ./scripts/cleanup-sqlite.sh [DB_PATH] [RETENTION_DAYS]
#
# Examples:
#   ./scripts/cleanup-sqlite.sh                          # uses default path, 30 days
#   ./scripts/cleanup-sqlite.sh ./axonhub.db 7           # custom path, keep 7 days
#   ./scripts/cleanup-sqlite.sh ./axonhub.db 0           # delete ALL historical data

set -euo pipefail

DB_PATH="${1:-axonhub.db}"
RETENTION_DAYS="${2:-30}"
BATCH_SIZE=5000

if ! [[ "$RETENTION_DAYS" =~ ^[0-9]+$ ]]; then
    echo "Error: RETENTION_DAYS must be a non-negative integer, got: $RETENTION_DAYS"
    exit 1
fi

if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database file not found: $DB_PATH"
    echo "Usage: $0 [DB_PATH] [RETENTION_DAYS]"
    exit 1
fi

if ! command -v sqlite3 &>/dev/null; then
    echo "Error: sqlite3 not found. Install it first:"
    echo "  macOS:  brew install sqlite3"
    echo "  Ubuntu: apt install sqlite3"
    exit 1
fi

echo "============================================"
echo "  AxonHub SQLite Cleanup"
echo "============================================"
echo "Database:  $DB_PATH"
echo "Retention: ${RETENTION_DAYS} days"
echo "DB Size:   $(du -h "$DB_PATH" | cut -f1)"
echo ""

# Show current table sizes
echo "--- Table sizes before cleanup ---"
if sqlite3 "$DB_PATH" "SELECT * FROM dbstat LIMIT 1;" &>/dev/null; then
    sqlite3 "$DB_PATH" "
    SELECT
        name AS table_name,
        printf('%.2f MB', SUM(pgsize) / 1024.0 / 1024.0) AS size_mb
    FROM dbstat
    WHERE name IN (SELECT name FROM sqlite_master WHERE type='table')
    GROUP BY name
    ORDER BY SUM(pgsize) DESC;
    "
else
    echo "  (dbstat unavailable, showing row counts instead)"
    sqlite3 "$DB_PATH" "
    SELECT name AS table_name, printf('%d rows', COUNT(*)) AS estimate
    FROM sqlite_master m
    LEFT JOIN (SELECT * FROM pragma_table_info('')) ON 0
    WHERE m.type='table'
    GROUP BY m.name
    ORDER BY m.name;
    " 2>/dev/null || echo "  (unable to display table info)"
fi
echo ""

# Count records to be deleted
CUTOFF_DATE=$(date -u -d "${RETENTION_DAYS} days ago" +%Y-%m-%dT%H:%M:%S 2>/dev/null || \
              date -u -v-"${RETENTION_DAYS}"d +%Y-%m-%dT%H:%M:%S 2>/dev/null || \
              echo "")

if [ -z "$CUTOFF_DATE" ]; then
    echo "Warning: Could not compute cutoff date, using SQL directly"
    CUTOFF_SQL="datetime('now', '-${RETENTION_DAYS} days')"
else
    echo "Cutoff date: $CUTOFF_DATE"
    CUTOFF_SQL="'${CUTOFF_DATE}'"
fi

echo ""
echo "--- Records to be deleted ---"
for TABLE in request_executions requests usage_logs traces threads channel_probes; do
    COUNT=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM ${TABLE} WHERE created_at < ${CUTOFF_SQL};" 2>/dev/null || echo "0")
    echo "  ${TABLE}: ${COUNT} rows"
done
echo ""

if [ "$RETENTION_DAYS" -eq 0 ]; then
    echo "WARNING: Retention=0 means ALL historical data will be deleted!"
    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 0
    fi
fi

echo "--- Starting cleanup ---"

# 1. Delete request_executions (child table first)
echo -n "  Cleaning request_executions... "
TOTAL=0
while true; do
    DELETED=$(sqlite3 "$DB_PATH" "
        DELETE FROM request_executions
        WHERE id IN (
            SELECT id FROM request_executions
            WHERE created_at < ${CUTOFF_SQL}
            LIMIT ${BATCH_SIZE}
        );
        SELECT changes();
    ")
    TOTAL=$((TOTAL + DELETED))
    if [ "$DELETED" -eq 0 ]; then break; fi
    echo -n "."
done
echo " done (${TOTAL} rows)"

# 2. Delete requests
echo -n "  Cleaning requests... "
TOTAL=0
while true; do
    DELETED=$(sqlite3 "$DB_PATH" "
        DELETE FROM requests
        WHERE id IN (
            SELECT id FROM requests
            WHERE created_at < ${CUTOFF_SQL}
            LIMIT ${BATCH_SIZE}
        );
        SELECT changes();
    ")
    TOTAL=$((TOTAL + DELETED))
    if [ "$DELETED" -eq 0 ]; then break; fi
    echo -n "."
done
echo " done (${TOTAL} rows)"

# 3. Delete usage_logs
echo -n "  Cleaning usage_logs... "
TOTAL=0
while true; do
    DELETED=$(sqlite3 "$DB_PATH" "
        DELETE FROM usage_logs
        WHERE id IN (
            SELECT id FROM usage_logs
            WHERE created_at < ${CUTOFF_SQL}
            LIMIT ${BATCH_SIZE}
        );
        SELECT changes();
    ")
    TOTAL=$((TOTAL + DELETED))
    if [ "$DELETED" -eq 0 ]; then break; fi
    echo -n "."
done
echo " done (${TOTAL} rows)"

# 4. Delete channel_probes (hardcoded 3 days like GC does).
# The probe time column is "timestamp" (an integer epoch) in the current schema; older
# databases used a datetime "created_at". Use whichever exists and compare it with a
# cutoff of the same type - the hardcoded created_at made the statement fail and, under
# `set -e`, aborted the script before VACUUM ever ran.
PROBE_TS_COL=$(sqlite3 "$DB_PATH" "SELECT name FROM pragma_table_info('channel_probes') WHERE name IN ('created_at','timestamp','probed_at') ORDER BY CASE name WHEN 'created_at' THEN 0 WHEN 'timestamp' THEN 1 ELSE 2 END LIMIT 1;")
echo -n "  Cleaning channel_probes (>3 days)... "
if [ -z "$PROBE_TS_COL" ]; then
    echo " skipped (no timestamp column)"
else
    PROBE_TS_TYPE=$(sqlite3 "$DB_PATH" "SELECT lower(type) FROM pragma_table_info('channel_probes') WHERE name = '${PROBE_TS_COL}';")
    case "$PROBE_TS_TYPE" in
        *int*) PROBE_CUTOFF="CAST(strftime('%s', 'now', '-3 days') AS INTEGER)" ;;
        *)     PROBE_CUTOFF="datetime('now', '-3 days')" ;;
    esac
    DELETED=$(sqlite3 "$DB_PATH" "
        DELETE FROM channel_probes
        WHERE ${PROBE_TS_COL} < ${PROBE_CUTOFF};
        SELECT changes();
    ")
    echo " done (${DELETED} rows)"
fi

# 5. Delete orphaned traces
echo -n "  Cleaning orphaned traces... "
DELETED=$(sqlite3 "$DB_PATH" "
    DELETE FROM traces
    WHERE trace_id IS NOT NULL
      AND trace_id NOT IN (SELECT DISTINCT trace_id FROM requests WHERE trace_id IS NOT NULL);
    SELECT changes();
")
echo " done (${DELETED} rows)"

# 6. Delete orphaned threads
echo -n "  Cleaning orphaned threads... "
DELETED=$(sqlite3 "$DB_PATH" "
    DELETE FROM threads
    WHERE thread_id NOT IN (SELECT DISTINCT thread_id FROM requests WHERE thread_id IS NOT NULL);
    SELECT changes();
")
echo " done (${DELETED} rows)"

# 7. Purge soft-deleted records (>90 days).
# deleted_at is not the same type in every table: most tables get it from the SoftDelete
# mixin as an INTEGER epoch where 0 means "not deleted", while a few store a datetime
# string (NULL meaning "not deleted"). SQLite orders every integer before every text
# value, so `deleted_at < datetime('now', '-90 days')` is true for *every* row of an
# integer column and the statement deletes the whole table - in production that wiped
# channels, api_keys, projects and users in a single run. Compare each column with a
# cutoff of its own type, and never read the 0 sentinel as a deletion date.
echo -n "  Purging soft-deleted records (>90 days)... "
TOTAL=0
for TABLE in users projects channels api_keys models prompts prompt_protection_rules channel_model_prices channel_override_templates api_key_profile_templates oidc_identities roles provider_quota_statuses; do
    if ! sqlite3 "$DB_PATH" "SELECT 1 FROM ${TABLE} LIMIT 0;" &>/dev/null; then
        continue
    fi
    DEL_COL_TYPE=$(sqlite3 "$DB_PATH" "SELECT lower(type) FROM pragma_table_info('${TABLE}') WHERE name = 'deleted_at';")
    if [ -z "$DEL_COL_TYPE" ]; then
        continue
    fi
    case "$DEL_COL_TYPE" in
        *int*) DEL_WHERE="deleted_at > 0 AND deleted_at < CAST(strftime('%s', 'now', '-90 days') AS INTEGER)" ;;
        *)     DEL_WHERE="deleted_at IS NOT NULL AND deleted_at < datetime('now', '-90 days')" ;;
    esac
    DELETED=$(sqlite3 "$DB_PATH" "
        DELETE FROM ${TABLE}
        WHERE ${DEL_WHERE};
        SELECT changes();
    ")
    TOTAL=$((TOTAL + DELETED))
done
echo " done (${TOTAL} rows)"

echo ""
echo "--- Running VACUUM to reclaim disk space ---"
echo "  WARNING: VACUUM requires exclusive access to the database."
echo "  If AxonHub is running, stop it first to avoid lock contention."
read -p "  Continue with VACUUM? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "  Skipped VACUUM. You can run it manually later:"
    echo "    sqlite3 \"$DB_PATH\" \"VACUUM;\""
else
    echo "  This may take a while for large databases..."
    sqlite3 "$DB_PATH" "PRAGMA journal_mode=WAL; VACUUM;"
    echo "  VACUUM complete."
fi

echo ""
echo "--- Table sizes after cleanup ---"
if sqlite3 "$DB_PATH" "SELECT * FROM dbstat LIMIT 1;" &>/dev/null; then
    sqlite3 "$DB_PATH" "
    SELECT
        name AS table_name,
        printf('%.2f MB', SUM(pgsize) / 1024.0 / 1024.0) AS size_mb
    FROM dbstat
    WHERE name IN (SELECT name FROM sqlite_master WHERE type='table')
    GROUP BY name
    ORDER BY SUM(pgsize) DESC;
    "
fi
echo ""
echo "DB Size:   $(du -h "$DB_PATH" | cut -f1)"
echo ""
echo "============================================"
echo "  Cleanup complete!"
echo "============================================"
echo ""
echo "IMPORTANT: Enable auto-cleanup in AxonHub to prevent this from happening again:"
echo "  1. Go to Settings > Storage Policy"
echo "  2. Enable cleanup for 'requests' (recommended: 7-30 days)"
echo "  3. Enable cleanup for 'usage_logs' (recommended: 30-90 days)"
echo "  4. Ensure 'Vacuum' is enabled"
