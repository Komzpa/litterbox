# Desktop reader regression — suite failure repair (0.1-40 cohort)

Date: 2026-10-20. Task: RepairDesktopReaderRegression (sub of Main).
ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018,
OUT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018, KIT=OUT/release-kit-0.1-40,
verification artifacts: OUT/reader-regression-20261020/ (`VER/` below).
Frozen cohort unchanged: source `df9b13e5513ed1f1739149a3d6048c27eee3c725`, native v27 all 7 PASS,
exact v27 APK `83ad492b…`. No Git metadata touched, no commit/push/publication, no packaging.

## 1. Baseline (user-reported ground truth, not re-run to confirm)

KIT/logs/ctest.log: 25/26, one fail `tst_mailopen::test_desktopSplitSelectionResizeNarrowBackAndArchiveClosure`
— `FAIL! … property readerOpen, Actual (): false, Expected (): true,
Loc: [src-immutable/qt/tests/tst_mailopen.qml(496)]` preceded by
`QWARN … src-immutable/qt/qml/InboxView.qml:836: TypeError: Cannot read property 'length' of undefined`.
Neighbor `tst_maildetail` (#19) Passed. Retrospective read: full; no instruction defect.
This was a new concrete suite failure, resolved not ignored.

## 2. Exact cause (two defects, one causal chain)

Diagnosed in the canonical reader-successor model with an instrumented scratch copy
(/tmp/lbdiag; product DIAG instrumentation removed afterwards):

1. **Harness timing (test clicks before the product's deferred anchor restore lands).**
   `inbox.width = 1480` (after 1480→1320→1180→Back) fires the width-change reflow; the product's
   anchor restore is intentionally deferred one event-loop turn (`Qt.callLater` in the
   `onWidthChanged` handler, qt/qml/InboxView.qml:1086–1093, contract comment 1083–85
   "preserve the visible card and its offset"). The test clicked in the same JS turn, before that
   restore ran: `contentY` stale 597 vs `row8.y` 549 → row8 in-list y = **-48** (vs anchorY 0).
   `mouseClick(item)` (one-arg) clicks the item *center in scene coordinates* = (478, 122), which is
   **above the list viewport top** (list scene y=140, clip=true). The click landed on the
   `connectionStatus` header Label (scene 8,113, 1412×15) and hit its TapHandler
   (qt/qml/InboxView.qml:823) → `offlineDetailsShown = true`. The product's restore itself is
   correct: when it runs, it lands the anchor exactly (offset 0), proven in the instrumented run.
2. **Fixture contract gap.** With `offlineDetailsShown` toggled true, the binding at
   qt/qml/InboxView.qml:836 (`!store.online && window.offlineDetailsShown && store.offlineReason.length > 0`)
   evaluated `store.offlineReason.length` on the test's `ListModel` fixture, which lacked
   `offlineReason` → TypeError. Production `CardStore` has it
   (qt/src/CardStore.h:29 `Q_PROPERTY(QString offlineReason …)`); the sibling fixture
   qt/tests/tst_mobileux.qml:22 models `property string offlineReason: ""`. So the fixture was
   incomplete versus the production store contract — not a product binding defect.

Net effect: the misdirected tap toggled offline details (TypeError) and the card's open handler never
fired → `readerOpen` false vs true. Both reported symptoms (readerOpen false + TypeError 836) are one
chain; the product anchor mechanism needs no change.

## 3. Fix (smallest causal edit; harness-side, no product change)

qt/tests/tst_mailopen.qml, **3 insertions, 0 deletions**:

1. Fixture store: `property string offlineReason: ""` (line 15) — completes the fixture to the
   production store contract (CardStore.h:29; tst_mobileux.qml precedent). Not a guard: the product
   binding is untouched and still asserts on the production property.
2. In `test_desktopSplitSelectionResizeNarrowBackAndArchiveClosure`, before the 1480 reopen click:
   `wait(150)` after `inbox.width = 1480`, plus the anchor assert
   `compare(Math.round(list.itemAtIndex(8).mapToItem(list, 0, 0).y), Math.round(anchorY))` —
   the file's own established pattern at the 1320 step (lines 477–481). This lets the deferred
   restore land and *strengthens* acceptance (the documented "preserve the visible card and its
   offset" contract is now asserted at the final resize step too, where it previously was not).

Product file qt/qml/InboxView.qml: investigation DIAG instrumentation fully removed; byte-identical
to KIT/src-immutable frozen copy (`cmp` equal). No behavior change → no REQUIREMENTS/changelog edit.

**Changed paths / hashes**
- qt/tests/tst_mailopen.qml — sha256 `ce79e13add876b7b33e0e7f6428972d946d73284c8b6282f150d2a0eabba3c49`
  (the frozen delta to hand to the Git owner; I made no Git changes)
- qt/qml/InboxView.qml — sha256 `b984460a722910fc5eafe28f562125af11bbc9b560c62d781391405af94d4c6f`
  (= frozen, unchanged)

## 4. Verification (scoped; project-wide validation remains the lead's)

| Check | Command | Exit | Result |
|---|---|---|---|
| Scoped build of changed path's consumer | `cmake --build KIT/build-desktop --target tst_maildetail` | 0 | `ninja: no work to do` (test QML is runtime-loaded; runner binary unchanged) |
| Full suite, collect all failures (not stop-first) | `QT_QPA_PLATFORM=offscreen QT_QUICK_CONTROLS_STYLE=org.kde.desktop KIT/build-desktop/tst_maildetail -input ROOT/qt/tests/tst_mailopen.qml -import ROOT/qt/tests/qml-imports` | 0 | **27 passed, 0 failed** (incl. repaired fn + all neighbors) |
| Failed test alone | same runner, `MailCardOpening::test_desktopSplitSelectionResizeNarrowBackAndArchiveClosure` | 0 | PASS |
| Neighbors | `test_narrowDesktopBackClosesReaderWithoutArchive`, `test_altLeftRoutesBackWhenFocusIsOutsideStack`, `test_backRestoresExactCardPosition` | 0 | PASS (w1440 + w598) |
| Real Qt Xvfb consumer — reader successor model | `xvfb-run -a dbus-run-session -- KIT/build-desktop/litterbox-qt --capture-script VER/run-j2-archive/profile KIT/journeys/scripts/j2-archive.json VER/run-j2-archive/captures` (private HOME/XDG 0700, `QT_QPA_PLATFORM=xcb`, `QT_QUICK_BACKEND=software`, `DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent`) | 0 | step9 `{alpha:true}`; step13 `{cardId:…0002, beta:true, hasA:false, hasB:true, status:"Archived"}`; step17 `{hasA:true, hasB:true, cards:6, status:""}` |
| Real Qt Xvfb consumer — desktop geometry/resize | same envelope, `j4-geometry.json` → `VER/run-j4-geometry/captures` | 0 | wide grid 343×54 vals 2026/10/20, OK/Cancel 108×54; narrow-500 same values + `depth:2`; `{pendingOps:0, hasB:true}` |
| Neighbor suite tst_maildetail (full, collect all) | same runner, `-input ROOT/qt/tests/tst_maildetail.qml` | 1 then 0 | first run 43/44: `test_originalThreadUsesNativeDisclosureInsideSingleDocumentScroll` verify() false (WebEngine click-timing flake, tst_maildetail.qml:336; kit==live file, untouched by this fix); single-fn rerun PASS; full rerun from qt cwd **44/44 PASS**. First failure kept, not hidden |

Logs/captures: VER/logs/{journey-j2-rerun,journey-j4-rerun,tst_maildetail-neighbor}.log;
VER/run-j2-archive/captures/j2-0{1,2,3}-*.png+json; VER/run-j4-geometry/captures/j4-0{1,2,3}-*.png+json.
j2-02-continued-B.png was opened and visually inspected: reader presents the real successor body
`BETA-BODY-MARKER Thanks for reading, this is the real B body.` with the `Archived … Undo` bar,
selected card highlighted beside the reader — body not hidden, not wrapped.

## 5. Negative controls (single-hunk removals on the fixed file; /tmp/lbctl/tests/)

| Control | Removed | Result (exit) | Proves |
|---|---|---|---|
| ctlA | `wait(150)` only | FAIL, anchor compare "Compared values are not the same" (1) | the settle wait is causal; the added assert catches the −48 drift |
| ctlB | `wait(150)` + anchor compare (fixture kept) | FAIL `readerOpen` false vs true, **no TypeError** (1) | the misdirected click is caused by clicking mid-restore; the fixture property alone removes the TypeError |
| ctlC | pre-fix state (also drop `offlineReason`) | FAIL `readerOpen` false vs true **+ TypeError InboxView.qml:836** (1) | exact replication of the ctest baseline failure |

## 6. Acceptance clauses → production entry → wiring → behavior proof

| Clause | Production entry | Wiring | Proof |
|---|---|---|---|
| Actual presented successor B body | `captureArchiveContinuation(cardId)` (InboxView.qml:423) invoked via `requestArchiveContinuation` callback (InboxView.qml:606) from the mail detail archive action | continuation entry passed to `openReaderEntry(entry)` (InboxView.qml:403/446); retained reader re-presented (comment 529) | j2 eval step13 `beta:true` + j2-02-continued-B.png (real B body rendered) |
| Undo keep B, restore A | `bundleArchiveUndoButton` (InboxView.qml:938–949) in the `bundleArchiveUndoBar` | undo restores archived card without dropping the kept successor | j2 eval step17 `{hasA:true, hasB:true, cards:6}` + j2-03-after-undo.png |
| No hidden / wrap | reader body column layout (mail detail page) | body loads through the same consumer path | j2-01/j2-02 PNGs show full single-line body text, not clipped; j4 evals show controls keeping widths at narrow 500 (`depth:2`) |
| Desktop resize/back behavior | width-change anchor capture/restore (InboxView.qml:1083–1093) + `backToInbox` | anchor preserved across 1480→1320→1180→Back→1480 | repaired test now asserts the anchor at every step incl. the final 1480 and reopens the reader; `test_backRestoresExactCardPosition(w1440/w598)`, `test_narrowDesktopBackClosesReaderWithoutArchive` PASS; j4 resize journey exit 0 |

## 7. Native proof status and hand-off

- Shared product source is **unchanged** (qt/qml/InboxView.qml byte-identical to the frozen kit copy;
  git diff of the tree = qt/tests/tst_mailopen.qml only). The native proof for product behavior
  (source df9b13e5, exact v27 APK 83ad492b, native 7 PASS) is therefore **not invalidated**.
- However, `tst_mailopen` is also part of the Android test surface (`add_test(NAME tst_mailopen …)`
  under `if(ANDROID)` in qt/CMakeLists.txt). An AVD-5590 run of `tst_mailopen` with the new test file
  is **unproved** here (no owned AVD/emulator in this lane).
- **Hand-off to the owner (default route, per contract):** exact frozen delta is the 3-line diff in
  §3 (qt/tests/tst_mailopen.qml, sha256 `ce79e13a…e3c49`). Unproved criteria that remain open until
  the owner re-runs on device: (a) native `tst_mailopen` green with the fixed file on AVD 5590;
  (b) cohort re-stamp if the published source is to include the delta. Source changes must go through
  a separate Git owner; nothing is packaged from this lane, so no false cohort is produced.

## 8. Gaps / non-claims

- No REQUIREMENTS/oracle/changelog edit: the change is test-only; product behavior and its documented
  contract are unchanged (the fix restores/asserts the documented contract, it does not alter it).
- The transient one-event-loop-turn window before the deferred restore is inherent to the product's
  documented design and unreachable by real input (input events are processed after the deferred call
  in production flows); the harness must settle first, as the file's own 1320-step pattern already
  required. No speculative Stack/C++ rewrite was made; no assertion was guarded, suppressed, weakened,
  or dropped; no real mail was mutated (journeys ran on isolated profile copies).
