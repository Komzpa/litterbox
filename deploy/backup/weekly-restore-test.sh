#!/usr/bin/env bash
set -euo pipefail

: "${BACKUP_DIR:?set BACKUP_DIR to the local backup directory}"
: "${RESTORE_TEST_ADMIN_URL:?set RESTORE_TEST_ADMIN_URL to an isolated disposable PostgreSQL instance}"
: "${RESTORE_TEST_URL:?set RESTORE_TEST_URL to the disposable litterbox_restoretest database on that instance}"

# Guard against accidentally restoring to a live application database.
case "$RESTORE_TEST_URL" in */litterbox_restoretest|*/litterbox_restoretest\?*) ;; *) echo "RESTORE_TEST_URL must target database litterbox_restoretest" >&2; exit 2;; esac
latest=$(find "$BACKUP_DIR" -maxdepth 1 -type f -name 'litterbox-*.dump' -printf '%T@ %p\n' | sort -nr | sed -n '1s/^[^ ]* //p')
if [[ -z "$latest" ]]; then echo "no backup dump found" >&2; exit 1; fi

# This exact disposable database is dropped both before and after restore; never
# point these variables at the production database.
dropdb --maintenance-db="$RESTORE_TEST_ADMIN_URL" --if-exists litterbox_restoretest
createdb --maintenance-db="$RESTORE_TEST_ADMIN_URL" litterbox_restoretest
cleanup() { dropdb --maintenance-db="$RESTORE_TEST_ADMIN_URL" --if-exists litterbox_restoretest; }
trap cleanup EXIT
pg_restore --no-owner --exit-on-error --dbname="$RESTORE_TEST_URL" "$latest"
# The check must not be foldable into a constant: `CASE WHEN <immutable predicate>
# THEN 1 ELSE 1/0 END` is folded by the planner and raises division by zero on a
# perfectly good restore. A DO block raises only when the restored schema is
# actually missing a required table.
psql --no-psqlrc --set=ON_ERROR_STOP=1 "$RESTORE_TEST_URL" <<'SQL'
DO $$
BEGIN
    IF to_regclass('public.tenants') IS NULL OR to_regclass('public.cards') IS NULL THEN
        RAISE EXCEPTION 'restored database is missing required tables tenants/cards';
    END IF;
END
$$;
SQL
psql --no-psqlrc --tuples-only --set=ON_ERROR_STOP=1 "$RESTORE_TEST_URL" \
  --command="SELECT 'restored rows: cards=' || (SELECT count(*) FROM cards) || ' ops=' || (SELECT count(*) FROM ops) || ' devices=' || (SELECT count(*) FROM devices) || ' accounts=' || (SELECT count(*) FROM accounts)"
echo "weekly restore test passed: $latest"
