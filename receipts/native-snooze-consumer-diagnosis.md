# native-snooze-consumer-diagnosis — discriminating checkpoint (no product change)

Run: DiagnoseNativeSnoozeConsumer, 2026-10-19 21:20–21:52 (+04:00). Owner slot: AVD
`litterbox-native-proof` via `adb -s emulator-5590` only (ANDROID_USER_HOME=NATIVE/android-user).
Physical `R5CX80MZ1SN` and TV `192.168.100.39:5555` never contacted. Zero `input tap`/`input
swipe` issued by this run; zero mail mutation; zero install; zero Git. Nothing dispatched to
publication.

## 1. Artifact identity (unchanged, re-read)

Candidate `NATIVE/release-kit-20261019/litterbox-qt-v24-ee307c35.apk` (SHA-256
0d21736c80d47a0a2f6e4d8024d633ccb2b0f7332f15018898065f524a26fddf, source
ee307c350f925bc3cf059d96ea8f3597141944fc) remains installed on the owned AVD as versionCode=24 /
versionName=0.1-37, lastUpdateTime 2026-10-19 19:35:27, app PID **32311 alive before and after
this run** (`pidof` 21:31 and 21:50). No APK was rebuilt (no source change was made).

## 2. Exact failing commands, process that crashed, and controlled aborts

| # | Exact command | Exit | What happened |
|---|---|---|---|
| C1 | `adb -s emulator-5590 shell "grep -o '<node[^>]*>' /sdcard/diag-pre-pf-probe.xml \| grep -F 'Custom date and time'; echo ---; grep -o '<node[^>]*>' /sdcard/diag-pre-pf-probe.xml \| grep -F 'Snooze'; echo ---SNIP---; grep -o '<node[^>]*>' /sdcard/diag-pre-pf-probe.xml \| grep -F 'In one hour'"` | 1 | Each of the three pipelines printed `Segmentation fault` (3 lines). **Crashed process: on-device toybox `grep`** (/system/bin/grep) parsing a 41 KB single-line uiautomator XML with `grep -o '<node[^>]*>'`. **Not an app crash**: app PID 32311 stayed alive and served every later call. Repro shape: `grep -o` with that regex over the single-line dump; the identical extraction succeeded twice elsewhere (inside `diag-chain.sh`'s `ndump`, which produced a complete 110-node file, and via host-side python over the pulled XML). Device-tool crash, transient/long-line class; host-side parsing used from here on. |
| C2 | `adb -s emulator-5590 shell "sh /sdcard/diag-chain.sh pf probe"` | 7 | **Controlled abort by this run's own guard**, not a failure: marker `pf/probe ABORT lingering menu node in pre-state` (logcat tag DIAGPROBE 21:45:51). The guard refuses to tap while ghost menu nodes are present in the pre-state dump. **Zero taps were issued.** |

## 3. Discriminating result (the concrete diagnostic of this run)

**Finding D1 — the accessibility tree exposes instantiated-but-closed snooze surfaces ("ghost
menu"); dumps cannot prove that a menu is on screen.**

Pixel confirmation (screenshot `/tmp/diag-prestate.png`, 21:50, zero interaction by this run):
the whole inbox renders **uniformly dimmed under a modal scrim** (Android system bars normal, app
window greyed) with **no menu, no picker and no dialog painted** — while the 21:45 dump of the
same untouched state contains the full ghost menu below and no dialog node. This reproduces the
old receipt §5.3 "lingering modal overlay" observation from a clean, non-interacted state: a
modal surface is in its opened/dimmed state while nothing of it is painted. Whether that state
accepts or swallows input was not exercised (this run issued zero taps).

Evidence, all from a pre-chain `uiautomator dump` taken at 21:45:48 with **zero interaction by
this run** (device state untouched since the predecessor era):

- The plain inbox dump (110 nodes, 4 live `More actions for …` buttons) **also contains a full
  snooze preset menu node set**: `In one hour · 19 Oct 21:40` bounds [847,1392][1132,1540],
  `Tomorrow morning · 20 Oct 09:00` [847,1689][1132,1837], `Next Monday morning · 26 Oct 09:00`
  [847,1837][1132,1985], `Custom date and time…` [847,2032][1132,2180].
- Label arithmetic (`qt/src/qml/CardActions.qml` `buildSnoozePresets`: hour preset
  `until = now + 1h`, `snoozeWhenText` renders `until`): `19 Oct 21:40` ⇒ the menu was built at
  **20:40**, i.e. the predecessor crawl's last minutes, and its nodes were still exposed **65
  minutes later** while no menu was opened again. These nodes coexist with the live list in one
  tree.

What this discriminates:

1. **Every dump-based "menu rendered / is dumpable" claim in `receipts/ui-corrections-android.md`
   §3 clause 5 and §5 is reclassified as ambiguous** — a dump cannot distinguish an open popup
   from the persistent item of a closed Loader-activated Menu. Pixel capture is the only valid
   render proof. (Clause statuses themselves are NOT changed: see §4.)
2. **"Preset taps close the menu without committing" and "tap landed on the list beneath" are
   consistent with taps aimed at ghost bounds of an already-closed menu** — matches the
   predecessor's own `42-preset-nodump.png` tap-through to the LWN reader. The tap-no-payload
   symptom therefore does NOT yet prove a defect in `pickSnooze`/`commitSnooze`/`enqueueOp`.
3. **Dump-to-tap latency is not the whole story**: at dump time (21:45) the menu was already
   closed while its nodes persisted; the surface state, not timing, is the discriminating
   variable.
4. **Cross-thread instantiation (`QObject: Cannot create children for a parent that is in a
   different thread`) remains an unproven mechanism**: the app-PID ring logcat (225 lines,
   20:30:42–21:45:50, captured to `/tmp/diag-ring-pre.txt`) contains **40** of those warnings
   (distribution across time not extracted) plus `W/Qt A11Y … AccessibilityNodeInfo with empty
   contentDescription` at 21:45:50.367 coinciding with my dump, but **zero**
   `TypeError`/`ReferenceError` — so no QML exception was logged in that window. Creation
   evidently succeeds and lays out (D1 bounds are real layout bounds).

**Finding D2 — popup anchoring geometry (observation, on-screen effect unproved).** Menu item
bounds in the three saved dumps end exactly at the right screen edge while starting at the anchor
button's left edge: `ui-dump-snoozemenu.xml` [938,301][1080,449] (screen 1080 wide, button x=938),
`ui-915-presets.xml` [847,971][1132,1119] (screen 1133, button x=847), `ui-412-snooze.xml`
[847,443][1132,591]. `chooseSnoozeDateTime()` pops at
`root.mapToItem(Overlay.overlay, 0, root.height)` (button bottom-left) with **no clamping**
(CardActions.qml). The menu block is ~789 px tall (items span y 301..1090 in `ui-dump-snoozemenu`),
so a button bottom edge below ~1730 px on the 2516 px screen pushes later rows — the Custom row is
last — past the bottom edge. Whether clipped content was ever painted is not established (D1
ambiguity applies to these dumps too). This is a starting point for the pixel check, not a
proven defect.

## 4. Clause mapping — statuses preserved exactly as received (nothing relabelled)

| # | Clause | Status after this run | Change |
|---|---|---|---|
| 1 | Exact candidate installed (24/0.1-37, signer, newer) | PROVEN (predecessor §1) | none; re-read confirmed versionCode=24 / 0.1-37 / 19:35:27 |
| 2 | Fresh normal-UI enrollment → Online | PARTIAL — retained session only | none |
| 3 | Displayed sender avatars per identity | PROVEN (predecessor) | none |
| 4 | Archive A → body B → Undo → keep B, one chain | UNPROVED (fragmentary) | none |
| 5 | Physical Snooze preset tap persists until payload | UNPROVED | none — but its dump-based sub-evidence is reclassified ambiguous (D1); the failing input §4 of the old receipt stands as the first exact failure |
| 6 | Custom picker OK commits / Cancel no-op | UNPROVED | none — "picker never visibly rendered" also gains the D1 ambiguity note (instantiation ≠ open) |
| 7 | 412×915 and 480×915, 150 % font, 48 dp at density | PARTIAL (inbox level; 48 dp and menu-level unproved) | none |

## 5. Changed paths

**Zero product/repo files changed.** No QML, C++, docs, oracle or normative clause was edited —
no concrete product defect reached the fix threshold (D1 is an oracle/harness defect plus an
a11y-hygiene observation; D2 is unproved on-screen). Throwaway tooling only: host
`/tmp/diag-chain.sh` (bounded chain script, outside any repo), `/tmp/diag-pre*.xml`,
`/tmp/diag-ring-pre.txt`, `/tmp/diag-prestate.png`, and `/sdcard/diag-*.{xml,nodes,png}` on the
owned AVD. Normative clause R20 and its oracle were NOT touched.

## 6. Mail-state and safety

- This run issued zero `input` gestures. The retained real session is byte-for-byte untouched by
  me; the D1 ghost menu (built 20:40) predates this run and belongs to the predecessor crawl.
- No credentials read or printed; no real backend call made; no isolated-profile setup attempted.
- Owned AVD display untouched (override 1133×2516 @ density 440, font_scale 1.5 as received).

## 7. Retro (required) — what kept this run out of the tap loop

- The predecessor loop re-derived coordinates across shell invocations and trusted dumps as
  proof of rendering. This run made the guard **abort instead of tap** when the pre-state
  contained menu nodes (C2), which is what surfaced D1 in the first place. Detection came from a
  state readback, not from another tap.
- Device-side `grep -o` on single-line dumps is unreliable (C1 segfaults); host-side parsing of
  pulled XML is the working path and is what produced the D1 evidence.
- A successor must verify effects from pixels and payloads, never from dump presence, and must
  never tap a coordinate taken from a dump of a lazy Loader surface.

## 8. Next concrete proof (bounded, no crawl, real account excluded)

1. **Pixel-anchored menu-open check** on the owned AVD, zero mid-gesture dumps: tap the already
   mapped `More actions` button → `screencap` (is the sheet painted?) → tap the sheet's Snooze row
   (bounds are screen-centered and scroll-invariant) → `screencap` at +0.5 s / +2 s / +5 s.
   Outcomes discriminate: menu pixels never appear (never-open) vs appear then vanish
   (instant-close) vs persist (predecessor failures were coordinate staleness). This is one
   check, three screenshots, no preset/OK tap, no mail mutation.
2. **Geometry arm**: repeat at one low-anchor card (button bottom edge ≈ 2158/2342 px) to
   discriminate D2 bottom overflow (partial menu / Custom row never on screen) from D1.
3. **Commit-path proof** (clause 5/6 payload halves) must run on an isolated owned app profile
   with a private backend fixture — the real account is excluded; the physical-OK half mutates
   mail by design.
4. If the pixel check proves a product defect, the smallest canonical QML fix belongs to
   `qt/src/qml/CardActions.qml` (`chooseSnoozeDateTime` popup point clamping and/or deferring the
   popup until after `actionSheet.close()` completes); the failing-before/passing-after consumer
   proof then runs on a rebuilt APK — release rebuild stays with the next stage.

## 9. Artefact index

- Ring logcat (app PID 32311, 20:30–21:45): `/tmp/diag-ring-pre.txt` (225 lines; 40 cross-thread
  warnings; 0 TypeError)
- Pre-state screenshot 21:50: `/tmp/diag-prestate.png` (inbox dimmed under modal scrim, no menu
  pixels — the D1 pixel confirmation)
- Ghost-menu dump: `/sdcard/diag-pre-pf-probe.xml` (host copy `/tmp/diag-pre-pf.xml`)
- Chain script: `/tmp/diag-chain.sh` → `/sdcard/diag-chain.sh` (ran once, aborted by guard)
