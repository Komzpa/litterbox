# Repaired Consumer Proof — QtRepairedConsumerGate

Gate run 2026-10-19 01:14–01:47 +04:00. OUT = /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018;
proof root OUT/repaired-consumer-a6cb742c; build OUT/build (existing kit, **no reconfigure**:
CMakeCache.txt untouched since 20:57, Qt6_DIR=/usr/lib/x86_64-linux-gnu/cmake/Qt6 = system Qt
6.11.2 + system Kirigami). ROOT touched only by this receipt; no source edits, no Git mutation,
no deploy/install/shared-installer/upgrader (not even dry-run), no physical device, no live or
default kom DB, no production card mutation.

## 1. Source identity (verified before build and re-verified at close)

- `git rev-parse HEAD` = **a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75** (parent 99a00861…).
  Required exact HEAD confirmed; no alternative checkout used.
- `git diff --stat HEAD -- qt server` → 0 lines at close: the tested tree is exactly the commit.
- HEAD delta = 7 paths / +58 −7 ("settle adaptive consumer clicks and isolate note events"):
  MailDetailPage.qml (appbar `z: 1`), tst_uirequirements/tst_inboxlayout/tst_mailopen/
  tst_bundleexpand.qml, qt/tests/CMakeLists.txt (consumer-contract comment), cards/handler_test.go
  (`NewEvents(tdb.DSN)`).

## 2. Build (build-j32, whole owning targets, no filtering)

`cmake --build OUT/build -j32` → **exit 0** (01:14:36–01:14:40, 4.3 s incremental).
log sha256 27876c9f…f0ee0f. Binaries (sha256):
- build/litterbox-qt ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981
- build/tests/tst_bundleexpand 80e3c7513b4cbc278e4425fe9ee233917352ecc7f7f6774907f6a17ff51539d6
- build/tst_maildetail a088902eea1bd8aab1cd5f4617aa3a5124cfe248d21834da406b6de4d82c7125

## 3. Four owning Qt consumer suites — full suites, no test filtering, real pointer consumers

Command (cwd OUT):
`xvfb-run -a env QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop
QT_QUICK_BACKEND=software LB_UI_PROOF_DIR=…/repaired-consumer-a6cb742c/qt ctest --test-dir build
--parallel 1 -R '^(tst_uirequirements|tst_inboxlayout|tst_mailopen|bundleexpand)$'
--output-on-failure --no-tests=error --output-junit …/ctest.xml`
(01:15:39–01:17:06, 87.4 s, **exit 8**). JUnit NEW/ctest.xml sha256
9419954043a631614d977c7e3edab3ad3773501073ea8ddb0223befdb0006d64; ctest.log sha256 afbf0900…af508.
(ctest resolves the relative junit path inside the build dir; byte-identical copy placed at
NEW/ctest.xml.)

**Counts — 3 of 4 suites PASS; assertion totals 74 passed / 10 failed of 84:**

| Suite | Assertions | Result | vs baseline remaining-r2 |
|---|---|---|---|
| tst_uirequirements | 3 / 0 | **PASS** | 1 fail fixed (realInboxActions…SheetPaletteIsLight) |
| tst_inboxlayout | 18 / 0 | **PASS** | 1 fail fixed (cleanPreviewGrid(1440)); 1200 + all rail/split rows pass |
| tst_mailopen | 23 / 0 | **PASS** | 3 fails fixed (archive-button + done-body depth, previewOpensMail…Scroll) |
| bundleexpand | 30 / **10** | **FAIL** | baseline 6 fails cleared, **10 remaining** — see failure map |

Baseline was 11 failures across 4 suites; **8 of the 11 are fixed and never recur**. Remaining
consumer failures are **10, all in one suite (bundleexpand), all one assertion**:

### Failure map (finite, exact)
All 10: `bundleexpand::BundleExpand::test_senderSummary(<tag>)` where tag ∈ {one-sender,
two-senders, more-senders, missing-sender, custom-topic-negative-control} × width ∈ {412, 520};
assertion `'The bundle viewport must render before pointer delivery' returned FALSE` at
qt/tests/tst_bundleexpand.qml:**96** (`verify(waitForRendering(list), …)`, the render gate Main
added in a6cb742c). Width **1356 rows all PASS**; `test_singleImportantMemberDisclosureAndCount`
now **PASS** (was failing). The failure is inside the *newly added* pre-click render precondition
at narrow widths under QT_QUICK_BACKEND=software/xcb — the pointer click and every downstream
oracle (sender identity, original subject, expansion) never get reached at 412/520. Not waived,
not tolerance-weakened, no autonomous source fix performed: this needs a new owned repair slice.
Reproduce: the exact command above; evidence in logs/ctest.log and NEW/ctest.xml.
Observed informational (in passing tests too): QWARN InboxView.qml:1268 "Unable to assign
[undefined] to QString".

## 4. Isolated note SSE persistence — PASS (no timeout weakening)

`env -u DATABASE_URL pg_virtualenv bash -c 'createdb -h $PGHOST -p $PGPORT -U $PGUSER
litterbox_test_note_a6cb742c; DATABASE_URL=<disposable localhost:5434 DSN>;
CARD_TEST_POSTGRES=1; cd ROOT/server; go test -count=1 -v -run '^TestPostgresNotePersistence$'
./internal/cards/'` → **exit 0**, `--- PASS: TestPostgresNotePersistence (0.12s)`; ambient
DATABASE_URL unset before cluster bootstrap, explicit litterbox_test_* DB on the private
pg_virtualenv cluster, dropped by its own trap. log sha256 3ff6fd38…00b93. This proves the HEAD
fix (handler_test.go:72 `NewEvents(tdb.DSN)` — LISTEN is database-scoped); the test's own
readEvent deadlines were untouched. Fixture setup handed to NativeFinalConsumerProof via Main.

## 5. Real consumer evidence — actual production binary, private HOME/XDG, Xvfb+dbus

Driver: production `litterbox-qt --capture-script <profile> <script> <outdir>` (real pointer
clicks by objectName; no fake operations). Profile = private copy of the CF17-fixed 15-card
fixture; three runs, all **exit 0** (logs/realapp-{warm,smoke,prove}-r2.log; 7 PNG+JSON shots in
realapp/captures/).

- **Changed reader appbar Back + action smoke (the a6cb742c MailDetailPage change, real clicks):**
  `openCard 5b8c…` → `depth:2 readerOpen:true`; real click **backToInbox** → `depth:1
  readerOpen:false`; `openCard 405a…` → `depth:2`; real click **archiveMail** → `depth:1
  cards:13 pendingOps:1`; real click **bundleArchiveUndoButton** → `cards:14 pendingOps:0
  has405:true`. Production entry/wiring exercised: MailDetailPage `mailAppBar`/`backToInbox`/
  `archiveMail` (z:1 stacking fix) → InboxView closeReader/handleBack → CardStore outbox → undo bar.
- **Cross-restart offline cached reader/history (Main's handoff, honestly scoped):** warm run
  opened the reader online then went offline; the **prove run is a fresh app process** (true
  kill/relaunch between runs) started with `store.online=false` against the same private profile:
  `cards:14 online:false`; `store.cachedMailBody(5b8c…)` → `hasHtml:true htmlChars:481
  threadChars:80 htmlHasQEP:true` (DB readback from profile cards.sqlite mail_bodies);
  `openCard` offline → `depth:2 readerOpen:true`; real mouse Back → `depth:1 readerOpen:false`.
  So offline cross-restart cached original-thread reopen + Back is now **proved on a6cb742c**.
  Honest boundary: the cache was warmed from the fixture/fixture-DB (server_url is a dead
  loopback port), so this covers process-restart + offline DB readback, **not** a live
  server-synced history pull; the Retry→online half remains the journey-proven part only.
- Four consumer pointer suites were observed as their own real runs (§3) — not helper stubs.

## 6. Frozen clauses not claimed here

- **restart_service_survival: SKIP exit 77 preserved as UNPROVED, not PASS** (no systemd user
  manager on the private bus; outside this four-suite selection, not re-run).
- Native Android — NativeFinalConsumerProof's owner; untouched here (its PG fixture request
  answered in §4).
- Server/26-suite Qt breadth — deliberately not repeated (no new artifact value).
- Remaining 10 bundleexpand render-gate failures — new owned repair slice; no criterion cut.

## 7. Retro (required)

- **Worked:** verifying HEAD readback before building; running the note-SSE test in its own
  private pg_virtualenv instead of the broad server suite; reusing the existing CF17-fixed
  capture driver for the appbar smoke instead of writing a new harness; splitting warm/smoke/
  prove into separate app processes, which turned the requested "kill/reopen" into a real
  cross-process restart for free.
- **Failed in execution (mine):** first ctest JUnit path landed under build/ (ctest resolves
  relative to --test-dir) — harmless, byte-identical copy placed at NEW; the first appbar smoke
  used a non-existent `store.undoLast()` and a non-existent `findChild` in eval (the capture
  engine has no global findChild) — fixed by using the real undo-bar control and root-scope
  evals; my earlier fixture DB schema guesses (state/section/body columns) were wrong and cost
  probes before reading PRAGMA table_info.
- **Surfaced for owners:** bundleexpand senderSummary 412/520 `waitForRendering` gate under
  software rendering (this receipt §3) → repair owner; native note-save PG fixture → answered.

## 8. Artifact index

OUT/repaired-consumer-a6cb742c: ctest.xml (JUnit); logs/ (build.log, ctest.log, note-sse.log,
binary-sha.txt, artifact-sha.txt, realapp-{warm,smoke,prove}-r2.log, realapp-r2-timeline.log,
pg-setup-for-native.txt, timeline txt files); realapp/scripts/ (3 capture scripts),
realapp/profile/ (private fixture), realapp/captures/{warm,smoke,prove}/ (7 PNG+JSON).
