#!/bin/sh
# Run from repository root. Expected result: six missing-invitation failures.
set -eu
root=$(pwd)
mkdir -p .tmp
baseline=$(mktemp -d "$root/.tmp/calendar-baseline-XXXXXX")
git archive 8b8a70e server | tar -x -C "$baseline"
mkdir -p "$baseline/server/internal/mailhtml/testdata/calendar"
cp server/internal/mailhtml/testdata/calendar/*.eml "$baseline/server/internal/mailhtml/testdata/calendar/"
python3 - "$baseline/server/internal/mailhtml/calendar_test.go" <<'PY'
from pathlib import Path
import sys
source = Path('server/internal/mailhtml/calendar_test.go').read_text()
# The first test uses only v46's Parse API; later tests use new typed fields.
Path(sys.argv[1]).write_text(source.split('func TestCalendarReplyFloatingAndMalformed')[0])
PY
cd "$baseline/server"
go test ./internal/mailhtml -run TestCalendarMIMEFixtures -count=1
