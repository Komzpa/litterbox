#!/bin/sh
set -eu
[ "$PGPORT" != 5435 ]
createdb litterbox_test_agent_state
export PGDATABASE=litterbox_test_agent_state
export DATABASE_URL="host=$PGHOST port=$PGPORT dbname=$PGDATABASE user=$PGUSER"
export AGENT_TEST_POSTGRES=1
cd server
go test -json ./internal/sources/agents -run TestPostgresCardsIntegration -count=1
