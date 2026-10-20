# Archive failed acceptance — row/bundle successor fix (R68)

## Source identity (CF26)
- Repo: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`
- Branch `ui-desktop-mobile-redesign-20261018`, HEAD `929f275946f7906cfbe67e0f4dfd08f83402a202` (settled scout base; uncommitted photo work by live `ProveGooglePhotoConsumer` preserved, no photo/avatar/cache hunks touched).
- Changed paths (only):
  - `qt/qml/InboxView.qml` sha256 `368486054caf404aa1783e95714d8171a6a64d238bdd2c3df8f59ac35841b676`
    - `archiveBundle(bundleId)` now captures `captureArchiveContinuation(selectedCardId)` before `store.archiveBundleNow` when the open card is in the bundle, invokes once after (R68 “advances once”).
    - Row `Litterbox.CardActions onCardHandled` now captures continuation when `action === "archive"` before `showArchiveUndo`, invokes after. Spam/done/snooze (`action !== "archive"`) unchanged — no continuation.
    - `captureArchiveContinuation` itself unchanged (pre-removal presented-order snapshot, strictly after archived index, skips hidden collapsed non-leaders and non-openable rows, opens first surviving via `openReaderEntry`, no wrap).
    - Async `Qt.callLater` close at ~1290 stays close-only for external/snapshot (R68 “External archive, Spam, Done and Snooze never auto-advance”).
  - `qt/tests/archive_undo_consumer.py` sha256 `b1f5b1bcd58263f287cd500753fd52dbf2888f95af844b54b96860dd7b0ca11f`
    - `--entry toolbar|row|bundle|end|external|spam|done|snooze` (default `toolbar` preserves prior invocation).
    - Desktop body probe reads `stack.currentItem.html` else `mailDetailPage.html` (embedded desktop reader); phone path unchanged.
    - Bundle script expands `bundleToggle-A` before `openCard` (leader-collapsed member button hidden until expanded).
    - External probe calls `POST /v1/__test/external-archive/{id}` (authenticated prefix; bare `/__test` bypasses `auth.Middleware` which only authenticates `/v1/`).
    - Row 5 states, bundle 6 states (extra expand), toolbar 6, end 4, external 3, spam/done/snooze 3.
  - `server/internal/gmailsync/archive_undo_test.go` sha256 `704175d98e8975262ddd108e561b8288b7eb918a793a346f4889f37c5404236c`
    - `POST /v1/__test/external-archive/{id}` sets `state='archived'` directly (disposable snapshot control, no ops/Gmail).
    - `runArchiveEntry` helper + `TestArchiveRow/Bundle/End/External/Spam/Done/SnoozeNativeConsumer`; `TestArchiveUndoNativeConsumer` (toolbar) preserved + `MkdirAll` for base output.
    - Done fixture archives fixture mail A, inserts manual `done-proof` + `card_bodies('<p>M</p>')` so `openCard` opens the reader via `has_body`.
    - Bundle fixture inserts `bundles('Archive bundle')` + assigns fixture A `bundle_id`.
- REQUIREMENTS.md R68 unchanged (oracle already specifies advance-on-local, bundle-once, Undo-keeps-successor, no-wrap close, external/spam/done/snooze no-advance). No avatar normative clauses touched (CF39).

## Executable (do NOT claim installed release)
- Out-of-tree disposable build: `/home/kom/proj/ai_pr/archive-successor-build-20261020/litterbox-qt` sha256 `618afa2e54315d3f84fdccfa6675c404ad400bc28be8d869fdcbfa44058faeb0`
- Built via `cmake -S qt -B <out> -DCMAKE_BUILD_TYPE=Release` + `cmake --build <out> --target litterbox-qt -j8` (configure ok, build 100% linked). No install, no deb, no owner desktop/session bus, no Git commit/push (lead `git-pr-owner` follows).

## Entry/wiring exercised (CF2 — real entries, not helpers)
- Toolbar: `archiveMail` → `mailActions.primaryAction()` → `enqueueOp archive` → `MailDetailPage.onCardHandled` captures `requestArchiveContinuation` before `closeReader`, then `undo`, then `next()` (pre-existing wired path; re-proved, no assumption).
- Row/keyboard: `doneButton-A` (and Delete/A via `cardActions.primaryAction()`) → row `onCardHandled` (NEW) captures before deferred rebuild, shows Undo, invokes `next()`.
- Bundle: `archiveBundle-A` → `window.archiveBundle(bundle_id)` (NEW) captures when selected is bundle member, archives via `archiveBundleNow` (one `archive` op per unpinned member), invokes once.
- End: toolbar archive on last card B → `captureArchiveContinuation` returns null (no candidates) → closes to list, no wrap.
- External: `POST /v1/__test/external-archive/A` → `state='archived'` → `store.refresh()` snapshot → async close-only, no advance.
- Spam: `inboxActions[0]` → `actionRow-spam-A` → `enqueueOp spam` → `cardHandled(op,"spam")` → no continuation → close.
- Done: `doneButton-M` (manual with body) → `dismiss` → `cardHandled(op,"done")` → no continuation → close.
- Snooze: `inboxActions[0]` → `actionRow-snooze-A` → `snoozePreset-hour` → `enqueueOp snooze` → `cardHandled("","snooze")` → no continuation → close.

## Behavior evidence (disposable pg_virtualenv + Xvfb, private HOME/XDG, loopback Gmail)
Full clean matrix `go test -count=1 -run "^(TestArchiveUndo|TestArchiveRow|TestArchiveBundle|TestArchiveEnd|TestArchiveExternal|TestArchiveSpam|TestArchiveDone|TestArchiveSnooze)NativeConsumer$" ./internal/gmailsync/` → **ok 201.050s** (private cluster `18/regress` created/dropped; `CARD_TEST_POSTGRES=1`, `LB_QT_UNDO_BINARY` above, `LB_QT_UNDO_OUTPUT=/home/kom/proj/ai_pr/archive-successor-run-20261020`).
- toolbar (412×915 phone): 6 states `[A,B]` → reader A → archive → `[B]` reader B → Undo → `[A,B]` reader B → settle/snapshot reader B; `body` at settle `<html>…<p>B</p>…</html>` via production `mailbody.Handler`; Qt cache `[A,B]`, outbox empty; offline leg queued-cancel preserved; exactly 2 durable / 2 Gmail (thread-1 remove/add INBOX).
- row (1440×1000 desktop split): 5 states; s2 `ids=[B] readerOpen=true selected=B body='<html>…<p>B</p>…</html>' message='Archived'`; s3–s4 Undo restores `[A,B]` reader stays B with B HTML; cache `[A,B]`, outbox empty.
- bundle (1440×1000, A single-member bundle, expanded before open): 6 states; s3 `ids=[B] reader B body B HTML`; s4–s5 Undo restores `[A,B]` keeps B; cache ordered, outbox empty.
- end: 4 states; archiving B closes (`readerOpen=false depth=1 selected!=A`), no wrap to A; Undo restores `[A,B]` stays in list.
- external: 3 states; `POST /v1/__test/... → 204`, refresh drops A → `readerOpen=false depth=1 selected!=B`, no advance; outbox empty, 0 durable/0 Gmail.
- spam/done/snooze: 3 states each; A (or M for done) removed, `readerOpen=false depth=1 selected!=B`, B survives, no advance; outbox empty after settle (spam/done/snooze ops acknowledged, no continuation).
- Negative controls: unauthenticated `GET /v1/cards → 401`; provider rejection covered by existing `TestArchiveUndoNativeConsumerReject` (untouched).

## Screenshots inspected (fresh, from these runs)
- `...-row-captures/row-successor-B.png` INSPECTED: desktop split, list shows `Successor B / Neighbor` selected, reader shows `Successor B / origin@example.test / Neighbor / B`, bottom bar `Archived · Undo` — successor visible with real B body.
- Also produced (hashes in `driver-receipt.json`): `row-before.png`, `row-undo-keeps-B.png`; `bundle-before/successor-B/undo-keeps-B.png`; toolbar `01-before/02-reader-successor-B/03-immediate-undo/04-settled/05-offline/06-offline-archived.png`; `end-closes-without-wrap/undo-stays-in-list.png`; `external-before-snapshot/no-advance.png`; `spam/done/snooze-no-advance.png`.
- Run dirs: `/home/kom/proj/ai_pr/archive-successor-run-20261020{,-row,-bundle,-end,-external,-spam,-done,-snooze}/` + `*-captures/` siblings; each `driver-receipt.json` carries `entry,exit,binary_sha256,states,cache_ids,outbox,screenshots`; each `server-receipt.json` carries `durable_ops,gmail_calls,actual_handlers`.

## Commands (exit status)
- `cmake -S qt -B …/archive-successor-build-20261020` → 0 (Qt plugin warnings only).
- `cmake --build … --target litterbox-qt -j8` → 0, binary sha above.
- `go test -run ^TestArchiveRowNativeConsumer$` → ok 28.095s (after desktop body-probe fix).
- `go test -run ^(Bundle|External|Done|Undo)$` clean → ok 117.669s.
- Full 8-entry matrix clean → ok 201.050s.
- No formatters/linters/project-wide suites (main gates later per CF58).

## Remaining gaps
- Pinned multi-member bundle successor gap **closed** (adversary `ChallengeArchiveEntryAcceptance` block resolved): single-member bundle proof could not catch a pinned survivor advancing. The `bundle_pinned` oracle now covers it — pinned P keeps P body; Undo/open A1/archive skips removed A2 to B once; Undo restores order keeps B body. Same-oracle fail/pass: broken `7c8df3a9bd131916bbb93275925798fe5704cb50f3bf6ea34046303d405afb0a` → `FAIL 46.405s (test 46.39s)`, TEST_EXIT 1, 9 states Qt 0/oracle 1 (s3/s4 sel B body B, wrong, expected P/P); fixed `6095038658606b38344be49ea13ff5c351135a304236c1ff4c80693ed69b4297` → `ok 47.357s`, TEST_EXIT 0, 9 states Qt 0/oracle 0 (s3 `[P,B]` sel P body P — pinned P retained; s6 `[P,B]` sel B body B — A1 archive skips removed A2 to B, once; s7 full sel B body B — Undo keeps B; s8 `cached==initial`, `outbox==[]`). Run dirs `/home/kom/proj/ai_pr/archive-pinned-run-20261020-bundle_pinned` + `-captures/-profile/-script.json`, `/home/kom/proj/ai_pr/archive-pinned-broken-run-20261020`. Pinned source hashes (exact exercised candidates, not current): `qt/qml/InboxView.qml` `dd6a444eea0d764548cb8762911971c2354aad8947ae107770692af586b7172c`, `qt/tests/archive_undo_consumer.py` `1ad62bd0bc30387cb11208c09cceaa47cdfd48925228c320eb47a8dfb6ef34dd`, `server/internal/gmailsync/archive_undo_test.go` `679e2c7dce4d37f90baf7d56dc39e44342d0ea1395f58b68fc4e51686181e203`. Remaining: full-suite/native/installed still pending (CF58); `git-pr-owner` owns commit/push; no installed release claimed.
- CF67 correction: row/bundle probe/setup reds (`:66-70`) were harness fixes, not an executed pre-fix archive→successor failure; the original row/bundle defect stays source-inspected/user-reported here. Executed failing-before/passing-after exists only for the pinned slice — evidence in this file (Remaining gaps above), not a follow-on receipt: same `bundle_pinned` oracle, same runpaths, broken `7c8df3a9bd131916bbb93275925798fe5704cb50f3bf6ea34046303d405afb0a` → `FAIL 46.405s` exit 1 vs fixed `6095038658606b38344be49ea13ff5c351135a304236c1ff4c80693ed69b4297` → `ok 47.357s` exit 0 (run dirs `/home/kom/proj/ai_pr/archive-pinned-broken-run-20261020` and `/home/kom/proj/ai_pr/archive-pinned-run-20261020-bundle_pinned`). No broader failing-before claimed: historical 8-entry passing-after (`ok 201.050s`) retained unchanged above.

## Retro (required)
- Row red was exact and mine: desktop `stack.currentItem.html` is empty because the reader is embedded (`mailDetailPage`), not pushed — the phone-only probe asserted `''`. Fixed by falling back to `mailDetailPage.html`; B HTML then asserted `<p>B</p>` on row/bundle/toolbar alike. Lesson: viewport changes the reader host; probe both.
- Bundle red was mine: leader-collapsed hides the member `openCard-A`, so `openCard` found nothing (1 state). Fixed by expanding `bundleToggle-A` first. Lesson: bundled rows have two visibility gates (leader + expanded).
- External 401 was mine: `auth.Middleware` only authenticates `/v1/`, so bare `/__test` never got a tenant. Fixed by moving the control to `/v1/__test/...`. Lesson: test routes must live under the authenticated prefix.
- Done red was mine: manual without `card_bodies` never opens the reader (`openCard` needs mail or `has_body`). Fixed by inserting `<p>M</p>`. Lesson: Done needs a readable manual to hold the reader open.
- Stale-capture reds were mine (exit 2 `output directory must be empty`): re-runs reused dirs. Fixed by `rm -rf` disposable outputs before each matrix. Lesson preserved from prior receipt.
- No whole-file writes; only owned hunks (`archiveBundle`, row `onCardHandled`, driver, Go harness). Photo/avatar/cache hunks untouched per live reservation with `ProveGooglePhotoConsumer` (ack received).

## Carry-forwards
- CF2: claims tied to real entries + production body route + visible B HTML + Undo-keeps-B (above).
- CF26: exact identities — HEAD `929f2759…`, binary `618afa2e…`, sources `36848605…/b1f5b1bc…/704175d9…`, run dirs above.
- CF39: owned display only; avatar clauses/docs untouched.
- CF58: full-suite gate deferred until both writers settle; this receipt is passing-after only for the 8-entry matrix (single-member row/bundle + end/external/spam/done/snooze negatives retained, no full re-run by this slice). No executed row/bundle failing-before is claimed here (see Remaining gaps/CF67).

## Release handoff (PrepareArchiveReleaseHandoff, 2026-10-20, read-only, no build/install/push)
- Typo fix: `:63` `InboxView.vue` → `InboxView.qml` (scribe-introduced; fresh-read verified).
- HEAD `929f275946f7906cfbe67e0f4dfd08f83402a202` on `ui-desktop-mobile-redesign-20261018`; worktree dirty (archive 3 paths + live photo writer `ProveGooglePhotoConsumer` + 17 other modified; photo hunks untouched, not owned).
- Archive sources (this receipt §Source identity): `qt/qml/InboxView.qml` `36848605…`, `qt/tests/archive_undo_consumer.py` `b1f5b1bc…`, `server/internal/gmailsync/archive_undo_test.go` `704175d9…`; disposable binary `/home/kom/proj/ai_pr/archive-successor-build-20261020/litterbox-qt` `618afa2e…` — NOT a release artifact.
- Kits on disk: 0.1-40 `KIT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/release-kit-0.1-40/artifacts/` (server deb `00d6285b…`/bin `e8d911d0…`, desktop deb `c7245d55…`/bin `56d975e1…`); APK `/home/kom/proj/ai_pr/litterbox-native-proof-20261018/repair-native-20261019/v27/litterbox-qt-v27-b984460a.apk` `83ad492b…` v27; plan lane `/home/kom/proj/ai_pr/litterbox-release-plan-20261020/`; 0.1-36 precedent `/home/kom/proj/ai_pr/litterbox-release-cohort-20261019/`.
- Installed now (= next before): server 0.1-40 `e8d911d0…`, desktop 0.1-40 `56d975e1…`, APK v27 `83ad492b…`, `db.env` `a7614256…` (per `ui-corrections-publication.md` §A.2); receipt `/var/backups/litterbox/release-20261020-df9b13e/receipt.json` `3728300c…` status success.
- Canonical manifest fields (`deploy/release.py` docstring + `validate`): `source_sha` full 40-hex; `artifacts.{server,desktop,android}` path/sha256/package/version(+architecture/binary_sha256, apk version_code/version_name/signer_sha256) each with matching `source_sha`; `proofs[]` {path,sha256,artifacts,accepted:true,bindings}; `target` {host `nucat`, service `litterbox-server.service`, environment `/etc/litterbox/db.env`, server_binary `/opt/litterbox/server/litterbox`, desktop_binary `/usr/bin/litterbox-qt`, unit_path, origins×3, credential_loader, aapt, apksigner, publisher_inputs/before_receipt {path,sha256}, rollback_debs, drop_ins[embeddings.conf `bb554e2f…`, gmail.conf `0c0440e3…`]}.
- Canonical commands (cwd ROOT, not run): `sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py plan <cohort.json>` then `... apply <cohort.json> --receipt /var/backups/litterbox/release-<id>`; rollback reference `... rollback /var/backups/litterbox/release-20261020-df9b13e` (recorded, not executed).
- Gates LEFT: (1) source freeze after both writers settle — new 40-hex `source_sha`, clean `git diff --name-status` read; (2) full suite once (CF58) + native re-proof on new cohort; (3) accepted 3-way proofs binding new source+artifact hashes; (4) fresh before-receipt + publisher-inputs at plan time; (5) Git tuple fresh `ls-remote` + commit/push by `git-pr-owner` only; (6) `plan`→`apply` by Main via `deploy/release.py` only.
- Unknowns (exact): post-freeze source SHA; post-rebuild server deb/bin + desktop deb/bin + APK hashes; before-receipt served signer/apk/old-manifest values at plan time; publisher-inputs env/server SHAs; proof receipt hashes.
- CF26 exact source/artifact above; CF65 durable receipt = this file + `ui-corrections-publication.md` + `/var/backups/litterbox/release-20261020-df9b13e/`; CF39 no user screen — no GUI/mail/phone/TV/installer touched.
- Retro: read-only handoff stayed out of the live writer's way; the `.vue` typo was confirmed by fresh raw read before edit; worktree-dirty fact came from `status/diff` read-only (no fetch/merge-tree/metadata writes, no builds).
