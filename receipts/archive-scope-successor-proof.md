# Archive-scope successor scoped proof receipt

## Gate and artifact

Gate: user request “Advance to next message after archiving.” Exact proof artifact is the frozen candidate copy at `/home/kom/proj/ai_pr/litterbox-archive-successor-frozen-20261019/src`; it is outside the Git worktree. Candidate Git HEAD at freeze: `e62b81c8cd3b541949fe0aa35fee290979f46563`. The run used the existing cached binary `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/build/tst_maildetail` (SHA-256 `a088902eea1bd8aab1cd5f4617aa3a5124cfe248d21834da406b6de4d82c7125`, built 2026-10-18 20:57); no build ran. Frozen source identity: `qt/qml/InboxView.qml` SHA-256 `19deee88a91c2e1905fb809938bf192d9f6b45ff47919aeb1edad3cbcff44811`, `qt/pages/MailDetailPage.qml` `8fb5a876abef36862e2ed47a38440270db04b9690c7edb58340a452adb8ba6aa`, `qt/tests/tst_mailopen.qml` `c0f6cbed05549c9b5feef6a2497acfed604038f901ad8a2b18c42022eff44811`. Frozen copy Git check: “fatal: not a git repository”.

## Command and evidence

Working directory: `/home/kom/proj/ai_pr/litterbox-archive-successor-frozen-20261019/src/qt`.
Command: existing target invocation `dbus-run-session -- /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/build/tst_maildetail -input /home/kom/proj/ai_pr/litterbox-archive-successor-frozen-20261019/src/qt/tests/tst_mailopen.qml -import /home/kom/proj/ai_pr/litterbox-archive-successor-frozen-20261019/src/qt/tests/qml-imports`, under private HOME/XDG, DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent, QT_QPA_PLATFORM=xcb and an owned Xvfb display. Start 2026-10-19 17:18:27 +04:00; finish 17:18:49 +04:00. Exit status **1**. Full log: `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/archive-scope-successor-proof/logs/tst_mailopen-frozen.log`; exit status file and timestamps in same run directory.

Totals: **26 passed, 1 failed, 0 skipped**. The sole failure is unrelated existing snooze fixture: `test_actionReturnsToSameInboxOffset(snooze)` uncaught exception, “Cannot read property 'activeFocus' of null”. Archive continuation cases all passed:
- middle: opened mailId(8), continuation mailId(9)
- filtered-not-backing-order: search “Message 1”; exactly five presented IDs `[1,10,11,12,13]`; opened ID 10 at presented row 1; rejected excluded backing neighbor ID 2; continuation ID 11. All assertions passed.
- batch-removal-skips-removed-candidates: archive ID 8 with batch removals IDs 9 and 10; successor ID 11 selected/opened.
- end: opened ID 39; reader closed, stack depth 1, no wrap.

Entry/wiring checked: `tst_mailopen.qml::test_localArchiveContinuesToNextPresentedReader` drives actual InboxView using real opener/archive UI, verifies selectedCardId, MailDetailPage.cardId and selected frame. `InboxView.qml::captureArchiveContinuation` snapshots subsequent visible/openable bundleModel rows before removal; returned continuation iterates candidates against current `store.cardIds()` and opens the first survivor through `openReaderEntry`. Thus this run proves the fixed actual consumer path, not a helper-only assertion.

## Undo scope and remaining work

Inspected existing `qt/tests/archive_undo_consumer.py`. It exercises actual native Qt cache/server-ACK/Undo loopback and checks restored card/order after a server snapshot, but requires `LB_UNDO_URL`, `LB_UNDO_TOKEN`, `LB_UNDO_CARD`, `LB_UNDO_NEIGHBOR`, a registered loopback fake Gmail and the `TestArchiveUndoNativeConsumer` PostgreSQL harness. No fixture invocation/wiring or credentials/source were available in this checkout (`grep` found only the driver reference; no `TestArchiveUndoNativeConsumer` wiring in `qt/CMakeLists.txt`). Did not synthesize credentials, run a different journey, or claim Undo. **Undo restoration with successor retained remains explicitly unproven for final cohort.**

## Isolation, warnings, changes, retro

No source edits, Git operations, owner desktop/session, phone, shared installer, or deploy. Created frozen copy and run artifacts; wrote this receipt only inside ROOT. Owned Xvfb was terminated; warning output includes non-fatal X server/keymap warnings. The unrelated snooze failure above remains. Retro: earlier frozen proof’s filtered test fixture used `opened:1` at filtered presented index 1 and failed before opening; repair changed it to `opened:10` and asserted filter IDs and exclusion before archive. The corrected fixture now passes, including batch deletion of 9/10 followed by 11. This confirms the prior failure was the fixture’s backing-vs-presented index mismatch, and the actual regression gates now exercise filtered and batch-successor semantics.
