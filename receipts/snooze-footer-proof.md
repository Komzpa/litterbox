# Snooze footer buttons — production fix proof (RepairSnoozeFooter lane)

ROOT = `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`
OUT = `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018` (cached production kit, reused)

Scope: one bounded visual fix — `ROOT/qt/src/qml/CardActions.qml` snooze dialog footer
(`footer: Controls.DialogButtonBox`, previously lines 495–534). No Git, no deploy, no
REQUIREMENTS/docs edits, no sibling files, no phone/native/owner-desktop access in this lane.

## 1. What changed (actual source identity)

`qt/src/qml/CardActions.qml` — only the two button declarations inside the snooze dialog
footer block. sha256 before = frozen production receipt value
`49e9ecedf93464578391d728c500b735de1931ec7f4a07cfd2ce9a97f0357fd8`;
sha256 after = **`d5e517abb525e23bc8b49b71cebe850d7b15586f1fe56c0bb7f807cc6fe950c3`**.

Per button (`snoozeOkButton`, `snoozeCancelButton`) added, nothing else touched:
- `implicitWidth: Math.max(96, Kirigami.Units.gridUnit * 6)` (same formula as the working
  `sheetCloseButton` footer button in this file, `CardActions.qml:233–268`);
- `contentItem: Controls.Label { text: <button>.text; color: root.actionInk;
  horizontalAlignment/verticalAlignment centered; elide }` — pinned ink, no style override.

Unchanged on both buttons: `onClicked: snoozeDialog.accept()` / `snoozeDialog.reject()`,
`implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)` (48 dp), `background: Rectangle`
with `border.width: visualFocus ? 2 : 1` (keyboard focus contrast) and the
down/hovered surface colors, `palette.button`/`palette.buttonText`.

Why not a literal copy of the working note dialog (`qt/qml/InboxView.qml:2189–2222`,
hash `19deee88a91c2e1905fb809938bf192d9f6b45ff47919aeb1edad3cbcff44811`, untouched):
the frozen red probes showed the snooze footer buttons already used that exact declaration
structure yet resolved with `contentItem == null`, `implicitWidth == 0` in this dialog
context (`Loader`/`Component` + custom `background` replacing the KDE style plate that
paints labels), while the note buttons resolve style content (`QQuickIconLabel`, w=76/87,
measured again in this lane's runs). The in-file working convention for footer buttons in
*CardActions.qml* is `sheetCloseButton`: explicit width + explicit pinned-ink content —
reused here rather than inventing a new pattern.

`qt/src/qml/CardActions.qml:667–701` (`completeDialog` footer) inspected only: it repeats
the same declaration pattern for the separate "Complete all unpinned cards" action. It is
NOT part of the snooze canonical issue and was left untouched per "don't expand other
actions". (Honest flag to the owner: that dialog likely carries the same zero-width defect;
out of scope for this lane.)

Preserved accept/reject behaviour (source-level + consumer-level):
- `onAccepted` future/roundtrip validation block untouched (`snoozeAcceptable`: parts
  roundtrip + `until.getTime() > Date.now()`, error `qsTr("Enter a valid future local date
  and time.")` + re-open). Frozen DST/invalid-input proof (`snooze-dst-berlin.json`, Feb-30
  negative control) covers it and its sources were not modified.
- Cancel remains a no-op: `reject()` only closes; no `enqueueOp` on that path.
- 48 dp minimum height and the 2 px `visualFocus` border with `#263b3a`-on-`#ffffff`
  (11.87:1) label ink unchanged.

## 2. Red — attempt-A on the unfixed build (actual binary, real controls)

Command (same recipe as the frozen production receipt):
`env -u WAYLAND_DISPLAY HOME=<tmp> XDG_CONFIG_HOME=<tmp>/config QT_QPA_PLATFORM=xcb
QT_QUICK_CONTROLS_STYLE=org.kde.desktop TZ=Pacific/Kiritimati xvfb-run -a dbus-run-session --
OUT/build/litterbox-qt --capture-script OUT/profile OUT/snooze-scripts/snooze-footer-redgreen-1280.json
OUT/captures-snooze/footer-red-1280`

- **exit 3**, `OUT/logs/snooze-footer-red-1280.log`.
- Probe step 24 (live scene graph): `snoozeOkButton`/`snoozeCancelButton`
  `w=0, implW=0, impBgW=0, impContW=0, padL=0, padR=0, cont=null, bg=QQuickRectangle`,
  footer `w=392 h=70` — zero width, nothing painted (blank footer band in
  `footer-red-1280/footer-1280-02-picker-before-cancel.png`).
- Step 26 `{"click":"snoozeCancelButton"}` → **`Capture script step 26 failed: unknown or
  non-visible objectName 'snoozeCancelButton'`** — the exact attempt-A failure criterion.

## 3. Green — attempt-A after the fix (exact red-green criteria)

Scripts: `OUT/snooze-scripts/snooze-footer-redgreen-{1280,412}.json` (49 steps each;
generator kept in session scratch `local://make_footer_redgreen.py`).
Profile reseeded (`OUT/smoke/gen.py`) + uuidized per run (fixture ids must be UUIDs for
`CardStore::enqueueOp` — fixture artifact documented in the frozen production receipt).

| run | script | TZ | exit | log | captures |
|---|---|---|---|---|---|
| red (pre-fix) | redgreen-1280 | Pacific/Kiritimati | **3** (step 26, expected) | `OUT/logs/snooze-footer-red-1280.log` | `OUT/captures-snooze/footer-red-1280/` (2 PNG) |
| green 1280x800 | redgreen-1280 | Pacific/Kiritimati | **0** | `OUT/logs/snooze-footer-green-1280.log` | `OUT/captures-snooze/footer-green-1280/` (6 PNG) |
| green 412x915 | redgreen-412 | Pacific/Kiritimati | **0** | `OUT/logs/snooze-footer-green-412.log` | `OUT/captures-snooze/footer-green-412/` (6 PNG) |

**Criterion 1 — `snoozeCancelButton` paints and clicks (real control click, attempt-A step):**
- 1280x800 probe step 24: `ok {w:108, h:54, implW:108, cont:"Label_QMLTYPE_90", contText:"OK", contColor:"#263b3a"}`,
  `cancel {w:108, h:54, implW:108, contText:"Cancel", contColor:"#263b3a"}`;
  dialog `392x240 @(444,280)`, footer `392x70`; item bbox x=606 (OK) / x=720 (Cancel), h=54.
- 412x915 probe step 24: same `w:108 h:54 implW:108` content labels; dialog `380x240 @(18,338)`,
  footer `380x70`; item bbox ok `108x54 @ (168,516)`, cancel `108x54 @ (282,516)` (probe
  coordinates match the rendered screenshot at 412; at 1280 the eval-context y is shifted
  vs render — the harness clicks via screen mapping and the screenshots show both buttons
  in the footer band, x extents 606–714 / 720–828 exactly as probed).
- Paint: `footer-1280-02-picker-before-cancel.png` and `footer-412-02-picker-before-cancel.png`
  show both buttons drawn with the `#dce5e3` frame and centered `#263b3a` OK/Cancel labels
  in the footer band (visually inspected at both sizes; footer band pixel scan: 769 border
  pixels at 1280, 776 at 412 within the dialog band).
- Click: step 26 `{"click":"snoozeCancelButton"}` executed on both runs (no
  "unknown or non-visible" failure) and closed the picker —
  step 28 state 1280 `{"pendingOps":1,"pickerVisible":false,"card1":false,"card2":true}`,
  412 `{"pendingOps":1,"pickerVisible":false,"card2":true,"depth":2}`: **cancel is a true
  no-op** (no operation enqueued, card retained; at 412 the reader even stays open).
  Shot `*-03-after-cancel-button.png` shows the picker gone.

**Criterion 2 — OK accepts the actual custom payload:**
- Step 43 `loadTarget(2027,3,14,9,30)` → step 44 `{loadTarget:true, year:2027, month:3,
  day:14, hour:9, minute:30}` (real spin values in the picker; shot `*-05-picker-custom-values.png`).
- Step 45 `{"click":"snoozeOkButton"}` (real control click) → step 48
  `{"pendingOps":2,"pickerVisible":false,"card2":false}` at both sizes: dialog accepted and
  closed, the snoozed card left the inbox (`card2` gone), one new operation enqueued.
- Outbox payloads read back from `OUT/profile/cards.sqlite` (`outbox`, desktop green run):
  - preset step: `{"type":"snooze","args":{"until":"2026-10-20T19:00:00.000Z"},...}` —
    "Tomorrow morning" at local +14:00 (run instant local Tue 20 Oct 04:55 GMT+1400).
  - **custom OK click: `{"type":"snooze","args":{"until":"2027-03-13T19:30:00.000Z"},...}`** —
    exactly 2027-03-14 09:30 local (Pacific/Kiritimati +14:00), i.e. the payload from the
    picker fields, validated by the untouched `snoozeAcceptable` roundtrip+future check.

## 4. Scoped frozen consumer proof after the edit (actual test, not a mock)

`ctest --test-dir OUT/build -R '^tst_mailopen$'` under fresh `HOME`, xvfb + dbus-run-session
→ **exit 0**, `1/1 Test #20: tst_mailopen .... Passed 22.10 sec`, `100% tests passed`
(`OUT/logs/snooze-footer-mailopen.log`). Test file untouched:
`qt/tests/tst_mailopen.qml` sha256 `1d6527c20dbb87bc4ef90e829094a1d97142ca15f64cbfa6571e9de1ed49a78b`
(same as the frozen production receipt). All 27 data-driven cases pass, including the
snooze case (menu → custom picker → reject-no-enqueue → `captureSnooze` accept) and every
archive/continuation case — i.e. accept/reject semantics and future validation are preserved
by behaviour, not just by source diff.

## 5. Entry wiring (source-confirmed, unchanged by this fix)

Reader top bar `⋮` `objectName: "mailActions"` (`qt/pages/MailDetailPage.qml:337–338`) →
action sheet row `objectName: "actionRow-snooze-<cardKey>"` (`CardActions.qml:343–349`) →
`chooseSnoozeDateTime()` → QQC2 `Menu` `objectName: "snoozeMenu"` (presets via
`buildSnoozePresets`) → `pickSnooze(key)` → `commitSnooze(until)` →
`store.enqueueOp(cardKey, "snooze", {until: toISOString})` → `cardHandled("", "snooze")`.
Custom row `objectName: "snoozeCustom"` → `openCustomSnooze()` → `Dialog objectName:
"snoozeDialog"` (`snoozeLoader`, spins `snoozeYear/Month/Day/Hour/Minute`) →
`snoozeOkButton` → `snoozeDialog.accept()` → `onAccepted` (validation → `commitSnooze`) /
`snoozeCancelButton` → `snoozeDialog.reject()` (no-op). The capture runs drove exactly this
entry wiring with real control clicks at both viewports.

## 6. Artifacts

- Changed path: `ROOT/qt/src/qml/CardActions.qml` (footer button blocks only).
- Durable copy of the prior production receipt (previously local-only):
  `ROOT/receipts/snooze-menu-production-receipt.md` (copied verbatim; the `local://` copy is
  scratch, this file is the publication).
- Captures: `OUT/captures-snooze/footer-{red-1280,green-1280,green-412}/` (14 PNG + shot JSON).
- Scripts: `OUT/snooze-scripts/snooze-footer-redgreen-{1280,412}.json`.
- Logs: `OUT/logs/snooze-footer-{red-1280,green-1280,green-412,mailopen}.log`.

## 7. Remaining gaps

- Native Android device screenshot/input proof — not attempted (no device/owner desktop in
  this lane); desktop 1280x800 (named) + phone-width 412x915 (neighboring) are the covered
  viewports.
- `completeDialog` footer (CardActions.qml:667–701) keeps the old pattern by scope decision
  ("don't expand other actions"); flagged to the owner above.
- Git commit/push/deploy/docs: untouched (Main's coordination).
