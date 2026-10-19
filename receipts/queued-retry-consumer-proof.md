# Queued Retry Consumer Proof — QueuedRetryExecution

Gate: QueuedRetryConsumerProof (receipt-only; no source edits, no Git mutation, no
build/rebuild, no native, no shared installer/upgrader — not even dry-run, no
production/real card mutation, no kom/live DB). Run 2026-10-19 03:22–03:33 +04:00.
OUT = /home/kom/proj/ai_pr/litterbox-desktop-proof-20261018; proof root OUT/queued-retry;
disposable server port **18125**, DB **litterbox_test_queuedretry_20261019**
(pg_virtualenv private cluster on localhost:5437, temp dir, dropped by its own trap),
tenant **e4d7b2c9-3f61-4a58-9c0d-7b2e5f8a1d33** (all mine; distinct from
offline-live-history's 18123/8d2f6c1e…, whose fixture I never touched).

## Verdict

**PASS — one real client scenario end to end**: normal fixture sync from the live
server → server killed (verified unreachable) → real UI Archive click on one disposable
mail card → visible `connectionStatus` row **"Can't reach the server · 1 change waiting"**
+ durable sqlite outbox row (1 pending op) → server restarted on the same disposable
DB → real retry posts **POST /v1/ops** → actual `{"ok":true}` ack → `pendingOps` 1→0,
outbox row deleted, server card state **archived**, neighbor card **unchanged** — with
server-table readback, an op-identity match (client outbox op_id == server ops.op_id),
and a never-synced negative control.

## 1. Source + binary identity (verified, no rebuild)

- `git rev-parse HEAD` = **fa01568ad3c4fff5ac61cf3354b6e9f4a100c367** at run time.
  `git diff --name-only a6cb742c HEAD` = **exactly one path: qt/tests/tst_bundleexpand.qml**
  (test-only, not linked into the app).
- Per-path `git ls-tree` blob OID equal at a6cb742c (client build commit) and fa01568
  for every path this gate exercises: qt/src/main.cpp 98239d0e…, qt/src/CardStore.cpp
  97bebc23…, qt/src/CardStore.h b47bf303…, qt/qml/InboxView.qml 6e601a3c…,
  qt/src/qml/CardActions.qml 30c16c01…, qt/pages/MailDetailPage.qml beb02ca1…,
  server/internal/ops/ops.go 4ca87717…, server/cmd/litterbox/main.go 53f1f343….
- Consumer binaries used **absolute and read-only, never rebuilt**:
  - Qt: OUT/build/litterbox-qt sha256 **ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981**
    (built at a6cb742c; blobs identical to fa01568 per above).
  - Server: OUT/remaining-r2/litterbox-server-stamped sha256
    **b39b02ee62fb992a01b08bcb11ae495db6495387e2b64ee7e54d17d468e49e30** (stamp 99a00861;
    served `/v1/version` → `{"api":1,"min_client_api":1,"server_build":"99a0086113305af72205c92326f93d44e873bf72"}`
    at start **and** after restart, logs/version.json + logs/version-restart.json).
- `git status`: only ` M docs/DEPLOYMENT.md` (pre-existing, other lane) + untracked
  receipts/PNGs (other lanes). This gate wrote **only this receipt** into ROOT;
  everything else lives under OUT/queued-retry.
- The stamped server carries 99a00861, not fa01568: valid because every server runtime
  path it executes is blob-identical at HEAD (§1); `/v1/version` was read back, not assumed.

## 2. Exact commands (all under `env -u DATABASE_URL`, never kom/live)

Server (long-lived supervised service; single script OUT/queued-retry/scripts/server.sh;
the control loop keeps the pg cluster alive across kill/restart of only the server binary):

    env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash \
      OUT/queued-retry/scripts/server.sh
    # createdb litterbox_test_queuedretry_20261019           (cluster localhost:5437)
    # psql -v ON_ERROR_STOP=1 -d $DB -f ROOT/server/db/*.sql  (001..021, 014 gap expected)
    # psql -v ON_ERROR_STOP=1 -d $DB -f scripts/fixture.sql   (tenant+device+account+
    #   2 mail cards+card_bodies+2 messages)
    # DATABASE_URL=<dsn in logs/dsn.txt> LISTEN_ADDR=127.0.0.1:18125 <stamped server>
    #   → QRY-SERVER-READY port=18125 pid=3355763
    kill $(cat logs/server.pid)                # offline: server binary only, cluster kept
    touch control/restart                      # → QRY-SERVER-RESTARTING, new pid 3405809
    touch control/exit                         # teardown after all reads (cluster dropped)

Consumers (private HOME/XDG, Xvfb+dbus, real pointer engine, fresh process per phase,
profile = OUT/queued-retry/profile/profile.json =
`{"server_url":"http://127.0.0.1:18125","token":"litterbox-queuedretry-token"}`;
fixture device token_hash = sha256('litterbox-queuedretry-token'), bearer auth as usual):

    BIN=OUT/build/litterbox-qt
    env -u WAYLAND_DISPLAY HOME=$NEW/home XDG_CONFIG_HOME=$NEW/home/config \
      XDG_CACHE_HOME=$NEW/home/cache XDG_DATA_HOME=$NEW/home/data \
      QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop QT_QUICK_BACKEND=software \
      xvfb-run -a -e $NEW/logs/<phase>-xvfb.err dbus-run-session -- \
      $BIN --capture-script $NEW/profile $NEW/scripts/<phase>.json $NEW/<captures-out>
    # phases: warm (captures/warm, exit 0), offline (captures/offline, exit 4 — see §5),
    #         offline2 (captures/offline2, exit 0), retry (captures/retry, exit 0),
    #         negative (profile-negative+home-negative, captures/negative, exit 0)

## 3. Production entry + wiring + real consumer (queuedRetry clause)

The offline archive and its retry run through the exact production paths, not a
synthetic harness:

1. **UI entry** — qt/pages/MailDetailPage.qml `archiveButton`
   (`objectName:"archiveMail"`, line 313) `onClicked: mailActions.primaryAction()`;
   qt/src/qml/CardActions.qml `primaryAction()` (line 30-36) calls
   `store.enqueueOp(cardKey, "archive", {})` for a mail card and reports the operation
   token upward; MailDetailPage then closes the reader (line 350-355).
2. **Optimistic model + durable outbox** — CardStore::enqueueOp (CardStore.cpp:589)
   builds `{op_id, card_id, type:"archive", args:{}}`, removes the card optimistically
   (line 611-616), appends to `m_outbox`, and `persistCards(..., {payload})` writes the
   sqlite row `INSERT INTO outbox(op_id,payload)` (line 264). `pendingOpsChanged` →
   `pendingOps` = outbox size (line 366). Cold start reloads the outbox (line 115-116,
   128-129), so the pending op survives process exit — proven across three processes.
3. **Visible waiting state** — qt/qml/InboxView.qml: `connectionStatusRow`
   (`objectName:"connectionStatusRow"`, visible only when `!store.online`, line 752) and
   the `connectionStatus` Label (line 756) whose text is `window.offlineSummary()`
   (line 590-595): "Can't reach the server" + " · 1 change waiting" from `store.pendingOps`.
4. **Retry wiring** — CardStore::setOnline(true) (line 367-377) → `flush()` →
   `flushNext()` (line 869-889) emits `requestPost(opId, "/v1/ops", payload)`; qt/src/main.cpp
   requestPost handler (line 384-399) routes any op whose `requestPath=="/v1/ops"` to
   `api.post("/v1/ops", payload)` and maps `Api::requestFinished`/`requestFailed` back to
   `store.reportPostResult(opId, status, response)` (line 406-429).
5. **Real consumer** — the stamped server's `POST /v1/ops` (server/cmd/litterbox/main.go:198,
   dev-only) → ops.API.ServeHTTP (server/internal/ops/ops.go:315+): bearer-authenticated
   identity, idempotency by `op_id` + payload_hash, then the registered **archive** handler
   (ops.go:67-91): `UPDATE cards SET state='archived'`, optional Gmail connector
   (`CallIfRegistered("gmail.archive")` — no connector registered in this disposable
   server, returns nil by design, ops.go CallIfRegistered), `ingest.OnDone` → durable
   `changes` row; response `{"ok":true}`. CardStore::reportPostResult treats 2xx+`ok:true`
   as acknowledged (line 893) → `DELETE FROM outbox WHERE op_id=?` (line 973) →
   `pendingOpsChanged` (line 1007).

## 4. Phase results (eval outputs verbatim from logs)

**A. Warm live sync** — 03:22:30–03:22:47, **exit 0** (logs/warm.log):

- `{"phase":"live-sync","online":true,"cards":2,"pending":0}` — both fixture cards
  pulled from the live server (GET /v1/cards readback: logs/live-cards.json, both open).
- `{"ids":["b1000000-…-0002","b1000000-…-0001"],"target":1,"neighbor":0}` — target = …0001,
  neighbor = …0002.
- Body prefetch cached both: `{"targetCached":["html","messages","source_url"],
  "neighborCached":[…]}`; sqlite readback later confirms mail_bodies rows (94/98 B html).
- Final: `{"phase":"warm-final","online":true,"pending":0,"cards":2}` — clean outbox.
  Screenshot captures/warm/01-warm-inbox.png.

**B. Server offline** — `kill 3355763` (my own service's child; cluster kept alive by the
control loop): `curl http://127.0.0.1:18125/v1/version` → **exit 7** (logs/offline-curl.txt),
`ss -ltn 'sport = :18125'` → **no listener** (logs/offline-ss.txt). (A different lane's
server on 18127/native-note DB was observed and left strictly alone.)

**C. New process, same profile, real UI Archive while offline** — 03:23, logs/offline.log:

- `{"phase":"offline-start","online":false,"cards":2,"pending":0,"target":1,"neighbor":0}` —
  full inbox restored from sqlite with server dead.
- Real pointer `openCard` on …0001 → `{"phase":"offline-reader","depth":1,"readerOpen":true}`
  (screenshot captures/offline/02-offline-reader.png), then real pointer click on
  **archiveMail** (step 9 of scripts/offline.json passed; the run later exited 4 on an
  eval-scope error, see §5 — the click itself succeeded, proven by the durable state below).
- **SQLite outbox after the process exited** (logs/sqlite-after-offline.txt, third process's
  view of the same profile DB):
  - `outbox_count = 1`
  - `outbox_payload = {"args":{},"card_id":"b1000000-0000-4000-8000-000000000001","op_id":"c60281b4-ade4-43d4-900b-53394d8d5867","type":"archive"}`
  - `cards_local = b1000000-…-0002` only — target removed optimistically, neighbor kept.
- **Fresh process confirming the waiting state** (logs/offline2.log, exit 0):
  `{"phase":"offline-status-row","online":false,"pending":1,"targetInList":false,
  "neighborInList":true,"labelFound":false,…,"offlineSummary":"Can't reach the server · 1 change waiting"}`
  — the string is exactly `window.offlineSummary()`, the function bound to the
  `connectionStatus` Label's text (InboxView.qml:757), and the **visible rendered row**
  is in screenshots captures/offline2/06-offline-waiting-status.png:
  "Can't reach the server · 1 change waiting" directly under the search bar, with the
  neighbor card still listed and the target gone.
  Cache readback while offline: both cards still cached
  (`{"phase":"offline-cache","targetCachedKeys":["html","messages","source_url"],…}`).

**D. Server back on the same disposable DB** — `touch control/restart` →
QRY-SERVER-RESTARTING, new pid 3405809, `/v1/version` healthy again
(logs/version-restart.json). The seeded DB — now holding the fixture — is the one the
retry will mutate.

**E. Real retry, actual ack** — 03:29, logs/retry.log, **exit 0**, fresh process:

- Cold start reloads the durable op: `{"phase":"retry-start","online":false,"pending":1,"cards":1}`
  — pending survived the kill/restart and two process deaths.
- `store.online = true` → flush → real HTTP `POST /v1/ops`:
  `{"phase":"retry-acked","online":true,"pending":0,"cards":1,"targetInList":false,
  "neighborInList":true}` — pendingOps cleared, neighbor intact, no failure warning in the
  client log (a failed POST would print `Operation request failed:` from main.cpp:427 —
  grep found none). Screenshot captures/retry/04-retry-acked.png: inbox clean, no
  waiting row.

**F. Server readback — the ack was real and target-scoped** (logs/server-readback.txt,
logs/server-ops-row.txt, logs/live-cards-after-retry.json):

- `cards`: …0001 → **archived**; …0002 → **open** (neighbor unchanged).
- `ops` table: exactly one row, `op_id = c60281b4-ade4-43d4-900b-53394d8d5867` — **byte-for-byte
  the op_id the client had durably queued while offline** (§C) — device_id present
  (bearer device auth attached), payload_hash 32 bytes, payload
  `{"args": {}, "type": "archive", "op_id": "c60281b4-…", "card_id": "b1000000-…-0001"}`.
- `changes` feed: card …0001 open → **archived** transition recorded (durable change for
  other devices); …0002 never transitioned.
- Client sqlite after retry: `outbox_after_retry = 0` — row deleted only after the ack.
- Live `GET /v1/cards` after retry: 1 open card, the neighbor; target absent.

**G. Negative control (never-synced profile)** — separate process, profile-negative +
home-negative, exit 0 (logs/negative.log):
`{"phase":"never-synced-negative","online":false,"cards":0,"pending":0}`; its
cards.sqlite has **0** outbox rows; and `SELECT count(*) FROM ops` on the disposable DB =
**1** — the only op the server ever saw was this gate's real client retry. The queued op
therefore originates from the offline UI click, not from the binary, the fixture, or a
carried profile. Screenshot captures/negative/05-never-synced-negative.png.

## 5. Honest boundaries (no scope inflation)

- Proven: production UI Archive → durable outbox → visible "1 change waiting" → server
  restart → automatic retry POST /v1/ops → real `{"ok":true}` ack → outbox cleared →
  server card archived + neighbor unchanged + durable change row, with an op_id identity
  match and a never-synced negative control. Android native, release/installer surfaces,
  and Gmail-provider label mutation are not claimed here.
- offline.json run exited 4 at its **last** eval step: I referenced `connectionStatus` as
  an eval identifier, but it is a nested QML id, not an eval-scope binding
  (`ReferenceError: connectionStatus is not defined`). Harness-side only; every prior step
  including the real archiveMail click had passed, and the durable sqlite state (§C) was
  read back after the process died. The status text was then captured correctly in the
  offline2 run via `window.offlineSummary()` — the exact binding the Label uses — plus the
  rendered screenshot. In that offline2 walk, `labelFound:false` because the status Label
  is not reachable from `window.contentItem` by a childItems walk in this build; the
  visible screenshot + binding-string evidence stand instead, and I did not claim a
  programmatic label read.
- The archive op reached a server without a Gmail connector: by design
  (`CallIfRegistered` no-ops when unregistered, ops.go); the durable server-side effects
  proven here are the card state, the ops receipt, and the changes row.
- Server runtime diff since the stamp is test-only (§1); `/v1/version` read back at both
  start and restart.

## 6. Retro (required — predecessor ran 40 min with no output)

- **Worked:** reading the completed offline-live-history recipe (server.sh + fixture.sql +
  receipt) first and copying it into my own port/DB/tenant in one step meant the very
  first service launch was already correct (migrations 001–021, fixture, ready). Designing
  server.sh as a control-loop supervisor (kill/restart only the server binary, cluster
  kept alive) turned the whole offline→retry scenario into two `touch` files instead of a
  fragile re-exec. Reading sqlite outbox *after process exit* rather than trusting an
  in-process eval made the durability claim a disk fact.
- **Fixed in flight:** `createdb` under be_BY.UTF-8 prints an initdb text-search warning —
  harmless, ignored; psql column guesses on `ops` (`card_id`, `type`) failed twice — the
  table stores the full payload blob only; I re-read `information_schema.columns` and
  re-queried with real columns instead of guessing a third time.
- **Avoided what the predecessor did:** 40 minutes of source crawling produced nothing; my
  first tool calls were the working recipe files, and the only source reads were the five
  call sites the receipt needed to name (§3), each verified to be blob-identical between
  the client build and HEAD rather than re-derived from scratch.

## 7. Artifact index (OUT/queued-retry)

scripts/: server.sh (control-loop supervisor), fixture.sql, warm.json, offline.json,
offline2.json, retry.json, negative.json (sha256s in logs/identities.txt);
logs/: dsn.txt, server.log, server.pid, version.json, version-restart.json, live-cards.json,
live-cards-after-retry.json, server-killed.txt, offline-curl.txt, offline-ss.txt,
sqlite-after-offline.txt, server-readback.txt, server-ops-readback.txt, server-ops-row.txt,
warm-start/end.txt, offline-start/end.txt, retry-start/end.txt, warm.log, offline.log,
offline2.log, retry.log, negative.log, identities.txt;
captures/warm/01-warm-inbox.png, captures/offline/02-offline-reader.png,
captures/offline2/06-offline-waiting-status.png (visible "1 change waiting" row),
captures/retry/04-retry-acked.png, captures/negative/05-never-synced-negative.png;
profile/ (cards.sqlite: outbox 1→0, mail_bodies 2 rows), profile-negative/ (0 rows).
Prior nav/search journey PASS receipts in ROOT/receipts are retained untouched.
