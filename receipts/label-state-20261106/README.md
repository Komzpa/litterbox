# Gmail state and retained owner content — local lane

Rebased integration onto main `3bd6a6b`: see [rebase.md](rebase.md) for the
combined migration mapping, conflict decisions and fresh verification. The original
lane results below are historical; they do not describe current production state.

Worktree: `/home/kom/litterbox-releases/fix-label-state-wt`, branch
`fix/label-state`; baseline `699f88f40ee3512e6c55c511500258111700d974`
(v44 published). Input `dataloss-audit.md` SHA-256:
`bce598a1ad23b7021be65d086641e1a158846dfd13b8aec96a86318b1e555f24`.
The audit was read in full and its hash checked.

Owner: “этот сценарий вообще то data loss. проверь где ещё такие проблемы” …
“чини”. Content the owner cannot see counts as data loss. This lane owns Gmail
label state, initial sweep, note/journal conflicts, ingest files and agent bodies.
It does not change MIME parsing, mailhtml/mailbody or the mail-content lane.
Authority: local commits only; no push, PR, release, production DML, Gmail writes,
service changes, owner desktop or phone. Production access below was SELECT only.

## Behavior and regression evidence

| Row | Before on 699f88f | After / test |
|---|---|---|
| Gmail re-inbox without new message | Remains archived; regression fails | `TestHistoryReinboxWithoutNewMessage` passes. History INBOX additions/removals are tracked independently of message arrivals; a provider read under the same thread guard as operations confirms current labels. |
| Stale labels versus owner archive | Message labels alone cannot identify a new provider intent | `TestLabelStateReconciliationAndIntentBoundary`, `TestArchiveRecordsProviderHistory` pass. Persist the pending local removal separately from provider INBOX state and its acknowledged history boundary. An older event cannot reverse the archive; a later INBOX addition can. History IDs remain decimal strings/PostgreSQL numeric, including values above 2^53. `TestArchiveSurvivesSyncUntilNewMessage` and `TestPendingSnoozeRetainsStateOnStaleInbox` retain archive/snooze and allow a new message to wake the thread. |
| Initial sweep | Missing from paginated list hides the card; moving-snapshot regression fails | `TestInitialAbsenceRequiresProviderConfirmation/moving_snapshot` passes; `/confirmed_archive` passes both before and after. Every absent active thread is checked through metadata `threads.get`; only confirmed absence of INBOX (or 404) hides it, and the reason is stored. A provider error leaves it for retry. |
| Owner note/journal edits | Old base silently replaces newer text; both regression subtests fail | `TestStaleOwnerEditsKeepVisibleConflictAndJournalProjection/notes` and `/journal` pass. Revision/base-version protects writes; stale text becomes an open conflict card, current text remains. Journal edits update their card projection in the same transaction. Legacy clients without a base use zero and get a conflict after the first revision. Operation replay creates no duplicate note conflict. |
| Ingest files | Upsert omitting files deletes accepted body; regression fails | `TestIngestFilesAndCardBodies` passes: omission leaves bytes intact; additive files preserve accepted data; only `files_mode=replace` or `delete` removes prior files. Legacy HTML stays readable. A later file-only update also retains an accepted full body. |
| Agent result | Result tail beyond list summary is absent; regression fails | `TestFullResultSurvivesSummaryLimit` passes for OMP and Codex; full `body` passes through ingest-agents separately from summary. `TestIngestFilesAndCardBodies` also proves the final paragraph through the real reader HTTP handler. |
| Qt offline note base | Missing base revision (0 instead of 7); baseline test fails | `noteBaseVersionIsDurableAndAdvances` passes: queued bases 7 and 8 survive restart and stale snapshots. Outbox payloads retain those exact revisions. |

Baseline was a disposable `git archive` of 699f88f under `.tmp/baseline`, not a
second worktree. Only compatible regression tests were copied into it, never
production changes or new migrations. For label tests the two original public
path regressions were copied (the additional internal-helper tests cannot compile
against the old implementation). Nine Go tests/subtests ran: eight failures
including failing parent groups, one passing confirmed-archive control. The Qt
baseline compiled and ran, then failed the base-revision assertion (expected 7,
received 0); it was not a compile-failure control. Log hashes are adjacent.

## Release-time reconciliation (not run here)

Apply migrations 028 through 031 before running the candidate server. The normal
Gmail sync worker calls `reconcileLabelState` before initial/history sync. It checks
at most 50 unchecked archived mail threads per account per poll, using provider
metadata. Successful checks set `gmail_state_checked`, `gmail_provider_inbox` and
`gmail_state_reason`. Provider-INBOX legacy cards reopen; provider-archived cards
stay archived. A pending local archive remains protected until its removal is
observed or a later INBOX addition supersedes its acknowledged history boundary.
Failures do not mark rows checked. Restart resumes remaining unchecked rows.
No direct production backfill is needed. At the default 60-second poll, three
accounts check up to 150 rows per pass; actual completion depends on account
sizes, provider latency and failures.

Production SELECT, **2026-11-06 12:19:43 +04**, all archived mail cards:

- 2,426 total candidate checks; all 2,426 satisfy audit Q8 (stored message INBOX).
- 1,502 have an individual archive/spam operation receipt; 924 have no such receipt.
- 0 archived mail cards lack stored INBOX; 0 nonempty card notes;
  0 journal entries with `updated_at > created_at`.

These are cards, not messages. See `counts.sql` for exact inclusion rules. The
original audit Q8 was 2,436 at 00:52–00:56; this is a later snapshot, not a claim
that ten cards were repaired by this lane. Expected one-time checked/metadata
updates on this frozen population: **2,426**. Expected state reopen count:
**unknown, bounded 0–2,426**, determined by live Gmail, not by stored labels.

No operational read-only Gmail sampling command was found in this checkout's
`tools`, `deploy` or `server/cmd`; the internal API client has metadata reads, but
no operator command with an authorized credential loader. No live sample was
made and no credentials were guessed. Stored history is only the account cursor;
individual label transitions were not retained by the old server. The 1,502
receipts establish a prior local action, not current Gmail membership; the 924
without receipts may be genuine re-inbox cases or historical sweep archives.
Neither group can honestly be classified as current Gmail INBOX/stale without
provider reads. The release worker supplies that missing evidence per row.

After release, SELECT the persisted verdicts without changing data:

```sql
SELECT gmail_state_reason, gmail_provider_inbox, state, count(*)
FROM cards
WHERE source='mail' AND gmail_state_checked
GROUP BY 1,2,3 ORDER BY 1,2,3;
SELECT count(*) FROM cards
WHERE source='mail' AND state='archived' AND NOT gmail_state_checked;
```

## Validation

Main Go suite: `pg_virtualenv receipts/label-state-20261106/test-full.sh`.
Every ordinary DB test selects `litterbox_test_label_state`, with an explicit
port-5435 refusal. CARD/BUNDLE PostgreSQL tests and production-worker tests are
enabled; the server test binary is built locally from this candidate.
Result: 375 passed, 11 skipped tests/subtests. The separate AGENT PostgreSQL
integration test passed, replacing one skip; three grant tests passed separately
in another disposable cluster. Combined: **379 passed, 10 opt-in native Qt/Gmail
consumer tests skipped** (require `LB_QT_UNDO_BINARY`/`LB_QT_UNDO_OUTPUT`).
No Go test failed. Grant tests create `litterbox_test_journal_grants_*` databases;
their existing guard deliberately requires an unset DATABASE_URL and the
throwaway cluster's postgres admin connection.

Full Qt build: `cmake -S qt -B .tmp/qt-full -G Ninja`,
`cmake --build .tmp/qt-full -j4`. Full offscreen CTest command:
`QTWEBENGINE_DISABLE_SANDBOX=1 QT_QUICK_BACKEND=software ctest --test-dir .tmp/qt-full --output-on-failure -j1`.
The first parallel run passed 31, skipped restart_service_survival and failed
scrollresilience plus one CardStore `durable.open(path)` assertion. The serial
full rerun also passed 31 and skipped restart_service_survival, with failures in
scrollresilience and CardStore's 5-second drain limit (5.843 seconds).
The SQLite-open case passed on retry. A matched isolated drain comparison passed
on both baseline (2.528 seconds) and candidate (2.319 seconds), each with 32
requests in flight. Host/storage timing variability is an inference, not a proven
root cause of either full-run failure.

The complete unchanged CardStore CTest was then rerun with
`TMPDIR=/dev/shm/litterbox-label-state-tests-699f88f`, keeping assertions and limits
intact: **PASS**, 104.70 seconds. No test was filtered inside CardStore. Combined
coverage of the full 34 registered tests: **32 passed**, known
restart_service_survival skipped and scrollresilience timing exception retained.
The ordinary-filesystem full suite was not wholly green; the full CardStore
retry supplies its passing result. Logs retain both failures and retries.

Not delivered: no release or deployed repair, as explicitly forbidden by this
lane. The published v44 runtime and production schema remain untouched. The
original pending mail-content integration and migration collision check are now
recorded in [rebase.md](rebase.md). Local generated logs/builds/captures are untracked.
