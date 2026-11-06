#!/bin/sh
set -eu
[ "$PGPORT" != 5435 ]
createdb litterbox_test_label_state
export PGDATABASE=litterbox_test_label_state
export DATABASE_URL="host=$PGHOST port=$PGPORT dbname=$PGDATABASE user=$PGUSER"
export CARD_TEST_POSTGRES=1
export BUNDLE_TEST_POSTGRES=1
export LITTERBOX_TEST_BINARY="$(pwd)/.tmp/litterbox-test"
cd server
go test -json ./... -skip "TestAllPublicTablesGrantAppInsert|TestJournalCreateAsAppRole|TestCreateReminderAsAppRole" -timeout=180s -count=1
