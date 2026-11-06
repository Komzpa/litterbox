#!/bin/sh
set -eu
[ "$PGPORT" != 5435 ]
createdb litterbox_test_label_state
export PGDATABASE=litterbox_test_label_state
export DATABASE_URL="host=$PGHOST port=$PGPORT dbname=$PGDATABASE user=$PGUSER"
export CARD_TEST_POSTGRES=1
cd .tmp/baseline/server
go test -json ./internal/gmailsync ./internal/ops ./internal/ingest ./internal/sources/agents -run "TestHistoryReinboxWithoutNewMessage|TestInitialAbsenceRequiresProviderConfirmation|TestStaleOwnerEditsKeepVisibleConflictAndJournalProjection|TestIngestFilesAndCardBodies|TestFullResultSurvivesSummaryLimit" -timeout=90s -count=1
