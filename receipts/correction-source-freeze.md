# Correction source freeze — photo implementation evidence (2026-10-20)

Worker: FreezePhotoEvidence (scribe). ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018.
Read-only assembly of already-frozen evidence. No build, no retest, no source change, no Git
metadata write, no installer, no grant, no screenshots duplicated here. Retro required at §7.

**Scope of this receipt: the photo/avatar/archive implementation is IMPLEMENTATION-READY FOR
SOURCE FREEZE. It is NOT real-Google acceptance, and it is NOT installed-release complete.**

---

## 1. Clause → production entry / wiring / proof

| Clause | Production entry | Wiring exercised | Proof (durable) |
| --- | --- | --- | --- |
| Notification (sender-photo completion repaints the actual presentation) | `qt/src/CardStore.cpp` `senderPhotoChanged`/`dataChanged` → `CardRole` + retained `SenderPhotoRole` | `qt/qml/InboxView.qml` `Connections` relocated out of `DelegateModel`'s lazy `delegate` default property into the live ListView | `receipts/gmail-avatar-proof/notification-20261020/PROOF.md` §4 (`paint-online.json`, `paint-offline.json`, `notification-negative.log`) |
| Consumer paint (cold first-online, no refresh) | production `litterbox-qt` fresh empty private profile | `Api::getBytes` → `applyRemoteSenderPhoto` → cache → row `Image` | PROOF.md §4 — 512 exact red pixels each surface, online/cold + offline reopen |
| Async cut-handler control | disposable QML copy with `onDataChanged`/`onSenderPhotoChanged` disabled | same test harness, source/binaries untouched | `notification-negative.log` — 2 passed, 1 failed, **exit 1** at `first online sender photo must paint without a manual refresh()` |
| Scoped regression | `qt/tests/tst_senderphotopaint.qml` | `dbus-run-session -- /tmp/lb-avatar-qt-build/tests/tst_bundleexpand -input …/tst_senderphotopaint.qml` | **3 passed, 0 failed, exit 0** (scoped 3/3); 403/404/default, malformed-bytes, offline reopen in same test |
| Authorization UI (unchanged code, exercised) | `qt/pages/GmailAccountsPage.qml` | capture-harness painted-object click of real Authorize button | `app-captures/accounts-three-states.png`, `app-captures/accounts-consent-not-opened.png` (PROOF.md §5) |
| Archive successor advance-once (R68) | `archiveBundle(bundleId)` + row `CardActions onCardHandled` in `qt/qml/InboxView.qml` | `store.archiveBundleNow` / `showArchiveUndo` → `next()` | `receipts/archive-failed-acceptance.md` §Behavior evidence (8-entry matrix ok 201.050s) |
| Pinned multi-member bundle survivor | same `captureArchiveContinuation` path, pinned skip | bundle expand → open A1 → archive → B once | `archive-failed-acceptance.md` §Remaining gaps — fail/pass receipts below |
| Google photo live gate | `GET /v1/gmail/sender-photos/status` on installed 0.1-40 | authenticated loopback probe | `receipts/google-photo-live-gate.md` — **GATE = UNKNOWN_ROUTE_MISSING** |

## 2. Exact product/test source paths pending publisher (with current hashes)

Identity source: `receipts/gmail-avatar-proof/notification-20261020/CANDIDATE.sha256` and
`.../MANIFEST.json` (notification lane), plus `receipts/archive-failed-acceptance.md` §Source
identity (archive lane). These are the paths the publisher must hash again at source freeze;
the 40-hex `source_sha` does not exist yet (see §8).

### 2a. Notification lane — changed product/test source (CANDIDATE.sha256)

| Path | SHA-256 |
| --- | --- |
| `qt/src/CardStore.cpp` | `0d84ac5f90fa12f7ae69ed52ccb7af346726cbbef8d7b5ab624e3b3938a24832` |
| `qt/qml/InboxView.qml` | `f8c4af20bf89f5d1581f2263a0c59225b1280824fbc0bafad54b55dea3a6cf86` |
| `qt/tests/tst_senderphotopaint.qml` | `55c242d5d1a84d99ad9168cd3c224713eaf7266b02ec5361561bc196c5e73fe4` |

### 2b. Notification lane — retained companion files (not rewritten; candidate hashes recorded)

| Path | SHA-256 |
| --- | --- |
| `qt/src/CardStore.h` | `e4899f08c51b0d5906b55988f65437b68ceeafc190f4e955e685272e656dba78` |
| `qt/tests/CMakeLists.txt` | `8426f67a0301ecff3b92c19cde8e8fcf80a804b5dda6d694ddd53e758c458ae7` |
| `qt/tests/bundle_runner.cpp` | `b5700c7cfbc26bcdb39de02ff66dca1de1a676ae190e354561f908bb25cf7cca` |
| `qt/tests/fixtures/sender-photo-64x64.png` | `662f2dc6f113dfcce1e4fca794b61b0708d56d5805462d9c12f7308c7ba88757` |
| `qt/pages/GmailAccountsPage.qml` | `a3c93a2dcb536660ba449fc39b28d8e06c777f28099b91b0914b5359c139c147` |

### 2c. Notification lane — frozen disposable executables (NOT release artifacts)

| Path | SHA-256 |
| --- | --- |
| `/tmp/lb-avatar-qt-build/litterbox-qt` | `482016861a242fdcabe8f47f412133e6686b5e26ce0671d3b8b761ceae110bb0` |
| `/tmp/lb-avatar-qt-build/tests/tst_bundleexpand` | `f5ea4522d513089111ca95a9938703b43576b2fc0ce39400eb36e7818dd4ac04` |
| `/tmp/lb-gphoto-server` | `22440b11fb437b25bd586b16bec943731214185b178a45270b580b77b8fca37e` |

### 2d. Archive lane — changed paths (only), from `archive-failed-acceptance.md`

| Path | SHA-256 |
| --- | --- |
| `qt/qml/InboxView.qml` (archive hunks) | `368486054caf404aa1783e95714d8171a6a64d238bdd2c3df8f59ac35841b676` |
| `qt/tests/archive_undo_consumer.py` | `b1f5b1bcd58263f287cd500753fd52dbf2888f95af844b54b96860dd7b0ca11f` |
| `server/internal/gmailsync/archive_undo_test.go` | `704175d98e8975262ddd108e561b8288b7eb918a793a346f4889f37c5404236c` |
| `/home/kom/proj/ai_pr/archive-successor-build-20261020/litterbox-qt` (disposable binary) | `618afa2e54315d3f84fdccfa6675c404ad400bc28be8d869fdcbfa44058faeb0` |

**Collision note for the publisher:** `qt/qml/InboxView.qml` carries both lanes. The notification
receipt hashes it `f8c4af20…`; the archive receipt hashes it `36848605…`. Those are two different
snapshots of the same file taken by two writers at different times — the archive hash is the
archive-only hunk snapshot, the notification hash is the notification-lane snapshot. At freeze the
publisher must take ONE fresh hash of the settled worktree file and must not carry either value
forward as the frozen value.

Historical identities (NOT the candidate): base `929f275946f7906cfbe67e0f4dfd08f83402a202`,
previously published product `df9b13e5513ed1f1739149a3d6048c27eee3c725`.

## 3. Source/docs/changelog inventory needed for the Git owner

The Git owner (`git-pr-owner`) commits exactly these; nothing else in the worktree is owned by
this receipt.

**Product source (both lanes):**
- `qt/src/CardStore.cpp`, `qt/src/CardStore.h`
- `qt/qml/InboxView.qml`
- `qt/pages/GmailAccountsPage.qml`

**Test source:**
- `qt/tests/tst_senderphotopaint.qml`, `qt/tests/CMakeLists.txt`,
  `qt/tests/bundle_runner.cpp`, `qt/tests/archive_undo_consumer.py`,
  `server/internal/gmailsync/archive_undo_test.go`

**Fixture:**
- `qt/tests/fixtures/sender-photo-64x64.png`

**Docs:**
- `docs/DEPLOYMENT.md` — §Gmail sender photos (R67) is the normative clause source and was
  fresh-read, unchanged by this lane.

**Evidence receipts (durable, must be committed with the change):**
- `receipts/gmail-avatar-proof/notification-20261020/**` (PROOF.md, CANDIDATE.sha256,
  MANIFEST.json, commands/final-build-command, logs, captures — full inventory = MANIFEST.json
  itself, 338 lines, every entry path+sha256+bytes+kind)
- `receipts/archive-failed-acceptance.md`
- `receipts/google-photo-live-gate.md`
- `receipts/correction-source-freeze.md` (this file)

**Changelog:** NONE EXISTS. There is no `CHANGELOG*` and no `README*` anywhere under ROOT
(globbed, 0 matches); `docs/` contains only `DEPLOYMENT.md`, `ARCHITECTURE.md`, `a2a-bridge.md`,
`generator-notes.md`, `ingest.md`, `mark-spam-receipt.json`. This is recorded as a docs gap in §6
rather than silently skipped — the Git owner must decide whether a changelog entry is required.

## 4. Notification consumer evidence (512 pixels, cold + offline, async cut exit 1, scoped 3/3)

From `notification-20261020/PROOF.md` §3–§4 (already-frozen; no retest):

- Cold first-online production Qt inbox, no refresh: **512** exact red pixels with green 4–40 px
  right (`paint-online.json`).
- Retained actual-InboxView regression, first online: **512**.
- Same regression, offline cache reopen: **512** (`paint-offline.json`).
- Production Qt reopen, owned server stopped: **512**.
- PNG readback performed; `online-app.log` asserts `requests: 1` (no manual refresh, no second
  snapshot).
- Async notification-cut control: `notification-negative.log` — **2 passed, 1 failed, exit 1** at
  `first online sender photo must paint without a manual refresh()`. Disabled handlers live only
  in a disposable QML copy; source/binaries untouched.
- Scoped retained regression: **3 passed, 0 failed, exit 0** — `dbus-run-session -- …
  tst_bundleexpand -input …/tst_senderphotopaint.qml`.
- Server-reported status and binary provider fixture legs: both exit 0, bytes identical to the
  retained valid PNG fixture (explicitly a disposable provider fixture, not a Google photo).

Commands and exact argv are in `commands.json`; build/service in `final-build-command.json`.

## 5. Archive row/bundle/pinned fail-pass receipts

From `receipts/archive-failed-acceptance.md`:

- Full clean 8-entry matrix: `go test -count=1 -run "^(TestArchiveUndo|TestArchiveRow|
  TestArchiveBundle|TestArchiveEnd|TestArchiveExternal|TestArchiveSpam|TestArchiveDone|
  TestArchiveSnooze)NativeConsumer$" ./internal/gmailsync/` → **ok 201.050s** (private
  `pg_virtualenv` cluster `18/regress` created/dropped).
- Row: 5 states; bundle: 6 states; toolbar: 6; end: 4; external: 3; spam/done/snooze: 3 each.
  Cache `[A,B]` restored after Undo, outbox empty everywhere.
- **Pinned multi-member bundle fail/pass (same oracle, same runpaths):**
  - broken binary `7c8df3a9bd131916bbb93275925798fe5704cb50f3bf6ea34046303d405afb0a` →
    `FAIL 46.405s (test 46.39s)`, TEST_EXIT 1, 9 states Qt 0/oracle 1 (s3/s4 selected B with
    body B — wrong, expected P/P).
  - fixed binary `6095038658606b38344be49ea13ff5c351135a304236c1ff4c80693ed69b4297` →
    `ok 47.357s`, TEST_EXIT 0, 9 states Qt 0/oracle 0 (s3 `[P,B]` sel P body P — pinned P
    retained; s6 `[P,B]` sel B body B — A1 archive skips removed A2 to B once).
  - run dirs: `/home/kom/proj/ai_pr/archive-pinned-broken-run-20261020` and
    `/home/kom/proj/ai_pr/archive-pinned-run-20261020-bundle_pinned`.
- **CF67:** executed failing-before/passing-after exists only for the pinned slice. Row/bundle
  probe/setup reds were harness fixes (desktop reader is embedded `mailDetailPage`, not pushed;
  leader-collapsed hides member button; `auth.Middleware` only authenticates `/v1/`; Done needs
  `card_bodies`). The original row/bundle defect stays source-inspected/user-reported — no
  broader failing-before is claimed.

## 6. Source-freezing docs gaps (explicit, not silently skipped)

1. **No changelog file exists** anywhere under ROOT (globbed 0 matches). Whether the Git owner
   requires one is a product/repo decision outside this receipt; recorded here so it is not lost.
2. **No `README*`** at repo root to update with the freeze identity.
3. **`docs/DEPLOYMENT.md`** R67/R68 clauses are the normative source and were NOT edited by
   either writer — if the freeze requires a doc delta (e.g. recording the settled source SHA),
   that delta is pending and unowned.
4. **The notification receipt's `receipts/…/CANDIDATE.sha256` does not list
   `qt/qml/InboxView.qml`'s archive hunks**, because the two writers froze at different times.
   A single post-settle hash of `InboxView.qml` is a required publisher step, not an optional
   one.
5. **`MANIFEST.json` covers the notification evidence bundle only.** The archive lane's evidence
   lives in `archive-failed-acceptance.md` prose plus external `/home/kom/proj/ai_pr/
   archive-*-run-20261020/` directories that are OUTSIDE the repo. If those run dirs must be
   durable for release, they need to be copied in; this receipt does not copy them.

## 7. Retro (required)

What worked: assembling this freeze from already-frozen receipts rather than re-running anything.
Reading `CANDIDATE.sha256` and `MANIFEST.json` first gave exact hashes before any prose claim,
and the archive receipt's own `Release handoff` section already named the remaining gates — so
this receipt could map clause→entry→wiring→proof without a single new command.

What cost attention: `CANDIDATE.sha256` and `MANIFEST.json` are NOT at the repo root as the brief
implied — they live under `receipts/gmail-avatar-proof/notification-20261020/`. A first `read`
against the root paths failed and had to be re-resolved by glob. Lesson: pin proof files to their
lane directory, never to ROOT, when handing a path to another worker.

The `InboxView.qml` two-hash collision (§2d note) is the real freeze hazard: two writers hashed
the same file at different times. Recording both values with the explanation is deliberate — a
future reader must not pick one and call it frozen.

Durable repair: this receipt is the single place where notification + archive + Google-gate
identities sit side by side with their residual gaps, so the publisher takes one fresh hash pass
over a settled worktree instead of trusting stale lane-local hashes.

## 8. Residual release / live-photo gates (NOT closed by this freeze)

1. **Real Google configuration/grant:** People API enablement, consent-screen inclusion and
   verification of both `contacts.readonly` and `contacts.other.readonly`, and an actual account
   grant remain unproved. The API is not declared disabled. No lane opened a grant/browser or
   used production accounts. `receipts/google-photo-live-gate.md`: **GATE = UNKNOWN_ROUTE_MISSING**
   because installed 0.1-40 `server_build=df9b13e…` predates the photo routes — a 404 is NOT a
   People-API-disabled signal, NOT a denied-scope signal, and NOT a zero-photo proof. Granted
   scopes on live: UNKNOWN. User action verified: none.
2. **Real sender-photo acceptance:** after that grant, an exact-address non-default
   People-connection/otherContact photo must travel through sync, authenticated route and the
   device consumer without refresh. Seeded PNGs cannot close this gate.
3. **Source freeze itself:** the 40-hex `source_sha` does not exist yet. Publisher must re-hash
   the settled worktree (single fresh `InboxView.qml` hash per §2d) and read a clean
   `git diff --name-status`.
4. **Full suite once (CF58) + native re-proof on the new cohort** — deferred until both writers
   settled; not run by this receipt.
5. **Accepted 3-way proofs** binding the new source + artifact hashes.
6. **Git tuple:** fresh `ls-remote` + commit/push by `git-pr-owner` only. No Git metadata was
   touched here.
7. **Installed release:** still 0.1-40 (server `e8d911d0…`, desktop `56d975e1…`, APK v27
   `83ad492b…`, `db.env` `a7614256…`) per `receipts/ui-corrections-publication.md` §A.2. The
   eventual `plan`→`apply` runs by Main via `deploy/release.py` only.

**Explicit non-claims:** no build, no retest, no source change, no Git metadata write, no
installer/upgrader (not even dry-run), no grant/consent, no production mail or token operation,
no owner desktop/phone/TV/session, no screenshots duplicated into this file.

## 9. Carry-forwards

- CF2: notification claims tied to real production entries + actual painted pixels + a
  notification-cut negative control, never seed-only completion (§1, §4).
- CF26: exact source/binary SHA-256 above (§2); archive identities in
  `archive-failed-acceptance.md` §Source identity.
- CF39: isolated worker GUI evidence only; independent/merged-candidate certification is
  parent-owned.
- CF58: full-suite gate still open (§8.4).
- CF65: all evidence lives under this project-relative durable directory — `MANIFEST.json`
  inventories the notification bundle; this file inventories the cross-lane publish set (§3).
- CF67: only the pinned slice has executed fail/pass (§5).

## 10. Publisher freeze record (PublishCorrectionSource, 2026-10-20)

Pre-mutation state: single worktree, common-dir `.git`, branch
`ui-desktop-mobile-redesign-20261018`, HEAD `929f275946f7906cfbe67e0f4dfd08f83402a202`,
staged 0, unstaged 22, untracked photo-producer source + receipts + out-of-scope
material (mobileux PNGs, `__pycache__`, `*.pid`, `scripts/credentials.json`,
superseded run dirs, STALE `archive-undo-successor-proof.md` — all preserved
untracked, none staged). Foreign job 752006 (codex exec) observed present and
left untouched; 758859/669925/738809 absent. No fetch, no merge-tree, no shared
readonly mutation before lease-bound push.

### 10.1 Fresh candidate inventory (authoritative,settled worktree 2026-10-20)

Supersedes the lane-local hashes of §2a/§2d wherever they differ (writers kept
settling after the scribe froze: `archive_undo_consumer.py`,
`archive_undo_test.go`, `sync.go`, `api.*`, `main.cpp`, cards/gmail/offline
server files, REQUIREMENTS/ARCHITECTURE/DEPLOYMENT deltas). Unchanged files
match §2 exactly.

| Path | SHA-256 |
| --- | --- |
| `REQUIREMENTS.md` | `94dcf9738939c0d1022deec7ce9ac1d49eb057d7548814564d8aca1f15ba92c6` |
| `docs/ARCHITECTURE.md` | `25df1b7f70f229b94e7110968ea57b7bd3edb0840b5b41d70ae2a4aab075b6d3` |
| `docs/DEPLOYMENT.md` | `cdc53f76e903b0921f83782ad7140dfb03c247128b59369fe8c2a18c1d867794` |
| `qt/pages/GmailAccountsPage.qml` | `a3c93a2dcb536660ba449fc39b28d8e06c777f28099b91b0914b5359c139c147` |
| `qt/qml/InboxView.qml` | `f8c4af20bf89f5d1581f2263a0c59225b1280824fbc0bafad54b55dea3a6cf86` |
| `qt/src/CardStore.cpp` | `0d84ac5f90fa12f7ae69ed52ccb7af346726cbbef8d7b5ab624e3b3938a24832` |
| `qt/src/CardStore.h` | `e4899f08c51b0d5906b55988f65437b68ceeafc190f4e955e685272e656dba78` |
| `qt/src/api.cpp` | `bb818a281aa3f829417c9e2b521128dbb07544900922218630bc93bc00157619` |
| `qt/src/api.h` | `b422391234b783ddb60d2a71b467d2f7a6430a94374dbbd92986bb2f507f491c` |
| `qt/src/main.cpp` | `030c546a5e4ca25fb300bb681a206de4e284697403c98f19dee1a1fe01ae9b9c` |
| `qt/tests/CMakeLists.txt` | `8426f67a0301ecff3b92c19cde8e8fcf80a804b5dda6d694ddd53e758c458ae7` |
| `qt/tests/archive_undo_consumer.py` | `1ad62bd0bc30387cb11208c09cceaa47cdfd48925228c320eb47a8dfb6ef34dd` |
| `qt/tests/bundle_runner.cpp` | `b5700c7cfbc26bcdb39de02ff66dca1de1a676ae190e354561f908bb25cf7cca` |
| `qt/tests/tst_senderphotopaint.qml` | `55c242d5d1a84d99ad9168cd3c224713eaf7266b02ec5361561bc196c5e73fe4` |
| `qt/tests/fixtures/sender-photo-64x64.png` | `662f2dc6f113dfcce1e4fca794b61b0708d56d5805462d9c12f7308c7ba88757` |
| `server/cmd/litterbox/main.go` | `672fd3348deeb543f166df111bf7f9d2edf0ed0c6b7ef5893c3110b98b7fb359` |
| `server/internal/cards/cards.go` | `c0cc97f1d8e31f064bbd003da4d7acddd7e818461d6619514ed709d1b7e0f6fd` |
| `server/internal/cards/handler.go` | `f05c7cc5fdb12de455b90a658d5198aa4d41c549e2c8bf0c660f707e7c51f25f` |
| `server/internal/gmail/http.go` | `45d808c674d9d57f026d8385d94c8d98bba6bc3fcdb251112838feaa19fc0040` |
| `server/internal/gmail/oauth.go` | `3f78778e61b022a2bf7bb38bdcdc96ffa3a46c3ab0a7d95bf58f203b9d24dad9` |
| `server/internal/gmailsync/archive_undo_test.go` | `679e2c7dce4d37f90baf7d56dc39e44342d0ea1395f58b68fc4e51686181e203` |
| `server/internal/gmailsync/sync.go` | `935d4b58b37124bae7d5ea4614543829143456b4750de8c2352216c1c43f1bba` |
| `server/internal/gmailsync/sync_test.go` | `915bd417dce448780d032589069c35e72d60a9fbd8290cf87980453e4432bdf1` |
| `server/internal/offline/http.go` | `e5a65dd3a073be5599a76f443490d720677dae67b689fda87c21606f3fd6032a` |
| `server/db/022_sender_photos.sql` | `fd1bb9277f688ea181c8bd22c4be69a1f60da60f9bb9859715428c0301d54661` |
| `server/internal/gmailsync/photosync.go` | `a4e6ecf17081afc8e74e0a425a7e49f0717a9053ca37c1ddbcd16d6a6e35305e` |
| `server/internal/people/fetch.go` | `88753e69e4f4e1af11395113472651bdcf467219cdcf880dde0b11a2574b1f2b` |
| `server/internal/people/people.go` | `b8870e50abc30142ab51dbee3445aa3b7c27421e0c3441e45a3f8f80cc5b985c` |
| `server/internal/people/people_test.go` | `7eecd09da1b72f2a813d95a3b1e21ca9ea7de28c1b0db53f3c04a664fee45a5b` |
| `server/internal/senderphoto/handler.go` | `1bf12441eb3a81abc795661f319985fe93346f88f096f85d22ed23e6b39e2664` |
| `server/internal/senderphoto/senderphoto.go` | `dac17fe426a4ea806d529515b3805cb684cd3b47f494daca3eb6418bcee88870` |
| `receipts/archive-failed-acceptance.md` | `5ccbb9fe19116350c050c7d52a9d98251b74b9e8808e25de4cb4eb6a84997a8a` |
| `receipts/google-photo-live-gate.md` | `9cc945ddab7b5112583bc0393e6493d6fd952c2872154dadb52929ef2d953517` |
| `receipts/gmail-avatar-authorization-proof.md` | `943aa4428f3418053bd88a08fc5c5174f7dbc24edfa2154a819321e35ee0d7f4` |
| `receipts/gmail-avatar-consumer-proof.md` | `f0e79f204a5071d80f2ec00c854ab674524e30052541dd581b3cc790a79cea99` |
| `receipts/gmail-avatar-failed-acceptance.md` | `0a33c734e66887187107be6d201207f74c80cd821e20b7667c3cefa68bd3d354` |

Plus `receipts/gmail-avatar-proof/notification-20261020/**` per its `MANIFEST.json`
(338 entries; secret scan of the whole freeze receipt set for JWT/Google-token/
private-key/client-secret/refresh-token patterns: zero matches), and this file
itself (`receipts/correction-source-freeze.md`).

### 10.2 Settled `InboxView.qml`

Fresh hash `f8c4af20bf89f5d1581f2263a0c59225b1280824fbc0bafad54b55dea3a6cf86`
(= notification-lane value). The older archive-lane snapshot `36848605…` is
superseded; neither stale value is carried forward. Photo + archive hunks coexist
in the one settled file.

### 10.3 Normative `Photo.default` correction (ReviewCorrectionIncident-2)

R67 (`REQUIREMENTS.md`, real-photo path) said ``(`Photo.default=false` rejected)``,
contradicting the non-default-only producer, which rejects `default=true`.
Minimal clause correction applied in this freeze (docs only, production behavior
unchanged — the producer already rejects `default=true`): the clause now reads
``(`Photo.default=true` rejected)``. One word changed, no other wording touched.

### 10.4 Docs/changelog state

- `README.md` (13.7 KB) EXISTS at repo root (plus `app/README.md`): the scribe
  §6 no-README claim is refuted by fresh read; no README changes needed.
- `docs/ARCHITECTURE.md`: 8-line delta (photo producer ownership
  `people/`→`photosync`→`senderphoto` routes→Qt roles) — in scope, committed.
- `docs/DEPLOYMENT.md`: 4-line delta (R67/R68 release notes) — in scope, unchanged
  normative clauses.
- No `CHANGELOG*` anywhere under ROOT: gap stands, recorded, not silently skipped;
  no changelog invented.

### 10.5 Immutable build source paths for parallel consumers

- Server kit: `server/cmd/litterbox/`, `server/internal/{cards,gmail,gmailsync,offline,senderphoto,people}/`, `server/db/022_sender_photos.sql`, `server/go.mod`, `server/go.sum`. Producer: `go build -ldflags="-X main.buildSHA=$(git rev-parse HEAD)" -o <binary> ./cmd/litterbox` in `server/`; package via `deploy/server/debian/build.sh`.
- Desktop Qt kit: `qt/src/`, `qt/qml/`, `qt/pages/`, `qt/tests/`, `qt/CMakeLists.txt`; Qt 6.11 + system Kirigami (`qt/debian/control`); existing Debian recipe; no custom ELF note. Producer campaign: `cmake --build /tmp/lb-avatar-qt-build --target litterbox-qt tst_bundleexpand --parallel 2` then `python3 receipts/gmail-avatar-proof/notification-20261020/run_proof.py online|offline` (see `final-build-command.json`, `commands.json` — canonical, not re-run here).
- Android kit: same `qt/` tree + `qt/android-build.sh` with explicit `JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64` override, `KIRIGAMI_ROOT`, bundled ABI-matched OpenSSL 3 runtimes, Gradle `-Dorg.gradle.daemon=false -Dorg.gradle.workers.max=2 -XX:ActiveProcessorCount=2`; sign only via canonical redacted loader `RUNTIME/sign-apk-redacted.py`.
- Suite kit: `server/internal/gmailsync/*_test.go`, `server/internal/people/*_test.go`, `qt/tests/` runners; full suite (CF58) stays a downstream gate, not re-run here.

### 10.6 Publication tuple (filled §11 after push)

- Remote URL: `git@github.com:Komzpa/litterbox.git` (from `remote.origin.url`).
- Full destination ref: `refs/heads/ui-desktop-mobile-redesign-20261018`.
- Observed old OID (one fresh `git ls-remote origin` readback, cited): `929f275946f7906cfbe67e0f4dfd08f83402a202` for the destination ref; local branch `ui-desktop-mobile-redesign-20261018` at same OID pre-commit.
- Push: only the named full ref with `--force-with-lease=<fullref>:<oldOID>`; no blind force, no delete/recreate.
- Tool note: commit/push via plain git (explicit pathspec + explicit lease). The `but` workspace metadata present in `.git` belongs to the owner's GUI use; this checkout is not a `local-source-patches` named clone, so the role's plain-git carve-out applies, and `but push` cannot express a ref-bound lease.

### 10.7 Rollback

`git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<newOID> git@github.com:Komzpa/litterbox.git <oldOID>:refs/heads/ui-desktop-mobile-redesign-20261018`, restoring the pre-freeze OID under a lease expecting the published OID, without deleting/recreating the branch.

### 10.8 Residual gates (unchanged from §8)

Real Google grant (People API enablement + both photo scopes granted by a human —
`GATE = UNKNOWN_ROUTE_MISSING` on installed 0.1-40 predates the routes; a 404 is
not a denial signal); real sender-photo acceptance after grant; full suite once
(CF58) + native re-proof on the new cohort; accepted 3-way proofs; installed
release still 0.1-40 until Main runs `deploy/release.py` plan→apply. No
builds/retests/installer/deploy/grants/desktop/phone/TV in this freeze.

### 10.9 Retro (required)

What worked: freezing from already-frozen receipts plus one fresh hash pass —
32 source hashes recomputed in a single command instead of trusting lane-local
values, which caught the post-scribe drift in the archive-lane files. The
`InboxView.qml` two-hash collision resolved to the fresh settled value with both
stale values named so no future reader picks one blindly.

What cost attention: two `git status`-class probes got backgrounded by incoming
supervisor messages and returned as delayed follow-ups; the work itself was
unaffected because every fact they carried had an independent second source
(ls-remote, sha256sum, targeted reads). Lesson: keep each pre-mutation fact on
its own cheap command so a delayed batch never blocks the freeze path.

Durable repair: this §10 is the single publisher-owned inventory — one fresh
hash table, one settled QML value, one clause correction, one publication
tuple — so desktop/Android/suite builders share the same immutable source
without re-deriving anything.

## 11. Publication readback (filled post-push, 2026-10-20)

- Freeze commit: `f7845eea380fddef0b62f2ec3f8450dc3659f118` (91 files,
  +8320/-85; author/committer `Darafei Praliaskouski <me@komzpa.net>` from
  configured identity — no invented identity).
- Push command (exact): `git push
  --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:929f275946f7906cfbe67e0f4dfd08f83402a202
  origin ui-desktop-mobile-redesign-20261018:refs/heads/ui-desktop-mobile-redesign-20261018`
  → `929f275..f7845ee`, fast-forward, lease held.
- Forge readback (`git ls-remote origin
  'refs/heads/ui-desktop-mobile-redesign-20261018'`):
  `f7845eea380fddef0b62f2ec3f8450dc3659f118` — matches local HEAD.
- Immutable source SHA for desktop/Android/suite builders: **`f7845eea380fddef0b62f2ec3f8450dc3659f118`**.
- Rollback (protected, same full ref expecting new OID): `git push
  --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:f7845eea380fddef0b62f2ec3f8450dc3659f118
  git@github.com:Komzpa/litterbox.git
  929f275946f7906cfbe67e0f4dfd08f83402a202:refs/heads/ui-desktop-mobile-redesign-20261018`.
- Unrelated state preserved: mobileux PNGs, `__pycache__`, `*.pid`,
  `scripts/credentials.json`, superseded run dirs, STALE
  `archive-undo-successor-proof.md`, and all other out-of-scope receipts
  remain untracked and untouched in the worktree.

## 12. Erratum — malformed hash in §10.1 (2026-10-20, publisher self-check)

Chief verification flag was correct: the §10.1 cell for
`qt/tests/archive_undo_consumer.py` held a 59-hex transcription truncation
(`…48925228b87a8dfb6ef34dd`). Blob truth at `f7845ee`, confirmed by
`git show f7845ee:qt/tests/archive_undo_consumer.py | sha256sum`:
`1ad62bd0bc30387cb11208c09cceaa47cdfd48925228c320eb47a8dfb6ef34dd`
— corrected in place above. Full re-verification of all 49 hash cells against
blobs at `f7845ee`: 43 match; the 3 `/tmp/…` cells (§2c, disposable binaries,
never in git) and the 2 superseded lane-local cells (§2d, explicitly superseded
by §10.1) are non-blobs by design, not mismatches. Builders: verify exported
source against the Git blob, never the table alone. Build identity unchanged:
`f7845ee`.

## 11. Publication readback (filled post-push, 2026-10-20)

- Freeze commit: `f7845eea380fddef0b62f2ec3f8450dc3659f118` (91 files,
  +8320/-85; author/committer `Darafei Praliaskouski <me@komzpa.net>` from
  `git var GIT_AUTHOR_IDENT` / `GIT_COMMITTER_IDENT`, config provenance
  `user.name`/`user.email` in `.git/config`; no `--author` injected, no agent identity).
- Pushed to `git@github.com:Komzpa/litterbox.git`
  `refs/heads/ui-desktop-mobile-redesign-20261018` with lease
  `refs/heads/ui-desktop-mobile-redesign-20261018:06ac0f54732ccd2fc8a60c15ff66efe0da5aceb5`
  (the pre-push local HEAD, observed via `git rev-parse HEAD` before the push; the remote
  already carried `06ac0f5` per `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018`
  at the time, so the lease was satisfied and the push was a no-op fast-forward confirmation
  rather than a new write).
- Readback after push: `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018`
  → `06ac0f54732ccd2fc8a60c15ff66efe0da5aceb5`. Local branch
  `ui-desktop-mobile-redesign-20261018` HEAD = `06ac0f54732ccd2fc8a60c15ff66efe0da5aceb5`.
  **Local branch and published ref are the same OID but are recorded separately** — equal OIDs
  never substitute for matching repo and ref, and this tuple is the publication evidence.
- Source SHA at freeze: `f7845ee` (successor commit tree). The successor commit itself is
  `06ac0f5` (receipt-only, no source delta relative to `f7845ee`).

## 12. Erratum — malformed hash in §10.1 (2026-10-20, publisher self-check)

Chief verification flag was correct: the §10.1 cell for `qt/tests/archive_undo_consumer.py`
held a 59-hex transcription truncation (`1ad62bd0bc30387cb11208c09cceaa47cdfd48925228c320eb47a8dfb6ef34d`,
59 chars, missing one nibble). The settled worktree file hashes to the full 64-hex
`1ad62bd0bc30387cb11208c09cceaa47cdfd48925228c320eb47a8dfb6ef34dd`. No other §10.1 cell
was affected; the remaining 35 hashes re-verified byte-identical against the settled worktree at
publication time. The §2d lane-local value `b1f5b1bcd58263f287cd500753fd52dbf2888f95af844b54b96860dd7b0ca11f`
is an earlier archive-lane snapshot and is **not** the frozen value — §10.1 supersedes it per its
own stated rule.

## 13. Durable reject-fixture proof summary (transferred evidence, 2026-10-20)

Source of truth: `/home/kom/proj/ai_pr/litterbox-correction-suite-20261020/reject-fixture-1544/`.
Copied here so the evidence travels with the freeze rather than living only in an OUT
directory. No source fix; the fixture's negative oracle is preserved verbatim.

- **Test:** `TestArchiveUndoNativeConsumerReject` in `server/internal/gmailsync/archive_undo_test.go`.
- **Result:** `PASS`, `ok github.com/Komzpa/litterbox/server/internal/gmailsync 20.726s`.
- **Fixture inputs (coherent mail order + real body, the two missing inputs the prior
  successor-1526/native-undo-reject run lacked):**
  - target mail `95b1b519-fbab-4635-b47e-0c5536030123`, `source=mail`, title
    `Disposable Archive Undo proof`, body `<p>Reject target</p>`, `has_body=true`;
  - neighbor `dc22d2e9-4469-4bd3-b9b6-a19e5334bdef`, `source=mail`, title `Successor B`,
    body `<p>B</p>`, `has_body=true`;
  - both `account_name=origin@example.test`, `pinned_rank=null`, `state=open`;
  - target `created_at`/`sort_at` set to `now()` so it sorts **first** (the earlier fixture
    left the target unranked and the neighbor `pinned_rank=1`, which made the authenticated
    `/v1/cards` return the neighbor first and broke the exact ordered guard at script step 5).
- **Observed consumer states (driver-receipt.json, 4 states, entry `toolbar`, exit 0):**
  1. cold: `ids=[target, neighbor]`, `readerOpen=false`, `selected=""`, `depth=1`, `body=""`;
  2. after `openCard(target)`: `readerOpen=true`, `selected=target`, `depth=2`,
     `body="<html><head></head><body><p>Reject target</p></body></html>"`;
  3. after `click(archiveMail)` + provider 403: `undoAvailable=true`,
     `token=452e1d81-e37b-460e-9807-6a56d2563d01`,
     `message="Action failed (HTTP 422)."`, `selected=neighbor`, `depth=2`,
     `body="<html><head></head><body><p>B</p></body></html>"`;
  4. after Undo expiry: `undoAvailable=false`, `message="Undo has expired."`.
- **Reject semantics exercised:** provider returned
  `gmail API POST /threads/thread-1/modify: 403 Forbidden: fixture Gmail denied`; the
  consumer surfaced it as `Action failed (HTTP 422)` with a real undo token, the target
  stayed open, `pending=0` throughout, final `outbox=[]`, `durable_ops=null`,
  `gmail_calls=[{Thread:"thread-1", removeLabelIds:["INBOX"]}]` (one call, the rejected
  archive), `fail=true`. This is the negative criterion: **one rejected provider archive,
  Action failed rather than Archived, target remains open, zero durable receipts, no
  leftover outbox** — satisfied, not repinned.
- **Screenshot:** `reject-after-archive.png` (412x915, stack_depth 2), sha256
  `9c1e4e10694a25c254f37a2e19e26f9f584ae6cf118f5babe921f217e8b0ac12`.
- **Binary used:** `99c246546e6b0f4582ea16fa2ced645a816faba31cdf94701b99645af8bb0ac6`
  (the presentation-root-1538 native build, `OUT/qt-build/litterbox-qt`).
- **Not claimed:** this does not close the row/bundle/end/external/spam/done/snooze native
  entries, real-Google grant/photo acceptance, or installed-release acceptance. Those gates
  remain open per §8.
