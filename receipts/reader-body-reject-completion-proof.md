# Reader body-route + fail=true Undo-rejection completion proof — GREEN

Closes the two open items from `receipts/reader-undo-successor-proof.md`:
(1) harness mail-body route registered so **B HTML is asserted** through the production
handler, and (2) a consumer-level **fail=true Undo-rejection leg** where the actual reader
shows the failure truthfully (no false success). Reuses the existing harness; small
fixture/driver changes only; no product behavior, Git, deploy, or owner-desktop/phone.

## Green run (2026-10-19, gate ok)

- Named command (ONE scoped gate, matches both tests):
  `go test -count=1 -v -run "^TestArchiveUndoNativeConsumer" ./internal/gmailsync/`
- Working directory: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/server`
- Bootstrap: `env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash -c ...` with
  `CARD_TEST_POSTGRES=1`,
  `LB_QT_UNDO_BINARY=/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`,
  `LB_QT_UNDO_OUTPUT=/home/kom/proj/ai_pr/litterbox-reader-evidence-run-20261019b`.
- Result: `--- PASS: TestArchiveUndoNativeConsumer (46.23s)`,
  `--- PASS: TestArchiveUndoNativeConsumerReject (21.27s)`,
  `ok github.com/Komzpa/litterbox/server/internal/gmailsync 67.504s`; **exit 0**.
- Private PG cluster `18/regress` created/dropped by pg_virtualenv (non-fatal `initdb` locale
  note as before). `operation rejected: ... 403 Forbidden: fixture Gmail denied` logged —
  the fail=true rejection path was actually exercised.

## Artifact identity

- Frozen private binary `.../litterbox-reader-undo-frozen-20261019/litterbox-qt-frozen`
  SHA-256 `c8c4ae891d112f2dcdf073e1bd4d5d148be93e76f5a911f6c9ef3bfe5de914cb`, used read-only,
  unchanged (no shared-OUT build/install/executed product; the Qt binary under test is the
  pinned frozen one). No Git mutation, deploy, phone, or owner-desktop interaction.

## Acceptance clause → production entry → wiring → behavior proof

### Remaining 1 — mail-body route registered, B HTML asserted

- **Production entry**: `GET /v1/cards/{id}/body` → `mailbody.Handler`
  (`cmd/litterbox/main.go:186`; `internal/mailbody/handler.go`).
- **Wiring**: registered in the native-consumer test mux behind the same `auth.Middleware`
  (same private Bearer/device auth), wrapped by the production `devOnly` cards→agents tenant
  bridge (`cards.TenantFrom` → `agents.WithTenant`, `cmd/litterbox/main.go:148-158`) — the
  handler reads `agents.TenantFrom`, which `auth.Middleware` alone does not populate.
  **No fake body**: it serves the fixture's real `card_bodies` row `<p>B</p>`.
  Helper `startNativeConsumer` builds the identical auth+cards+ops+version+mailbody surface
  for both tests (reuse, no new abstraction).
- **Behavior proof** (driver, `qt/tests/archive_undo_consumer.py`): asserts
  `states[5]["body"] != "" and "B" in states[5]["body"]`. Captured reader-on-B body
  (`states[2..5].body`) = `'<html><head></head><body><p>B</p></body></html>'` — the real B body
  served through the production handler (was `''` before the route existed).
- **Preserved ACK scope**: main server receipt still `durable_ops 2 / gmail_calls 2` (archive
  then `gmail.label_add(INBOX)`, both `thread-1` `Bearer test-access`); the `len(durable) != 2
  || len(f.calls) != 2` assert still enforced and held.

### Remaining 2 — fail=true Undo-rejection leg, reader shows failure truthfully

- **Production entry**: reader `archiveMail` → `mailActions.primaryAction()` → `archive` op →
  `POST /v1/ops` → Gmail `threads/{id}/modify`; rejection surfaces via
  `reportPostResult` (`CardStore.cpp:891`) → `operationFailed` → `showOperationFailure`
  (`InboxView.qml:332`) → `bundleArchiveStatus`.
- **Wiring**: Go-side fixture `f.fail = true` (loopback Gmail returns `403 fixture Gmail denied`
  → ops API `422`). **Separate test** `TestArchiveUndoNativeConsumerReject` (own `newSpamFixture`,
  own output subdir `<out>-reject`) so the main test's preserved 2-durable/2-call ACK scope is
  untouched. Driver runs in reject mode via `LB_UNDO_REJECT=1` (short open→archive script).
- **Behavior proof** (driver reject assertions, non-vacuous):
  - `any("Action failed" in message)` — the rejection is surfaced: `states[2].message ==
    "Action failed (HTTP 422)."`.
  - `"Archived" not in message` for every post-rejection state — **no false success** (the
    optimistic "Archived" is never the settled message).
  - `card in ids` for every post-rejection state — the card is restored, not left falsely
    archived; `outbox` empty.
  - `states[3].message == "Undo has expired."` — a truthful undo-window expiry, not a false
    success (recorded honestly, not asserted as a failure).
- **Server-side no-false-success**: reject server-receipt `durable_ops` empty (rejected archive
  persisted **no** op receipt), `gmail_calls = 1` (one rejected `removeLabelIds:[INBOX]`), and
  `f.state(card,"open")` asserts the card is truthfully unarchived at the DB level.

## Negative controls read back

- Main path preserved ACK control: `durable_ops 2 / gmail_calls 2` still asserted and green.
- Reject no-false-success controls: `durable_ops` empty + `gmail_calls 1` + `f.state open`.
- Provider rejection at API level remains owned by `TestArchiveUndoFailureAndIsolation` (unchanged).

## Scope notes

- Product behavior unchanged: the two edited files are the Go fixture and the throwaway Python
  driver (test scaffolding). The mailbody registration and the reject leg are harness-only.
- The `Undo has expired.` lingering undo-bar state after a rejected archive is a pre-existing UI
  quirk (truthful, not a false success). Not changed here — product behavior change is out of
  scope for this task.
- Middle/filtered/batch/end successor semantics and the offline queued-cancel leg remain per
  `receipts/archive-scope-successor-proof.md` / `receipts/reader-undo-successor-proof.md`; the
  native final cohort is still separate/later.

## Changed paths

- `server/internal/gmailsync/archive_undo_test.go`
  - imports `mailbody`, `mailhtml`, `sources/agents`.
  - new `startNativeConsumer(t, f)` helper: migrations + pg-check + cards/ops/version mux +
    **production `mailbody.Handler` behind auth with the devOnly cards→agents tenant bridge**.
  - `TestArchiveUndoNativeConsumer` refactored to call the helper (assertions unchanged).
  - new `TestArchiveUndoNativeConsumerReject`: fail=true Undo-rejection leg (own fixture).
- `qt/tests/archive_undo_consumer.py`
  - `reject` mode (`LB_UNDO_REJECT=1`) with a short open→archive script and reject assertions.
  - success path gains the **B HTML assertion** (`states[5]["body"] != "" and "B" in ...`).

## Risk

- Reject-leg message evolution is timing-sensitive (undo window ≈5 s). The assertion is robust
  to it: it requires "Action failed" to have surfaced and forbids any "Archived" false success,
  rather than pinning an exact per-state message, so it will not flap on expiry timing.
- B-HTML assertion keys on the fixture body `<p>B</p>` (sanitized to `<html>…<p>B</p>…</html>`);
  a fixture body change would need a matching assertion tweak (test-only).
- Extracting `startNativeConsumer` moved the migrations/pg-check/mux setup into a shared helper;
  both tests run the identical surface, and the main test's ACK assert is byte-preserved.

## Retro

- First red of this series was a self-inflicted over-strict assertion, caught by the oracle, not
  retried blindly: I required **every** post-rejection state to read "Action failed", but the
  reader truthfully transitions to "Undo has expired." after the undo window. That is a truthful
  expiry, not a false success — so I relaxed the assertion to the honest acceptance ("failure
  surfaced, no false success, card restored") instead of forcing a fixed message. Re-ran once to
  green. No product code was touched to make it pass.
- Reused the existing harness end-to-end (spam fixture + loopback Gmail + native consumer mux +
  the same driver + pinned frozen binary) instead of a new abstraction; the only consolidation is
  one shared setup helper so the two tests cannot drift on the auth/handler surface.
- The mailbody wiring was derived from production (`devOnly` bridges `cards.TenantFrom` →
  `agents.WithTenant`), not guessed: without that bridge the production handler 401s because
  `auth.Middleware` sets only the `cards` tenant key. Registered the real handler with the real
  auth and the real `card_bodies` row — no fake body.

## Artifacts

- Run dir (clean green): `/home/kom/proj/ai_pr/litterbox-reader-evidence-run-20261019b/`
  (driver-receipt.json with states+body, server-receipt.json, consumer-driver.log, native.log,
  authenticated-before.json, unauthenticated-control.json, PNG+JSON captures, private profile)
  plus the reject leg `/home/kom/proj/ai_pr/litterbox-reader-evidence-run-20261019b-reject/`
  (driver-receipt.json, server-receipt.json with `durable_ops: null` / `gmail_calls: 1`).
- Frozen binary+source unchanged: `/home/kom/proj/ai_pr/litterbox-reader-undo-frozen-20261019/`.
- Changes by this run: this receipt + `server/internal/gmailsync/archive_undo_test.go` +
  `qt/tests/archive_undo_consumer.py`. No production source edit, build, install, Git, deploy,
  phone, or owner-desktop interaction. Native/docs/Git handoff preserved for the lead.
