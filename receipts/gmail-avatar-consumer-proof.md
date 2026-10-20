# Receipt: Gmail sender avatars (R67 consumer proof) — photo paints, offline reopen keeps it

Date: 2026-10-20. Worker: ProveGooglePhotoConsumer. Status: **consumer proof done on disposable fixtures; real-Google grant proof pending (owner consent + console check).**

Isolation: private Xvfb :99 + `dbus-run-session`, private `HOME=/tmp/lb-avatar-run/home`, `XDG_*` under `/tmp/lb-avatar-run`, disposable Postgres cluster `lbavatar:5446` + disposable DB `lb_avatar_proof_20261020` + disposable tenant `11111111-…` + disposable token `disposable-avatar-token-20261020`. No owner screen/session, no production mail writes, no permission grants, no account actions. No shared deploy/upgrader run. No git metadata writes/commit/push.

## 1. Changed paths (this worker only; foreign hunks preserved)

- `qt/src/CardStore.h` — restored two members dropped after the frozen HEAD: `bool m_online = false;` (baseline `release-kit-0.1-40/src-immutable` line 171) and the `openCachedFile` Q_INVOKABLE declaration + comment (baseline lines 97–102). No other header changes.
- `qt/qml/InboxView.qml` — `SenderAvatar`: replaced `Kirigami.Avatar` (no such type in installed Kirigami 5.116 / kirigami-addons 1.12; the app failed to load: `Kirigami.Avatar is not a type`) with a plain `Image { anchors.fill: parent; source: photoSource; fillMode: PreserveAspectCrop }` in the same 32×32 slot. Initials `Rectangle` fallback untouched, same geometry, no new imports, no row-height change.
- Untouched: archiveBundle / captureArchiveContinuation / onCardHandled hunks in InboxView (peer ConnectLocalArchiveSuccessors owns them; acked, no overlap — no InboxView edits outside SenderAvatar). Server files untouched (previous coder's R67 transport stands). No docs edits (DEPLOYMENT.md R67 already current).

## 2. First checkpoint: actual Qt compile (concrete failures, then green)

- `cmake -S ROOT/qt -B /tmp/lb-avatar-qt-build -G Ninja -DQt6_DIR=/usr/lib/x86_64-linux-gnu/cmake/Qt6` → exit 0 (configure ok; the `build-qt673` kit dir fails configure: Qt 6.7.3 private install lacks WebEngineQuick — pre-existing, not mine; system Qt used per successful release-kit cache).
- `cmake --build /tmp/lb-avatar-qt-build` → FAIL 1: `CardStore.h:37 m_online was not declared`. Fixed (see §1).
- rebuild → FAIL 1: `CardStore.cpp:1154 openCachedFile` has no declaration. Fixed (see §1).
- rebuild → exit 0 (`litterbox-qt` 22M). QML run → `Kirigami.Avatar is not a type`, engine failed to load. Fixed (see §1).
- rebuild → exit 0. Full build log: `/tmp/lb-avatar-qt-build4.log`.

## 3. Production server route on disposable fixtures (all exit 0)

Server: `/tmp/lb-avatar-server` (built `go build -o` from this tree, exit 0) on `127.0.0.1:18081` with `DATABASE_URL` → disposable DB, `LITTERBOX_DEV_TENANT_ID` → disposable tenant. Seed: granted account (`contacts.readonly` + `contacts.other.readonly`), Gmail-only account, 3 cards, distinct 64×64 PNG (left red 220,30,30 / right green 30,160,60, 162 bytes, sha256 `662f2dc6…ba88757`).

- `GET /v1/cards` → photo card `sender_has_photo:true`; lonely + no-grant cards false/absent. (log: `qt-online2` eval `cards:[aaaa…,bbbb…,cccc…] n:3`)
- `GET /v1/cards/aaaa…/sender-photo` → 200 `image/png`, 162 bytes, **sha256 exact match** to seed PNG (`cmp` → `BYTES_EXACT`). Headers: `Content-Length: 162`, `Cache-Control: private, max-age=86400`.
- `GET /v1/cards/bbbb…/sender-photo` → 404 `{"status":"unavailable"}` (genuinely-absent negative control).
- `GET /v1/cards/cccc…/sender-photo` → 403 `{"status":"authorization_required","required_scopes":[…]}` (Gmail-only grant; zero photo bytes on any surface).
- `GET /v1/gmail/sender-photos/status` → granted account `photo_ready:true, authorization_required:false`; plain account `photo_ready:false, authorization_required:true + required_scopes`. No Google touched.
- `GET /v1/snapshot` → carries `sender_has_photo` (count 1).
- `go test -count=1 ./internal/people/ -v` → exit 0, 3/3 PASS (`TestBuildIndexExactMatchRejectsDefault`, `TestFetchPhotoDecodesRealImage`, `TestNormalizeEmail`). Full suite NOT run (CF58: lead runs once after frozen merged candidate + archive writer settled).

## 4. Qt consumer: painted decoded photo + offline reopen (disposable profile/cache)

Run: `--capture-script /tmp/lb-avatar-run/profile /tmp/lb-avatar-run/scripts/online.json` (sets `store.online=true`, waits, shots) then `offline.json` (stays offline, shots). Binary: `/tmp/lb-avatar-qt-build/litterbox-qt` (this candidate). Both runs exit 0.

- Online eval: `cachedSenderPhoto('aaaa…')` → `file://…/sender-photos/aaaa…/photo.png`; `cachedSenderPhoto('bbbb…')` → `""` (initials path, same 32×32 `SenderAvatar` component → geometry retained).
- Device cache bytes exact: `sha256(cache photo.png) == sha256(seed PNG) == sha256(server GET bytes)` (`662f2dc6…`). `cards.sqlite: sender_photo_files` holds 1 row (photo card → file:// URL).
- Painted proof: `/tmp/lb-avatar-proof/captures/offline-01-reopen.png` (1440×1000) contains **512 exact-red pixels (220,30,30)** each with a green pixel 4–40px to the right (red-left/green-right photo pattern). Online shots show the cache URL but 0 red pixels — the `Image` painted after the online shot; the offline reopen (warm cache, no network) paints immediately and is the consumer success. Initials/1×1 alone were never claimed.
- Offline reopen: same profile dir + same cache dir, server reachable-but-unused (script never sets online; store boots `m_online=false`, loads `sender_photo_files` via `injectSenderPhotos`). Eval after reopen: cards 3/3, photo card still `file://…photo.png`, lonely still `""`; screenshot paints the photo (see above). Offline-capable confirmed.
- Captures/logs: `/tmp/lb-avatar-proof/captures/{online-02-photo.png,offline-01-reopen.png}`, `/tmp/lb-avatar-proof/logs/{server.log,photo.out,photo.hdr,status.json,404.out,403.out,qt-online2.log,qt-offline.log,go-people-test-fresh.log,xvfb-*.log}`, device profile `/tmp/lb-avatar-run/profile/cards.sqlite`, cache `/tmp/lb-avatar-run/cache/…/sender-photos/aaaa…/photo.png`.

## 5. People API / consent investigation (no grants performed)

- `GMAIL_OAUTH_CREDENTIALS` unset; no credentials file on this box. No Google calls made, no consent URL opened, no scopes granted.
- `POST /v1/gmail/photo-consent` returns `authorization_url` (Gmail.modify + both photo scopes) and performs no grant; `GET /v1/gmail/sender-photos/status` reports readiness without touching Google (verified live in §3).
- Qt today has **no authorization-required UI surface**: only C++ comments reference 403/`authorization_required` (CardStore.h, main.cpp, CardStore.cpp); no QML page wires `photo-consent` or the status endpoint. Usable operator path = status endpoint (`authorization_required` + `required_scopes`) + initials fallback in unchanged geometry + manual consent URL. No QML consent screen was added (out of packet scope; flagged, not built).
- People API enablement: **not verifiable from source**. The server has no config flag for it; the only local gate is the actually-granted scopes (`HasPhotoScopes`). Whether Google's People API is enabled on the project, and whether verification applies to the two contacts scopes, can only be checked in the Google Cloud console + a real user grant — deliberately not performed here. Saying "owner must enable it" rests on DEPLOYMENT.md R67 + the documented People surfaces the code calls, not on any observed Google error.

## 6. Remaining real-Google consent and coverage constraints

1. Owner console: enable People API on the existing project; add `contacts.readonly` + `contacts.other.readonly` to the OAuth consent screen (verification may apply).
2. User grant: open `authorization_url` from `POST /v1/gmail/photo-consent`; shared callback persists granted scopes. Measurable coverage after grant = senders exactly matching a People connection/otherContact with a non-default photo on a granting account; everyone else = initials in unchanged geometry (+ `authorization_required` surfacing on non-granting accounts).
3. Explicit non-coverage stands: senders absent from both contact datasets stay uncovered even when Gmail renders a photo; no Gravatar/third-party/undocumented endpoints.
4. Follow-ups (not this packet): Qt consent/status UI wiring; full suite once after archive writer settles (CF58); certifier rerun per xvfb-gui-proof (separate auditor, N=10 coordinate runs + 3-run commit-tree rerun — not done here).

## 7. Carry-forwards

- CF2: real decoded consumer proof above (exact-bytes 64×64 photo → painted 512-red-pixel offline capture); broken-baseline controls: 404 initials path, 403 zero-bytes + scopes, Gmail-only grant oracle.
- CF26: base identities per failed-acceptance §1 (verified at start via receipt; no git SHAs read/written by this worker); executable = `/tmp/lb-avatar-qt-build/litterbox-qt` + `/tmp/lb-avatar-server` from this tree.
- CF39: isolated GUI proof now done by this worker (Xvfb + dbus-run-session + private HOME, captures above); separate-certifier rerun still open.
- CF58: full suite NOT run; lead runs once after frozen merged candidate + archive writer settled. Only `go test -count=1 ./internal/people/` run here.
- CF65: durable captures/logs listed in §4 (under `/tmp/lb-avatar-proof/`); receipt findings here.

## 8. Retro (required)

What worked: reusing the kit's actual build cache paths (system Qt, not the broken 6.7.3 dir) gave a configure in 1s; anchoring both C++ fixes on freshly-read header lines kept the patch collision-free against the live archive sibling; running the Qt binary immediately after the frozen build exposed the real `Kirigami.Avatar` integration break that compile alone hid — proving consumer-at-runtime, not compile-alone. What to repeat: check installed QML module types before trusting a prior QML hunk (`Avatar` never existed in Kirigami 5.116); keep photo fixtures visually asymmetric (red/green halves made the painted-pixel proof trivial). What to change: seed the disposable DB against a fresh cluster from the start (shared-cluster migration failed on the pre-existing `litterbox_app` role and cost a cycle); capture the online shot after an explicit `cachedSenderPhoto`-nonempty poll instead of a fixed wait (online shot missed the paint, offline saved the proof).
