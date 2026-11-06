# Label-state integration onto main — 2026-11-06

Objective: rebase the whole `fix/label-state` branch from
`a9c6d529d8c9e85f6f6420aab0e30fb96106e038` (base `699f88f`) onto
`3bd6a6b076db610fc2fb5e815e59053bcf2c3b56`, retaining both lanes.
Rebased implementation: `3868cf310ec43ace50677331fcf17d3085a19c2d`.
Authority: local rebase, tests and commits only; no push or release. No production
DB access, /etc, services, owner desktop/phone, or other-worktree mutations.
The mail-content receipt was read from the explicitly named other worktree only.
Existing generated files were left untracked. Original author and author date
were retained by rebase; the original branch commit is one whole commit.

## Resolution decisions

- `server/internal/gmailsync/sync.go`: retain main's cached-content reads and
  preparation/persistence, then use the fetched thread's actual INBOX labels.
  Retain label history transitions, thread guards and owner intent boundaries.
  Account polls retain both reconciliation (50 threads/account/poll) and content
  repair (10 cards, 20 messages/card, 60 seconds, one-hour retry cooldown).
- `server/internal/gmailsync/sync_test.go`: retain main's lexical migration
  discovery so both lanes' complete schema reaches every sync fixture.
- `qt/src/CardStore.cpp`: automatic merge retained authenticated file transport,
  recipient/file persistence, and label-state's durable note base revisions.
- `qt/tests/tst_cardstore.cpp`: automatic merge retained both lanes' tests.
- `server/internal/gmailsync/content_test.go`: strengthen the existing bounded
  repair regression to assert 12 label checks run exactly once across the same
  two account polls as the 10+2 content attempts, retaining failed-fetch cooldown
  and unchanged archive state controls.

Migration mapping: `028_content_preservation` → `029_content_preservation`,
`029_gmail_label_state` → `030_gmail_label_state`,
`030_owner_revisions` → `031_owner_revisions`. All tracked references updated,
including journal tests, deployment guidance and the original receipt.
Main's `028_mail_content` creates message metadata/message_parts and the cards
content-check timestamp; `029_content_preservation` adds source_files, legacy_html
and source_body to card_bodies. There are no duplicated columns/tables to merge.

## Proof commands

Logs are local in `.tmp/rebase-proof/`; hashes are in `rebase-log-sha256.txt`.
The Qt build uses `.tmp/qt-full`, reconfigured and rebuilt from this worktree.

```sh
(cd server && go build -o ../.tmp/litterbox-test ./cmd/litterbox)
cmake -S qt -B .tmp/qt-full -G Ninja
cmake --build .tmp/qt-full -j4
pg_virtualenv -o port=55441 env -u DATABASE_URL sh -c '
  test "$PGPORT" != 5435
  createdb litterbox_test_rebase_migrations
  for f in server/db/*.sql; do
    PGDATABASE=litterbox_test_rebase_migrations psql -X -v ON_ERROR_STOP=1 -f "$f" || exit
  done'
pg_virtualenv -o port=55442 env -u DATABASE_URL sh -c '
  test "$PGPORT" != 5435
  createdb litterbox_test_rebase
  export PGDATABASE=litterbox_test_rebase CARD_TEST_POSTGRES=1 BUNDLE_TEST_POSTGRES=1 AGENT_TEST_POSTGRES=1
  export LITTERBOX_TEST_BINARY="$PWD/.tmp/litterbox-test"
  cd server
  go test -json ./... -skip "TestAllPublicTablesGrantAppInsert|TestJournalCreateAsAppRole|TestCreateReminderAsAppRole" -timeout=240s -count=1'
pg_virtualenv -o port=55443 env -u DATABASE_URL PGDATABASE=postgres CARD_TEST_POSTGRES=1 sh -c '
  test "$PGPORT" != 5435
  cd server
  go test -json ./internal/ops -run "TestAllPublicTablesGrantAppInsert|TestJournalCreateAsAppRole|TestCreateReminderAsAppRole" -count=1'
mkdir -p /dev/shm/litterbox-rebase-tests
TMPDIR=/dev/shm/litterbox-rebase-tests QT_QPA_PLATFORM=offscreen QTWEBENGINE_DISABLE_SANDBOX=1 QT_QUICK_BACKEND=software ctest --test-dir .tmp/qt-full --output-on-failure -j1
```

Fresh migration replay: PASS, all 30 existing SQL files numbered 001..031
(014 is absent in the repository), applied in lexical order.
Go final full run plus separate grants: 240 top-level tests / 401 cases including
subtests PASS, 10 optional native Archive consumer tests SKIP; zero failures.
All 26 named Go regressions from both lane receipts passed, including their
subtests. See `rebase-regressions.txt` for exact names and results.

Full Qt CTest: **32 passed**, `restart_service_survival` skipped,
`scrollresilience` failed (accepted exceptions only), 553.19 seconds. The latter
reports 308-member expansion at 444 ms against its <200 ms limit; archive storm,
wheel input, recluster and snapshot anchor controls passed. No assertion or test
was filtered or weakened. CardStore passed in 103.73 seconds. All four named Qt
regressions in the lane receipts passed in the current full run.

Local task is complete; no push, publication, deployment, native Android proof
or owner-device action was attempted. Ten optional native Go Archive consumer
cases remain skipped as recorded above. Release and production repair remain
outside this task's explicit authority.
