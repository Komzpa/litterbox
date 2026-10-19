# ui-corrections-android — signed v27 native acceptance

**PASS: all seven assigned native clauses on the exact signed v27 / 0.1-40 APK.**
The final geometry check passes at both requested widths. This is native acceptance,
not a claim that Git handoff, desktop re-proof, or publication has happened.

Owners: RepairNativeSnoozeConsumer (canonical repairs, build/sign/install and six
physical consumer clauses); FinishNativeGeometryProof (inspect failed geometry,
complete clause 7, restore transient state, and produce this final receipt).
Final installed/source identity readback: **2026-10-19 19:20:19 UTC**.
Final geometry checker: **19:21:09 UTC**, exit 0.

Paths used throughout:

- `ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`
- `NATIVE=/home/kom/proj/ai_pr/litterbox-native-proof-20261018`
- `P=NATIVE/repair-native-20261019`

## 1. Frozen source and exact signed/installed artifact

The source identity is the settled menu-repair commit
`7bbfab265327d918ac68a88d630316503299f6e7` **plus these four exact modified files**.
It is not the unchanged commit alone. No product source was edited by this finisher.
All **172** files in the frozen source manifest still match ROOT at final readback.

| Modified path | SHA-256 |
|---|---|
| `qt/src/qml/CardActions.qml` | `74876909774938398d3907fffb6cb9374b80344ddd9f00d242e8d730aa0a6082` |
| `qt/qml/InboxView.qml` | `b984460a722910fc5eafe28f562125af11bbc9b560c62d781391405af94d4c6f` |
| `REQUIREMENTS.md` | `a5a71589429b0eed3e77ce75667adedcf289e90ebbea464643842c69a9c39866` |
| `qt/debian/changelog` | `3b5838eac3114bdb21bff6a0ffc8d6bf642655e8e1dc5b6bbf5d25870f2f9527` |

| Artifact fact | Observed value / evidence |
|---|---|
| APK | `P/v27/litterbox-qt-v27-b984460a.apk` |
| APK SHA-256 | `83ad492b9f3bbb8841a647cb78a304432622aa212b3b056665475fa94d9d8e54` |
| Package / versionCode / versionName | `org.qtproject.example.litterbox_qt` / `27` / `0.1-40` |
| Canonical signer SHA-256 | `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37` |
| Frozen source manifest | `P/v27/source-manifest.json`, SHA `ea73c4d633a1783293958b89163db85d0436e7036660d3857b9462350446a03a` |
| Frozen artifact manifest | `P/v27/artifact-manifest.json`, SHA `345d07e31828374f1d43fb43464896186d7fa2302d1d572cd4fe1936d144d514` |
| Installed bytes | Fresh 19:20:18 owned-AVD `pm path` + `sha256sum` exactly match the APK SHA; `v27/logs/installed-final-{package,sha256}.txt` |
| Signing/package verification | Fresh canonical `apksigner verify --print-certs` and `aapt dump badging`, both exit 0; `v27/logs/final-{apksigner,aapt}.txt` |
| Fresh intended HTTPS served readback | `https://komzpa.net:9443`, 19:17:13 UTC: version **23**, actual downloaded bytes SHA `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70`, exactly matching its manifest; `v27/logs/served-final.json` |

Candidate **27 > served 23**. The predecessor's v27 build used the existing Android
cache, Java 17, bounded two-worker execution, only `litterbox-qt_make_apk`, and the
canonical redacted signer. Configure, build and signing each exited 0, recorded in
`P/v27/logs/build-commands.jsonl`. This finisher did not rebuild or reinstall merely
to repair a proof harness. `P/v27/final-native-identity.json` binds the final source,
APK, installed bytes, signing identity, served readback and geometry result.

## 2. Ownership and safety

The sole mutation target was `adb -s emulator-5590`, AVD
`litterbox-native-proof`, with `ANDROID_USER_HOME=NATIVE/android-user`.
Two fresh ADB listings + guest AVD-name queries and an independent host `/proc`
command-line mapping preceded each completed geometry run. The final host mapping
is PID **1908686**, `qemu-system-x86_64-headless -avd litterbox-native-proof ...
-port 5590`; the old PID 3352649 was not assumed to be current.
`P/v27/logs/finish-mappings.json` and `final-device-state.json` retain these facts.

No command targeted phone `R5CX80MZ1SN`, TV `192.168.100.39:5555`, the owner's
 desktop bus, or the shared installer. No Git metadata operation, commit, push,
publication, or production mail-changing operation was performed in this lane.
The TV appears only in device listings; it was never a target.

Mail-changing consumer proof used the disposable **Android user 13** and the
private real-server/PostgreSQL fixture `litterbox_test_native_repair_20261019` at
localhost:5434, reached through the owned ADB reverse on port 18129. Geometry used
only a separately copied synthetic cache under a user-0 diagnostic sandbox with
an unreachable fake profile URL on port 18130. Normal user-0 enrollment/storage
was not replaced. Fresh intended-HTTPS enrollment used newly created full
**user 12**, not a copied profile and not the private fixture. Credential entry
was redacted; no filled-secret frame or secret value is included in the receipt.

## 3. Seven-clause acceptance map

Screenshot numbers below refer to the actual PNG contents in `P/screenshots/`,
not to filename claims. The six physical-consumer clauses were inspected from
the predecessor's images, command journal and persistent readbacks; they were
not silently replaced by scripted geometry.

| # | Required clause | Result and actual evidence |
|---|---|---|
| 1 | Exact newer, canonically signed APK installed | **PASS.** Section 1 pins the exact APK and all source deltas. Fresh installed-base SHA, package/version and canonical certificate match. The authenticated served manifest and separately downloaded APK remain version 23; candidate 27 is newer. |
| 2 | Fresh normal-UI enrollment on intended HTTPS route reaches Online | **PASS.** Journal records fresh full-user 12 creation, `install-existing`, then an ordinary launch without `LB_SERVER`, capture arguments, token injection or copied profile. `116` shows the no-token state; `117`/`118` open the normal enrollment form; `121` shows exact `https://komzpa.net:9443` and `NativeRepair-v27-fresh` before secret entry. Redacted invite entry and physical Enroll occur at 19:05:29 UTC. `124` actually shows loaded intended-mailbox cards with no offline banner. Settled `127` shows the same HTTPS URL, **Enrolled**, the intended device name, device ID `5749f9b0-345a-43d5-a54e-3a5010efef15`, and live connected accounts. R55 makes Online silent (`connectionStatusRow.visible = !store.online`), so absence of an Online badge is intentional. `123` is only an initial caught-up frame; `125` is a transient black frame and is not accepted as enrollment identity proof. |
| 3 | Displayed sender/member avatars; differing members; shared identity once | **PASS.** Actual expanded captures `112` (1320 px / 480 dp) and settled `114` (1133 px / 412 dp) show shared AL once, Ada Lovelace AL purple and differing Grace Hopper GH blue beside their displayed identities, plus standalone avatars. Resize frame `113` still had the old 1320 px surface and is not counted as narrow-width proof. |
| 4 | One continuous Archive A → actual B body → physical Undo → keep B | **PASS.** `archive-final-v27.py`, `v27/logs/archive-physical-chain.json`, and `88` → `89` → `90` → `91` are one user-13 chain with **no Back**. Physical labelled Archive `(566,770)` opens B with its actual original HTML; the native body and painted Undo are captured at +2.278 s. Physical Undo `(1000,2263)` at +2.372 s restores A without changing the selected B; B's formatted original HTML remains at +9.240 s. Copied `v27/after-archive-undo.sqlite` has outbox **0**, restored A position 2 and B position 3. Private PG still has only its original four operations, including the old v26 archive; A/B remain open. This proves queued cancellation, not an unsupported acknowledged inverse. |
| 5 | Physical Snooze preset persists the displayed until payload | **PASS.** Physical More → labelled Snooze produces painted presets at +0.5/+2/+5 s (`93` → `94`/`95`/`96`). Physical hour preset `(630,625)` removes synthetic card 108 (`97`). Private PG `logs/preset-v27.json` records op `9783fc50-efec-4a59-84a7-3a9903f9f638`, payload `{"type":"snooze","args":{"until":"2026-10-19T19:55:57.702Z"}}`, and card state snoozed with the same instant. It matches the painted local **19 Oct 23:55** target, including the persisted milliseconds in UTC. |
| 6 | Physical custom OK commits; physical Cancel is a no-op | **PASS.** `99` shows readable date values at 412. Actual unobscured future picker `107` at 480 shows **20 Oct 23:57** and labelled Cancel; physical `(1029,1730)` → `108` leaves exactly five operations, card 109 open and `snooze_until=null` (`logs/actual-cancel-v27.json`). Then `109` shows future **20 Oct 23:58** with labelled OK; physical `(710,1730)` → `110`/`111` persists `2026-10-20T19:58:00.000Z`, op `e9639e92-b339-43bd-975a-3648a7d461fc` (`logs/custom-ok-v27.json`). The earlier IME-obscured attempted press `100`/`101` is explicitly not counted as Cancel. |
| 7 | 412×915 and 480×915; 150% font; actual-density ≥48 dp targets; visible labels/no overlap | **PASS.** Complete native Qt measurements in `v27/logs/finish-geometry-matrix.json`, accepted with no errors at either width by `geometry-acceptance.json`. Fresh device density is 440 dpi (2.75 px/dp); display sizes are 1133×2516 and 1320×2516 px, approximately 412×915 and 480×915 dp. Platform font scale is actually 1.5, not a source assumption. Section 4 records all targets and body/Undo separation. Final menu/custom captures and actual native B screenshots were inspected. |

For clause 4, removing the reverse alone had previously left an established TCP
socket alive. The final physical chain removed the reverse **and cold-restarted
the private app before Archive**, so no old socket could transmit/ACK the action.
Fixture resets and explicit A-before-B creation-order setup are separately
journalled as test preconditions, never presented as Undo's effect. The standalone
fixture's unsupported acknowledged `gmail.label_add` control was not counted as a
pass. No real mail was used for these operations.

## 4. Final native geometry, not source constants or accessibility ghosts

The final run completed all 18 recorded phases at each width, including the
canonical inbox opener with its actual Archive Undo callback. Its current
synthetic original HTML is present in both reader measurements. Geometry is
introspection on the installed signed APK; the independently preserved physical
chain supplies consumer/input proof. No accessibility dump was used in that
physical chain or in the Snooze-consumer sequence.

All dimensions below are measured Qt device-independent units:

| Surface / target | 412 dp display | 480 dp display |
|---|---|---|
| More target | 48×48 | 48×48 |
| Preset popup | x=72.359375, y=241, 327.640625×303 | x=140.359375, y=241, 327.640625×303 |
| Each visible preset and Custom row | 327.640625×54 | 327.640625×54 |
| Custom dialog | x=16, y=130, 380×396 | x=44, y=129, 392×396 |
| Year/Month/Day field | 283×54; value region **175** wide | 295×54; value region **187** wide |
| Each date up/down indicator | 54×54 | 54×54 |
| Hour/Minute field | 138×54; value region **30** wide | 144×54; value region **36** wide |
| Each time up/down indicator | 54×54 | 54×54 |
| OK and Cancel target | each 108×54 | each 108×54 |
| Active B reader | x=0, width=412, bottomPadding=72 | x=0, width=480, bottomPadding=72 |
| Actual visible Undo bar | x=12, y=767, 388×56 | x=12, y=766, 456×56 |
| Actual visible Undo button | x=332, y=771, 64×48 | x=400, y=770, 64×48 |
| Actual native body | y=56, height=711; bottom **767** | y=56, height=710; bottom **766** |

The body ends exactly at the bar's top; it cannot cover Undo. The page reserves
the measured 56 dp bar plus its 16 dp margin, not an assumed constant bar height.
At 440 dpi, the smallest 48 dp dimension is **132 physical pixels**; 54 dp is
148.5 pixels. OS system chrome accounts for the smaller measured app-content
heights (839/838 dp); the display configuration is still the full requested phone
height, not the earlier erroneous 480-dp-height substitute.

The checker examines every visible preset/custom row, all ten date/time indicator
targets, both footer targets, More and active-reader Undo; checks popup/target
viewport bounds, label/value extents and truncation, and pairwise picker/menu
non-overlap. The conditional evening row was genuinely invisible at the sampled
late-evening time and is not falsely counted as a painted target. All visible
labels and numeric values fit. Actual painted evidence is in:

- `P/v27/finish-captures-{412,480}/{menu,custom}.png` (app-native capture; dimensions in adjacent JSON).
- `P/screenshots/128-v27-finish-reader-{412,480}.png` (full Android screencap including the native WebView, not a Qt-only substitute).
- Physical final-candidate captures `94`/`95`/`96`, `99`, `104`, `107`, `109`, `112`, `114`.

## 5. Exact failed geometry checkpoint and its resolution

The inherited run **really exited 1**. Its original `v27/logs/geometry-matrix.json`
and `artifact://2967` are preserved as failures:

1. At **412**, the log contained **no eval phases at all**: missing More, Menu,
   Date, Time/footer and reader-B. There was no measured small-target product
   failure to repair. A fixed 24-second delay had been accepted without checking
   that the capture script ran. The final cold launch waits for an explicit
   complete marker and rejects absent phases.
2. At **480**, the measured targets already met minimum size. B was x=0/w=480,
   but Undo was invisible and bottomPadding was zero. The exact old input called
   `openPage('MailDetailPage', {...})` without `requestArchiveUndo`. That callback
   is supplied by the real `openReaderEntry` path, and `MailDetailPage` only calls
   Undo when the supplied callback exists. This was a **capture-path defect**, not
   a new product regression. The final run enters through canonical
   `openReaderEntry`; the actual callback is true, Undo visible, and padding 72 at
   both widths on **unchanged APK/source bytes**.

The finisher's first richer harness attempt also exposed two harness-only errors:
Popup has no `mapToItem`, and ordinary `adb pull` cannot read app-private output.
It was corrected to map the Popup background and copy diagnostic output through
owned-AVD `su 0` readback. The next attempt showed that the capture runner's
synthetic `openCard` QMouseEvents did not activate the Android TapHandler; the null
reader was rejected, not called a product failure or a pass. The final geometry
uses the canonical opener, while retaining the separate actual physical-input
chain. Failed attempt logs are preserved as `finish-geometry-*-prior-*-app.txt`;
command failures remain in the journal and `artifact://2973` / `artifact://2975`.

Neighboring negative control: the checker was run against the **original saved
failed matrix**, without repeating a device investigation. It exits **1** and
retains all five missing 412 phases and the 480 absent-Undo failure in
`v27/logs/geometry-negative-control.json`. The final measured matrix exits **0**
with no errors at either width. No thresholds or required clauses were relaxed.

## 6. Transient-state restoration

The predecessor's actual command at **19:01:45.502 UTC** read
`getprop fw.max_users` as exactly `"\n"` (empty property). Its next override set
that property to `8`; `pm get-max-users` returning 4 was a default, not the saved
property value.

At **19:20:17 UTC**, after fresh guest/host identity checks, this finisher restored
`fw.max_users` to that exact empty value. Readback is `"\n"`, and the supported
maximum again reports 4. **No users were deleted:** 0, 10, 11, 12 and 13 all remain.
The foreground is restored to private user 13 with its ordinary app running;
display override is restored to 1133×2516, physical density remains 440, and users
0/12/13 all still read font_scale=1.5. See `P/v27/logs/final-device-state.json` and
the exact ADB journal. The user-0 diagnostic process was force-stopped.

## 7. Commands, exits and evidence scope

`P`/`NATIVE` below expand to the absolute paths above. Exact subprocess argv,
timestamps, outputs and statuses are in `P/logs/commands.jsonl`, the frozen build
journal and the named output files.

| Command | Actual exit / scope |
|---|---|
| `python3 P/build-v27.py` | **0**, predecessor: cached Java-17 app-only configure/build/canonical redacted signing; all three child exits 0. Not rerun by this finisher. |
| `python3 P/archive-final-v27.py` | **0**, predecessor: one physical A→B→Undo→keep-B chain; actual command journal and DB readbacks inspected. |
| `ps -C qemu-system-x86_64 -o pid=,args=` | **1**, unsupported ps option. `ps aux` then succeeded (**0**) and current `/proc` identity checks independently resolved the owned AVD. |
| `python3 P/finish-geometry-v27.py` | Attempts **1**, **1**, then final **0** at 19:19:56 UTC. Errors and corrections are stated in §5; final full output `artifact://2978`. No product source changes between attempts. |
| `python3 P/check-geometry-v27.py` | **0**, both widths pass; final default-path execution 19:21:09 UTC. |
| `python3 P/check-geometry-v27.py P/v27/logs/geometry-matrix.json` | **1 expected**, preserved failed-capture negative control. |
| `python3 P/read-served.py v27/logs/served-final.json` | **0**, fresh authenticated manifest plus separately downloaded intended-HTTPS APK, hash equality true. |
| `python3 P/finish-native-state.py` | **0**, all 172 frozen source-file hashes match; restored actual saved property; fresh installed package/bytes; canonical certificate and aapt each exit 0. |
| `sha256sum qt/src/qml/CardActions.qml qt/qml/InboxView.qml REQUIREMENTS.md qt/debian/changelog P/v27/source-manifest.json P/v27/artifact-manifest.json P/v27/litterbox-qt-v27-b984460a.apk` | **0**, exact hashes in §1. |

The finisher ran only scoped installed-consumer geometry, identity/readback and
negative-control checks. No broad test suite, linter, formatter or build matrix
was run. Signing/password loaders and intended-route credentials stayed in-process.

## 8. Required retrospective and handoff

The existing actual retrospective `history://ReviewNativeTapLoopIncident` was
read in full. Its conclusion remains **no instruction change warranted**: the old
78-minute guessed popup crawl, wrong viewport, filename/claim drift and unanswered
save-partial steers were execution/application failures. No new worker report or
instruction defect is invented here.

The confirmed canonical repairs are already documented in the frozen R20/R6/R70
oracle and changelog: Material Menu width zero, date SpinBox content width zero,
and animated pop/re-push of the retained reader leaving x=206. The final source
uses intrinsic menu sizing, labelled full-width date rows, immediate pop before
retained-page reuse, and actual reader Undo reservation. No C++ body-placement
change was made. The old accessibility/thread/overlay hypotheses were not promoted
to causes or patched speculatively. This finisher changed only proof/evidence and
this receipt; no additional product behavior or normative change was needed.

**Native remaining gaps: none for the seven assigned clauses on this exact
cohort.** The separate Git owner must commit the four frozen deltas and these
receipts; the desktop owner must rebuild/re-prove the new same-source cohort before
publication. This native result is not proof of those downstream activities and
must not dispatch or publish them implicitly.
