# Reader-undo successor proof — GREEN: reader A→B continuation + Undo keeps B + offline queued-cancel

## Green run (2026-10-19, gate ok)

- Named command: `go test -count=1 -run "^TestArchiveUndoNativeConsumer$" ./internal/gmailsync/`
- Working directory: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/server`
- Bootstrap: `env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash -c ...` with
  `CARD_TEST_POSTGRES=1`,
  `LB_QT_UNDO_BINARY=/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`,
  `LB_QT_UNDO_OUTPUT=/home/kom/proj/ai_pr/litterbox-reader-undo-run-20261019`.
- Exit status: **ok** (`ok github.com/Komzpa/litterbox/server/internal/gmailsync 46.029s`,
  also ok 46.142s on the preceding identical run). Private PG cluster `18/regress`
  created/dropped by pg_virtualenv (non-fatal `initdb` locale note as before).

## Artifact identity (pinned BEFORE run, verified AFTER, unchanged)

- Frozen private binary `/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`
  SHA-256 `c8c4ae891d112f2dcdf073e1bd4d5d148be93e76f5a911f6c9ef3bfe5de914cb` before and after.
- Frozen QML copies: `src/InboxView.qml` `19deee88a91c2e1905fb809938bf192d9f6b45ff47919aeb1edad3cbcff44811`,
  `src/MailDetailPage.qml` `8fb5a876abef36862e2ed47a38440270db04b9690c7edb58340a452adb8ba6aa`, unchanged.
- Canonical consumer `qt/tests/archive_undo_consumer.py`
  SHA-256 `5cfa82981092d3454640647ccd2cf77c92c3ea9417f8d967ad3c940acbb2075b`
  (this run's updated reader-journey + offline-leg revision; the change under proof).
- Fixture (prior worker, unchanged by this run):
  `server/internal/gmailsync/archive_undo_test.go` successor B is `source='mail'` + `card_bodies`
  row (`<p>B</p>`, `thread-2`), presented order pinned `[A,B]` via `created_at/sort_at`.
- Candidate Git HEAD observed: `e62b81c docs: append post-push readback to release publication receipt`.
  No shared-OUT binary built/installed/executed; no Git mutation, deploy, phone, or owner-desktop
  interaction by this run.

## Exact consumer entry and wiring exercised

- Reader open: capture-script `{"openCard": <A>}` (`qt/src/main.cpp:608-609`) synthesizes a click
  on `openCard-<cardId>`; row `onReadCachedRequested` → `window.openReaderEntry(entry)` →
  `openPage("MailDetailPage", ...)` owns identity (`InboxView.qml:403-413`):
  `readerOpen=true`, `selectedCardId=A`.
- Reader archive: physical click on `archiveMail` (`qt/pages/MailDetailPage.qml:315-321`) →
  `mailActions.primaryAction()`; `onCardHandled` captures
  `root.requestArchiveContinuation(root.cardId)` BEFORE close, then `closeReader()`,
  `undo(operation, action)` (= `window.showArchiveUndo`), then `next()` opens B
  (`MailDetailPage.qml:354-366`; continuation `qt/qml/InboxView.qml:423-460`, strictly after
  the archived card, skips collapsed/hidden and non-reader rows, opens first survivor).
- Undo with reader open: `window.undoBundleArchive()` by eval — the exact function the
  `bundleArchiveUndoButton.onClicked` calls (`qt/qml/InboxView.qml:935`); the bar itself is
  visible only at `stack.depth === 1`, so the eval exercises the same function while the
  successor reader is open (same queued vs acknowledged branches, `InboxView.qml:347-365`).
- Offline leg: explicit `stack.pop()` to inbox first (reader closes on offline transition —
  observed, see Retro), then PHYSICAL clicks: `doneButton-<B>` (CardActions.primaryAction →
  store.enqueueOp queues while offline) and `bundleArchiveUndoButton` (real bar button at
  depth 1 → undoQueuedOp cancels the queued op).
- Server wiring unchanged: `auth.Middleware`, `cards.Handler`, `ops.API`,
  `gmailsync.RegisterOps`; only Gmail is loopback HTTP (see `server-receipt.json`).

## Observed behavior — main reader journey (6 states, exit 0)

IDs: A (mail, `thread-1`), B (mail, `thread-2`). Initial order `[A, B]`.

| # | ids | pending | undo | readerOpen | selected | note |
|---|-----|---------|------|------------|----------|------|
| 0 | [A,B] | 0 | no | false | — | loaded, guard ok |
| 1 | [A,B] | 0 | no | **true** | A | reader A open — entry works |
| 2 | [B] | 0 | yes `Archived` | **true** | **B** | reader archive ACK fast, **B continued open** |
| 3 | [A,B] | 1 | no | **true** | **B** | Undo in flight, reader still on B |
| 4 | [A,B] | 0 | no | **true** | **B** | settled, order restored, **B kept** |
| 5 | [A,B] | 0 | no | **true** | **B** | after snapshot refresh, order kept |

- Qt cache order after run `[A,B]` equals initial; outbox empty.
- Server receipt: exactly two durable ops (`archive`, then `gmail.label_add(INBOX)` on A) and
  exactly two Gmail calls, both `thread-1` with `Bearer test-access`: remove INBOX, then add
  INBOX — ACK inverse thread/token correct for reader-initiated archive, ACK 2-call asserts hold.

## Observed behavior — offline queued-cancel leg (4 legs, same run, exit 0)

| leg | ids | pending | undoAvailable | note |
|-----|-----|---------|---------------|------|
| offline-start | [A,B] | 0 | no | inbox, reader popped |
| offline-archived | [A] | 1 | yes | B archived OFFLINE via physical `doneButton`, op queued |
| offline-undone | [A,B] | 0 | no | physical Undo button cancelled the queued op |
| offline-settled | [A,B] | 0 | no | back online + refresh, order kept, outbox empty |

- Server receipt still exactly 2 durable / 2 Gmail calls AFTER the cancel leg ran inside the same
  process: the offline archive→Undo produced **zero Gmail calls** (queued-cancel proof), and the
  ACK 2-call assert scope is preserved.

## Negative controls read back (same run)

- `GET /v1/cards` with token → **200** (A mail `has_body:false` first, B mail `has_body:true`
  second — presented order `[A,B]` as the fixture pins).
- `GET /v1/cards` without token → **401** `{"error":"unauthorized"}`.
- Provider rejection re-proved fresh this session (2026-10-19 ~18:19+0400) at its owned level:
  `go test -count=1 -run "^TestArchiveUndoFailureAndIsolation$" ./internal/gmailsync/` →
  **PASS** (0.052s, same pg_virtualenv harness): `fail=true` inverse → 422
  `fixture Gmail denied`, no receipt persisted, unrelated-label and foreign-tenant isolation hold.
  A consumer-level `fail=true` reader leg needs a Go-side driver leg (fixture `f.fail` flag)
  owned by the fixture owner — still open (see below); it cannot run inside the native consumer
  without breaking the preserved ACK 2-durable/2-call assert scope, since even a rejected flow
  persists its archive op.

## Scope notes

- KEEP-B evidence is **selection-on-B** (non-vacuous: selection moved A→B on continuation and
  stayed B through Undo + settle). B body *HTML* is not asserted through the UI: the test mux
  registers no `GET /v1/cards/{id}/body` route (served by the `mailbody` handler in production,
  not wired into this harness), so Qt `mail_bodies` stays empty and `stack.currentItem.html`
  reads `''`. Owner option if body-content proof is wanted: register the mailbody route in the
  test mux — harness-only change, no product behavior.
- Middle/filtered/batch/end successor semantics still PASS per
  `receipts/archive-scope-successor-proof.md`; not repeated here. The native final cohort
  remains separate/later.

## Retro

- First red of this run series was exact and environmental, not product:
  `Capture script output directory must be empty and separate from the test profile`
  (driver exit 2, zero states) — stale PNG/JSON captures from the prior throwaway run occupied
  the captures dir. Fixed by clearing the dedicated captures dir before the run; later made
  robust by isolating profile/script/captures into sibling dirs (`<name>-profile`,
  `<name>-script.json`, `<name>-captures`) so run-dir receipts can never collide with the
  harness's empty-dir guard.
- Two self-inflicted driver bugs, both caught by the oracle, none retried blindly:
  (1) Python implicit string concatenation swallowed the `""` fallback in the state expr
  (`: "")` → `: )`), breaking the QML eval — fixed with single-quoted JS `''`;
  (2) `receipt["legs"]` added in-memory but the receipt file was written before legs were
  parsed — fixed write order so the on-disk receipt carries legs.
- Offline-leg entry learned from evidence, not guessed: the reader closes on the offline
  transition (screenshot 05 showed inbox while the flag still read open), and
  `stack.currentItem.mailActions` is unreachable across component scope from eval — so the leg
  pops to the inbox first and uses physical `doneButton` + `bundleArchiveUndoButton` clicks.
- Reused the exact managed harness (native consumer + loopback Gmail + private pg_virtualenv +
  private Xvfb/HOME, frozen 555 binary) instead of inventing credentials; no product source
  touched — only the canonical consumer driver (the assigned path).

## Artifacts

- Run dir: `/home/kom/proj/ai_pr/litterbox-reader-undo-run-20261019/` (gate.log, gate-exit.txt,
  native.log, driver-receipt.json with states+legs, server-receipt.json,
  authenticated-before.json, unauthenticated-control.json, 7 PNG+JSON captures, private profile).
- Sibling dirs: `/home/kom/proj/ai_pr/litterbox-reader-undo-run-20261019-{profile,script.json,captures}`
  plus `...-run-20261019-{home,runtime,config,data,cache}` XDG dirs inside the run dir.
- Frozen binary+source: `/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/`.
- Changes by this run: this receipt + `qt/tests/archive_undo_consumer.py` (reader journey +
  offline leg). No production source edit, build, install, Git, deploy, phone, or
  owner-desktop interaction.

---

# Prior baseline (stale red run — kept for history, NOT the current proof)


## Command, working directory, exit

- Working directory for Go: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/server`.
- Named command: `go test -count=1 -run "^TestArchiveUndoNativeConsumer$" ./internal/gmailsync/`.
- Bootstrap: `env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash -c ...` with
  `CARD_TEST_POSTGRES=1`,
  `LB_QT_UNDO_BINARY=/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`,
  `LB_QT_UNDO_OUTPUT=/home/kom/proj/ai_pr/litterbox-reader-undo-run-20261019`.
- Private PostgreSQL cluster `18/regress` created and dropped by pg_virtualenv (log notes
  `initdb: could not find suitable text search configuration for locale "be_BY.UTF-8"`, non-fatal).
- Start: 2026-10-19T17:45:27+0400. Finish: 2026-10-19T17:45:59+0400.
- Exit status: **1** (`gate-exit.txt` = 1; `--- FAIL: TestArchiveUndoNativeConsumer ... native oracle: exit status 1`).
  The Qt process itself exited 0; the throwaway reader verdict exited 1. This is a **true
  negative**, not a pass: the reader never continued to B.

## Artifact identity checked (before and after)

- Frozen private binary (pinned BEFORE run, used for the whole run, no shared OUT build/install):
  `/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`
  SHA-256 `c8c4ae891d112f2dcdf073e1bd4d5d148be93e76f5a911f6c9ef3bfe5de914cb` before and after.
  (First freeze used mode 444 which removed the exec bit and failed with
  `dbus-run-session: ... Permission denied`; fixed to mode 555 without changing bytes;
  rerun used the identical hash.)
- Frozen source copies: `src/InboxView.qml`
  `19deee88a91c2e1905fb809938bf192d9f6b45ff47919aeb1edad3cbcff44811`,
  `src/MailDetailPage.qml` `8fb5a876abef36862e2ed47a38440270db04b9690c7edb58340a452adb8ba6aa`,
  unchanged after the run (same hashes in the live tree).
- Throwaway driver `qt/tests/archive_undo_consumer.py` temporarily extended for this run only,
  then byte-restored to `b7227dd9d7b3dff4cb04a389cf505fd0406d64731f3932972a33999fcb00339b`
  (`git status` shows only pre-existing worktree modifications; the driver is unmodified).
- Candidate Git HEAD observed earlier: `e62b81c8cd3b541949fe0aa35fee290979f46563`.
- No shared-OUT binary was built, installed, or executed; no Git mutation, deploy, owner
  desktop/phone interaction.

## Exact consumer entry and wiring exercised

- Reader open: capture-script `{"openCard": <A>}` (`qt/src/main.cpp:608-609`) synthesizes a click
  on `openCard-<cardId>` (`qt/qml/InboxView.qml:1707`); the row's `onReadCachedRequested`
  calls `window.openReaderEntry(entry)`, which `openPage("MailDetailPage", ...)` owns
  (`InboxView.qml:403-413`), setting `readerOpen=true` and `selectedCardId=A`.
- Reader archive: `archiveMail` button (`qt/pages/MailDetailPage.qml:315-323`) calls
  `mailActions.primaryAction()`; `onCardHandled` captures
  `root.requestArchiveContinuation(root.cardId)` BEFORE close, then `root.closeReader()`,
  `undo(operation, action)` (= `window.showArchiveUndo`), then `next()` opens B
  (`MailDetailPage.qml:354-366`; continuation `InboxView.qml:423-460` scans strictly after the
  archived card, skips hidden collapsed bundle members and rows where
  `card.source !== "mail" && !card.has_body`, and opens the first survivor via `openReaderEntry`).
- Undo while a reader is open: `bundleArchiveUndoButton.onClicked` is exactly
  `window.undoBundleArchive()` (`InboxView.qml:935`), but the bar is visible only at
  `stack.depth === 1` (`InboxView.qml:901`). The throwaway therefore invoked the identical
  function `window.undoBundleArchive()` by eval while the successor was expected open —
  same function, same queued (`mailArchiveUndoOp` → `store.undoQueuedOp`) vs acknowledged
  (`bundleArchiveToken` → `store.undoBundleArchive`) branches (`InboxView.qml:347-365`).
- Server wiring unchanged: `auth.Middleware`, `cards.Handler`, `ops.API`,
  `gmailsync.RegisterOps`; only Gmail is loopback HTTP (see `server-receipt.json`).

## Observed behavior (7 reader states)

IDs: A (mail) = `c66cce9b-...`, N (manual neighbor) = `967153ea-...`. Initial order `[N, A]`.

| # | ids | pending | readerOpen | selected | note |
|---|-----|---------|------------|----------|------|
| 0 | [N,A] | 0 | false | — | loaded |
| 1 | [N,A] | 0 | **true** | A | reader A open — entry works |
| 2 | [N] | 0 | **false** | A | reader archive removed A, ACK fast, **reader CLOSED, no B** |
| 3 | [N] | 0 | false | A | settled, Undo available, message `Archived` |
| 4 | [N,A] | 1 | false | A | Undo restored order, in flight |
| 5 | [N,A] | 0 | false | A | settled |
| 6 | [N,A] | 0 | false | A | after snapshot refresh, order kept |

- Throwaway verdict (`reader-verdict.json`):
  `readerA=true, continuationB=false, undoRestoresOrder=false, keepsBSelected=true (vacuous —
  selection never left A because B never opened), ok=false`.
- Qt cache order after run `[N,A]` equals initial; outbox empty.
- Server receipt: exactly two durable ops (`archive`, then `gmail.label_add(INBOX)` on A) and
  exactly two Gmail calls, both `thread-1` with `Bearer test-access`: remove INBOX, then add
  INBOX. So the **reader-initiated ACK inverse carries the correct thread/token** — but it
  proves the inverse only, not the A→B→Undo succession.
- Negative controls read back: `GET /v1/cards` with token → 200 (lists N manual
  `has_body:false` first, A mail `has_body:false` second); without token → 401
  `{"error":"unauthorized"}`. Provider rejection (`fail=true` → 422, no receipt) is covered
  only by `TestArchiveUndoFailureAndIsolation` at API level, **not** through this consumer run;
  queued-cancel (offline archive → Undo, zero Gmail calls) is **not exercised** by this run.

## Why B never opens — fixture producer blockage (no production code touched)

`captureArchiveContinuation(A)` requires a candidate strictly after A that is presented,
expanded, and reader-openable (`source === "mail"` or `has_body`). The fixture provides:

- initial presented order `[N, A]` — A is last, so there is nothing after it;
- N is `source='manual'`, `has_body=false` (`authenticated-before.json`), i.e. explicitly
  skipped by the continuation filter even if ordering were fixed.

Hence the continuation returns null, `onCardHandled` gets `next=null`, the reader closes, and
no throwaway script can manufacture B. Per the gate rule this is reported — not worked around.

## Exact hunk for the code owner (fixture producer, NOT applied)

File: `server/internal/gmailsync/archive_undo_test.go`, in `TestArchiveUndoNativeConsumer`.
Current line:

```go
if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,title) VALUES($1,$2,'manual','native-neighbor','Untouched neighbor')`, f.tenant, neighbor); err != nil { t.Fatal(err) }
```

Replace with a reader-openable successor ordered AFTER the mail card (owner to bind to the
presented-order column used by `GET /v1/cards`; `sort_at` exists on `cards`, `has_body` comes
from `card_bodies` per `011_card_bodies.sql` / `cards/handler.go:152`):

```go
if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject) VALUES($1,$2,$3,'thread-2','mail','Neighbor','Successor B')`, f.tenant, neighbor, f.account); err != nil { t.Fatal(err) }
if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>B</p>')`, f.tenant, neighbor); err != nil { t.Fatal(err) }
// + order the successor AFTER f.card in the presented order of GET /v1/cards
//   (owner: set the sort column accordingly) so initial ids == [A, B].
```

Still missing after that hunk (for a later run): an offline queued-cancel leg asserting zero
Gmail calls on cancel, and a `fail=true` consumer leg asserting truthful rejection through the
reader. `TestArchiveUndoFailureAndIsolation` covers rejection only at API level.

## Scope notes

- Middle/filtered/batch/end successor semantics already PASS per
  `receipts/archive-scope-successor-proof.md`; they were not repeated here.
- Middle/filter/batch/end plus this reader run do not compose into final-cohort evidence.
  The native final cohort remains separate.
- Do NOT cite `receipts/archive-undo-successor-proof.md` (inbox-row Undo, STALE) or this run
  as reader proof. This run proves: reader-A opens, reader-archive closes without successor on
  this fixture, ACK inverse thread/token correct for reader-initiated archive, Undo restores
  inbox order without reopening a reader.

## Retro

Reused the exact managed harness (`TestArchiveUndoNativeConsumer` + loopback Gmail + private
`pg_virtualenv` + private Xvfb/HOME) instead of inventing credentials; froze an immutable
private binary copy and verified identity before/after instead of trusting the shared build;
extended only the throwaway driver and restored it byte-identically. Mistakes: first freeze
used mode 444 and cost one harness cycle with `Permission denied`; the `keepsBSelected` flag
in the throwaway reads vacuously true when B never opens — the states table above is the
honest record, not the flag. Stopping rule honored: exit 1 from the oracle is reported as a
fixture blockage with the exact owner hunk, not retried or claimed green.

## Artifacts

- Run dir: `/home/kom/proj/ai_pr/litterbox-reader-undo-run-20261019/` (gate.log, gate-exit.txt,
  start/finish, script.json, native.log, driver-receipt.json, reader-states.json,
  reader-verdict.json, server-receipt.json, authenticated-before.json,
  unauthenticated-control.json, 5 PNG+JSON captures, private profile).
- Frozen binary+source: `/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/`.
- Changes: this receipt only (plus the restored throwaway driver, byte-identical). No
  production source edit, build, install, Git, deploy, phone, or owner-desktop interaction.
