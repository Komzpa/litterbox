# Archive-Continuation Scoped Gate Receipt (read-only source, no source edits)

Gate row: "Advance to next message after archiving" — frozen-source scoped proof of the
uncommitted local-Archive continuation change (MailDetailPage.onCardHandled →
InboxView.captureArchiveContinuation). Owner-screen/phone/installer/Git untouched.

## 1. Source freeze (per brief: FIRST, private non-Git copy outside candidate)

| Item | Value |
|---|---|
| Frozen copy | `/home/kom/proj/ai_pr/litterbox-archive-continuation-frozen-20261019/src` (rsync of candidate tree, `.git/` excluded) |
| Non-Git verified | `git rev-parse --is-inside-work-tree` in copy → `fatal: not a git repository` |
| Candidate HEAD at freeze | `e62b81c8cd3b541949fe0aa35fee290979f46563` |
| Preserved changed files | `qt/pages/MailDetailPage.qml` `8fb5a876…ba6aa`, `qt/qml/InboxView.qml` `3cad75b8…b1e30`, `qt/tests/tst_mailopen.qml` `6dec7531…3596f` |
| Tree hash (qt/) | `b32ab14adc3d0186b2c6a28df1a258ffd4734ef617cb4fa324de80d37a75cc06` — identical to candidate `qt/` at freeze time; unchanged on recheck at 17:06 |
| Patch + manifests | `…/litterbox-archive-continuation-frozen-20261019/{uncommitted-3files.patch, changed-files.sha256, src-qt-manifest.sha256, source-head.txt, source-status.txt}` |
| Avatar owner released | `agent://RepairAvatarCoverage` told **safe** immediately after snapshot (message delivered 16:18); scope reminder: only InboxView.qml avatar hunks, not the continuation surface |

**Warning — live candidate tree has since moved** (expected: avatar owner editing).
`git status` at 17:06 shows a 4th modified file `qt/src/qml/CardActions.qml` that was NOT
in the frozen 3-file state. All proof below ran against the frozen copy only; nothing was
reverted or written in the live tree.

## 2. Cached isolated build — freshness verdict

`OUT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018` (no build ran for this gate; live mutating tree never built).

- `build/CMakeCache.txt`: `CMAKE_HOME_DIRECTORY=…/litterbox-ui-desktop-mobile-20261018/qt`, `LB_TEST_PLATFORM=xcb` — i.e. the cached CTest wiring points at the **live** tree, so `ctest` in that build dir was NOT used.
- Test binary used: `build/tst_maildetail`, SHA-256 `a088902eea1bd8aab1cd5f4617aa3a5124cfe248d21834da406b6de4d82c7125`, built 2026-10-18 20:57.
- **Fresh for its inputs**: the binary's compiled sources (`qt/src/maildocumentprofile.*`, `qt/tests/maildetail_runner.cpp`) are byte-unchanged since build (`git status` clean for `qt/src`, runner mtime 19:40 < build 20:57). QML is loaded at runtime from `-input`/`-import`, so the frozen QML is what executed — no rebuild required, none performed.

## 3. Exact command (CMake's existing tst_mailopen target, paths redirected to frozen copy)

CMake target (`qt/CMakeLists.txt:236-240`, desktop branch): `tst_maildetail -input tests/tst_mailopen.qml -import tests/qml-imports`, `WORKING_DIRECTORY=qt`, `ENVIRONMENT QT_QPA_PLATFORM=xcb;QT_QUICK_CONTROLS_STYLE=org.kde.desktop`.

```bash
FROZ=/home/kom/proj/ai_pr/litterbox-archive-continuation-frozen-20261019/src
OUT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018
RUN=$OUT/archive-continuation-proof
Xvfb -displayfd 3 -screen 0 1440x1000x24 -nolisten tcp 3>"$RUN/xvfb-display" &   # owned :1, stopped afterwards
env HOME=$RUN/home XDG_RUNTIME_DIR=$RUN/runtime XDG_CONFIG_HOME=$RUN/config \
    XDG_DATA_HOME=$RUN/data XDG_CACHE_HOME=$RUN/cache \
    DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent DISPLAY=:1 \
    QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop \
  dbus-run-session -- $OUT/build/tst_maildetail \
    -input $FROZ/qt/tests/tst_mailopen.qml -import $FROZ/qt/tests/qml-imports   # cwd $FROZ/qt
```

| Run | Start/End (+04:00) | Exit | Log |
|---|---|---|---|
| Scoped full target | 16:45:38 → 16:46:00 | **1** | `$RUN/logs/tst_mailopen-frozen.log` |
| Targeted `-functions test_localArchiveContinuesToNextPresentedReader` | 16:53:49 | 1 (did not select: QuickTest needs the data-tag form for `_data()` tests — no test ran; not evidence) | `$RUN/logs/tst_mailopen-red-diagnostic.log` |

**Result: Totals: 25 passed, 1 failed, 0 skipped, 0 blacklisted, 20516ms.**

Actual consumer path proven from the log: every QML warning resolves into
`file:///home/kom/proj/ai_pr/litterbox-archive-continuation-frozen-20261019/src/qt/qml/InboxView.qml` and
`…/src/qt/tests/tst_mailopen.qml` — the frozen copy, via `qml-imports/litterbox/qmldir` → `../../../qml/InboxView.qml`
and `pagesDir=Qt.resolvedUrl("../pages/")` → frozen `MailDetailPage.qml`. Live tree not loaded.

## 4. First red (verbatim)

```
FAIL!  : maildetail::MailCardOpening::test_localArchiveContinuesToNextPresentedReader(filtered-not-backing-order) 'verify()' returned FALSE. ()
   Loc: [/home/kom/proj/ai_pr/litterbox-archive-continuation-frozen-20261019/src/qt/tests/tst_mailopen.qml(299)]
```
Line 299 is `verify(opener)` — the test could not find `openCard-<mailId(1)>` inside
`list.itemAtIndex(1)` after search "Message 1" reduced the list (right after: `DelegateModel::cancel: index out range 9 5`, i.e. 5 filtered rows).

**Diagnosis (code-derived, not silently accepted):** a defect in the *uncommitted test data
row*, not in the continuation product code. `populateStore()` gives titles `Message 0…13`
then `Additional mail`; `matchesSearch()` matches every query word, so "Message 1" filters
the presentation to indices {1,10,11,12,13} (5 rows — matches the log). Filtered presented
index 1 is therefore `mailId(10)`, while the row `{opened: 1, expected: 10, rejected: 2}`
asks for `mailId(1)` at presented index 1 (`mailId(1)` sits at filtered index 0). The test
conflates backing index with presented index under an active filter. Consequence for the
ledger: **the "filtered excluded neighbour never opens" acceptance criterion is UNPROVEN**
— the row dies at the opener lookup, before archiving. Fix belongs to the test author
(pick the row by cardId, or use the filtered presented index); no source edit by this gate.

## 5. Multi-removal successor — BUG REPORT (inspection, per brief)

`qt/qml/InboxView.qml` (frozen) `captureArchiveContinuation()`:

```js
441:  if (!next) return null
442:  return function() {
445:      if (store.cardIds().indexOf(next.cardId) < 0) return   // closes instead of continuing
446:      openReaderEntry(next)
```

The returned opener captures exactly **one** candidate and checks only that candidate's
presence. If a wider action scope (bundle op, server snapshot, queued mutation) removed
that first candidate too, the reader **closes** rather than skipping to the next surviving
presented card. The brief's rule — "skip all removed to true next" — is NOT implemented;
the inline comment ("in which case there is nothing to continue to and we close normally")
codifies the wrong behavior. Reachable whenever the captured neighbour disappears between
capture and `next()` (the code's own premise). **Not accepted; not silently fixed** (no
source-edit authority in this gate). Required repair: iterate the pre-removal snapshot from
the archived position and open the first candidate still present in `store.cardIds()`.

## 6. Acceptance ledger for this row

| Criterion | State | Evidence |
|---|---|---|
| Source frozen before avatar edit; avatar owner released after snapshot | **DONE** | §1 hashes, delivered "safe" message |
| Cached build freshness verified, live tree not built | **DONE** | §2 |
| A: open archive → B body + matching selection | **PROVEN** | `test_localArchiveContinuesToNextPresentedReader(middle)` PASS (opened 8 → expected 9, body + selection highlight asserted in-test) |
| End of list closes, never wraps | **PROVEN** | `(end)` PASS (opened 39 → depth 1) |
| Filtered excluded neighbour never opens | **UNPROVEN (red)** | §4 — test data-row defect blocks the assertion |
| Undo restores order, keeps B | **UNPROVEN** | not exercised by tst_mailopen; the real journey tool `qt/tests/archive_undo_consumer.py` needs the loopback server + `LB_UNDO_*` env (server-ACK lane); left for the native consumer cohort |
| External / unrelated negative controls | **PROVEN (scope of suite)** | PASS: `test_actionReturnsToSameInboxOffset(snooze/done-body)`, `test_bodylessCardDoesNotOpenEmptyDetail`, `test_realPopupsProtectRetainedReaderAndArchiveShortcut(6 rows)`, `test_searchFiltersWithoutReorderingAndClearRestoresAnchor` |
| Multi-removal successor | **BUG REPORTED** | §5 |
| Native/cross-platform final proof | **NOT CLAIMED** | desktop-only, xcb, frozen QML; Android/native cohort separate |

## 7. Isolation / resource record

Owned Xvfb `:1` started 16:42, stopped 17:04 (pid recorded in `$RUN/xvfb.pid`); pre-existing
`Xvfb :193` (pid 3680645) and pre-existing services left untouched. Private HOME/XDG per
run, `DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent`, no owner desktop/userbus, no physical
phone, no shared installer, no Git mutation (no add/commit/checkout; receipt file is the only
write into ROOT). No stray `tst_maildetail` processes. Retro required.

## 8. What changed / what remains

Changed: created frozen copy + this receipt + isolated run artifacts under
`OUT/archive-continuation-proof/` (logs, private XDG dirs). No source, build, Git, install,
or production-state change.

Remains (lead-owned): (1) repair the `filtered-not-backing-order` data row so the
filtered-exclusion criterion can be proven; (2) repair the multi-removal successor (§5) and
re-gate; (3) Undo-restores-order-keeps-B via the archive-undo journey; (4) native/cross-platform
final cohort; (5) docs/Git/publication rows; (6) note the unexpected `CardActions.qml`
modification in the live tree — owner of that edit is not this gate.
