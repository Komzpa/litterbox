# Mobile native UI proof — a6cb742c freeze + actual acceptance-gap closure (2026-10-19)

Gate: MobileProof / native final consumer proof. This receipt is the durable record for source
freeze **a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75** (parent `99a0086113305af72205c92326f93d44e873bf72`,
the popup-visibility fix; a6cb742c adds the MailDetail `mailAppBar z: 1` fix plus consumer test
fixtures — HEAD delta 7 paths / +58 −7 per `receipts/repaired-consumer-proof.md` §2, and
`git diff --stat HEAD -- qt server` → 0 lines at its close, so tree == commit).

ROOT = `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`;
RUNTIME = `/home/kom/proj/ai_pr/litterbox-native-proof-20261018`. Sections 1–10 recover read-only historical freeze evidence; §11 records the later owned native proof run.

**Provenance:** the `NativeFinalConsumerProof` owner built, signed, installed, and journaled this
freeze but was cancelled (≈62 min) before writing a receipt. This receipt was recovered from
`history://NativeFinalConsumerProof` plus direct re-inspection of the archived RUNTIME artifacts
by the `NativeEvidenceFreeze` lane, which ran **no emulator, adb, service, build, install, upgrader,
Git, or source commands** — every verdict below is mapped to a file that exists on disk and was
read by this lane (or is explicitly marked carried/transcript-sourced).

**Current scope:** sections 1–10 preserve the historical recovered verdicts; **§11 is the latest
actual-consumer closure and supersedes the old touch-target and note-save gaps**. All 18 native
checklist criteria remain. This is not a declaration of full mobile/release completion: the named
R61 B2B source is now available (§11k), but its named-source native rendering was not exercised in this frozen run. Main owns the wider QA/release/Git handoff.

**Historical vs current:** `receipts/mobile-ui-proof.md` remains the historical run-3 record for
**99a0086** (18 clauses: 14 pass / 2 partial / 2 unproved). Sections 1–10 below recover the
**a6cb742c** freeze (16 pass / 1 not pass / 1 unproved), including its then-unsettled AX finding.
Section 11 proves the canonical inclusive-endpoint cause, actual >=48dp input span, real native
note Save/persistence, and generic original-message bytes/disclosures without a source or Qt patch.

## 1. Artifact identity (current release freeze candidate, a6cb742c)

| Item | Value | Verified how |
|---|---|---|
| Signed APK | `RUNTIME/artifacts/litterbox-qt-v23-a6cb742c.apk` (+ `.idsig`) | file exists (RUNTIME listing) |
| sha256 / size | `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` / 21 237 919 bytes | **owner sign step as received in the incident brief**; this lane has no shell and did not recompute the hash — stated as recorded, not re-derived |
| Source | `a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75` | owner `git rev-parse` readback in `history://NativeFinalConsumerProof`; HEAD-delta readback in `repaired-consumer-proof.md` §2 |
| package / versionCode / versionName | `org.qtproject.example.litterbox_qt` / **23** / **0.1-36** | enforced by `RUNTIME/sign-apk-redacted.py` (badging package+versionCode check at :67–70; `versionCode >= 23` gate at :32–34) |
| Signer cert SHA-256 | `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37` (CN=Android Debug, O=Android, C=US) | `sign-apk-redacted.py:24` `EXPECTED_CERT`, refuse-on-mismatch at :65 — canonical lineage, matches run-2/run-3 receipts |
| App build | **rc=0** — `RUNTIME/logs/build-app-a6cb742c.log:417–421` ends `BUILD SUCCESSFUL in 4s` / `41 actionable tasks` / `Android package built successfully in 5,143 ms` / `[100%] Built target litterbox-qt_make_apk` | read directly by this lane |
| Install / enrollment | `adb install -r` on owned AVD preserved enrollment; app reached **Online** | transcript-sourced install; **consumer evidence: `RUNTIME/screenshots/46-final-inbox.png` re-inspected by this lane — inbox with live real mail (Google/Bumble/Google Play/gody01 rows), 12:56, no re-enroll** |
| Superseded | `litterbox-qt-v23-99a00861.apk`, `litterbox-qt-v23-3eb373fe.apk`, `litterbox-qt-v23.apk` — preserved in RUNTIME, NOT this deliverable | RUNTIME listing |

Build recipe reused the proven run-3 configuration verbatim (out-of-tree
`RUNTIME/build-repaired-3eb373fe`, app target `litterbox-qt_make_apk`, Qt 6.7.3 android_arm64_v8a +
KirigamiAndroid + OpenSSL3 extra-libs, `JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64`,
`GRADLE_OPTS=-Dorg.gradle.workers.max=2`); no `qt/android-build.sh`, no version override, no
deploy/release.py, no installer. The log still prints the known-benign
`Cannot open input file … kirigami …` deployment warnings (same class as the passing run-3 build);
the build ends rc=0 (§ header lines above).

### 1a. Blob/mode equivalence per assigned path — a6cb742c → current HEAD (per Main's audit contract)

Audit identity is path-based: the receipt names freeze a6cb742c, while the branch has since moved
to **998564614812f708411542a6fff9bac8d9ce1b75** ("fix: expose bundle test window before awaiting
frames"). A production artifact proven on a6cb742c is **not** stale if every assigned path is the
same blob+mode at 9985646. Verified per path, no carryforward by receipt name:

- **Commit chain (read directly by this lane):** `.git/logs/HEAD` shows
  `… 99a0086 → a6cb742c → 9985646`, i.e. 9985646's parent is a6cb742c — one commit apart.
- **`git diff --stat a6cb742c 9985646` → 1 file changed, 6 insertions(+)** — only
  `qt/tests/tst_bundleexpand.qml` (test-only `inbox.show()` + comment), as recorded in
  `receipts/bundle-visibility-proof.md` §1 and corroborated by Main's authoritative pass. Because
  exactly one file differs, every other tree entry — including all wiring paths this receipt cites
  that are not individually tabulated below (`qt/android/src/org/litterbox/MailView.java`,
  `qt/src/androidmailview.cpp`, `qt/src/main.cpp`, `qt/pages/MailBodyNative.qml`) — has an
  **identical blob OID and mode (100644) at both commits** (same tree entry ⇒ same object).
- **Per-path `git ls-tree` at both commits** (recorded output in
  `receipts/bundle-visibility-proof.md` §1; this lane has no shell and did not re-run git — an
  AskClaude delegate attempt failed with OAuth-expired, so the sibling receipt's recorded ls-tree
  output is the cited source):

| Path | Mode @ both commits | Blob OID | Equal a6cb742c = 9985646 |
|---|---|---|---|
| `REQUIREMENTS.md` | 100644 | `fb6e01a35a1ad8fe6bf9ae569a8b949028b6720c` | **IDENTICAL** |
| `qt/pages/MailDetailPage.qml` | 100644 | `beb02ca184c989d2cf74e5c58eac71f76268370a` | **IDENTICAL** |
| `qt/qml/InboxView.qml` | 100644 | `6e601a3c99ca128235ba9fe4a46d30bdbb183aa4` | **IDENTICAL** |
| `qt/src/CardStore.cpp` | 100644 | `97bebc231b492a37f2aba968e6878263e7450f79` | **IDENTICAL** |
| `qt/src/CardStore.h` | 100644 | `b47bf3039c7136bfb66671ab4896fbe6142f6912` | **IDENTICAL** |
| `qt/src/qml/CardStoreBridge.qml` | 100644 | `bd10f85cd6e7e61a8bd9e7bf9d9b273586e1ffe4` | **IDENTICAL** |
| `qt/tests/tst_bundleexpand.qml` | 100644 | `2108b855287d7141dfd1913beaec63f4d84ac541` @9985646 vs `9f753138…` @a6cb742c | **differs — test-only, not a production path** |

- **Exact consumer coverage of the changed production file:** the one production change between
  99a0086 and a6cb742c is `MailDetailPage.qml` (appbar `z: 1`). It is covered by the realapp
  **Back/archive/Undo smoke** in `receipts/repaired-consumer-proof.md` §5/§7 (three real-click runs,
  all exit 0): `backToInbox` depth 2→1 readerOpen false; `archiveMail` depth 1, cards 14→1
  pendingOps 1; `bundleArchiveUndoButton` cards 14 pendingOps 0 has405 true — exercising
  `mailAppBar`/`backToInbox`/`archiveMail` (z:1 stacking) → InboxView closeReader/handleBack →
  CardStore outbox → undo bar (evidence: `OUT/repaired-consumer-a6cb742c` realapp
  `logs/realapp-{warm,smoke,prove}-r2.log` + `realapp/captures/`, 7 PNG+JSON). That smoke runs on
  a6cb742c blobs that are byte-identical at 9985646 (table above), so coverage carries by proven
  equivalence, not by receipt name; nothing in 9985646 alters it.
- **Distinct gaps preserved** (not covered by any carryforward): the 48dp NOT PASS and the
  native note-save UNPROVED are path-local to `InboxView.qml`'s toolbar sizing and the note dialog
  respectively and remain open at both commits — equivalence carries passes only, never masks a
  fail.

## 2. Actual command source (what is recoverable)

The owner's shell history survived only as elided transcript forms; these are the actual sources
available, quoted where the transcript or a file preserves them:

- Build (history, prefix preserved): `set -o pipefail; export JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64; export GRADLE_OPTS=-Dorg.gradle.workers.max=2; cd …` → app-target `litterbox-qt_make_apk` (full tail not recoverable; the **output** is durable in `logs/build-app-a6cb742c.log`).
- Sign (file-preserved interface): `RUNTIME/sign-apk-redacted.py <unsigned.apk> <versionCode> <output-signed.apk>`; keystore password loaded at runtime from the v6 release record, never printed; prints `signed=… package=… versionCode=…` and `certificate_sha256=…`.
- Capture (evidence-implied): `uiautomator dump` → `dumps/*.xml`; `dumpsys activity top` →
  `dumps/dumpsys-top-reader1-*.txt` and `dumps/popup{A,B,C}-dumpsys-*.txt`; `logcat` redirected to
  `logs/logcat-final-a6cb742c.txt` and `logs/logcat-popupcycle-a6cb742c.txt`; GATE markers emitted
  by the capture driver (`I/GATE … freeze=a6cb742c`) time-align every artifact (§5).
- Degrade rule applied: where the exact command line is gone, the receipt cites the durable output
  file rather than reconstructing a command this lane cannot prove.

## 3. Runtime state (historical-as-of; no live claims)

- Owned supervised AVD unit `lb-native-proof-20261018.service`, serial `emulator-5590`, was
  **restarted by the owner; MainPID 2313535 as-of the owner run (historical value from the
  incident brief)**. This lane did not probe systemctl, qemu, or adb and makes **no liveness
  claim** about current state.
- AVD is the owned disposable device (`hw.lcd.density=440`, 1080×2340, `avd/config.ini:83`,
  `avd/hardware-qemu.ini:34`). Real phone `R5CX80MZ1SN` and TV `192.168.100.39:5555` untouched.
- No card/archive/task/note mutations were performed anywhere in this run; real mail read-only.

## 4. Native body re-proof at the freeze

- `dumps/dumpsys-top-reader1-a6cb742c.txt:626` — `org.litterbox.MailView{1311a78 VFEDHVC.. …
  0,154-1081,2137}`: exactly one attached native view at the recorded rect (baseline for run 2 was 0).
- `dumps/reader1-a6cb742c.xml` — `android.webkit.WebView` node(s) present in the accessibility
  tree (two-occurrence pattern matched), subject row `Open Вы абагулілі … "iLands".` present.
- `screenshots/48-popupA-reader.png` **re-inspected by this lane** — fully rendered HTML body
  (Russian Google security mail: wrapped subject, sender line, inline logo image, rich layout).
  Production wiring (re-verified by direct reads at ROOT HEAD): `qt/src/main.cpp:123`
  `AndroidMailView::initialize()` → `qt/src/androidmailview.cpp` qmlRegisterType/JNI place (line
  refs carried from run-3 receipt; unchanged by the a6cb742c delta) → `qt/pages/MailBodyNative.qml`
  → `qt/android/src/org/litterbox/MailView.java:64–65` `loadDataWithBaseURL` → shown via
  `MailDetailPage.qml:385–392` `bodyLoader`.

**Verdict: native body render PASS at a6cb742c.** "Exact original HTML" remains visual-fidelity
only — no byte-level diff was ever run (carried residual, §8).

## 5. Popup cycle — hide/restore same document, no reload (durable, upgraded from PARTIAL)

Evidence (all finite, archived, read by this lane):

- `logs/logcat-popupcycle-a6cb742c.txt` — **non-empty** (this closes the run-3 0-byte gap):
  `I/GATE popup-cycle-start freeze=a6cb742c` @01:19:31.909, `popup-open` @01:19:38.243,
  `popup-close` @01:19:43.966 (also mirrored in `logs/logcat-final-a6cb742c.txt:4497/4825/5101`).
- Accessibility-tree WebView node count across the owner-archived uiautomator dumps:
  **popupA (reader) = 2 → popupB (sheet open) = 0 → popupC (restored) = 2** — verified by regex
  occurrence tests on `dumps/popupA-reader-a6cb742c.xml` (2, not 3),
  `dumps/popupB-sheet-a6cb742c.xml` (**0** occurrences), `dumps/popupC-restored-a6cb742c.xml` (2, not 3).
- Real view hierarchy (`dumps/popup{A,B,C}-dumpsys-a6cb742c.txt:626`): the **same** native view
  instance `org.litterbox.MailView{1311a78}` at the **identical rect `0,154-1081,2137` in all
  three states** — with popupB's flags differing (`GFEDHVC.. …I.` vs `VFEDHVC..`), i.e. the body
  was hidden, not destroyed/recreated [flag decode inferred; handle+rect identity is direct].
- Negative control: **no `loadData|reload|onPageFinished|removeView|destroy` lines from the app
  pid (4256) anywhere in the popup-cycle window** (regex over the full log → no matches).
  Caveat, stated honestly: `MailView.java` contains no `Log` statements at all, so logcat silence
  alone is weak — the proof rests on the 2→0→2 counts **plus** the unchanged view handle/rect.
- `screenshots/48-popupA-reader.png` → `49-popupB-sheet.png` (Qt action sheet over the reader:
  Archive/Mark as spam/Open in Gmail/Read cached/Note/Snooze/Pin/Close — **re-inspected**) →
  `50-popupC-restored.png`: restored reader shows the same Google security-mail document.
- Same subject row text `… "iLands".` present in popupA/B/C dumps (grep), confirming one journey.

Production wiring re-verified at HEAD: `qt/qml/InboxView.qml:448–449` `actualPopupsVisible`
counts only named popups (drawer/headerMenu/noteDialog/createDialog), `:555` binds it into the
pushed reader as `ownerPopupVisible`, `:954` Alt+Left disabled while popups open;
`qt/pages/MailDetailPage.qml:57–66` combines owner+local popup state, `:391` hides `bodyLoader`
`Qt.platform.os !== "android" || !root.actualPopupsVisible` — comment: "without unloading/resetting
its document".

**Verdict: PASS (was PARTIAL at 99a0086; both missing artifacts — non-empty cycle log and archived
WebView-count dumps — now exist).**

## 6. New hard finding — 48dp touch targets NOT PASS (no waiver, no workaround)

Measured from `dumps/inbox-final-a6cb742c.xml` (archived, read by this lane):

| Control | bounds (px) | size | dp @440dpi (÷2.75) |
|---|---|---|---|
| `content-desc="Navigation"` clickable | `[22,142][153,273]` | 131×131 px | **47.64 dp** |
| `content-desc="Search inbox"` clickable | `[663,142][794,273]` | 131×131 px | **47.64 dp** |
| `content-desc="Inbox commands"` clickable | `[927,142][1058,273]` | 131×131 px | **47.64 dp** |

Density is device-fact: `avd/config.ini:83 hw.lcd.density=440`,
`avd/hardware-qemu.ini:34`, and `dumpsys … mCurrentConfig={… 440dpi …}` in
`dumps/dumpsys-top-reader1-a6cb742c.txt`. Criterion is **>= 48 dp → NOT PASS**. No tolerance
waiver; no 49-dp workaround chosen.

Production entry: `qt/qml/InboxView.qml:616–631` — `openNavigation` and `toggleSearch`
`ToolButton`s declare `implicitWidth: 48 / implicitHeight: 48` (Qt logical units) [also
`addCardButton` in the same row]. Qt's declared 48 logical maps to 48×2.75 = **132 px** expected,
but Android reports **131 px** hit bounds — a 1 px logical-to-device rounding/scale discrepancy
between Qt's devicePixelRatio handling and Android's reported bounds (end-exclusive bounds
semantics not excluded). **The actual Android-vs-Qt rounding cause is NOT settled** by the frozen
evidence; identifying the canonical scaling cause is handed to Main/doctor as an open source slice
with the >= 48 dp invariant retained. Edge-tap probes were archived alongside:
`I/GATE hitrect-probe-x-794-200` @01:37:24.847 and `hitrect-probe-y-720-273` @01:37:33.354 in
`logs/logcat-final-a6cb742c.txt` (tap coordinates on the Search control's right/bottom edges),
with `dumps/probe-x794-*.xml`, `dumps/probe-y273-*.xml` and `screenshots/53/54` — these bound the
hit rect but do not lift the sub-48 measurement above.

## 7. Clause → production entry / wiring → real consumer proof (18 rows, a6cb742c)

Provenance key: **[fresh]** = artifact re-inspected by this lane at this freeze;
**[stale]** = proven at 99a0086, wiring unchanged by the freeze delta, not re-executed (carried
by proven per-path blob/mode equivalence a6cb742c→9985646, §1a — not by receipt name);
**[hist]** = owner-transcript evidence in `history://NativeFinalConsumerProof`.

| # | Clause | Entry + wiring (a6cb742c) | Real consumer proof | Verdict |
|---|---|---|---|---|
| 1 | Native chain renders real HTML body | `main.cpp:123` → `androidmailview.cpp` register/create/synchronize [carried] → `MailBodyNative.qml:4` → `MailView.java:64` `loadDataWithBaseURL` → `MailDetailPage.qml:385–392` bodyLoader | `dumps/dumpsys-top-reader1:626` MailView{1311a78} 0,154-1081,2137; `dumps/reader1.xml`; `screenshots/48` full HTML [fresh] | **PASS** (exact original HTML still visual-only) |
| 2 | Click older details | bundle child → `MailDetailPage` push (`InboxView.qml:555`) [carried] | run-3 `34` [stale] | **PASS (stale)** |
| 3 | Single-scroll reader | one WebView surface [carried] | run-3 `16`→`17` [stale] | **PASS (stale)** |
| 4 | Selectable code | native WebView selection [carried] | run-3 `18` [stale] | **PASS (stale)** |
| 5 | Live link | `androidmailview.cpp:16` link interceptor [carried] | run-3 `19`–`21` [stale] | **PASS (stale)** |
| 6 | Popup hide/restore SAME DOCUMENT, no reload | `InboxView.qml:448,550–555,954` + `MailDetailPage.qml:57–66,391` [fresh] | WebView a11y count 2→0→2 (popupA/B/C dumps); same MailView{1311a78} + same rect in popupA/B/C dumpsys; non-empty `logcat-popupcycle` with GATE start/open/close; no app-pid reload/destroy lines; `screenshots/48→49→50` [fresh] | **PASS** (upgraded from PARTIAL) |
| 7 | phoneBack reader→inbox + list anchor/selection | `InboxView.qml:450 handleBack`, `:993/:998/:2091` back wiring [fresh] | `I/GATE back-to-inbox smoke` @01:37:21 in `logcat-final`; `screenshots/52-inbox-after-back.png` re-inspected: inbox with previously opened row still highlighted [fresh] | **PASS** |
| 8 | Pause-resume | R69 background hide/place (`androidmailview.cpp:46–48,102`) [carried] | run-3 `35`→`36` [stale] | **PASS (stale)** |
| 9 | Navigation drawer | drawer items; `openNavigation` ToolButton `InboxView.qml:616` [carried] | run-3 `37` [stale] | **PASS (stale)** |
| 10 | Nav search | drawer "Search inbox" → search page [carried] | run-3 `41` [stale] | **PASS (stale)** |
| 11 | **48dp touch targets (>= 48 dp)** | `InboxView.qml:616–631` ToolButtons `implicitWidth/Height: 48` [fresh] | `dumps/inbox-final-a6cb742c.xml` Navigation/Search/Inbox-commands all **131×131 px @440dpi = 47.64 dp**; density from AVD config + dumpsys; edge probes archived (§6) [fresh] | **NOT PASS** — sub-48 on 3 controls; cause not settled; no waiver/no 49 workaround |
| 12 | 150% font + long subject, code intact | Kirigami font scaling, system `font_scale 1.5` [carried] | run-3 `42`,`43` [stale] | **PASS (stale)** |
| 13 | Overflow menu (reader) | reader header ⋮ action sheet [carried] | run-3 `28`; sheet also open at freeze in `screenshots/49` [fresh visual, same surface] | **PASS** |
| 14 | Note dialog popup hide/restore | `noteDialog` in `actualPopupsVisible` (`InboxView.qml:449`) [fresh wiring] | run-3 `30` open/cancel [stale] | **PASS (stale)** |
| 15 | **Note save/mutation via native UI** | note Save path (`InboxView.qml` noteDialog) | never exercised; no isolated-synthetic-tenant enrollment was performed in this run | **UNPROVED** — fixture work open (§8) |
| 16 | Offline restart + list anchor | offline banner + cached list [carried] | run-3 `44`,`45` [stale] | **PASS (stale)** |
| 17 | UI enrollment → Online `http://192.168.100.74:8081` | normal UI enroll, preserved by `install -r` | `screenshots/46-final-inbox.png` live inbox at freeze [fresh] | **PASS** (carried enroll, fresh Online) |
| 18 | Screenshots + logcat of the journey (durable archive) | capture driver GATE markers | `screenshots/46–54` (9 files), `dumps/` (11 files), `logs/logcat-final` + **non-empty** `logs/logcat-popupcycle`, `logs/build-app-a6cb742c.log` [fresh]; both run-3 archive gaps for this surface (0-byte cycle log, unarchived uiautomator XMLs) now closed | **PASS** (for this freeze's executed surface) |

**Counts: 18 rows — 16 PASS (5 fresh + 11 stale/carried), 1 NOT PASS (48dp), 1 UNPROVED (note
save).** Versus the frozen 99a0086 record (14/2/2): both PARTIAL rows (popup no-reload, archive
completeness) are now durable PASS; the 48dp borderline-UNPROVED is now a concrete **NOT PASS**;
note-save remains UNPROVED. Launch-time QML warnings persist at the freeze (`logcat-final`:
`QObject: Cannot create children for a parent that is in a different thread` +
`QML … Accessible must be attached to an Item` across InboxView/MailDetailPage/CardActions —
e.g. lines 2204–2474, 3519–3553) — open hygiene defect, not a consumer-visible failure.

## 8. Residuals / what is left (no criterion dropped)

1. **48dp cause slice** (§6): Qt logical-48 vs Android-reported 131 px at 440 dpi — needs the
   canonical scaling-cause identification (devicePixelRatio rounding vs Android end-exclusive
   bounds), >= 48 dp invariant retained, no waiver. Owner: Main/doctor source-scaling slice.
2. **Native note-save on an isolated synthetic tenant** (row 15): the server-side note pipeline is
   proven (`receipts/repaired-consumer-proof.md` §4 —
   `env -u DATABASE_URL pg_virtualenv … litterbox_test_note_a6cb742c; CARD_TEST_POSTGRES=1 …
   go test -run '^TestPostgresNotePersistence$'` → exit 0 PASS), but **that is server SSE proof,
   not native-UI proof** and is not counted here. The missing prerequisite is a disposable native
   enrollment context (separate app-data / isolated tenant on the known pg_virtualenv setup) so
   the real UI can save+persist a note without touching the enrolled owner profile. Owner:
   Main/doctor fixture slice.
3. Exact original-HTML byte-level fidelity diff (row 1 sub-clause) — visual only, never run.
4. QML Accessible/cross-thread launch warnings (§7 tail) — hygiene.
5. Stale rows (2–5, 8–10, 12, 14, 16) rest on run-3 evidence; wiring is unchanged by the freeze
   delta, but any future re-freeze that touches those surfaces must re-execute them.

## 9. Retro (required; received incident brief)

1. **A cancelled owner spent ~62 min with no durable receipt — again.** The build, sign, install,
   journey, popup-cycle, probes, and screenshots all landed in RUNTIME, but the ROOT receipt did
   not exist, which forced this recovery lane. Repeat of run 3's lesson: write the receipt
   immediately after install/first proof, then extend. This recovery recovered the useful results
   (artifact identity, body re-proof, durable popup 2→0→2, Online, and the new 48dp NOT PASS
   finding) from files rather than transcript claims.
2. **Archive-as-you-go worked.** This freeze's popup-cycle log is non-empty and uiautomator XMLs
   sit next to screenshots — exactly the two run-3 gaps — so a PARTIAL clause could be honestly
   upgraded without re-running anything.
3. **The 48dp criterion finally got durable per-control evidence** and it is a fail, not a pass:
   recording the borderline range in run 3 without archiving the XMLs produced no actionable
   verdict; the archived dump now pins three exact controls at 47.64 dp for the source slice.
4. **Honest degradation held:** hash/size are recorded-not-recomputed (no shell in this lane),
   elided commands are cited as elided, carried rows are labeled stale rather than silently
   re-passed, and the logcat negative control is reported with its MailView-logging caveat.
5. **Kept:** no emulator/adb/service/build/install/Git/source commands by this lane; no device
   reset; no card/note/archive mutation; criteria and docs intact; authorized Git/docs handoff
   untouched (Main owns).

## 10. Artifact index (frozen evidence this receipt maps to)

- APK: `RUNTIME/artifacts/litterbox-qt-v23-a6cb742c.apk` (+ `.idsig`)
- Build: `RUNTIME/logs/build-app-a6cb742c.log` (rc 0)
- Logs: `RUNTIME/logs/logcat-final-a6cb742c.txt`, `RUNTIME/logs/logcat-popupcycle-a6cb742c.txt`
- Dumps (11): `inbox-final`, `reader1`, `dumpsys-top-reader1`, `popupA-reader`, `popupA-dumpsys`,
  `popupB-sheet`, `popupB-dumpsys`, `popupC-restored`, `popupC-dumpsys`, `probe-x794`, `probe-y273`
  (all `-a6cb742c.*`)
- Screenshots (9): `46-final-inbox`, `47-final-reader`, `48-popupA-reader`, `49-popupB-sheet`,
  `50-popupC-restored`, `51-appbar-over-scrolled-body`, `52-inbox-after-back`, `53-probe-x794`,
  `54-probe-y273`
- Historical: `ROOT/receipts/mobile-ui-proof.md` (99a0086, 18 clauses 14/2/2),
  `ROOT/receipts/repaired-consumer-proof.md` (HEAD delta + server note SSE PASS §4),
  `RUNTIME/receipt/mobile-ui-proof-run2-3eb373fe.md`, `RUNTIME/receipt/mobile-ui-artifactproof-2026-10-18-v1.md`
- Source-of-record transcript: `history://NativeFinalConsumerProof` (cancelled owner)

## 11. Native acceptance-gap closure run (NativeTouchRepairStrong, 2026-10-19)

This section supplements the historical freeze above; it does not erase any of the 18 criteria.
ROOT remains `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`; all new runtime evidence is
under `RUNTIME/native-touch-note` (abbreviated **NATIVE** below). Neither owned QML file, Qt itself,
the APK, nor any other production source has been modified by this lane. No build, formatter,
project test, Git command, shared installer, upgrader, push, or deployment was run.

### 11a. Frozen cause: Qt inclusive endpoints exported as Android exclusive endpoints

The exact Qt **v6.7.3** canonical source is archived at
`NATIVE/sources/androidjniaccessibility-v6.7.3.cpp`, SHA-256
`873f7fc9a4a3ecf4f12814c0c4c490bc0ad8723f040d53f4b23552e47d259260`, fetched from
`https://raw.githubusercontent.com/qt/qtbase/v6.7.3/src/plugins/platforms/android/androidjniaccessibility.cpp`.
Lines 211–236 convert the logical accessibility QRect with `QHighDpi::toNativePixels`, then call
Android's `Rect(int,int,int,int)` constructor with Qt's **inclusive** `rect.right()` /
`rect.bottom()` values. Android Rect right/bottom are **exclusive**; this loses one reported pixel
on each dimension without changing the underlying QQuickItem pointer target. The Java consumer
does not repair it: `QtAccessibilityDelegate-v6.7.3.java:414–422` transfers that Rect directly to
the node (`setBoundsInScreen`, `setBoundsInParent`). That exact source is archived alongside it,
SHA-256 `57b2c56a54a5232ee3659356bc047046b8d8dbfbf52e421b5790c647e22a72a9`.

The installed Qt Android Java archive has SHA-256
`d5647a35042ec89abbcaf48387547e32a8fec4828434e995b8ff46833980d20c`; the unchanged signed APK was
re-hashed as `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` (versionCode 23,
versionName 0.1-36 confirmed from the current device package record). Actual isolated-app Android
DisplayMetrics were observed inside the app process: **density 2.75, densityDpi 440,
scaledDensity 2.75**, archived in `NATIVE/logs/webview-observer.jsonl`.

Therefore the historical `131 / 2.75 = 47.64` **AX-export measurement is real but is not itself a
measurement of the effective pointer target**. The acceptance invariant remains actual >=48dp;
no 49-unit padding, tolerance waiver, or Qt dependency patch is authorized. Actual pointer-edge
actions and immediately-outside negative controls are being recorded separately; this cause
finding alone is not a PASS.

### 11b. Isolation and preserved consumer identity

Current qemu command line and ADB serial mapping were checked together before targeted ADB:
owned AVD `litterbox-native-proof`, port 5590, serial `emulator-5590`. The user-scoped supervised
unit was failed at entry and restarted with its existing definition/data (not a duplicate AVD or
device reset). The real phone and TV received no commands. `NATIVE/logs/commands.jsonl` records
the exact targeted commands, UTC timestamps, exit statuses and outputs.

The original Android **user 0** enrollment was opened and inspected without mutations:
`screenshots/02-primary-inbox.png` shows the preserved real inbox (Google, Bumble, Google Play).
A separate Android **full secondary user 10**, `NativeNoteProof20261019`, was created; the already
installed frozen APK was made available with `cmd package install-existing --user 10`. This gives
separate Android app data without overwriting user 0 settings, token, database or enrollment.

The real-server fixture is adapted from the completed offline-live-history launcher, but uses
**port 18127**, private `pg_virtualenv` PostgreSQL cluster 18/regress at port **5434**, database
`litterbox_test_native_note_20261019`, tenant
`c9100000-0000-4000-8000-000000000019`, and sole target synthetic mail card
`92100000-0000-4000-8000-000000000001`. Its fixture, launcher and complete migration/start log are
`NATIVE/scripts/{fixture.sql,server.sh}` and `NATIVE/logs/server-launch.log`. The command begins
`env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash …/server.sh`; it never falls back
to default kom/live data. Existing stamped server SHA-256 is
`b39b02ee62fb992a01b08bcb11ae495db6495387e2b64ee7e54d17d468e49e30`.

The pre-mutation GET `/v1/cards/notes?since=2000-01-01T00:00:00Z` returned `{"notes":[]}`
(`NATIVE/logs/note-baseline.json`); snapshot and body API responses are archived before enrollment.
Native UI enrollment, actual Save, server readback and a fresh-process persistence read remain
the acceptance actions, not replaced by that server baseline.

### 11c. Invalid first probe is not product evidence

The first automated run was rejected: it tapped after a fixed one-second cold start without
checking that QML was ready. It recorded false-negative edge actions, then aborted on ADB
`error: closed`. The user-scoped service journal identifies an emulator **gfxstream host crash**
(`TextureResize` / `libgfxstream_backend.so`, SIGSEGV), followed by supervised restart. This is not
an app hit-target failure. The revised run uses one warm process, confirms the inbox accessibility
header before each probe, restores each UI state, and keeps the invalid run artifacts distinct.
The later manual warm Navigation left-edge tap `(22,207)` really opened the drawer
(`09-navigation-left-edge.png` + XML). Other edges are not implied by that one action.

### 11d. Actual effective touch target: PASS, no source change or waiver

`python3 NATIVE/scripts/boundaries.py` (revised warm-process version) exited **0**, with
**24/24** actual ADB pointer-action assertions passing. Each control had four inside-edge taps
and four immediately-outside negative controls, restoring and checking the same inbox UI before
the next case. The complete per-case coordinates, expected/observed action, PNG and XML path are
`NATIVE/logs/boundary-v2-results.json`; exact commands + rc are in `commands.jsonl`.

| Control | Actual inclusive pixel interval proved by input | Interior action | Immediately-outside negatives | Effective size |
|---|---|---|---|---|
| Navigation | x **22…153**, y **142…273** | drawer opens at (22,207), (153,207), (87,142), (87,273) | x21/x154/y141/y274 do not open drawer | **132×132 px / 2.75 = 48×48 dp** |
| Search inbox | x **663…794**, y **142…273** | search field appears at (663,207), (794,207), (728,142), (728,273) | x662/x795/y141/y274 do not open search | **132×132 px / 2.75 = 48×48 dp** |
| Inbox commands | x **927…1058**, y **142…273** | commands menu opens at (927,207), (1058,207), (992,142), (992,273) | x926/x1059/y141/y274 do not open commands | **132×132 px / 2.75 = 48×48 dp** |

Negative controls distinguish adjacent actions, not just a generic screen delta: Search's x795
opens **Create card**, not Search; Commands' x926 likewise opens Create card, not Commands.
No card was created, and the dialog was dismissed before continuing. Other outside edges leave
the unchanged inbox. This is a real consumer hit-span proof; it does not reinterpret Android
Rect subtraction as a passing AX metric. **The Qt accessibility-export off-by-one remains an
explicit dependency defect.** The app's actual R70 target meets >=48dp with its existing logical
48 dimensions, so padding it or rebuilding Qt would be the wrong cutover.

Fresh PNGs inspected directly: `09-navigation-left-edge.png` visibly shows **Litterbox / Inbox /
Accounts / Agents / Settings / Search inbox / Close navigation**; boundary-v2 Search inside-right
shows a focused **Search inbox** field and keyboard; its outside-right negative shows **Create
card / Task / Title** instead; Commands inside-bottom shows the **Accounts / Agents / Settings /
Enroll device / Refresh / Check updates / Comfortable density / Compact density** menu, whereas
outside-bottom shows the plain empty synthetic inbox with no menu. Each PNG has its XML beside
it. All measurements here use **1080×2340 physical pixels at 440dpi** (density 2.75).

### 11e. Actual native note Save + persistence: PASS on isolated tenant

The existing APK was enrolled through the normal **Enroll device → Connect to Server** UI in
Android user 10, entering `http://127.0.0.1:18127`, the synthetic fixture invite and a distinct
device name, then tapping **Enroll**. The form and successful saved-enrollment page are captured
in `11-isolated-enrollment-form`, `12-enrollment-filled`, `13-enrollment-result`; `14-synthetic-inbox`
shows only the synthetic card. ADB reverse exposes only owned port 18127 to that app context.

The real consumer journey was **open synthetic mail → reader overflow → Note → type
`native-note-proof-saved-20261019` → tap the visible Save button**. This is ordinary native pointer
and key input, not a QML property assignment, direct saveNote call or API mutation. Screenshots +
XML: `15-synthetic-native-reader`, `16-synthetic-reader-actions`, `17-native-note-dialog`,
`18-native-note-entered`, `19-native-note-saved`. The exact entered string is legible in shot 18;
the Save target `(638,1470)` was derived from that screenshot. The note editor's plain TextEdit
is accessible; the visible custom footer buttons do not appear in the current Qt AX tree, so the
Save assertion uses the painted button and observed persistence rather than pretending it has
an AX node. Slow individual key events avoid emulated-input queue reordering; no input helper
changes the app's note state directly.

Independent authenticated **read-only** server readback after Save returned:

```json
{"notes":[{"card_id":"92100000-0000-4000-8000-000000000001","title":"Native synthetic note proof","note":"native-note-proof-saved-20261019","updated_at":"2026-10-19T03:28:27.690853+04:00"}]}
```

This is `NATIVE/logs/note-after-native-save.json`, from GET
`/v1/cards/notes?since=2000-01-01T00:00:00Z` (curl rc 0), compared with the empty pre-save baseline.
The post-save snapshot and actual on-device SQLite database are preserved at
`logs/snapshot-after-native-save.json` and `dumps/cache-after.sqlite`.

For persistence, the **owned 18127 reverse transport was removed**, only **user 10's app** was
force-stopped, and the normal activity was launched as a **fresh process**. Its offline inbox
shows **Can't reach the server** and the exact saved note (`20-offline-fresh-process`). Reopening
that cached mail's Note through its real menu shows the same exact text with no server route
available (`21-offline-native-reader`, `22-offline-reader-actions`, `23-offline-note-reopened`,
`24-offline-persisted-note`). Shot 24 was opened and inspected directly: **Card note**, exact
`native-note-proof-saved-20261019`, visible **Save / Cancel**. This closes row **15**, including
actual Save, server persistence, and offline fresh-process consumer persistence. No real card
or user 0 enrollment was modified.

Production path: `InboxView.qml:requestNote/noteDialog` → explicit Save `noteDialog.accept()` →
`onAccepted: store.saveNote(cardId,noteField.text)` (:2126/:2173) → `CardStore.h:64–65` enqueues
the real note operation → `CardStore.cpp:628–632` optimistic card note + durable cards/outbox
storage → real server `/v1/ops` note operation → server notes feed/snapshot → reopen from SQLite.
Both owned QML files and CardStore.cpp/.h were freshly hashed without a Git command:
`NATIVE/logs/source-identity.json` computes their exact blob IDs from file bytes and matches all
four frozen a6cb742c OIDs in §1a. Thus the original path-local passes are carried on the **same
installed artifact and matching production bytes**, not by the name of this receipt.

### 11f. Actual original HTML consumer: generic byte-preservation mechanism PASS

The named R61 email is **not** substituted by this fixture. This is a mechanism/consumer proof
on synthetic originals. The later canonical named-source discovery is recorded separately in §11k.

The official pinned Frida 16.7.19 observer attaches only to the isolated user-10 app process.
`scripts/observe-webview.js` observes `org.litterbox.MailView.loadMail` and the real
`android.webkit.WebView.loadDataWithBaseURL`, calls the originals with the **same arguments**,
and archives the actual UTF-8 input document. It does not assign QML state, replace mail HTML,
invoke app save methods, or modify the APK. WebView debugging was enabled only in this observed
isolated process; CDP is used for read-only rendered state, never `click()`/`open=true`.

The initial native-note document is **2341 bytes**, with identical Qt and WebView argument
SHA-256 **`2c91797a878fa98923582ad3b8e12e0d186cb7d0ab24afa9dc72416ce96b7128`**:
`dumps/webview-document-01.html` and `-02.html`. `python3 scripts/compare-native.py` exited **0**:
source fixture → API → actual app SQLite → original section of the actual load argument agree;
metadata/source URL agree; a changed-original negative control is detected. Its rendered single
original/code readback is archived in `logs/native-dom.json` and `logs/native-fidelity-result.json`.
This establishes generic native byte preservation, **not named B2B identity/fidelity**.

For the requested multi-original disclosure clauses, `scripts/fixture-originals.sql` adds only two
synthetic cards to the same private tenant: A `92100000-0000-4000-8000-000000000002` (four originals)
and B `…0003` (one original). Fixture SHA-256:
`332e4fdea3248d102d70e230e547d386d97094c1285261f0447e099d7a694ab5`.
The seed script checks the owned process's exact HTTP port 18127 / PG port 5434 / private database
before inserting; no real user data or another lane's server was used.

`python3 scripts/compare-originals.py` exited **0**. `logs/native-originals-result.json` records
**5/5 exact original byte matches**, actual SQLite metadata/source URL equality, one occurrence
per original, identical Qt→WebView full document bytes, and a one-changed-byte negative control.
The comparator uses extracted byte slices of the **load argument**, not DOM `innerHTML` entity
normalization. Original API/cache/native-load SHA-256 values are equal per row:

| Original | Bytes | Equal SHA-256 at API / SQLite / actual native-load original |
|---|---:|---|
| A/m1 Ada, named MIME From | 133 | `3b18fd4e24c7c1f793f2694d71fc2d4031e04911436fafb43fe283b9eb2d2ac8` |
| A/m2 unknown legacy From | 106 | `5d2a260c3cf821579cb1275f136cf154c5e33906c2fd0fc4fc56331b07c2dc4a` |
| A/m3 address-only From | 131 | `cb5f97bf83fd4345073e1f7c0b8d9839a5eb81e518f927163881d872304af52c` |
| A/m4 latest, Linus | 69 | `7ae9687de40347f9cb1d6d411218e18b1544d2a1604f385f1529a3a900cbdeb4` |
| B/m1 single original | 69 | `528b54cda54f7295a813c01c562ce32c450cb7a90dfabff1fb02a70b33021428` |

Exact full-document pair A `webview-document-07.html` / `-08.html`: **3057 bytes**,
SHA-256 `56991d05285cd4de9efa364a33e69bba28bf212b51aa19e9fc4b82bdc23a3756`.
Pair B `-13.html` / `-14.html`: **2277 bytes**,
SHA-256 `935cef1bed940de0c894a66a1a7df0d4316b0c448ca9e73c453146f8483fb16e`.
Both actual WebView calls use `https://litterbox.invalid/`, `text/html`, `UTF-8`.

API captures `logs/body-a.json` / `body-b.json` have SHA-256
`190d4a549b12bb97e6af45f55ef271b1d1966a0ba7d9e4e7a337f4a8e12b683d` /
`c899c2c131defeb9507c6e533f6c23aec8418d20acfc0ef227f76095ab36035b`.
The actual app database copy is `dumps/cache-originals.sqlite`, SHA-256
`5d43186a634adf3e45fe2f8707b58be7ab30b8624fa2ea1ecc15ea642671ff78`.
The one-byte m3 code change `3`→`9` gives negative-control SHA-256
`d07acb86428e775788f376d32829ea5b347e4a38a9d28dc6bde0b92dcbe87001`, correctly unequal.

### 11g. Real native disclosure taps, truthful labels and single-message negative: PASS

The owned emulator display was temporarily **1133×2516 physical pixels at density 2.75**:
**412×915 rounded logical screen**, actual native WebView viewport **412×759 CSS pixels**.
This distinguishes the requested phone screen from the body surface below Android/Qt chrome.
Normal native refresh fetched the fixture; real row input opened A and B. Three **actual ADB
pointer taps**, with no JavaScript activation, produced these read-only CDP states:

| Artifact | Native input | Earlier-message `details.open` readback (m1,m2,m3) |
|---|---|---|
| `logs/a-closed.json`, shot `28-native-originals-a-closed` | Open A via real inbox row | false, false, false |
| `logs/a-open-m1.json`, shot `29-native-originals-a-open-m1` | `(566,622)` | true, false, false |
| `logs/a-open-m2.json`, shot `30-native-originals-a-open-m2` | `(566,1199)` | true, true, false |
| `logs/a-open-m3.json`, shot `31-native-originals-a-open-m3` | `(566,1624)` | true, true, true |

The exact sender/date summaries are **Ada Lovelace <ada@example.test> · 15 Oct 14:00**,
**Earlier message · 16 Oct 15:00**, and **<grace@example.test> · 17 Oct 16:00**. Unknown From
stays truthfully unknown; address-only From is not replaced by the card sender. The latest
**Linus Torvalds <linus@example.test> · 18 Oct 17:00** is expanded once, with ORIGM4 body/code.
The observed timezone renders the fixture UTC times as the displayed +04:00 local times.

Shots 29–32 were opened and inspected directly. All four full body markers become visible in the
same document scroll, original links m1/m2/m3 remain real anchors, and code ORIGM1/ORIGM3/ORIGM4
is preserved. A real upward swipe exposes the last code block (`32-native-originals-a-bottom`);
it does not create another reader scroll. This run proves link presence/code visibility, while
the older link-activation and text-selection passes remain explicitly carried, not re-executed.

Real input then opened B: `33-native-originals-b-single.png` was inspected, showing ORIGS1 body
and code. `logs/b-single.json` reports **one original, zero `<details>`**, no invented disclosure.
Neighboring viewport was also inspected after restoring physical **1080×2340**:
`34-native-originals-b-single-neighbor` and `35-native-originals-a-neighbor`, with actual native
WebView **394×698 CSS pixels** in `logs/b-single-neighbor.json` / `a-neighbor.json`. Sender/date
lines wrap without losing attribution; three collapsed earlier messages and the expanded latest
remain correct, and B still has no disclosure. Every listed screenshot has its AX XML beside it.

### 11h. Current 18-clause coverage, no criterion deleted

The historical table in §7 is not silently rewritten. On the **same installed a6cb742c artifact**,
the former 16 native passes remain mapped there, with their original carried/fresh labels; the two
acceptance gaps now have actual consumer proofs. The current native-checklist verdicts are:

| # | Retained criterion | Current verdict / evidence basis |
|---|---|---|
| 1 | Native chain renders HTML | **PASS**, historical real mail + fresh generic actual-load byte proof (§11f); named R61 source now available, named-source native rendering not exercised (§11k) |
| 2 | Click older details | **PASS**, historical bundle path retained; fresh original disclosures (§11g) |
| 3 | Single-scroll reader | **PASS**, historical path retained + fresh full-original scroll (§11g) |
| 4 | Selectable code | **PASS carried**, historical selection oracle; new run adds visibility, not a selection claim |
| 5 | Live link | **PASS carried**, historical activation oracle; new run adds exact anchor preservation |
| 6 | Popup hides/restores same document | **PASS**, frozen §5 evidence retained |
| 7 | Phone Back + inbox anchor/selection | **PASS**, frozen §7 evidence retained |
| 8 | Pause/resume | **PASS carried**, run-3 evidence retained |
| 9 | Navigation drawer | **PASS**, retained + fresh actual edge activation (§11d) |
| 10 | Navigation search | **PASS carried**, historical drawer route retained; fresh toolbar Search separately proved |
| 11 | Actual touch targets >=48dp | **PASS fresh**, 24/24 boundary/negative controls prove 132×132 pixels / 2.75 (§11d), **not an AX waiver** |
| 12 | 150% font, long subject/code | **PASS carried**, historical scaled-font oracle retained, not re-executed here |
| 13 | Reader overflow | **PASS**, retained + actual note journey menu (§11e) |
| 14 | Note dialog popup hide/restore | **PASS carried**, historical popup oracle retained; actual note editor also used (§11e) |
| 15 | Native UI note Save/mutation | **PASS fresh**, real Save → real server → offline fresh-process reopen (§11e) |
| 16 | Offline restart + anchor | **PASS carried**, historical anchor oracle retained; fresh offline note persistence separately proved |
| 17 | Normal enrollment/Online | **PASS**, frozen primary enrollment retained; isolated user-10 normal UI enrollment + server operation (§11e) |
| 18 | Durable journey screenshots/logs | **PASS**, historical logcat retained; new per-command statuses, PNG/XML, load bytes and readbacks archived |

**Counts within the original native checklist: 18 PASS = the retained 16 + fresh rows 11 and 15.**
This is not 18 freshly re-executed rows and is **not full R61 acceptance**. Production-byte
identity is supported by §11e's exact current owned-QML/CardStore blob matches; this lane does
not invent current Git tree modes from filesystem mode 0664 or claim a new artifact build.

### 11i. Exact residuals / bounded handoff

1. **Named R61 source AVAILABLE; named-source native rendering not exercised here.**
   `REQUIREMENTS.md:392–396` names **«Шаблон B2B-договора TwójStartup»**, Darafei message 15798,
   screenshot `img_c63f7a65bd36.jpg`. The initial ROOT/proof-directory/tasks-loop search did not
   establish global source absence: a later bounded canonical read-only liveDB lookup found the
   exact named card/message and archived the actual bytes. §11k gives the identities, paths and
   hashes. The former **UNAVAILABLE** conclusion is **stale and withdrawn**. TECHSHIP newsletters
   remain non-matching and no synthetic fixture is substituted. The generic native consumer /
   disclosure proofs above stand; this lane has not rendered the newly archived named source
   natively and does not claim that acceptance. Main owns any further named-source native gate;
   the desktop owner is `OriginalMessageConsumerProof`.
2. **Qt AX inclusive-endpoint defect remains.** Android reports the erroneous end-exclusive
   rect because Qt passes inclusive right/bottom (§11a). Actual physical input is >=48dp; the
   AX export itself is not called correct or waived. No dependency or app-size patch is needed
   for the assigned actual-hit criterion, per Main's accepted canonical-cause decision.
3. **Carried evidence and historical warning hygiene remain labeled.** This run did not rerun
   150% font, code selection, external-link activation, pause/resume, or all old anchor paths;
   their prior passing oracles and current relevant production-byte identity remain the basis.
   Historical QML Accessible/cross-thread warnings are not claimed repaired. All wider criteria,
   documentation and Main's authorized Git/release handoff remain intact.

### 11j. Commands, restoration and brief retro

Exact targeted ADB calls, timestamps, stdout/stderr and exit status are in
`logs/commands.jsonl`; host invocations, failures and intentional service stops are summarized
in `logs/host-command-status.json`. The revised `boundaries.py`, `compare-native.py`,
`source-identity.py`, `seed-originals.py`, `compare-originals.py`, named CDP observations and
the final restoration commands all exited **0**. The rejected cold boundary run exited **1**;
it remains archived and is not counted. Initial parser/404/private-PG-auth failures remain in
the journal rather than being erased. Named proof-service cancellation is not called an rc0 exit.

After freezing the finite proofs, the native observer and both owned proof services
(`lb-native-originals-observer2-20261019`, `lb-native-html-server-20261019`,
`lb-native-note-server-20261019`) were explicitly stopped. Only user 10's app was force-stopped;
owned ADB reverse 18127 and forwards 19227/27045 were removed. `wm size reset` restored
**1080×2340**, `am switch-user 0` restored the original user, and normal `am start -W --user 0`
returned **Status: ok**. Readback is current user **0**, physical size **1080×2340**.
`37-primary-user-restored-ready.png` + XML show the original real Google/Bumble/Google Play inbox;
the initial shot 36 captured QML loading and is not used as a ready-state oracle. User 0 was not
re-enrolled, cleared, or mutated. The isolated secondary user is retained as an offline archived
proof context. The original owned AVD supervisor remains running; real phone and TV are untouched.

Brief retro:

- **What changed the conclusion:** the canonical Qt endpoint source plus positive/negative
  physical input separated an AX export bug from actual hit coverage. This avoided an incorrect
  49dp padding change, tolerance waiver or Qt dependency campaign.
- **What initially failed:** a fixed cold-start delay mistook unready QML for an input failure.
  Warm readiness + explicit UI-state reset made each of the 24 assertions meaningful. Restoring
  user 0 likewise used the ready inbox oracle, not its first loading screenshot.
- **What made the note proof safe:** separate Android full-user app data plus a verified private
  real-server tenant allowed normal UI Save and offline fresh-process persistence without
  touching the owner's enrollment/cards. Actual load-argument capture avoided treating normalized
  DOM HTML as source bytes. The later named-source availability correction (§11k) does not turn
  these synthetic mechanism proofs into named-mail visual proof.
- **Scope held:** no product source/Qt/APK changes, build/test suite, installer/upgrader, deploy,
  Git mutation or sibling ADB use. Only this receipt and owned runtime evidence changed; all
  criteria/docs and Main's authorized Git/release handoff are preserved. No new proof campaign
  was started after the finite freeze.

Original proof hashes remain frozen in `NATIVE/logs/final-native-artifact-sha256.txt`; its ROOT
receipt entry is the **pre-amendment** hash `c3abfce006ea4fb7b9f47f73ee2dce63034ab2e99259886a22c69cfe505afd67`.
The source-availability amendment hashes are recorded separately in
`NATIVE/logs/r61-source-availability-amendment-sha256.txt` and delivered to Main, without a
self-referential receipt hash.

### 11k. Source-availability correction after native freeze

**Correction:** the named R61 source is **AVAILABLE**, not UNAVAILABLE. At canonical lookup
as-of **2026-10-19 04:00:40.805833+04**, `OriginalMessageConsumerProof`'s Main-directed bounded
read-only lookup of `127.0.0.1:5435/litterbox` returned **cards_exact = 1**. This lane did not
repeat a live DB query; it directly read the archived original, identity, card-row, hash and
lookup artifacts. The archived lookup log is
`/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/original-message-proof/logs/b2b-live-lookup.txt`.

Source archive root:
`/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/original-message-proof/b2b-source`.

| Identity / source | Actual archived value |
|---|---|
| Card / Gmail thread | `f508a4fc-92ef-49b2-a06b-5ec11bb72bc0` / `1a139ec80dd50597` |
| Original message / count | `da3aeae7-406a-4a0b-90cb-34cf87d1d2f3` / **1** |
| Card sender (not invented message MIME From) | Aliaksandr Baranau `<abaranau@maumap.com>` |
| Original sender_name / sender_address | **both empty**, legacy unknown From; preserve truthful unknown attribution |
| Original received_at | `2026-10-14 14:19:11+04` |
| Stored body_hash | `0375bbb171057b52a029265767143c70fcd00c31d13f5bcf63bf3cf79bc0c559` |
| `named-b2b-original.html` | **107 bytes**, SHA-256 `fec8152d95fb7035d39cd0856a7eab0db6ecd168ca3486958e998dc5336c83c1` |
| `named-b2b-original.txt` | **48 bytes**, SHA-256 `bb39c252bdca7f075d1be764ac824dc3cb41cd537dbb35f079d4510dd9a94ffc` |

Supporting archive files read directly: `identities.txt`, `card-row.txt`, `msg-bodyhash.txt`,
`named-source-sha256.txt`, and the actual `named-b2b-original.html`. The latter contains the
named original body, not a body reconstructed from the requirements subject. Source availability
does not itself prove a render; **this lane adds no new native visual PASS** for these bytes.

The previous absence conclusion overgeneralized a KB/filesystem-only search. The canonical
database lookup corrected the evidence/receipt layer, not production code. All actual native
touch, note Save/persistence, generic byte-preservation and disclosure proofs remain unchanged.
No new ADB use, emulator/user switch, server start, build, Git mutation, live query or native
proof campaign was performed for this amendment. User 0 and the restored viewport are left as
recorded in §11j. Main was notified of the correction and retains the next acceptance decision.

