# Remaining Consumer Proof — Litterbox desktop/mobile (RemainingConsumerGate)

Gate: RemainingConsumerGate. Run 2026-10-19 00:38–01:10 +04:00. ROOT READONLY except this
receipt. No source edits by this gate, no Git mutation, no deploy/install/shared-installer,
 no live/default DB, no native emulator. OUT = /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018;
 proof root OUT/remaining-r2; build OUT/build; harness generator OUT/repair-r2-harness
 (writer: DesktopHarnessRecovery — never edited by this gate).

## 1. Source + build identity (verified before build)

- Verified frozen SHA **99a0086113305af72205c92326f93d44e873bf72** ("Restore Gmail ingestion and
  native reader popup visibility"). Re-read `git rev-parse HEAD` = 99a00861 before build.
- Rebuilt existing OUT/build with `cmake --build OUT/build -j32`, **no reconfigure** (proven
  kit kept: Qt 6.11.2 system + system Kirigami, LB_TEST_PLATFORM=xcb). **Build exit 0.**
  litterbox-qt sha256 **6b1d3473aa1f7e0557db3d28f1fc5e6724db4f16ed5328b99f1190c430d31b0e**.
- Server build stamp (CF26): `go build -ldflags "-X main.buildSHA=99a00861…"` → stamped binary
  sha256 b39b02ee62fb992a01b08bcb11ae495db6495387e2b64ee7e54d17d468e49e30; isolated
  `/v1/version` on a disposable pg_virtualenv cluster returned
  `{"server_build":"99a0086113305af72205c92326f93d44e873bf72"}` — exact full SHA, not "unknown".
- **Staleness boundary (honest):** during this gate Main's *ConsumerRepairCommit* landed as
  HEAD **a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75** (99a00861 is its parent; delta = 7 test/
  QML paths + cards handler_test). Main is sole Git owner; every build/suite/journey in this
  receipt ran against **99a00861**, so this proof is valid for that SHA. Re-verification on
  a6cb742c is QtRepairedConsumerGate's job (rebuild + four affected Qt consumers + isolated
  note SSE), per Main's directive. No broad Qt/server suites repeated here after the freeze.

## 2. Full Qt CTest suite — whole owning suite, non-stop-first, all 26

`xvfb-run -a env QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop
QT_QUICK_BACKEND=software LB_UI_PROOF_DIR=…/remaining-r2/qt ctest --test-dir OUT/build
--parallel 1 --output-on-failure --no-tests=error --output-junit …/remaining-r2/qt/ctest.xml`

- **Exit 8. 21 Passed / 4 Failed / 1 Skipped of 26.** (Baseline frozen: 16/9/1. repair-r1: 22/4/1.)
- **Corrected JUnit counts** (old receipt's 22+4+1 was inconsistent): recount from ctest.xml =
  total 26 / pass 21 / fail 4 / skip 1. JUnit sha256
  8dcbec71ca23486624ca1bda2a3bd1d70042da7c1972f2e091bbcc151dfd77d4; log sha256
  d6944b27a3ff5a6189289c3369f8966601f1338261e63ea167f6cfe82686eda9.
- **Finite failure map (4 suites / 11 assertions), no broad repeats:**
  1. **tst_uirequirements** (2 pass/1 fail): realInboxActionsAreNamedAndLargeAndSheetPaletteIsLight
     `verify()` false @qml:100.
  2. **tst_inboxlayout** (17 pass/1 fail): cleanPreviewGrid(1440) compare @qml:428 (rail/split
     boundary + 1200 other comparisons pass).
  3. **tst_mailopen** (20 pass/3 fail): actionReturnsToSameInboxOffset archive-button + done-body
     `depth` @qml:248; previewOpensMailAndBackPreservesScroll `depth` @qml:103 — reader-Back
     depth regressed (expected vs observed).
  4. **bundleexpand** (34 pass/6 fail): senderSummary one/two/more/missing/custom-topic-1356
     `sender` @qml:107 (renders messages-noreply@linkedin.com); singleImportantMemberDisclosure
     AndCount `count` @qml:337.
  - **Skip (NOT pass):** restart_service_survival exit 77 (no systemd user manager on private
    bus) — unproved, recorded as skip.

## 3. Full Go server suite — pg_virtualenv private cluster, explicit gates, all packages

`env -u DATABASE_URL pg_virtualenv bash -c 'createdb litterbox_test_remaining_*;
DATABASE_URL=<disposable localhost:5434>; CARD_TEST_POSTGRES=1; BUNDLE_TEST_POSTGRES=1;
TECHSHIP_SOURCE_SNAPSHOT=…; LB_QT_UNDO_BINARY=OUT/build/litterbox-qt; LB_QT_UNDO_OUTPUT=…;
go test -count=1 ./…'` — ambient DATABASE_URL unset first; kom/default postgres and live
services untouched; cluster dropped by its own trap.

- **Exit 1** — but the two *frozen-source build errors* from the old receipt are now **fixed**
  (HEAD 99a00861 restored Gmail ingestion): `cmd/litterbox` ok, `internal/gmailsync` compiles.
- **Both required real gates RAN (not skipped-green):**
  - **TestNewsletterPersistedList — PASS.** TECHSHIP snapshot 0ee5ad3da7397fcfef4ac8d1f8d3c975e1d7b1d648791e0fb54ab6dc5381dcca, 3 immutable cards, hash matched inside the test; the exact-hash
    provenance/repair/rollback fixtures (title-origin-r2.json/-repair.sql/-rollback.sql) enabled
    and read. Newsletter consumer **proved**, not skip-green.
  - **TestArchiveUndoNativeConsumer — PASS (23 s).** Acknowledged Undo via the real gmailsync
    consumer + real `qt/tests/archive_undo_consumer.py` driving the rebuilt binary
    (LB_QT_UNDO_BINARY=OUT/build/litterbox-qt) against a loopback fake-Gmail httptest server.
    Neighbor card untouched, own token revoked, disposable schema/role/cluster cleaned. This is
    the acknowledged-Undo native-consumer gate the old receipt left UNPROVED.
- **1 real failure, correctly attributed:** TestPostgresNotePersistence —
  `cards/handler_test.go:177 timed out waiting for cards event` (readEvent 2 s deadline @:127).
  Reproduced in isolation → not a load flake. **Main reclassified it: a test-listener miswire**
  (real SSE consumer test used `NewEvents("")` = default DB instead of `NewEvents(tdb.DSN)`;
  PostgreSQL notifications are database-scoped; production already uses
  `StartEvents(databaseURL)`). Fixed in Main's a6cb742c. **Not a product bug**; its proof belongs
  to QtRepairedConsumerGate on the new commit. No timeout/assertion changed here; tenant controls
  preserved.

## 4. CF17 — control-UUID / visible-target mapping (validated BEFORE the journeys)

- Generator output checked across all 5 journey scripts: **0 non-UUID action/lookup tokens**;
  every `openCard` is a bare v5 UUID; bundle controls rekeyed to the leader card UUID
  (`bundleToggle-8d68d94f-…` etc.); a live-model reveal (`positionViewAtIndex`) precedes every
  openCard/bundle action (7 desktop / 5 phone). Leader-UUID rekey + reveal **correct**.
- **Known-good trivial harness smoke against the real binary PASSED** (the Main-required CF17
  gate before the full journeys): reveal returns `{ok:true,visible:true}`, leader-UUID
  `bundleToggle` click succeeds (no "unknown/non-visible"), UUID `openCard` → `readerOpen:true,
  depth:1`. Control reachability proved end-to-end.
- **Two harness generator bugs found, reported to DesktopHarnessRecovery, and worked around in my
  own copies only** (gen.py never edited): (a) `reveal_card` IIFE was **never invoked** (string
  ended `…})` not `…})()`) → reveal returned empty and never scrolled; (b) it used the **invalid
  enum `ListView.AlignContain`** (probe: `String(ListView.AlignContain)='undefined'`; correct is
  `ListView.Contain`). Both are harness-side, not product. Reported verbatim to the owner for the
  canonical freeze.

## 5. Five visual journeys — actual production binary, private HOME/XDG, Xvfb+dbus, pure-UI
##     server-endpoint NONE — **ALL FIVE PASS (exit 0)**

Entry: `litterbox-qt --capture-script <profile> <script.json> <outdir>` (README mechanism),
reseeded per run from the CF17-fixed fixture. Each journey drives the real QML via production
objectNames; no fake operations, no production-UUID bypass.

| Journey | Exit | Shots | Stages reached |
|---|---|---|---|
| desktop-1440x1000 | **0** | 27 | 1–26 complete |
| desktop-1280x800 | **0** | 27 | 1–26 complete |
| phone-412x915 | **0** | 17 | inbox→drawer→nav→search→reader→Back→Archive/Undo→bundle expand/options/archive/undo→external-removal→offline→scrolled |
| phone-480x915 | **0** | 17 | same |
| phone-412x915-largetext | **0** | 17 | same; **`largeText:true`** confirmed |

- Previous stopping points cleared: desktop reached 19–26 (was aborting at 19–26 in repair-r1),
  phones reached 53+ then the external-removal/offline/scrolled stages (were stuck at 53).
- **ArchiveUndo durable-outbox between states, actual app, real outbox path** (evals, not
  assumptions):
  - *single Archive→Undo* (desktop-1440): `archive-clicked pendingOps:1 status:"Archived"
    hasQgis:false` → `before-undo pendingOps:1` → `after-undo hasQgis:true cards:15` — card
    removed then restored by the real Undo bar; op queued (pendingOps 1) then cleared.
  - *bundle Archive→Undo* (desktop-1440): `bundle-archive-clicked pendingOps:3 cards:12
    status:"Archived 3 emails" hasOdm:false` → `bundle-after-undo cards:15 hasOdm:true` — 3
    per-member ops, whole bundle restored.
  - *external removal while reader open*: `selected jasmine readerOpen:true depth:1` →
    `readerOpen:false hasSelected:false cards:14` — reader closes, card permanently gone.
  - *offline + back-online + scrolled anchor*: offline banner shot, Retry, back-online, scrolled
    landmark (contentY 320→320 restore on desktop; contentY 672 on phone).
  - Final profile: `outbox=0, cards=14` — all queued ops flushed/undone, only the externally
    removed card absent. **No production card mutated; production DB never touched.**
- Harness-side fixture retargets applied to my own copies (faithful, not fabricated):
  (i) `state:"open"` added to fixture mail cards — verified server producer contract emits
  `c.state` (`cards.go:98` filters `c.State != "open"`, `handler.go` selects `c.state`), so the
  fixture matches production; (ii) phone Archive/Undo opener odm→qgis and phone removal
  salt→jasmine, because the **collapsed bundle-leader's `openCard` grid is intentionally hidden**
  (InboxView.qml:1588 `visible: !cardRow.bundleLeader || cardRow.bundleExpanded`) — read from
  source, not a zero-width bug; snapshot rebuilt to exclude the new removal card; 150 % removal
  reveal uses `ListView.End` so the tall row sits fully in-viewport.

## 6. Clause → production entry / wiring / behaviour (no clause waived, helpers never counted as pass)

| Frozen clause | Production entry + wiring | Behaviour result |
|---|---|---|
| Adaptive lifecycle (Back/resize/selection) | InboxView openPage/closeReader/handleBack; MailDetailPage close callback | **PASS in journeys** (phone Back depth 2→1, narrow-Back, mouse/Escape/handleBack each logged) + touchdefects/embeddedBack CTest PASS; 3 tst_mailopen depth assertions FAIL → owned by a6cb742c |
| Meaningful rows/geometry | rail/split breakpoints, preview grid, subject/date/density | **PASS in journeys** (density toggle, 1440/1280/598 resize, scroll-restore); cleanPreviewGrid(1440) FAIL → a6cb742c |
| Navigation/search/sync | drawer, navigateTo, search clear + anchor restore | **PASS** (mobileux 3/3 + all 5 journeys: drawer/nav/search 2→0→15→clear) |
| Distinct reversible actions | CardActions → CardStore outbox → Undo bar | **PASS** (real app Archive+Undo, bundle Archive+Undo, pendingOps 1/3, restore; acknowledged server Undo consumer PASS) |
| Originals/offline/accessibility | MailDetailPage native disclosure, offline cache, 150 % text | disclosure CTest PASS; **offline banner/Retry PASS in all journeys**; **150 % text `largeText:true` proved** (source-determined kdeglobals 15 pt mechanism) |
| Durable outbox / external removal / offline retry / scrolled anchor / pinned-important metadata | CardStore outbox, applyRemoteCards, offline state, ListView contentY | **PASS** (external removal closes reader + card gone; offline→Retry→back-online; contentY restore; fixture pinned postgis6144 rank1/important) |
| TECHSHIP newsletter consumer | TECHSHIP_SOURCE_SNAPSHOT-gated cards consumer | **PASS** (real snapshot + provenance/repair/rollback, hash-verified) |
| Acknowledged ArchiveUndo consumer | gmailsync consumer + real Qt consumer driver | **PASS** (23 s, loopback Gmail, neighbor untouched) |
| Server build stamp | `main.buildSHA` → `/v1/version` | **PASS** (exact 99a00861) |
| restart-service survival | systemd user manager | **SKIP exit 77 — unproved, not passed** |
| Documentation/requirements writeback; Git handoff | Main-owned | **UNPROVED here (Main-owned)** |
| Native Android | NativeAcceptanceNext (emulator-5590) | **UNPROVED here — other owner, untouched, no AVD used** |
| Note SSE persistence | cards event stream | **reclassified: test-listener miswire, fixed a6cb742c; proof → QtRepairedConsumerGate** |

## 7. What this gate did NOT do (explicit, no scope shrink)

No source edits, no Git mutation beyond read-only rev-parse/status/diff/log/merge-base, no
deploy/install/updater/shared-installer even dry-run, no live/default/production DB, no production
card mutation, no real phone/TV/owner-desktop, no credentials, no native emulator, no test
filtering or stop-on-failure, no fabricated Undo, no production guard change, no QT_FONT_DPI
assumption (150 % is kdeglobals 15 pt). The 4 Qt suite failures and the note-SSE item are left
for the owning lanes on a6cb742c — not waived, not silently dropped.

## 8. Retro (required)

- **Worked:** verifying the SHA readback before building (caught a mistyped-SHA class of error
  from a prior lane); building a *trivial* CF17 smoke first — it exposed the two gen.py reveal
  bugs (un-invoked IIFE + invalid `AlignContain`) cheaply, before they could poison five long
  journeys; reading the visibility rule from InboxView.qml:1588 instead of guessing that the
  phone `openCard` failure was a zero-width layout bug; verifying the server `c.state` producer
  contract before adding `state:"open"` to the fixture, so the retarget stayed faithful.
- **Failed in execution (mine):** my first trivial smoke used a bare `bundleToggle` identifier
  (not a context property) and a non-invoked reveal — my script bug, fixed before the journeys;
  I initially misattributed the phone openCard abort as a possible product defect until Main
  pointed me at the intentional collapsed-leader rule; my first removal-card patch swapped only
  the visible-assert eval, not the preceding position reveal, so the 150 % run still aborted —
  corrected by swapping both and using `ListView.End`.
- **Surfaced for owners:** gen.py reveal_card (invoke + `ListView.Contain`) — DesktopHarnessRecovery;
  4 Qt consumer suites + note SSE — QtRepairedConsumerGate on a6cb742c; native Android —
  NativeAcceptanceNext.

## 9. Artifact index

OUT = /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018; proof root OUT/remaining-r2:
- logs/: build.log, build-timeline.log, binary-sha.txt, prebuild-binary-sha.txt, ctest.log,
  ctest-timeline.log, server.log, server-timeline.log, journey-runs-r2.log (first full pass),
  journey-runs-r3.log, journey-phones.log, journey-largetext.log, journey-timeline.log,
  smoke-trivial*.log, diag/enum/cw/probe logs, phone-probe logs, font evidence.
- qt/ctest.xml (JUnit, corrected counts).
- server: litterbox-server-stamped (CF26 stamp proof); native-undo/ (ArchiveUndo consumer output).
- captures-r3/ (desktop-1440x1000, desktop-1280x800 — 27 shots + profile snapshots),
  captures/ (phone-412x915, phone-480x915, phone-412x915-largetext — 17 shots + profile
  snapshots), captures-r2/ (aborted intermediate run, retained for the record).
- scripts/ (5 CF17-fixed journey JSON), scripts-orig/ (pre-fix copies), patch_reveal.py,
  patch_journeys.py, patch_removal.py, run_all.sh, run_phones.sh, run_largetext.sh, profile/
  (reseeded fixture DB, 15 cards, state:"open").
