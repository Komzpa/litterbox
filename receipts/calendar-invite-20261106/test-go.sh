#!/bin/sh
# Run from repository root: pg_virtualenv sh receipts/calendar-invite-20261106/test-go.sh [grants]
set -eu
[ "$PGPORT" != 5435 ]
export CARD_TEST_POSTGRES=1 BUNDLE_TEST_POSTGRES=1
if [ "${1:-}" = grants ]; then
    unset DATABASE_URL
    export PGDATABASE=postgres
    cd server
    go test ./internal/ops -run 'TestAllPublicTablesGrantAppInsert|TestJournalCreateAsAppRole|TestCreateReminderAsAppRole' -count=1 -v
else
    createdb litterbox_test_calendar_invite
    export PGDATABASE=litterbox_test_calendar_invite
    export DATABASE_URL="host=$PGHOST port=$PGPORT dbname=$PGDATABASE user=$PGUSER"
    cd server
    go test ./... -skip 'TestAllPublicTablesGrantAppInsert|TestJournalCreateAsAppRole|TestCreateReminderAsAppRole' -timeout=240s -count=1
fi
