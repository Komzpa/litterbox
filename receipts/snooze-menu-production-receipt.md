# Snooze preset menu — production proof receipt (test+receipt lane)

Owner: assigned lane "Replace snooze text entry with preset menu" (own files only: `qt/tests/tst_mailopen.qml` + this receipt). No production source, Git, deploy, REQUIREMENTS.md, or sibling files were edited by this lane.

Artifact root: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (ROOT)
Proof output root: `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018` (OUT, the existing cached production kit)

## 1. Test hunk applied (own file)

`qt/tests/tst_mailopen.qml`, branch `row.action === "snooze"` of `test_actionReturnsToSameInboxOffset` — applied exactly the "New" hunk from `local://snooze-preset-qt-test-receipt.md` (approved handoff): menu opened via `actions.snoozeMenu()`, `snoozePreset-hour` + `snoozeCustom` visibility asserted, `menu.close()`, `actions.openCustomSnooze()`, picker focus on `snoozeDay`, `keyClick(Qt.Key_E)` must not archive, `picker.reject()` must not enqueue, then the existing `captureSnooze(localDateTime(tomorrow))` action. Zero `snoozeDateTime` references remain (grep count 0). No archive-continuation assertion was touched.

- Post-edit sha256: `1d6527c20dbb87bc4ef90e829094a1d97142ca15f64cbfa6571e9de1ed49a78b`
- Patch method: local session script (`local://apply_snooze_hunk.py`), exact-old-hunk match count asserted = 1.

## 2. Frozen source identity (NEW full Qt copy)

Freeze command: `cp -a ROOT/qt /tmp/snooze-menu-qt-freeze-20261019` (after the hunk, before build).

| file | frozen sha256 | worktree sha256 after all runs |
|---|---|---|
| `src/qml/CardActions.qml` | `49e9ecedf93464578391d728c500b735de1931ec7f4a07cfd2ce9a97f0357fd8` | identical (re-verified at end) |
| `tests/tst_mailopen.qml` | `1d6527c20dbb87bc4ef90e829094a1d97142ca15f64cbfa6571e9de1ed49a78b` | identical |
| `tests/tst_cardstore.cpp` | `4acbe1e8a1523e268714a17ae679b290a0e2b423a49f1e45f75b62aca6e6a513` | identical (matches predecessor's frozen value) |

No stale-source warnings: hashes re-checked after the last capture run and matched.

## 3. Build — existing cached production kit (not a component harness)

Kit: `OUT/build`, CMake source = `ROOT/qt` (`CMAKE_HOME_DIRECTORY` in `OUT/build/CMakeCache.txt`), `LB_TEST_PLATFORM=xcb`.

- `cmake --build OUT/build --target litterbox-qt tst_maildetail -j2` → **exit 0** (7.10 s; incremental — CardActions qmlcache + link). Note: the QML test is registered as ctest `tst_mailopen` running `build/tst_maildetail -input ROOT/qt/tests/tst_mailopen.qml`; there is no `tst_mailopen` ninja target (first build attempt with `--target tst_mailopen` failed with `ninja: unknown target`, rebuilt with `tst_maildetail`).
- All captures below ran the actual production binary `OUT/build/litterbox-qt --capture-script …` with real app controls (C++ `pickVisibleScriptItem` mouse injection + `eval` probes). No component-only harness was used or substituted.

## 4. Scoped consumer test after the hunk (run once)

```
test_home=$(mktemp -d /tmp/litterbox-snooze-mailopen.XXXXXX)
env -u WAYLAND_DISPLAY HOME=$test_home XDG_CONFIG_HOME=$test_home/config \
  xvfb-run -a dbus-run-session -- \
  ctest --test-dir OUT/build -R '^tst_mailopen$' --output-on-failure
```
→ **CTEST_EXIT=0**, `1/1 Test #20: tst_mailopen … Passed 22.16 s` — all 27 data-driven cases inside the file passed, including the adapted snooze case and every archive case (`archive-button`, `archive-key`, `done-body`, the four `test_localArchiveContinuesToNextPresentedReader` rows, `test_realPopupsProtectRetainedReaderAndArchiveShortcut` incl. the `snooze` popup row, and the archive-shortcut resume assertions). No archive case was weakened.

## 5. Capture runs (README `--capture-script` recipe, cached kit)

Common template (from `OUT/smoke/run_all.sh` + README):

```
python3 OUT/smoke/gen.py                       # reseed OUT/profile  → GEN=0
cat local://uuidize_profile.py | python3 - OUT/profile/cards.sqlite   # → exit 0
env -u WAYLAND_DISPLAY HOME=$(mktemp -d) XDG_CONFIG_HOME=$HOME/config \
    QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop TZ=<tz> \
    xvfb-run -a dbus-run-session -- OUT/build/litterbox-qt \
    --capture-script OUT/profile OUT/snooze-scripts/<script>.json OUT/captures-snooze/<dir>
```

Fixture note (important): the smoke profile's card ids are non-UUID (`qgis`, `jasmine`, …) and `CardStore::enqueueOp` (ROOT `qt/src/CardStore.cpp:590`) rejects them (`QUuid(cardId).isNull() → return {}`), so preset clicks on the raw fixture were a silent no-op. This is a **fixture artifact, not a production bug** (real server cards are UUIDs). The uuidize helper rewrites `cards.id`, the JSON `id` inside `cards.payload`, and `mail_bodies.card_id` to fixed UUIDs (`f0a1b2c3-0001-…` / `f0a1b2c3-0002-…`). It lives only in session scratch (`local://`), not in the repo.

| # | TZ | script | exit | shots | log |
|---|---|---|---|---|---|
| 1 | Pacific/Kiritimati (+14) | `snooze-desktop-1280.json` (attempt A, cancel via button click) | **3** (first failing proof, see §7) | 4 | `OUT/logs/snooze-desktop-1280.log` |
| 2 | Pacific/Kiritimati | desktop v2 (walker probes) | **3** (same step 29) | 4 | same log name, overwritten by later runs; step data quoted below |
| 3 | Pacific/Kiritimati | desktop v3/final (cancel via Escape + note-dialog comparison probe) | **0** | 7 | `OUT/logs/snooze-desktop-1280.log` |
| 4 | Pacific/Kiritimati | `snooze-phone-412.json` | **0** | 5 | `OUT/logs/snooze-phone-412.log` |
| 5 | Europe/Berlin (DST weekend) | `snooze-dst-berlin.json` | **0** | 2 | `OUT/logs/snooze-dst-berlin.log` |
| 6 | Asia/Yangon (+6:30, evening already past) | `snooze-evening-absent-1280.json` | **0** | 1 | `OUT/logs/snooze-evening-absent.log` |

Scripts: `OUT/snooze-scripts/{snooze-desktop-1280,snooze-phone-412,snooze-evening-absent-1280,snooze-dst-berlin}.json`.
Captures: `OUT/captures-snooze/{desktop-1280,phone-412,dst-berlin,evening-absent}/` (19 PNG + JSON metadata total).

## 6. Evidence (all observed, actual app)

**Painted menu, real dates, ≥48 dp rows, text contrast** — eval probes over the live scene graph + screenshots:

- 1280x800 (`OUT/captures-snooze/desktop-1280/desktop-1280-02-snooze-menu.png`, Kiritimati local Tue 20 Oct 04:07): rows all `visible=true`, `h=54` (≥48 dp), `w=297`, x=979: `snoozePreset-hour "In one hour · 20 Oct 05:07"`, `snoozePreset-evening "This evening · 20 Oct 18:00"`, `snoozePreset-tomorrow "Tomorrow morning · 21 Oct 09:00"`, `snoozePreset-monday "Next Monday morning · 26 Oct 09:00"`, `snoozeCustom "Custom date and time…"`. fg `#263b3a` on bg `#ffffff` → WCAG contrast **11.87:1** (≥4.5) measured from both row palette and menu palette.
- 412x915 (`phone-412-02-snooze-menu.png`): same rows, `h=54`, contrast 11.87, x=111 w=297 — fully inside the 412 px frame; painted screenshot confirms no clipping.
- **Evening absent**: `evening-absent/absent-01-snooze-menu.png` under `TZ=Asia/Yangon` local Mon 19 Oct 20:41: probe `snoozePreset-evening {visible:false, h:0, text:""}` and the screenshot shows only 3 preset rows + separator + custom — the conditional evening row hides exactly as `buildSnoozePresets` specifies.

**Actual menu row → operation + card removed** (real control click, `click:"snoozePreset-tomorrow"`):

- After click: `{"pendingOps":1,"qgis":false,"jasmine":true,"depth":1,"readerOpen":false}` at both 1280x800 and 412x915 — operation enqueued, the snoozed card leaves the inbox, reader returns to inbox (depth back to 1).
- Outbox payload (desktop final run, `sqlite3 OUT/profile/cards.sqlite`): `{"args":{"until":"2026-10-20T19:00:00.000Z"},…,"type":"snooze"}` — equals Tue 21 Oct 09:00 at local +14:00 (probe recorded `nowLocal "Tue Oct 20 04:11 … GMT+1400"`), i.e. the exact local "Tomorrow morning" instant, not an arbitrary wall clock.

**Custom picker painted at both sizes**:

- `desktop-1280-04-custom-picker.png` and `phone-412-04-custom-picker.png`: dialog painted (`dialogVisible=true`, 392x240 at x444,y280 desktop; 380x240 at x18,y338 phone), all five spins found with `h=54` and correct values (2026/10/20 or 5/9…), hint "Brings the card back …" painted.

**Cancel = no-op** (actual control, keyboard): `{"key":"Escape"}` on the open picker → `{"pendingOps":1,"pickerVisible":false,"jasmine":true,"depth":1}` at both sizes (only the earlier preset op remains; card stays; nothing enqueued). Shots `*-05-after-cancel.png` show the picker gone. The on-screen Cancel **button** could not be clicked — see §7.

**Deterministic TZ weekend/DST check** (`TZ=Europe/Berlin`, explicit inputs, existing `captureSnooze`/Date path, no wall-clock dependence; DST ended Sun 2026-10-25 03:00→02:00 local):

- Menu probe at `Mon Oct 19 16:10:16 2026 GMT+0200`: all 4 weekend-dated rows present (`19 Oct 17:10`, `19 Oct 18:00`, `20 Oct 09:00`, `26 Oct 09:00`), h=54, contrast 11.87 — `dst-01-menu-berlin.png`.
- Explicit captures (`snooze-dst-berlin.json` eval step 12): `r1/r2/r3 = true` with empty `snoozeError`, `pendingAfterValid=3`; `r4` (Feb 30) `= false`, error `"Enter a valid future local date and time."`, `pendingAfterInvalid=3` (negative control, pending unchanged).
- Outbox payloads read back from sqlite — **exact local→UTC round trip**:
  - `2026-10-24 12:00` (Sat, CEST +02) → `2026-10-24T10:00:00.000Z` ✓ matches in-app JS check `check.sat`
  - `2026-10-25 12:00` (Sun, DST-end day, CET +01) → `2026-10-25T11:00:00.000Z` ✓ `check.sun`
  - `2026-10-26 09:00` (Mon, CET) → `2026-10-26T08:00:00.000Z` ✓ `check.mon`
- All three inputs were future local times at run time (run instant `2026-10-19T14:10:20Z` recorded in the same run).

## 7. First failing proof → production bug handoff (exact hunk; NOT fixed by this lane)

First failure (attempt A, desktop): `Capture script step 29 failed: unknown or non-visible objectName 'snoozeCancelButton'`, run exit 3, after `*-04-custom-picker.png` was painted.

Diagnosis (measured on the live app, both sizes):

- `snoozeOkButton` / `snoozeCancelButton`: `found=true, visible=true, enabled=true, text="OK"/"Cancel"`, **`w=0, implW=0, contentItem=null`** → the footer DialogButtonBox exists (`footerVisible=true, h=70, w=392`, its contentItem is a `QQuickListView` from the org.kde.desktop style) but both buttons lay out at zero width, are unpainted (blank footer band in both picker screenshots; pixel scan of the footer band shows white + hint text only), and are unreachable by real mouse control.
- Working comparison in the same run/style/binary: the note dialog's sibling buttons are correct — `noteSaveButton {w=76, implW=76}`, `noteCancelButton {w=87, implW=87}`, clickable via real control (step 39 clicked it, dialog closed, run exit 0).

**Exact source hunk for the code owner** (ROOT `qt/src/qml/CardActions.qml`, not edited by this lane): the snooze dialog footer block **lines 495–534** —

```qml
                footer: Controls.DialogButtonBox {
                    background: Rectangle { color: root.actionSurface }
                    Controls.Button {
                        id: snoozeOkButton
                        objectName: "snoozeOkButton"
                        text: qsTr("OK")
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        …
                    }
                    Controls.Button {
                        id: snoozeCancelButton
                        objectName: "snoozeCancelButton"
                        text: qsTr("Cancel")
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        …
                    }
                }
```

inside `Loader { id: snoozeLoader … Controls.Dialog { id: snoozeDialog … } }` (lines 469–536). The identical declaration pattern also exists in `completeDialog` (lines 667–701) and should be checked by the owner. The working reference is `qt/qml/InboxView.qml:2189–2222` (note dialog). Suggested direction (owner-verified): the buttons resolve with `contentItem == null` and `implicitWidth == 0` under this dialog context — give each button an explicit `implicitWidth` (or `Layout.preferredWidth`) sized from its text, or reproduce whatever keeps the note-dialog buttons' content item alive; then re-run `OUT/snooze-scripts/snooze-desktop-1280.json` with step `{"click":"snoozeCancelButton"}` (attempt A) as the red/green check. This lane did not touch source per assignment (own = test + receipt only).

Caveat recorded honestly: cancellation itself is proven no-op via the real keyboard control (Escape → reject path) in the exit-0 runs; only the Cancel **button** control is defective.

## 8. Actual entry wiring behavior (observed + source-confirmed)

Reader top bar `⋮` button `objectName: "mailActions"` (`qt/pages/MailDetailPage.qml:337–338`, CardActions instance `cardKey: root.cardId`) → `onClicked: actionSheetItem().open()` → action sheet row `objectName: "actionRow-snooze-<cardKey>"` (`CardActions.qml:343–349`) → `onClicked: { actionSheet.close(); root.chooseSnoozeDateTime() }` → `chooseSnoozeDateTime()` builds presets from local `Date` parts, opens the QQC2 `Menu` (`objectName: "snoozeMenu"`, `parent: Overlay.overlay`) → row `onClicked: root.pickSnooze(key)` → `commitSnooze(until)` → `store.enqueueOp(cardKey, "snooze", {until: toISOString})` → `cardHandled("", "snooze")`. Custom row → `openCustomSnooze()` → `Dialog objectName: "snoozeDialog"` with spins `snoozeYear/Month/Day/Hour/Minute`. The `e` archive shortcut stays gated while either surface is open through `MailDetailPage.actualPopupsVisible` → `actions.snoozeDialogItem()` (returns picker if visible else menu). Verified live: while menu/picker open, injected `E` produced no operation (test) and clicks inside the surfaces produced no archive.

## 9. What remains (native final cohort / out of scope for this lane)

- **Native device (Android) proof**: real-device screenshot of the painted menu/picker at device densities, and a native input run — not attempted here; this lane had no device/owner-desktop/shared-installer access. Xvfb/xcb desktop + phone-width (412x915) software-rendered captures are the scope completed.
- Production fix of §7 is with the code owner (exact hunk handed off).
- Git commit/push, deploy, REQUIREMENTS.md, docs/changelog updates, sibling receipts: untouched by this lane (main agent's coordination).
- `REQUIREMENTS.md` and other dirty worktree files belong to sibling lanes; frozen-hash table above covers only this lane's artifacts.

## 10. Retro

Pathfinding first paid off again: the approved handoff hunk applied verbatim (single exact match) and the pre-existing cached kit reused without a fresh configure. Two false leads were corrected mid-flight instead of being reported as bugs: (1) my first capture run's "preset click does nothing" was the smoke fixture's non-UUID card ids hitting `enqueueOp`'s UUID guard — fixed with a documented profile patch, not a source change; (2) QML `children` traversal misses runtime-parented popup items — replaced with a deep walker (`children`/`data`/`item`/`contentItem`/`footer`/`header`/`contentData`) that found the Menu, rows, dialog and footer buttons. The real defect (zero-width snooze dialog buttons) was isolated by contrast against the working note-dialog buttons in the same run and handed off with the exact hunk instead of being patched outside my ownership. Lesson: put the diagnostic probe *before* the risky click in a capture script so an exit-3 run still yields the measurement that explains the failure.
