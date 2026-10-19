# Desktop correction cohort 0.1-40 — build + proof receipt (RebuildDesktopCohortFromNativeRepair)

ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018
OUT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018 (system Qt 6.11.2 kit)
KIT=OUT/release-kit-0.1-40 (new outputs only; sibling release-kit-20261019 read-only, never mutated)
Gate start: see KIT/gate-start.txt (2026-10-19T19:49:31Z). Gate finish: see KIT/gate-finish.txt.
Working directory for all commands below: ROOT, except where `cwd` is stated.

## 0. Source identity (verified, immutable, no Git writes, no source edits)

- `git rev-parse HEAD` = `df9b13e5513ed1f1739149a3d6048c27eee3c725` (exact required value; still HEAD at finish).
- Published tuple independently proved by ROOT/receipts/final-native-git.md: remote
  `git@github.com:Komzpa/litterbox.git`, full ref `refs/heads/ui-desktop-mobile-redesign-20261018`,
  new OID `df9b13e5513ed1f1739149a3d6048c27eee3c725`. This lane issued no push/fetch/commit.
- `git status --short` shows only pre-existing untracked files (receipts/*.md, qt/mobileux-*.png,
  qt/tests/__pycache__/); zero tracked modifications before and after the build.
- Source changelog already 0.1-40 (top entry `litterbox-qt (0.1-40)`); NO `dch` edit, NO version drift,
  NO Git metadata operation in this lane.
- Frozen source bytes (ROOT, fresh sha256):
  - `qt/debian/changelog` `3b5838eac3114bdb21bff6a0ffc8d6bf642655e8e1dc5b6bbf5d25870f2f9527`
  - `qt/src/qml/CardActions.qml` `74876909774938398d3907fffb6cb9374b80344ddd9f00d242e8d730aa0a6082`
  - `qt/qml/InboxView.qml` `b984460a722910fc5eafe28f562125af11bbc9b560c62d781391405af94d4c6f`
  - `REQUIREMENTS.md` `a5a71589429b0eed3e77ce75667adedcf289e90ebbea464643842c69a9c39866`
  All four match ROOT/receipts/ui-corrections-android.md and the final-native-git receipt.
- Isolated immutable copy: `rsync -a --delete --exclude=.git --exclude=__pycache__ ./ KIT/src-immutable/`
  (exit 0); the three source shas above re-verified identical inside `KIT/src-immutable`.
  Desktop configure+build ran with `-S KIT/src-immutable/qt` so ROOT sources were never written.
  Server `go build` ran in ROOT/server (read-only source, out-of-tree output to KIT/artifacts);
  the first attempt from `src-immutable/server` failed (exit 1, `error obtaining VCS status`,
  `Use -buildvcs=false`) because the copy has no `.git` — recorded, then built from the exact
  git-present source instead. No source file was edited either way.

## 1. Native cohort preserved (not rebuilt, not re-proved)

- Exact APK `litterbox-qt-v27-b984460a.apk` sha256
  `83ad492b9f3bbb8841a647cb78a304432622aa212b3b056665475fa94d9d8e54`, v27 / 0.1-40.
- ROOT/receipts/ui-corrections-android.md records all seven native clauses PASS; inspected only.
- Served-vs-candidate ordering (23 served < 27 candidate) was proved on the native side;
  no fresh served readback was needed here and none was performed (no credentials touched).

## 2. Builds (same EXACT source, exits recorded, each run ONCE)

| Step | Actual command (cwd) | Exit | Evidence |
|---|---|---|---|
| configure | `cmake -S KIT/src-immutable/qt -B KIT/build-desktop -G Ninja -DCMAKE_BUILD_TYPE=RelWithDebInfo -DQt6_DIR=/usr/lib/x86_64-linux-gnu/cmake/Qt6` (ROOT) | 0 | KIT/logs/configure.log |
| desktop+tests | `cmake --build KIT/build-desktop -j$(nproc)` (ROOT) | 0 | KIT/logs/build.log; binary KIT/build-desktop/litterbox-qt sha256 `b5e62a31f384bf8437777ccc428483d510d1e3ef32b013357c509811f3be06f5` (KIT/logs/binary-sha.txt) |
| server | `go build -o KIT/artifacts/litterbox-server-df9b13e ./cmd/litterbox` (ROOT/server) | 0 | KIT/logs/server-build.log; sha256 `d5d5a8dab1f8307a519c1f831b913449d3d47ad555079327b5f23c8f9ffb57dd` (KIT/logs/server-sha.txt) |
| scoped checks | `ctest --output-on-failure` (KIT/build-desktop) | 8 | KIT/logs/ctest.log: **25/26 passed, 1 failed**, restart_service_survival skipped (1/1). All failures collected in the single run; no broad re-runs |
| server deb | `sh deploy/server/debian/build.sh KIT/artifacts/litterbox-server-df9b13e 0.1-40 KIT/artifacts` (ROOT) | 0 | KIT/logs/server-deb.log; `litterbox-server_0.1-40_amd64.deb` (`Package: litterbox-server, Version: 0.1-40, Arch: amd64`) sha256 `d8348203e1c1f0688c928cd7342c2316b0b140e90b2792860e52875fcb7d404b` |
| desktop deb | `dpkg-buildpackage -b -us -uc` (KIT/pkg-src/qt, staged by rsync from KIT/src-immutable/qt, NO dch edit) | 0 | KIT/logs/desktop-deb.log; `litterbox-qt_0.1-40_amd64.deb` (`Package: litterbox-qt, Version: 0.1-40, Arch: amd64`) sha256 `c7245d558ae86e8847d20eec1d9afce94e5ff5cdde9e33e147c07e39687ed3ad`; dbgsym sha256 `f6be1c61b665ed165e4e2b12a32616e369dfe9349600fc7dee13e367760a0b9c` (copies in KIT/artifacts/, hashes in KIT/logs/deb-sha.txt) |

Forbidden actions: none performed — no install/restart/deploy/publication, no owner desktop/phone/TV,
no shared installer, no Git mutation/fetch, no real mail, no `qt/android-build.sh`.

## 3. Package provenance (inner binary vs proof binary — disclosed, not assumed)

- Desktop deb inner `/usr/bin/litterbox-qt` (via `dpkg-deb --fsys-tarfile | tar -xO`, KIT/logs/deb-inner.log,
  exit 0) sha256 `56d975e1f87c4c6ce325700cb6ce085c91d156b55637d5278d8394c7bb3cd5cd` —
  DIFFERENT from the proof binary `b5e62a31…` (packaging rebuild from identical source; both hashes
  recorded, neither assumed equal). Disclosure as required.
- Server deb inner `./opt/litterbox/server/litterbox` sha256
  `d5d5a8dab1f8307a519c1f831b913449d3d47ad555079327b5f23c8f9ffb57dd` — EQUAL to the built
  server binary (verified).
- Because the desktop hashes differ, the actual packaged binary was smoked on the changed path
  (reader Stack continuation, j2-archive): exit 0, `cardId=A alpha:true` → `cardId=B beta:true,
  hasA:false, hasB:true, status:Archived` → `hasA:true, hasB:true, cards:6`
  (KIT/logs/journey-packaged-smoke.log; captures KIT/journeys/run-packaged-smoke/captures/).
  First packaged-smoke attempt exited 1 (`dbus-run-session: failed to exec ... Permission denied`)
  because `tar -xO` redirect drops the exec bit; after `chmod +x` the rerun is green.
  First-failure kept, not hidden.

## 4. Desktop journeys (real Qt Xvfb production consumer; private HOME/bus/DB; isolated fixture)

Fixture: KIT/journeys/profile copied from sibling release-kit-20261019/journeys/profile (READ-ONLY copy;
sibling untouched): 4-column `cards.sqlite` (6 cards, 6 mail_bodies with ALPHA/BETA markers),
`profile.json` (`http://127.0.0.1:9` unreachable + smoke token), per-journey copies under
KIT/journeys/run-*. Scripts KIT/journeys/scripts/{j1-avatars,j2-archive,j3-snooze}.json are byte copies
of the prior kit (method reuse, not pass reuse); j4-geometry.json is new supplementary geometry for the
date-grid/reader-Stack neighborhood. Each run: private HOME/XDG dirs (0700), `QT_QPA_PLATFORM=xcb`,
`QT_QUICK_BACKEND=software`, `DBUS_SYSTEM_BUS_ADDRESS=unix:path=/nonexistent`,
`xvfb-run -a dbus-run-session -- <binary> --capture-script <run-profile> <script> <captures>`.
All runs use the proof binary except the packaged smoke above.

| Journey | Exit | Key evidence (from logs + opened PNGs, not filenames) |
|---|---|---|
| j1-avatars (bundle expand, differing senders) | 0 (KIT/logs/journey-j1.log) | `cards:6`; `m1vis:true, m2vis:true`. j1-03-members.png actually shows the expanded bundle: `OpenDroneMap previews mesh quality leap via OpenMVS / 3 emails`, member rows `LinkedIn` (L), `Re: … / Darafei via LinkedIn` (DL), `follow-up on mesh quality / Martin Dobias` (MD), plus standalone MQ/DL/G/L avatars and the shared head `LinkedIn, Darafei via LinkedIn +1` |
| j2-archive (open A → archive → continuation B → Undo keep B, restore A) | 0 (KIT/logs/journey-j2.log) | A opened `cardId=A alpha:true`; after `archiveMail`: `cardId=B beta:true, hasA:false, hasB:true, status:Archived` (real B body, no manual open); after `bundleArchiveUndoButton`: `hasA:true, hasB:true, cards:6`. j2-01-A-open.png shows `ALPHA-BODY-MARKER Hello QGIS team, this is the real A body`; j2-02-continued-B.png shows `BETA-BODY-MARKER Thanks for reading, this is the real B body` with the `Archived … Undo` bar; j2-03-after-undo.png shows the restored list (Alpha + Beta + Weekly + bundle) with B selected and its real body |
| j3-snooze (preset menu + persisted until + custom OK + Cancel no-op; Kiritimati TZ) | 0 (KIT/logs/journey-j3.log) | menu rows all `vis:true h:54 w:297` (hour/evening/tomorrow/monday/custom); preset-tomorrow → `pendingOps:1, hasC:false, readerOpen:false`; outbox payload `{"type":"snooze","card_id":C,"args":{"until":"2026-10-20T19:00:00.000Z"}}` (real local→UTC payload, `17024cd4-…`); custom OK `108x54 vis:true` / Cancel `108x54 vis:true`; Cancel → `pendingOps:1, hasB:true` (no-op). j3-01-snooze-menu.png shows labelled `In one hour / Tonight / Tomorrow morning / Next Monday morning / Custom date and time`; j3-03-custom-picker.png shows `Snooze until / Year 2026 / Month 10 / Day 20 / Time 10 56`, `Brings the card back 20 Oct 10:56`, labelled `OK` + `Cancel` |
| j4-geometry (date-grid + reader-Stack neighborhood; wide 1280 + narrow 500) | 0 (KIT/logs/journey-j4.log) | wide: Year `vis:true w:343 h:54 val:2026`, Month `w:343 val:10`, Day `w:343 val:20`, Hour `w:169`, Minute `w:168`, OK/Cancel `108x54 vis:true txt:OK/Cancel`; narrow (500px): Year/Month/Day still `vis:true w:343 h:54 val:2026/10/20`, OK/Cancel `108x54`, `depth:2`; after Cancel `pendingOps:0, hasB:true`. j4-02-custom-narrow.png shows `Year 2026 / Month 10 / Day 20 / Time 0 57`, `Brings the card back 20 Oct 00:57`, labelled `OK` + `Cancel` — values beside step controls, no zero-width digits, no missing buttons at narrow width |

Outbox note: j2 nets zero ops (archive+Undo, correct); j3 holds exactly the one persisted snooze op;
j4 Cancel holds zero (correct no-op). No real mail was touched (unreachable fixture server + local DB only).

## 5. Residual failures (all of them; no half-completed proposal)

1. **ctest `tst_mailopen` 1/26 FAILED (exit 8 overall; the single scoped run).**
   Failing case: `maildetail::MailCardOpening::test_desktopSplitSelectionResizeNarrowBackAndArchiveClosure`,
   `property readerOpen — Actual: false, Expected: true`,
   `Loc: […]/src-immutable/qt/tests/tst_mailopen.qml(496)`, with
   `InboxView.qml:836: TypeError: Cannot read property 'length' of undefined` in the same case
   (KIT/logs/ctest.log). Every other test passed (25/26; restart_service_survival skipped, 1/1).
   This case exercises the desktop split-selection resize/narrow/back/archive-closure path — i.e. the
   reader-Stack neighborhood this cohort changes. It is a REAL residual failure on the exact 0.1-40
   source, collected once, not rerun, not waived, and NOT covered by the journey evidence above
   (journeys prove archive→B→Undo and wide/narrow date-grid, not this resize/back closure).
   Owner triage required; cohort artifacts below are exact-ready but the suite is not all-green.
   **RESOLVED 2026-10-20 (test-only; product unchanged).** The settled `RepairDesktopReaderRegression`
   worker closed this residual: `qt/tests/tst_mailopen.qml` sha256
   `ce79e13add876b7b33e0e7f6428972d946d73284c8b6282f150d2a0eabba3c49` — 3 insertions only: the fixture
   store gains `offlineReason` to match the production `CardStore` contract, `wait(150)` lets the
   product's one-turn deferred anchor restore land before the click, and the file's own anchor assert
   is added at the final 1480 resize step. `qt/qml/InboxView.qml` is byte-unchanged
   (`b984460a722910fc5eafe28f562125af11bbc9b560c62d781391405af94d4c6f`), so frozen cohort `df9b13e`
   and its native proof stand. Exact proof: ROOT/receipts/desktop-reader-regression.md (cause chain,
   3-insertion fix, negative controls ctlA/ctlB/ctlC); OUT/reader-regression-20261020/logs/tst_maildetail-neighbor.log
   (tst_mailopen 27 passed / 0 failed incl. the repaired case + neighbors); OUT/reader-regression-20261020/logs/journey-j2-rerun.log
   + run-j2-archive/captures/j2-0{1,2,3}-* (archive → real B body → Undo) and
   OUT/reader-regression-20261020/logs/journey-j4-rerun.log + run-j4-geometry/captures/j4-0{1,2,3}-*
   (wide/narrow geometry), each exit 0. The baseline failure above is retained unchanged as the
   pre-fix record.
2. **First packaged-smoke attempt exited 1** (`Permission denied`: `tar -xO` redirect drops exec bit).
   Fixture/harness-side only; after `chmod +x` the rerun exits 0 on the changed path (§3). Not residual,
   recorded as the kept first-failure.
3. **Server build from the git-less immutable copy exited 1** (`error obtaining VCS status`).
   Build-side only; rebuilt from the exact git-present source, exit 0 (§2). Not residual, recorded.

No other failures: configure 0, desktop build 0, server build (final) 0, server deb 0, desktop deb 0,
j1/j2/j3/j4 0, packaged smoke (final) 0.

## 6. Cohort-ready artifacts (exact paths + sha256)

- KIT/build-desktop/litterbox-qt — `b5e62a31f384bf8437777ccc428483d510d1e3ef32b013357c509811f3be06f5`
- /tmp/lb-deb-inner-0140/litterbox-qt (deb inner, differs — see §3) — `56d975e1f87c4c6ce325700cb6ce085c91d156b55637d5278d8394c7bb3cd5cd`
- KIT/artifacts/litterbox-qt_0.1-40_amd64.deb — `c7245d558ae86e8847d20eec1d9afce94e5ff5cdde9e33e147c07e39687ed3ad`
- KIT/artifacts/litterbox-qt-dbgsym_0.1-40_amd64.deb — `f6be1c61b665ed165e4e2b12a32616e369dfe9349600fc7dee13e367760a0b9c`
- KIT/artifacts/litterbox-server-df9b13e — `d5d5a8dab1f8307a519c1f831b913449d3d47ad555079327b5f23c8f9ffb57dd`
- KIT/artifacts/litterbox-server_0.1-40_amd64.deb — `d8348203e1c1f0688c928cd7342c2316b0b140e90b2792860e52875fcb7d404b`
  (inner `./opt/litterbox/server/litterbox` identical `d5d5a8da…`)
- Native (preserved, not rebuilt): repair-native-20261019/v27/litterbox-qt-v27-b984460a.apk —
  `83ad492b9f3bbb8841a647cb78a304432622aa212b3b056665475fa94d9d8e54`
- Captures: KIT/journeys/run-{j1-avatars,j2-archive,j3-snooze,j4-geometry,packaged-smoke}/captures/*.png+json
- Logs: KIT/logs/{configure,build,server-build,ctest,server-deb,desktop-deb,deb-inner,journey-j1,journey-j2,journey-j3,journey-j4,journey-packaged-smoke}.log + *-sha.txt

## 7. Retro (required)

Existing `history://ReviewNativeTapLoopIncident` was read in full: its conclusion stands —
**no instruction change warranted** (prior popup crawl, wrong viewport, claim drift, unanswered steers
were execution/application failures already covered). This cohort adds three execution lessons, all
fixture/harness-side, no product change, no requirement weakened: (1) seeding `cards.sqlite` MUST use the
4-column schema with populated `sender_name` (legacy 3-column migration defaults it to `''`, hiding sender
lines/avatars); (2) popup/dialog probes must walk from `window` (they live outside `contentItem`), while
`click` steps resolve fine; (3) a deb-inner binary extracted via `tar -xO` redirect loses the exec bit —
`chmod +x` before smoking. The one product-adjacent signal is the §5.1 ctest failure in the
resize/narrow/back/archive-closure path: it overlapped the reader-Stack change neighborhood and was
triaged to the owner; it is now resolved by the fixture-only repair recorded in
§5.1: the cause was a harness timing/fixture gap, not a product defect, and no requirement was
weakened. No new instruction campaign is proposed.

## 8. Handoff

Cohort is exact-ready (same-source desktop+server binaries and both 0.1-40 debs, journeys green on the
proof binary, packaged binary smoked on the changed reader path) with the former suite residual (§5.1) resolved by the fixture-only test repair
(tst_mailopen 27/27, tst_maildetail 44/44; product bytes unchanged).
No install/restart/deploy/publication was performed; publication-time fresh served readback stays with
the owner (native 27 vs served 23 already proved strictly newer on the native side). This receipt and
ROOT/receipts/desktop-reader-regression.md are committed together with the test-only fixture delta by the
fixture-handoff lane; product/source files, APK/AVD, release artifacts, and release publication state are
unchanged by this lane. Stale warning: none — pre-commit HEAD was `df9b13e…`; resource warning: none —
sibling kit only read, private bus/HOME/DB per journey, no owner surfaces touched.
