# Gmail sender-photo completion — frozen consumer proof

2026-10-20. Worker: CompleteSenderPhotoNotification.

**Notification fix complete; cold online and offline provider-fixture paint proved on the actual Qt consumer. Real-Google grant/production-photo acceptance remains unproved and is NOT closed by this fixture.** No commit, push, deployment, shared installer, Google grant, production token/mail operation, owner desktop/session bus, physical phone or TV was used.

## 1. Exact scope and source identity (CF26)

Changed product/test paths in this continuation:

- `qt/src/CardStore.cpp`: photo completion explicitly notifies `CardRole` and the retained `SenderPhotoRole`.
- `qt/qml/InboxView.qml`: source `Connections` moved out of `DelegateModel`'s lazy delegate property into the live ListView; cache completion now rebuilds the actual presentation. The shared-head, standalone and member avatars all consume the same cache-backed presentation photo value. No manual `refresh()` workaround.
- `qt/tests/tst_senderphotopaint.qml`: completed the retained single-consumer regression; binary completion is asynchronous, the real visible row avatar and painted window pixels are checked, then 403/404/default and malformed-byte controls and persisted offline reopen run in the same test.
- `receipts/gmail-avatar-proof/notification-20261020/**`: this durable evidence bundle and reproducible scoped runner/scripts.

Retained companion files were not rewritten: `qt/src/CardStore.h`, `qt/tests/CMakeLists.txt`, `qt/tests/bundle_runner.cpp`, `qt/tests/fixtures/sender-photo-64x64.png`, `qt/pages/GmailAccountsPage.qml`. Their candidate hashes are recorded beside the changed source and executable hashes in `CANDIDATE.sha256`. Existing API/pipeline/auth UI/offline implementations were reused. ArchiveBundle guard and row-continuation hunks were not edited. `qt/tests/archive_undo_consumer.py` was not edited. No Git metadata was changed; publisher records the eventual revision.

Earlier receipt identities remain historical only: base `929f275946f7906cfbe67e0f4dfd08f83402a202`, previously published product `df9b13e5513ed1f1739149a3d6048c27eee3c725`. This proof identifies the actual uncommitted candidate by source and executable SHA-256, not either historical revision. Foreign Git process 393457 was not signalled or mutated.

## 2. Baseline and causal diagnosis

The named inputs existed: `/tmp/lb-avatar-qt-build` and `receipts/gmail-avatar-proof/newrun-20261020-1302/PROOF.md` (`stat`, exit 0). The retained Qt diff was the first repository inspection, not a broad inventory/history search. `DIAGNOSTIC.md` records that initial checkpoint.

The predecessor's actual app observation was accepted as baseline: the downloaded cache painted only after a manual `store.refresh()`; offline reopen already painted. It was not rerun merely to confirm the report.

The first scalar-role change alone did **not** fix the cold native consumer. `cold-role-failure/online-app.log` records cache populated but `Image.source == ""` with exactly one initial cards request. The discriminator in `cold-boundary-state.json` then showed:

1. `senderPhotoChanged` had fired with a nonempty device-local URL.
2. The model's `sender_photo` role already contained that URL.
3. The actual presentation's source entries/rows still had `photo: ""`.

**Root cause:** the `Connections` object was nested in `DelegateModel`, whose default property is a lazy `delegate` Component. Because this model is used for role/group access rather than instantiated visual delegates, that signal receiver was not live. Installed Qt type evidence: `/usr/lib/x86_64-linux-gnu/qt6/qml/QtQml/Models/plugins.qmltypes:1041-1043` (`QQmlDelegateModel`, `defaultProperty: "delegate"`). Relocating the existing receiver fixes the main notification path; no polling, retry layer, provider lookup, model reset or refresh workaround was added.

## 3. Frozen build and exact commands/exits

After the final product/test edits froze:

```text
cmake --build /tmp/lb-avatar-qt-build --target litterbox-qt tst_bundleexpand --parallel 2
exit 0
python3 receipts/gmail-avatar-proof/notification-20261020/run_proof.py online
exit 0
# Stop owned lb-photo-notify-server before the offline leg.
python3 receipts/gmail-avatar-proof/notification-20261020/run_proof.py offline
exit 0
```

`final-build-command.json` records the build, service commands/environment and service stops. `commands.json` records every final scoped subprocess argv, exact command, start/end timestamps, exit and isolation paths. The expanded commands include:

| Check | Command surface | Exit |
| --- | --- | --- |
| Retained consumer regression | `dbus-run-session -- /tmp/lb-avatar-qt-build/tests/tst_bundleexpand -input <ROOT>/qt/tests/tst_senderphotopaint.qml -import <ROOT>/qt/tests/qml-imports` | 0 — 3 passed, 0 failed |
| Notification-cut control | Same test, disposable copied QML imports with both completion handlers disabled | 1 — expected failure at first-online/no-refresh assertion |
| Server-reported status | authenticated loopback `GET /v1/gmail/sender-photos/status` | 0 |
| Binary provider fixture | authenticated loopback `GET /v1/cards/aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa/sender-photo` | 0; bytes identical to retained valid PNG fixture |
| Actual cold online/auth app + offline reopen | `litterbox-qt --capture-script <fresh-private-profile> online.json ...`, then same profile with `offline.json` after server stop | 0 each |

The complete literal paths/argv are in `commands.json`, not abbreviated there. No formatter, linter, full suite or shared installer was run. The parent owns the final merged-candidate validation and separate certification.

## 4. Actual consumer outcome and negative controls (CF2)

The final online app used a **fresh empty private profile and cache**, not the earlier warm diagnostic profile. `online-app.log` asserts `requests: 1`: only `setOnline`'s initial cards request, no manual refresh or second snapshot. The granted row's local file URL is nonempty, Image is Ready/visible, and initials are hidden. Both default/missing rows keep initials and no image source.

The binary download is a valid 64×64 red-left/green-right PNG served through the existing authenticated server route and production `Api::getBytes` → `applyRemoteSenderPhoto` wiring. It is explicitly a disposable **provider fixture**, not a real Google photo or grant.

Pixel evidence (`paint-online.json`, `paint-offline.json`):

| Actual painted surface | Exact red pixels with green 4–40 px right |
| --- | ---: |
| Production Qt cold first-online inbox, no refresh | 512 |
| Retained actual-InboxView regression, first online | 512 |
| Same regression, offline cache reopen | 512 |
| Production Qt reopen, owned server stopped | 512 |

PNG readback was performed. Online visibly shows `Photo card` with the red/green avatar, `Default card` and `Missing grant card` with `?` initials. Offline visibly shows the same photo and fallbacks plus `Can't reach the server`. These are screenshots of the application, not fixture-only image checks.

The scoped regression also covers: no request for an unadvertised/default-only photo; 403 authorization-required and 404 unavailable responses preserve initials; malformed bytes emit failure, never enter cache and never paint; reopened offline store issues zero photo requests.

The targeted break-the-code control disables `onDataChanged` and `onSenderPhotoChanged` **only in a disposable QML copy**, leaving source/binaries untouched. It goes red at `first online sender photo must paint without a manual refresh()` (`notification-negative.log`: 2 passed, 1 failed, exit 1). This proves that initial load or cached fixture bytes alone cannot satisfy the notification test.

## 5. Authorization controls (CF39)

No auth UI code was changed. The existing actual Accounts page was exercised in the same final scoped campaign:

- `accounts-three-states.png` visibly reads `Authorized — photos appear after they sync`, `Sender photos ready`, and `Sender photos need authorization`, with both server-reported contact scopes and `Authorize sender photos…`.
- The real rendered Authorize button was clicked using the existing capture harness's painted object lookup. `online-app.log` records an `https://accounts.google.com/...` authorization URL containing Gmail.modify and both contact scopes, with empty status/consent errors.
- `accounts-consent-not-opened.png` visibly shows the grant explanation, read-only returned URL field and `Open authorization page` control. The field is horizontally scrolled to the URL tail; the full URL is machine-read in the log. The Open control was **never clicked**, no browser opened, no consent/grant performed.

Isolation: private Xvfb `:196`, private HOME/XDG directories, fresh `dbus-run-session` per application/test, nonexistent system-bus address. No owner desktop/session bus was inherited. The disposable server had dead HTTP(S) proxies (`127.0.0.1:1`); its observed Google token attempts failed locally at the proxy, not at Google. No production secrets were loaded.

Owned services were stopped: `lb-photo-notify-server` before offline proof; `lb-photo-notify-xvfb` and `lb-photo-notify-pg` afterward. Foreign ports/processes and the named read-only planning lane were preserved.

## 6. Remaining Google and release gates — NOT closed

1. **Real Google configuration/grant:** owner-authorized verification of People API enablement, consent-screen inclusion/verification of both `contacts.readonly` and `contacts.other.readonly`, and an actual account grant remain unproved. The API is not declared disabled. This lane did not open a grant/browser or use production accounts.
2. **Real sender-photo acceptance:** after that grant, prove an exact-address non-default People connection/otherContact photo travels through sync, authenticated route and the device consumer without refresh. Absent/default/non-granting senders remain fallback/explicit authorization states. Seeded PNGs cannot close this gate.
3. **Release ownership:** main/publisher must validate the frozen merged candidate once, independently certify relevant GUI journeys, retain archive-next/ArchiveBundle/row-continuation acceptance, assign the exact revision and perform any authorized packaging/install/release. This lane did not run archive tests, full suite, phone/TV checks or installer/deployment, and did not claim those gates passed.
4. **Existing retry observation, outside this notification repair:** the retained prototype test's extra no-retry-spin assertion observed two 403 fixture requests after a 4 s settle (`fallback-requests/senderphotopaint.log`, exit 1). The prefetch code removes negative marks using an eligibility set that already excludes those marks. No retry code was changed. The final assigned fallback proof checks initials/no bad paint, not a no-retry contract; the extra prototype no-spin assertion was removed explicitly, not claimed fixed. Parent retains this observation if retry policy is a release gate.

CF2: actual consumer pixels plus a notification-cut negative control, never seed-only completion. CF26: exact source/binary SHA-256 in `CANDIDATE.sha256`. CF39: isolated worker GUI evidence complete for the named desktop fixture scenario; independent/merged-candidate certification remains parent-owned. CF65: all screenshots, logs, scripts, hashes and this receipt are under this project-relative durable directory; no result depends on scratch surviving.

## 7. Retro (required)

What worked: the final cold-boundary probe distinguished cache/model success from presentation failure and identified the canonical owner; moving the lazy receiver, rather than adding more refreshes or rewriting the pipeline, fixed the actual symptom. Fresh-profile production paint plus the asynchronous notification-cut control caught the false confidence of a preloaded/synchronous QML test.

What cost time: the first attempt assumed scalar roles alone were enough; a warm native reopen appeared green but was correctly excluded from first-online acceptance. Sub-item pixel grabs initially returned white, even at Image.Ready; sampling the existing full-window capture at mapped avatar coordinates produced the real painted oracle. These failed attempts are preserved in `initial/`, `frame-wait/`, `fallback-requests/` and `cold-role-failure/`, not presented as green proof.

Durable repair: the regression now delays the binary reply beyond initial layout, samples actual window pixels, and has an explicit notification-cut control. The production source explains why notification receivers must stay outside DelegateModel's lazy default delegate property. Do not repeat the predecessor retro's suggestion to bake a post-download `refresh()` into smoke scripts: that was a product defect, not a valid acceptance workaround.
