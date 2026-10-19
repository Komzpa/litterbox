# Offline Live History Proof — OfflineLiveHistoryGate

Gate: OfflineLiveHistory (receipt-only; no source edits, no Git mutation, no build/rebuild, no
native, no shared installer/upgrader — not even dry-run, no production/real card mutation, no
kom/live DB). Run 2026-10-19 02:32–03:01 +04:00. OUT =
/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018; proof root OUT/offline-live-history;
disposable server port **18123**, DB **litterbox_test_offlinelive_20261019** (pg_virtualenv
private cluster, temp dir, dropped by its own trap), tenant
**8d2f6c1e-4b7a-4c3d-9f21-5a7e0b3c9d44** (all mine; QueuedRetryConsumerProof was given these
to stay distinct).

## Verdict

**PASS — live original-thread sync → cache → terminate own process → server offline → new
process, same isolated profile → original history (sender/order/html/code) + Back anchor**,
with a byte-level cache-DB provenance proof and two negative controls. This closes the exact
residual repaired-consumer-proof.md §5 left open ("fixture-warmed cache … not a live
server-synced history pull").

## 1. Source + binary identity (verified, no rebuild)

- `git rev-parse HEAD` = **fa01568ad3c4fff5ac61cf3354b6e9f4a100c367** at run time (chain
  99a00861 → a6cb742c → 9985646 → fa01568). `git diff --name-only a6cb742c HEAD` = **exactly
  one path: qt/tests/tst_bundleexpand.qml** (test-only, not linked into the app).
- Per-path `git ls-tree` blob OID+mode equal at a6cb742c and fa01568 for every path this gate
  exercises: qt/src/CardStore.cpp 97bebc23…, qt/src/CardStore.h b47bf303…,
  qt/pages/MailDetailPage.qml beb02ca1…, qt/qml/InboxView.qml 6e601a3c…,
  server/cmd/litterbox/main.go 53f1f343…, server/internal/mailbody/handler.go a3ab302b….
  Server runtime diff 99a00861 → fa01568 = only server/internal/cards/handler_test.go (test).
- Consumer binaries used **absolute and read-only, never rebuilt**:
  - Qt: OUT/build/litterbox-qt sha256 **ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981**
    (built at a6cb742c; blobs identical to fa01568 per above).
  - Server: OUT/remaining-r2/litterbox-server-stamped sha256
    **b39b02ee62fb992a01b08bcb11ae495db6495387e2b64ee7e54d17d468e49e30** (stamp 99a00861;
    served `/v1/version` → `{"server_build":"99a0086113305af72205c92326f93d44e873bf72"}`,
    logs/version.json).
- `git status`: only ` M docs/DEPLOYMENT.md` (pre-existing, other lane) + untracked
  receipts/PNGs. This gate wrote **only this receipt** into ROOT; everything else lives under
  OUT/offline-live-history.

## 2. Exact commands (all under `env -u DATABASE_URL`, never kom/live)

Server (long-lived service, single script, scripts/server.sh sha256 b9c2976e…):

    env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash \
      OUT/offline-live-history/scripts/server.sh
    # createdb litterbox_test_offlinelive_20261019
    # psql -v ON_ERROR_STOP=1 -d $DB -f ROOT/server/db/*.sql   (001..021, glob order, 014 gap expected)
    # psql -v ON_ERROR_STOP=1 -d $DB -f scripts/fixture.sql    (tenant+device+account+mail card+
    #   card_bodies+3 ordered messages+15 manual fillers; fixture.sql sha256 13275e28…)
    # DATABASE_URL=postgres://localhost:5434/<db>?user=kom&password=<pgpw>&sslmode=disable
    # LISTEN_ADDR=127.0.0.1:18123 <stamped server>   → OLH-SERVER-READY

Consumers (private HOME/XDG, Xvfb+dbus, real pointer engine, fresh process per phase):

    BIN=OUT/build/litterbox-qt
    env -u WAYLAND_DISPLAY HOME=$NEW/home XDG_CONFIG_HOME=$NEW/home/config \
      XDG_CACHE_HOME=$NEW/home/cache XDG_DATA_HOME=$NEW/home/data \
      QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop QT_QUICK_BACKEND=software \
      xvfb-run -a -e $NEW/logs/<run>-xvfb.err dbus-run-session -- \
      $BIN --capture-script $NEW/profile $NEW/scripts/<phase>.json $NEW/<captures-out>

Profile = OUT/offline-live-history/profile/profile.json =
`{"server_url":"http://127.0.0.1:18123","token":"litterbox-offline-live-token"}`.
The negative control uses profile-negative + home-negative (never synced).

## 3. Phase results (eval outputs verbatim from logs)

**A. Live endpoint fixture check** (02:54, before warm): `GET /v1/cards` Bearer
litterbox-offline-live-token → 16 cards (`now`), target card present, state open, has_body true
(logs/live-cards.json sha256 602ec3f9…). `GET /v1/cards/90100000-…-0001/body` →
3 messages in order Ada Lovelace 2026-10-15T22:54Z → Grace Hopper 2026-10-16T00:54Z →
Linus Torvalds 2026-10-16T03:54Z, latest html contains `olh_code_marker`, html contains
OLH-CARDBODY-MARKER (logs/live-body.json sha256 bc814a89…).

**B. Warm run (real server → sync → cache → terminate own process)** — 02:54:39–02:55:01,
**exit 0** (logs/warm.log sha256 f675f552…):

- `{"phase":"live-sync","online":true,"cards":16}` — cards pulled from the live server.
- reveal index 15, contentY 406 → real pointer `openCard` → `{"phase":"warm-reader","readerOpen":true}`.
- cache readback from the app after the live body fetch:
  `{"cachedKeys":["html","messages","source_url"],"htmlChars":160,"msgCount":3,"senders":[Ada…,
  Grace…, Linus…],"order":["2026-10-15T22:54…","2026-10-16T00:54…","2026-10-16T03:54…"],
  "codeInLatest":true}`; final readback before exit: `{"htmlChars":160,"msgCount":3}`.
- Real Back → `{"readerOpen":false,"contentY":406}`; screenshots
  `captures/warm/01-warm-live-reader.png` (ed5037c9…) + `02-warm-after-back.png` (a13eff30…).
- **Own process terminated**: capture run exited 0 and `pgrep -af "capture-script.*profile"` →
  no matching process (recorded in the run log tail).

**C. Cache-DB provenance (live-synced, not fixture-warmed)** — sqlite3 on
profile/cards.sqlite (post-warm, pre-kill):

- rows: mail_bodies = **1** (`90100000-…-0001`, html_len 160, messages 867 B).
- byte comparison vs the live endpoint response: `cache html == live /body html` **True**,
  `cache messages == live messages` **True**, `cache source_url == live source_url` **True** —
  the cached bytes are exactly what the real server served over the wire.
- server-side rows (psql, before kill, logs/server-rows.txt): cards=16, messages_total=3,
  senders/order as seeded.
- **negative control #1 (cache DB)**: filler card `…-0010` (manual, no body route) →
  **0 rows** in mail_bodies; never-synced profile → **0 rows**.

**D. Terminate server → offline** — 02:56:34: `kill 3160101` (my own child); gone within 1 s;
`curl http://127.0.0.1:18123/v1/version` → **exit 7 "Could not connect to server"**
(logs/offline-curl.txt); `ss -ltn sport = :18123` → no listener; no
litterbox-server-stamped process; pg_virtualenv cluster dropped on service exit (supervised
exit 0). Server stays dead for every subsequent phase.

**E. New process, same isolated profile, server offline** — 02:56:47–02:57:04, **exit 0**
(logs/offline.log sha256 bcf1552e…), a genuinely fresh app process:

- `{"phase":"relaunch-offline","online":false,"cards":16,"depth":1}` — full inbox restored
  from the local DB with the server unreachable.
- cache readback **while offline**: same senders/order/code payload as the live response
  (`msgCount:3`, `codeInBody` on latest original, `fillerNegativeEmptyKeys:0`).
- reveal → `anchorBeforeOpen:429` → real pointer openCard → `{"phase":"offline-reader",
  "readerOpen":true}` → **rendered document** (eval on the reader page, logs/offline2.log
  sha256 6f8870fa…): `{"pageMessages":3,"documentChars":1370,"documentHasCode":true,
  "documentHasAda":true,"documentHasGrace":true,"documentHasLinus":true,
  "documentHasLatestHtml":true,"documentTitle":"Offline history proof thread",
  "sourceUrl":"https://mail.google.com/mail/?authuser=owner%40example.test#…","online":false}`.
- real Back → `{"readerOpen":false,"anchorAfterBack":429}` — **Back anchor exact** (429→429).
- Screenshots: `captures/offline/03-offline-inbox.png` (490e773f…),
  `04-offline-original-history.png` (09b2c0ac…) shows the offline banner “Can't reach the
  server” plus the full original history — collapsed originals
  “Ada Lovelace <ada@example.test> · 16 Oct 02:54”, “Grace Hopper <grace@example.test> ·
  16 Oct 04:54”, latest “Linus Torvalds <linus@example.test> · 16 Oct 07:54” and the code
  block `olh_code_marker = "ok"` in monospace — `05-offline-after-back.png` (1a4b0fdf…);
  `captures/offline2/07-…` (09b2c0ac…) + `08-…` (1a4b0fdf…) byte-identical render from the
  second offline process.

**F. Negative control #2 (never-synced profile)** — separate process, profile-negative,
**exit 0**: `{"phase":"never-synced-negative","cards":0,"cachedKeys":[]}`; its cards.sqlite
has **0** mail_bodies rows; screenshot `captures/negative/06-never-synced-negative.png`
(6c29fae9…). The cache contents therefore come from this gate's live pull, not from the binary,
the fixture or a carried profile.

**G. Cache DB unchanged after all offline phases**: mail_bodies still exactly 1 row
(html_len 160, msg_len 867) — the offline reader consumed, never re-fetched.

## 4. Honest boundaries (no scope inflation)

- Proven: live server pull (cards + `/v1/cards/{id}/body` thread) → SQLite cache → process
  termination → server dead → fresh process offline reopen of the original history with
  sender/order/html/code and exact Back anchor, plus two negative controls.
- Not claimed here: retry-to-online journey (covered elsewhere), Android native, any release/
  installer surface, systemd restart_service_survival (still SKIP/unproved by its owner),
  POST /v1/ops archive-ack (not exercised by this gate; QueuedRetryConsumerProof owns it).
- Server binary carries stamp 99a00861 (not fa01568): valid because server runtime files are
  byte-identical to HEAD (§1); /v1/version readback recorded rather than assumed.
- The app at 1280×900 uses the split-view embedded reader (`depth:1, readerOpen:true`); the
  appbar-push `depth:2` shape is the narrow-width path proven in other receipts.

## 5. Retro (required — prior restart overclaim)

- **Worked:** starting with the fixture-DB provenance comparison (cache bytes == wire bytes)
  made the live-vs-fixture question a machine check instead of a narrative claim; keeping the
  server as one supervised script meant migrations, seed, readiness and teardown could not get
  out of order; running the negative-control profile as its own process turned the
  "carried-fixture" loophole into an executed test.
- **Failed in execution (mine):** fixture INSERT used uuid||text without a cast → first service
  attempt exited 3 (fixed with `::uuid`); first psql dump tried the default socket/port instead
  of the pg_virtualenv cluster (5434) and then hit the password prompt — recovered the password
  from my own server's `/proc/<pid>/environ`, never touched another cluster; offline2's first
  two attempts failed on harness rules I had not read in main.cpp: capture output dir must be
  empty and separate (`Capture script output directory must be empty…`, exit 2) and a transient
  Xvfb display collision with sibling gates (`could not connect to display :99`) — both are
  harness-side, no product change; the first offline document eval probed `stack.currentItem`
  (inbox) instead of the embedded reader page, so I re-ran it against `mailDetailPage` rather
  than letting an empty `documentChars:0` stand as evidence.
- **Surfaced for owners:** capture engine requires empty output dir — should be in README next
  to `--capture-script`; Xvfb `-a` display collisions happen under parallel gates.

## 6. Artifact index (OUT/offline-live-history)

scripts/: server.sh, fixture.sql, warm.json, offline.json, offline2.json, negative.json
(all shas in logs/identities.txt); logs/: server.log, version.json, server-rows.txt,
live-cards.json, live-body.json, warm-start/end.txt, warm.log, server-killed.txt,
offline-curl.txt, offline-start/end.txt, offline.log, offline2.log, negative.log,
identities.txt; captures/warm/{01,02}.png, captures/offline/{03,04,05}.png,
captures/offline2/{07,08}.png, captures/negative/06.png (shas in logs/identities.txt);
profile/cards.sqlite sha256 bc410192…; profile-negative/ (0 rows).
