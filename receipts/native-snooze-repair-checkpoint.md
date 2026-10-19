# Native Snooze repair — discriminator checkpoint

Owner: RepairNativeSnoozeConsumer, continued by FinishNativeGeometryProof. Historical discriminator checkpoints below retain their as-observed statuses. **Final disposition: all seven native clauses PASS on unchanged signed v27; see the final section and `receipts/ui-corrections-android.md`.** This does not claim Git integration, desktop proof or publication.

Two fresh `adb devices -l` + `getprop ro.boot.qemu.avd_name` mappings identify only `emulator-5590` as `litterbox-native-proof`; host process 3352649 independently names that AVD at port 5590. All targeted commands use this serial and NATIVE/android-user. No phone/TV command, Git mutation, real-mail operation, or install was issued.

## Actual pixel discriminator

Artifacts: NATIVE/repair-native-20261019/{screenshots,logs/commands.jsonl}; earlier before/after Back captures are NATIVE/release-kit-20261019/screenshots/repair-{before,after}-back.png.

1. Android Back alone did not clear the invisible scrim, including the settled frame `00-back-settled.png`.
2. First physical tap on the visible first-card More actions landmark `(913,460)` cleared the invisible scrim without opening a sheet (`01-more-actions.png`). No card operation occurred.
3. Second tap on exactly that visible landmark opened the actual painted action sheet (`02-more-actions-painted.png`). It shows the first card title, action rows, and Close.
4. Physical tap `(566,1508)` is pixel-anchored to the painted `Snooze · choose when` row. No accessibility dump occurred anywhere in this chain. The sheet closed and the app remained uniformly dimmed with **no preset-menu pixels** at exactly +0.500, +2.000, +5.000 seconds (`03-snooze-0p5s.png`, `04-snooze-2s.png`, `05-snooze-5s.png`). Every ADB command exited 0; the journal records input and screenshot timestamps/elapsed times.

**Discriminator result: never painted at any sampled time**, not a stale XML-bounds tap and not a menu that persists. Instant appearance/closure inside the first 0.5 seconds is not ruled out. Source mechanism remains to be confirmed; no speculative QML edit has been made. No preset/custom OK was tapped in the real account. All seven acceptance clauses from receipts/ui-corrections-android.md remain required and their existing statuses are unchanged.

## Confirmed cause and first passing signed consumer (18:18 UTC)

The same installed v24 native Qt scene, in a copied disposable profile, reports the
directly opened Menu as `visible=true`, `opened=true`, `opacity=1`, but `width=0`,
`implicitWidth=0`, `height=303`. Its preset items have nonzero implicit widths but
actual widths zero (`NATIVE/repair-native-20261019/logs/geometry-direct-app.txt:40`).
This control does not transition from the action sheet, so a sheet-close race or
Loader/thread rewrite is not needed to explain the failure. Material derives Menu
width from its background/contentWidth; the app replaced the background with a
Rectangle without an intrinsic width.

The canonical repair is only intrinsic row sizing plus viewport bounds in
`qt/src/qml/CardActions.qml`. R20's oracle and the 0.1-38 changelog were updated.
Exact frozen modified bytes (`NATIVE/repair-native-20261019/source-manifest.json`):

- `qt/src/qml/CardActions.qml`: `91d4297a090331fe75eedf59a1e36022c544ab3c0a8296ba8cff4f459e2c2a97`
- `REQUIREMENTS.md`: `42bc6118c7b3ed47ba07116877e30e4918b91d3b42ae2e79f9087102ee78683e`
- `qt/debian/changelog`: `2e97044d767600593f5ebf14f487cd0f03b72fee3d432fc441201884edc02d11`

Source identity is base `ee307c350f925bc3cf059d96ea8f3597141944fc` **plus those
exact source bytes**, not the unchanged old pin. Complete copied source and hashes
are retained under `NATIVE/repair-native-20261019/sources` and the manifests.

App-only cached build (Java 17, `litterbox-qt_make_apk`) and canonical redacted
signing succeeded. The first build failed from D8 native-thread exhaustion; the
same frozen build succeeded with Java active processors / Gradle workers bounded
to two. No source change was made for that resource error. Candidate:
`NATIVE/repair-native-20261019/litterbox-qt-v25-91d4297a.apk`, SHA-256
`6d82b5892f4f041c4ff227df823672c30a29fa5ff2a73b443e8ca6c5b917dc99`,
versionCode 25 / 0.1-38, canonical signer
`a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37`.
Installed only on the owned AVD at 18:12:49 UTC. Fresh authenticated intended-HTTPS
served readback at 18:06:33 UTC is still version 23, bytes matching manifest SHA
`b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70`; 25 is newer.

Fresh Android user 13 was enrolled through the normal app form into the private
real-server fixture at `http://127.0.0.1:18129`, via the owned ADB reverse, with no
pre-enrolled profile or LB_SERVER. `24-fixture-enrollment-result.png` shows success;
`26-fixture-enrolled-inbox.png` shows only the six synthetic mails. No real mail
operations are involved. The separate intended-HTTPS normal-UI enrollment remains
required, not replaced by this fixture route.

Actual native sequence at 412×915, 440 dpi, 150% font: pixel More on the synthetic
preset card → painted action sheet (`27`) → physical Snooze row press → menu painted
at +0.500/+2.000/+5.000 seconds (`28`/`29`/`30`), all labels fully in the viewport.
Physical In one hour press → target disappears (`31`/`32`). Inputs are ordinary
120 ms stationary finger presses, not QML method calls; zero accessibility dumps
were used in this consumer sequence. The earlier zero-duration synthetic tap did
not activate the header-menu row, while a 120 ms press at the identical pixel did.

Private PostgreSQL readback `logs/preset-after.json` (exit 0, 18:18:26 UTC) proves
card `c9300000-0000-4000-8000-000000000103` is `snoozed`, with
`snooze_until=2026-10-19T23:17:34.186+04:00` and durable physical-input operation
`3173b185-e6ce-44bb-8f2b-275b7d7bf321` payload
`{"type":"snooze","args":{"until":"2026-10-19T19:17:34.186Z"}}`.
That is the painted `19 Oct 23:17` instant. This closes the formerly unproved
preset-consumer/payload clause on these exact signed bytes.

This is **not yet a release pass**: intended-route fresh enrollment, continuous
archive A → actual B body → Undo keep B, custom physical OK/Cancel, member/shared
avatar recapture, and neighboring-width / actual 48 dp measurements remain open.
The shared QML change invalidates the old desktop cohort; separate Git/publication
owners must use the new source/artifact cohort. No Git mutation or publication
was performed here.

## v26 date-field pass and confirmed native reader cause (18:44 UTC)

The separate Git owner committed/published only the frozen menu-repair bytes as
`7bbfab265327d918ac68a88d630316503299f6e7` and released its writer gate. No Git
mutation was issued by this native owner. v25 remains archived as an intermediate,
not a release pass.

The newly reachable custom picker on v25 exposed an additional real phone failure:
`37-custom-412-keyboard-hidden.png` paints only the date steppers, no date values.
Actual native geometry (`logs/geometry-picker-v25-app.txt:43`) gives Year width 93
vs intrinsic 148, Month/Day widths 89 vs intrinsic 140, and all three content widths
zero; Material needs 54+54 dp for the two indicators. A two-column labelled
Year/Month/Day grid fixes this without changing the Time row or its 54 dp controls.

Signed v26/0.1-39 is retained under `NATIVE/repair-native-20261019/v26`, APK SHA
`6434ff1b06051e0205a6c5083a5fad2c3109700619b3976d0adb695759e266ea`.
Installed-base APK SHA was read from the owned AVD and matches. Fresh intended-HTTPS
served readback at 18:29:12 UTC is still version 23 with matching served bytes.
Source is settled menu commit `7bbfab2` plus exact v26 manifest deltas, not the old
unchanged source pin.

Actual v26 consumer results, all on user 13's private real-server fixture:

- `46`/`47`: all date values paint at 412×915, 150%; physical Day+ changes the
  displayed target to 20 Oct. `48` shows the same readable picker at 480×915.
- Physical Cancel (`49`) leaves exactly the original one operation, card 104
  open and `snooze_until=null` (`logs/cancel-v26.json`).
- Physical future Day+ and OK (`53`→`54`/`55`) persists card 104 at
  `2026-10-20T19:31:00.000Z`, matching painted `20 Oct 23:31`; operation
  `534521e7-c332-4c46-b521-c6b4f272c83a` (`logs/custom-ok-v26.json`).
- Fresh synthetic target 107 was added without altering any v25 record. Physical
  More→Snooze paints at +0.5/+2/+5 (`57`→`58`/`59`/`60`); physical hour preset
  (`61`/`62`) persists `2026-10-19T19:32:24.936Z`, matching `19 Oct 23:32`, operation
  `0df0615f-990b-4384-b556-c3a5ac30a5c1` (`logs/preset-v26.json`).
- `63`/`64` are actual expanded member-avatar captures at 480/412: shared AL once,
  Ada AL purple and differing Grace GH blue beside their displayed identities.

The still-required continuous reader clause exposed another confirmed defect:
actual labelled reader More→Archive A presents B's real original HTML but leaves
the native body halfway offscreen (`73`); Undo is absent there. Android Back exposes
the inbox Undo (`74`), which is not the required keep-B journey. The first control
mistook Open in Gmail for Archive; it entered Chrome's first-run screen and did no
mail action (`68`), then its pixel guard aborted instead of pressing an absent bar.
Only the labelled Archive row in `71`/`72` is the actual archive input.

The private queued-control assumption also needed correction: removing an ADB
reverse mapping does not close an already established TCP connection. Archive
`00619b4f-2b96-45de-9ef5-65316a143e80` therefore ACKed in this disposable fixture;
its physical inbox Undo then got HTTP 400 from the standalone fixture's unregistered
`gmail.label_add`, not a claimed success (`75`, `logs/archive-control-v26.json`).
No real mail was touched. The final queued proof must remove the reverse and cold
restart the private app before archiving, so no old socket can transmit the op.

Bounded geometry control uses only a copied user-13 synthetic cache in a separate
user-0 `repair-reader-geometry` sandbox, fake unreachable profile URL, and no real
profile. `v26/logs/reader-geometry-complete-app.txt:61,65,66` proves:

- Before Archive A: retained QML page x=0, width=412.
- After Archive and after settling: B selected, page x=206, width=412, visible;
  StackView x=0, width=412, depth=2, busy=false; native body scene x=206.
- Undo bar exists with 388×48 dp bounds but `visible=false` at depth 2.

The native body is following the displaced QML ancestor correctly. No native C++
placement patch is justified or made. The canonical cause is `closeReader()`'s
animated pop followed immediately by re-pushing the same retained page for B;
the old pop animation finishes at half the page width. The narrow fix completes
the pop immediately before reuse. The existing Undo visibility rule must also
include the active phone reader; its actual height plus bottom margin is reserved
inside the Android page so the native WebView cannot cover the Qt Undo target.
The inbox itself is not reflowed. Main granted this exact canonical owner extension.

Current candidate work is v27/0.1-40, same menu/date fixes plus only the confirmed
`qt/qml/InboxView.qml` reader/Undo changes and requirements/changelog updates.
`qt/src/androidmailview.cpp` is unchanged. v26 is preserved as the failing-before
reader baseline. **No full release pass yet:** fresh intended-route normal-UI
enrollment, one continuous native Archive→B body→Undo keep B, final-candidate
preset/custom/avatar recapture and actual-density target measurements remain
required. All seven acceptance clauses remain intact; no narrowed replacement.

## v27 installed consumer matrix (19:09 UTC; target geometry in progress)

Final frozen candidate is `P/v27/litterbox-qt-v27-b984460a.apk`, v27/0.1-40,
SHA `83ad492b9f3bbb8841a647cb78a304432622aa212b3b056665475fa94d9d8e54`,
where P is `NATIVE/repair-native-20261019`. App-only configure/build/sign all exited
0. Two fresh AVD mappings at 18:49:27 and 18:49:31 precede install at 18:49:38;
installed-base SHA exactly matches, and installed package reports 27/0.1-40.
`P/v27/logs/served-before.json` at 19:06:19 UTC independently reads intended HTTPS
served version 23 and its matching APK bytes; final candidate 27 is strictly newer.

Observed full-matrix inventory on these unchanged final bytes:

1. Exact newer signed install: PASS, `v27/logs/installed-{package,apk-sha256}.txt`.
2. Fresh normal-UI intended HTTPS enrollment: PASS, newly created full user **12**
   (not retained user 0/13, not the private fixture). `116`/`117`/`118` show the
   fresh no-token route. `121` shows exact `https://komzpa.net:9443` and device name
   before invite entry; invite entry is redacted and no filled-secret frame was
   captured. Physical Enroll at 19:05:29, normal Back-to-inbox, then real intended
   mailbox cards load (`124`) with no offline banner. Settled `127` explicitly shows
   the same HTTPS origin, Enrolled, `NativeRepair-v27-fresh`, device ID
   `5749f9b0-345a-43d5-a54e-3a5010efef15`, and live connected accounts. No real mail
   action was issued. An immediate capture (`125`) was a transient black frame,
   not accepted as proof; `127` is the actual settled settings screen.
3. Displayed avatars: PASS, actual expanded shared/member captures `112` at 480
   and `114` at 412. Shared AL once, Ada AL purple and differing Grace GH blue,
   plus standalone sender avatars. `113` still had the old 1320 px surface during
   resize and is explicitly not the narrow-width evidence; `114` is 1133 px.
4. One continuous actual Archive A→real B HTML→physical Undo→keep B: PASS,
   `archive-final-v27.py` and `v27/logs/archive-physical-chain.json`, `88`→`89`→`90`→`91`.
   Reverse was removed AND user-13 app cold restarted before A, so no old socket
   could ACK. Real B body captured at +2.278 s; painted Undo at physical
   (1000,2263) pressed at +2.372 s; B still selected with full original HTML at
   +9.240 s. No Back at any point in this chain. Local copied DB has outbox=0,
   A position 2 and B position 3 restored. Private PG remains at the original four
   ops with only the old v26 archive; A/B open. This proves queued cancellation,
   not the standalone fixture's unsupported ACKed inverse. Explicit fixture
   preconditions (reset A and make A-before-B creation times unambiguous) are
   separately journalled, never represented as Undo.
5. Physical preset and persisted exact until: PASS, `93`→`94`/`95`/`96` at +0.5/2/5 s,
   physical hour preset `97`, `logs/preset-v27.json`. Card 108 snoozed until
   `2026-10-19T19:55:57.702Z`, matching displayed 19 Oct 23:55; op
   `9783fc50-efec-4a59-84a7-3a9903f9f638`.
6. Physical custom OK and actual physical Cancel no-op: PASS. `99` has all date
   values at 412. `107` shows changed future 20 Oct 23:57 at 480 with the keyboard
   hidden and painted Cancel; physical (1029,1730) Cancel `108` leaves five ops,
   card 109 open and null until (`logs/actual-cancel-v27.json`). `109` then shows
   changed future 20 Oct 23:58 with painted OK; physical (710,1730) OK `110`/`111`
   persists `2026-10-20T19:58:00.000Z`, op
   `e9639e92-b339-43bd-975a-3648a7d461fc` (`logs/custom-ok-v27.json`). Earlier
   `100`/`101` were an IME-obscured attempted press and are NOT counted as Cancel;
   the actual labelled, unobscured press was repeated and separately proved.
7. Both actual widths/font and no clipping are painted in the final captures;
   live native Qt target geometry (not source constants or accessibility ghosts)
   is the remaining acceptance evidence. `geometry-final-v27.py` is collecting
   More, every visible preset/custom row, every date/time stepper, footer and
   active-reader Undo at both widths in a copied synthetic sandbox. Density is
   440 dpi (2.75 px/dp), 1133/1320×2516 px, and users 0/12/13 all have font scale
   1.5. No current product failure in the six exercised clauses; no new build is
   started and no C++ change is made. This section is not yet the final seven-clause pass.

The fresh full-user harness hit the AVD's four-user cap without touching preserved
users. A transient owned-AVD `fw.max_users=8` allowed fresh user 12; the original
empty property will be restored after the remaining measurements. Normal user-0
enrollment remains untouched; all diagnostic launches use copied synthetic caches
and an unreachable fake profile URL. All source bytes remain frozen; only evidence
files change. Final clause-mapped receipt is still to be replaced after geometry.

## Final v27 geometry and native acceptance (19:21 UTC)

The unread exit-1 result was inspected first, not replaced by a speculative build.
Original `P/v27/logs/geometry-matrix.json` has five missing phases at 412 and an
invisible Undo at 480 despite correctly sized menu/picker controls and B at x=0.
The 480 harness called `openPage` without `requestArchiveUndo`; the canonical
`openReaderEntry` supplies that callback. Missing evals at 412 are a capture
failure, not evidence of a product geometry defect. Both original records remain.

`python3 P/finish-geometry-v27.py` completed exit 0 at 19:19:56 UTC using the real
inbox opener and copied synthetic storage. `P/check-geometry-v27.py` passed both
widths, last checked 19:21:09 UTC; its negative-control input from the original
matrix correctly exits 1. No product source, C++ code, APK or build identity changed.
More is 48×48 dp, every visible preset/custom row 327.640625×54, all ten spinner
indicators 54×54, OK/Cancel 108×54, and visible reader Undo 64×48. Date content widths
are 175/187 dp; time value widths 30/36 dp at 412/480. B is x=0 with 72 dp reserved
for the actual 56 dp Undo bar plus 16 dp margin. Body bottom equals Undo-bar top
(767/766 dp), with no overlap. Actual-density 440 dpi and font_scale=1.5 readbacks,
per-control scene geometry, all labels and both painted native consumer widths
are under `P/v27/{logs/finish-geometry-*,finish-captures-*}` and screenshots `128-*`.

The six other clauses were retained only after reading their actual signed-v27
screenshots, physical command journal, SQLite and private-PG evidence. The final
receipt maps all seven individually, including fresh intended-HTTPS enrollment
(user 12; actual loaded mailbox `124`, settled identity/accounts `127`, rejected
black `125`), shared/differing avatars, continuous physical A→B-body→Undo keep B,
the physical preset payload, and unobscured physical Cancel/OK persistence.

At 19:20:17 UTC, `finish-native-state.py` restored `fw.max_users` to the exact
original empty property saved in `P/logs/commands.jsonl:340`; it did not write 4.
Readback is empty and default supported-users count is 4. Users 0/10/11/12/13 remain;
foreground user 13, 1133×2516 display, 440 dpi and all established fonts are preserved.
Fresh installed APK SHA is still
`83ad492b9f3bbb8841a647cb78a304432622aa212b3b056665475fa94d9d8e54`.
All 172 frozen source files match; base remains `7bbfab265327d918ac68a88d630316503299f6e7`
plus the exact four-path delta in manifest
`ea73c4d633a1783293958b89163db85d0436e7036660d3857b9462350446a03a`.
Artifact manifest stays `345d07e31828374f1d43fb43464896186d7fa2302d1d572cd4fe1936d144d514`.
`P/v27/final-native-identity.json` binds the final installed SHA, canonical signer,
source hashes, passing geometry and fresh intended-HTTPS served version 23 readback.

Native work has no remaining acceptance gap. Git/docs integration, new-cohort
desktop rebuild/proof and publication remain with the separate owners; none was
dispatched or performed here. The existing required incident retro was read in
full and its **no instruction change warranted** conclusion is preserved.

