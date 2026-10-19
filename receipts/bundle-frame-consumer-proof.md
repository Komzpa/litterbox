# Bundle Frame Consumer Proof — Litterbox desktop (BundleFrameConsumerProof)

Gate: BundleFrameConsumerProof. Run 2026-10-19 02:36:10–02:36:26 +04:00 (Asia/Tbilisi).
ROOT read-only except this receipt: no source edits, no Git mutation, no build
reconfigure/rebuild, no deploy/install/shared-installer/upgrader (not even `--dry-run`), no
physical device. OUT = /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018; the pre-existing
OUT/build (system Qt 6.11.2 + system Kirigami, `LB_TEST_PLATFORM=xcb`) was used as-is; another
desktop gate keeps reading that binary read-only and it was not deleted or reconfigured.

## 1. Artifact identity (checked before and after the run)

- ROOT exists and `git rev-parse HEAD` = **fa01568ad3c4fff5ac61cf3354b6e9f4a100c367**
  ("fix: request a frame before bundle pointer assertions"), parent = **9985646** =
  `998564614812f708411542a6fff9bac8d9ce1b75` — matches the packet expectation exactly. No
  replacement repo was used; HEAD re-read after the run was still fa01568 (no drift mid-run).
- Frozen test QML `qt/tests/tst_bundleexpand.qml` sha256 =
  **ede2dbf2e756fbf352264015025583341cdd433e677668ec82eec4beb66676b3** — byte-identical to the
  frozen hash in the packet (checked before run).
- Runner source `qt/tests/bundle_runner.cpp` sha256 =
  **9572a05f33a17f7d5691faef6cd7f025f5db5c476fee4401717c68db46c32b87**.
- Test binary `OUT/build/tests/tst_bundleexpand` sha256 =
  **80e3c7513b4cbc278e4425fe9ee233917352ecc7f7f6774907f6a17ff51539d6** (mtime Oct 18 20:57,
  newer than bundle_runner.cpp Oct 18 19:40 — compiled artifact matches its compiled inputs).
- Production binary `OUT/build/litterbox-qt` sha256 =
  **ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981** (not executed by this
  gate; recorded so the wiring/entry citations below bind to a specific blob).
- **Why no rebuild was essential:** HEAD fa01568 touches only `qt/tests/tst_bundleexpand.qml`
  (1 file, +11/−9); no `.cpp`/`.h`/CMake target input changed, and QML is loaded at runtime via
  `-input <ROOT>/qt/tests/tst_bundleexpand.qml`. The pre-built runner therefore executes HEAD's
  QML without a rebuild — confirmed by the diff (`git show --name-status HEAD` → single QML
  file) and by the runtime-loaded source path in `OUT/build/tests/CTestTestfile.cmake:13`.
- Working tree at start and end: ` M docs/DEPLOYMENT.md`, untracked
  `qt/mobileux-inbox-*.png`, `qt/mobileux-reader-long-mail.png`, `receipts/` — identical before
  and after (pre-existing state, not created by this gate).

## 2. Command and working directory (exact as run)

Working directory: `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018` (OUT).

```
env QT_QUICK_BACKEND=software QT_QUICK_CONTROLS_STYLE=org.kde.desktop QT_QPA_PLATFORM=xcb \
  xvfb-run -a -s "-screen 0 1280x900x24 -nolisten tcp" \
  ctest --test-dir /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/build \
  -R '^bundleexpand$' --output-on-failure --no-tests=error \
  --output-junit /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/bundle-frame-fa01568.xml
```

Envelope: isolated Xvfb (`xvfb-run -a` picks its own free display; the pre-existing `Xvfb :193`
belongs to another gate and was not touched), xcb, `QT_QUICK_CONTROLS_STYLE=org.kde.desktop`,
`QT_QUICK_BACKEND=software`. The test's own ENVIRONMENT property also pins
`QT_QPA_PLATFORM=xcb;QT_QUICK_CONTROLS_STYLE=org.kde.desktop` (CTestTestfile line 14).

- Start 2026-10-19T02:36:10+04:00, finish 2026-10-19T02:36:26+04:00.
- **Exit status: 0.** `1/1 Test #24: bundleexpand .... Passed 15.42 sec`;
  `100% tests passed, 0 tests failed out of 1` (single run, no retry, no timeout, no crash).

## 3. Full counts (whole suite, 40 data rows, non-stop)

From `OUT/build/Testing/Temporary/LastTest.log` (Qt Quick Test output of this run):

- **Totals: 40 passed, 0 failed, 0 skipped, 0 blacklisted, 15134 ms.**
- ctest level: tests 1 / failures 0 (JUnit `testsuite tests="1" failures="0" skipped="0"`).
- `test_senderSummary` rows: **15/15 PASS** — 5 cases (one-sender, two-senders, more-senders,
  missing-sender, custom-topic-negative-control) × 3 widths (412, 520, 1356).
- Baseline comparison (same command envelope, prior receipts): a6cb742c and 9985646 both ran
  **30 passed / 10 failed of 40**, all 10 failing rows on the render-gate assertion
  (`'The bundle viewport must render before pointer delivery' returned FALSE` @tst_bundleexpand
  :96, then :102) across senderSummary 412 and 520. **Those 10 rows now PASS; net 30/10 → 40/0.**

Evidence artifacts:

- JUnit: `OUT/bundle-frame-fa01568.xml` sha256
  **d37d7c8d40df9eb3d81706c65988f54d017b561b0add6024fddaa31c95218021** (absolute path honoured,
  landed in OUT, not inside build/).
- Console log: `OUT/logs-gate-bundle-frame-fa01568.log` sha256
  **2e329dd7206ba2c45bd1cdcf86f79cbfef3e94e63082294886da5ea82700500b**.
- Per-row log: `OUT/build/Testing/Temporary/LastTest.log` (rewritten by this ctest run).

## 4. Finite specific failures — none in this suite; nothing waived

- **This gate's selection has 0 failing rows and 0 failing assertions.** No assertion was
  removed, disabled, tolerance-weakened, or blacklisted (0 blacklisted in Totals).
- The packet's finite target — the render-gate rows that failed at :96 (a6cb742c
  `waitForRendering` without `show()`) and at :102 (9985646 `inbox.show()` + `waitForRendering`)
  — is exactly the set that now passes:
  senderSummary(one|two|more|missing|custom-topic)-412 ×5 and -520 ×5.
- Preservation check on the frozen QML: the only delta at HEAD is the render-gate block
  (`SignalSpy { signalName: "frameSwapped" }` :29; `waitForPolish` :98; then
  target/valid/clear/`inbox.update()`/`wait()`/detach :99–104) plus its comment. Every original
  oracle is intact and among the 40 passing rows: 16 `mouseClick` real-pointer deliveries
  (toggle :120/:167/:285/:302/:401/:747, archive :183/:350/:509/:539/:568/:633, real Undo
  :186/:355/:576, done :603), 39 `compare`, 30 `tryVerify`, sender identity/custom-topic
  (`:117` latest original subject / custom topic stays custom, `:125` expansion preserves
  subject, `:159` newest unpinned original subject wins), Undo/order
  (`:357` "Real Undo restores both original members in order", `:526` token deadline,
  `:169` bundle member order), exact anchor/48 px target and popup/palette oracles per the
  consumer contract in `qt/tests/CMakeLists.txt:44-51`.
- **Other suites are out of scope here and were not rerun**: the 11-assertion finite map from
  RemainingConsumerProof (tst_uirequirements 1, tst_inboxlayout 1, tst_mailopen 3,
  bundleexpand 6 at 99a00861) spans four suites; this gate dispatched only `-R '^bundleexpand$'`.
  bundleexpand's share of the consumer map is resolved at fa01568 by this run; the uirequire /
  inboxlayout / mailopen rows are untouched and remain with their owner gates. The other three
  suites + note SSE PASS state and all production behaviour are **unchanged and not rerun**
  (production entry not executed by this gate).

## 5. Production entry + wiring + real consumer proof

- **Entry:** `qt/src/main.cpp:116 int main(int argc, char *argv[])` → production binary
  `OUT/build/litterbox-qt` (sha256 `ead409d5…de981`, built Oct 19 01:14 from this tree).
- **Wiring:** `qt/qml/InboxView.qml:318` `store.archiveBundleNow(bundleId)` is the bundle
  archive call; bundle state renders through `window.expandedBundles` / `bundleExpanded`
  (`InboxView.qml:1273`), the `bundleToggle` control (`:1381`) and `window.toggleBundle`
  (`:1302`, `:1306`); the C++ side exposes `Q_INVOKABLE QVariantMap archiveBundleNow(...)`
  (`qt/src/CardStore.h:76`), the Undo surface `bundleArchiveUndoActive` /
  `bundleArchiveUndoRemainingMs` (`CardStore.h:71-72`), `m_bundleArchives` / `m_bundleUndoTimer`
  (`CardStore.h:154`, `:165`). Production QML blobs are unchanged by HEAD (diff touches only
  `qt/tests/tst_bundleexpand.qml`), so prior production blob equivalence carries forward.
- **Real consumer proof:** (a) this suite is itself real-pointer consumer evidence on the
  production CardStore/InboxView code path — 16 `mouseClick` deliveries through the real
  `bundleToggle`/`archiveBundle`/`bundleArchiveUndoButton` objects, now 40/40 including the
  previously unreachable 412/520 senderSummary rows and the 102/1356 render-gated rows;
  (b) production Back/archive/Undo and offline cross-restart coverage is cited from
  `receipts/bundle-visibility-proof.md` §4 (repaired-consumer-a6cb742c realapp warm/smoke/prove
  runs, exit 0, real pointer clicks by objectName) — carried forward by blob identity, not by
  receipt name; this gate ran no production app.

## 6. Retro (required)

- **Prior two render hypotheses were required and both failed** (as recorded, not re-litigated):
  1) `waitForRendering(list)` with no `show()` — ctest FAIL at `tst_bundleexpand.qml:96`;
  2) `inbox.show()` + `waitForRendering(list)` — FAIL at `:102`, all 412/520 senderSummary rows,
  window mapped but no frame scheduled at compact bounds under software/xcb.
- **This hypothesis (HEAD fa01568) passed:** register `SignalSpy(frameSwapped)` first, then
  `waitForPolish` → `inbox.update()` → `spy.wait()` observes one presented frame before pointer
  delivery; 40/40, exit 0, 15.42 s.
- **Scope discipline:** finite whole-consumer result only — no source campaign, no new tests
  beyond the named selection, no production rerun, no other suite dispatched, no build touched.
  All product criteria, docs (pre-existing `docs/DEPLOYMENT.md` modification left as found) and
  authorized Git handoff remain preserved for the owner.

## 7. Stale / resource warnings

- **Stale: none.** Source artifact (ROOT HEAD, frozen QML hash) verified identical before and
  after the run; binary and runner-source hashes recorded before the run.
- **Resource: none.** Another gate's `Xvfb :193` was left alone (this run used its own
  `xvfb-run -a` display); OUT/build was used read-only apart from ctest's own
  `Testing/Temporary/LastTest.log` and the two OUT proof files written above.
