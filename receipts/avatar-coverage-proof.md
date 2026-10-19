# Avatar Coverage Proof — sender avatars on desktop and mobile (frozen candidate, real app)

Gate: AvatarCoverageProof. Run 2026-10-19 ~16:45-17:05 Asia/Tbilisi. No source edits, no Git mutation,
no deploy/install, no owner desktop/bus/phone, no shared installer. Native mobile (Android/iOS) is explicitly
out of scope here: all renders below are the real `litterbox-qt` desktop binary (system Qt); no cross-platform
claim is made — the native final cohort runs later.

## 0. Gate record

- Command (this gate): prove rendered sender-avatar coverage on the real app — standalone mail, bundle head,
  agent card, and a differing-identity bundle member — at desktop 1280/1440 (+900 wide-pane) and phone width 412,
  offline, with fresh screenshots plus pixel/bbox oracles.
- Working directory: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (ROOT; receipt only — code untouched).
- Artifact identity: frozen candidate source copy
  `/home/kom/proj/ai_pr/litterbox-avatar-coverage-proof-20261019/source` (rsync of ROOT minus `.git`,
  `receipts`, images) + isolated build `.../build/litterbox-qt`
  sha256 `f6e7b344f878a5c5df4dab6c4696d3d41f17d0e3e21ea35f48ed9d1b70b575e6`.
- Exclusive resources: private Xvfb displays (per-run `xvfb-run -a`), private `dbus-run-session` bus, fresh
  `mktemp` HOME/XDG per run, dummy profile server `http://127.0.0.1:18099` (never contacted — store kept
  offline). Distinct from ProveArchiveContinuation's frozen tree, build, ports, and profiles.
- Exit status: all 5 capture runs exit 0; `cmake` configure exit 0; `cmake --build` exit 0;
  scoped `ctest -R tst_inboxlayout` exit 0 (1/1 passed).
- Stale warnings: none. Source was frozen (sha256 match, below) before configure; the live tree was not
  touched during the proof runs. Log chrome (`xdg-desktop-portal-kde` crash noise, `org.freedesktop.systemd1`
  activation failures) is harness background noise under the private bus; app exit stays 0 with all shots written.

## 1. Source identity (read-only; nothing edited by this gate)

- Live ROOT HEAD `e62b81c8cd3b541949fe0aa35fee290979f46563`; working tree carries the RepairAvatarCoverage
  change set (uncommitted): `M qt/qml/InboxView.qml`, `M qt/pages/MailDetailPage.qml`,
  `M qt/src/qml/CardActions.qml`, `M qt/tests/tst_mailopen.qml`.
- Frozen copy `qt/qml/InboxView.qml` sha256 `ed6453aeb0b9fb78abcdb5955f428fad01365d505285a59283fc985943cd0e78`,
  byte-identical to the live file at freeze time (both hashes compared before configure).
- Avatar code lives in `InboxView.qml` only (helpers ~L137-150, `SenderAvatar` component ~L160-176,
  bundle-head instance ~L1516-1526, row-handle `Loader` ~L1667-1675). `MailDetailPage`/`CardActions` diffs are
  the lane's adjacent work, not avatar rendering; this proof renders inbox rows only.

## 2. Wiring under test (what the proof exercises)

- `avatarColor(name)`: deterministic hash over the lowercased name into an 8-entry palette
  (`#397d73 #5b6abf #b0592f #7a5aa6 #2f7fa8 #9a6a12 #a3465f #4f7d2f`); pure function of the name — offline-safe.
- `avatarInitials(name)`: strips `[bot]`, `@domain`/brackets, non-letters; first+last word initials, uppercased.
- `SenderAvatar`: circle (`radius = width/2`), white DemiBold initials; 32px when `compactInbox`, 28px otherwise;
  bundle head uses the same sizes (`compactInbox ? 32 : 28`).
- Bundle head: one `SenderAvatar` named by the first shared sender (`bundleSummary.senders[0]`), i.e. the R36
  shared identity rendered once; chevron `bundleToggle` (`arrow-right`/`arrow-down`, R36) sits in the same
  leading column on wide panes (`visible: !compactInbox`).
- Row handle: `dragHandle Loader -> SenderAvatar` named `mailSender(card)` for mail / `cardKindLabel(card)`
  for non-mail, but gated `active: cardRow.visible && !cardRow.bundled` with `dragHandle visible: !bundled`.
- Member sender text: `showMemberSender = !bundled || senders.length !== 1 || senders[0] !== mailSender(card)`,
  so a differing-sender member always shows its name — text only (see section 6 finding).

## 3. How it was proved (real consumer entry, isolated runtime)

Built the frozen copy with the proven kit and drove the production binary (not qmltestrunner) under Xvfb:

    cmake -S <frozen>/source/qt -B <proof>/build -DLB_TEST_PLATFORM=xcb   # exit 0
    cmake --build <proof>/build -j32                                       # exit 0
    H=$(mktemp -d /tmp/litterbox-avXXXX.XXXXXX)
    env -u WAYLAND_DISPLAY -u DISPLAY HOME="$H" XDG_CONFIG_HOME="$H/config" XDG_DATA_HOME="$H/data" \
      QT_QPA_PLATFORM=xcb xvfb-run -a dbus-run-session -- \
      <proof>/build/litterbox-qt --capture-script <proof>/profile <proof>/scripts/<run>.json <proof>/out-<run>

Fixture (seeded via the real `store.applyRemoteCards({now, later:[], missed:[]})` path, all 5 ids echoed back
by `store.cardIds()` on every run): standalone mail `av-standalone-1` (Alice Example), bundle
`sender:bundle@example.test` with leader `av-bundle-leader` (Bob Bundle), same-sender member `av-bundle-same`
(Bob Bundle), differing-sender member `av-bundle-diverge` (Carol Diverge), agent card `av-agent-1`
(`source: agent` -> kind label "Agent"). `store.online` false for the whole render (offline banner on screen).

| Run | Script | Exit | Shots |
|---|---|---|---|
| desktop-1280x800 (compact pane) | `scripts/desktop-1280.json` | 0 | 01-collapsed, 02-expanded, 03-top |
| desktop-1440x1000 (compact pane) | `scripts/desktop-1440.json` | 0 | same 3 |
| phone-412x915 | `scripts/phone-412.json` | 0 | same 3 |
| desktop-900x800 (wide pane, `compactInbox:false`, `listWidth:720`) | `scripts/desktop-900.json` | 0 | 01-wide-collapsed, 02-wide-expanded |
| desktop-1440 chevron/handle toggle | `scripts/chevron-1440.json` | 0 | expanded, recollapsed |

## 4. Behavior proof — what the screenshots show (certifier-opened PNGs)

Collapsed inbox (all densities) renders, top to bottom: navy rail with Inbox isolated-pill; search field;
"Now · Yesterday" section; **`AE` rust circle + "Standalone mail from Alice / Alice Example / Standalone mail
snippet..."**; **`BB` ochre circle + "Bundle latest from Bob / 3 emails chip / Bob Bundle, Carol Diverge ·
bundle@example.test / Newest bundle mail."`** (ONE avatar for three emails — R36 shared identity once);
"Earlier" section header; **`A` ochre circle + "Agent report / Agent · agents / Agent summary text."**;
light **"Can't reach the server — new mail will arrive when you're back online. [Retry]"** banner (offline
render proven, avatars unaffected). Phone 412 shows the same three circles at 32dp with identical text;
detail crops (`crops/d1280-head-avatars.png`, `crops/p412-head-avatars.png`, `crops/d1440-*.png`) show crisp
circles with legible white initials.

Expanded inbox (all densities): the three member rows appear — "Bundle latest from Bob / Bob Bundle",
"Earlier bundle mail from Bob / Bob Bundle", **"Bundle mail from Carol / Carol Diverge /
Differing-sender member."** — with sender-name text on every member and **no avatar circle on any member
row** (see section 6). Toggle-back run: `bundleExpanded` false->true->false across two `bundleToggle` clicks
with `visibleRows` back to 3 — chevron discoverability + expand/collapse behavior proven on the real control.

Eval oracles (identical on all 5 runs): `cards` = all 5 ids, `rows` = 5; leader `{visible:true, leader:true,
bundled:true}`; members all `{bundled:true, showSender:true, handle-registered:true}`;
`avatars` = Alice `["AE","#b0592f"]`, Bob `["BB","#9a6a12"]`, Carol `["CD","#9a6a12"]`, agent `["A","#9a6a12"]`,
`aliceAgain` same color (deterministic re-query); `offline:true`. Standalone/agent dragHandles
`visible:true`; bundle-head handle `visible:false` (bundled) — its circle comes from the head instance, not
the handle. (The `store.online=false` seeding eval returns `false` because the store isn't open yet at step 2;
seeding at step 3 returns `true` and the closing `offline:true` eval + on-screen banner prove the offline state.)

## 5. Pixel/bbox oracles (Pillow connected-component analysis, tolerance 24)

Avatar-color blobs (>200px) per shot — bboxes `(x0,y0,x1,y1,px)`:

- desktop-1280 collapsed: rust `(204,189,235,220,679)` = AE 32x31; ochre `(204,421,235,452,708)` = agent A,
  `(196,255,227,286,658)` = BB head (starts x196: head instance at toggle x:0). Expanded: same 3 blobs
  (agent shifts to y653 as members insert); member band contains only <=2px text-AA specks — **zero circles**.
- desktop-1440: identical blob structure (AE 679px, head 658px, agent 708px); expanded again 3 blobs, none
  in member rows.
- phone-412: AE `(24,141,55,172,679)` 32x31, BB `(16,207,47,238,658)`, agent `(24,373,55,404,708)`; expanded
  keeps exactly these 3, members none.
- desktop-900 (non-compact): rust `(216,231,243,258,499)` = AE **28x27**; ochre head `(216,315,243,342,478)`
  **28x27**, agent `(216,415,243,442,528)`; expanded still exactly 3 blobs. So 28dp desktop / 32dp compact+phone
  sizes proven in pixels, and member rows are avatar-free in both densities.

Residual specks (6px at the search rail, 1-2px text AA) are chrome/text antialiasing, not circles — verified by
size (<10px vs 478-708px avatar blobs).

## 6. FINDING (flagged, not accepted): differing-sender member renders no avatar

`av-bundle-diverge` (Carol Diverge) expanded shows its sender name (`showMemberSender:true`, text on screen)
but **no `CD` circle**: the writer kept the `!bundled` gate on both `dragHandle.visible` and the avatar
`Loader.active`, so *no* expanded member — same-sender or differing — ever renders `SenderAvatar`. The shared
head keeps showing `BB` alone while a Carol row sits beneath it. This is exactly the `!bundled`-omits-avatar
risk: do not accept "shared identity once" as covering it without an explicit owner decision — either render
a per-member avatar where `showMemberSender` is true, or spec members as text-only and close this gate
accordingly. Everything else in this proof passes.

Observation (not a failure): Bob Bundle, Carol Diverge, and the Agent label all hash to `#9a6a12` — stable and
deterministic across runs, but three adjacent rows share one color. Palette size vs sender count is a design
trade-off for the lane owner.

## 7. Scoped existing check

`ctest --test-dir <proof>/build -R tst_inboxlayout` (Xvfb, private HOME/bus): **1/1 passed, 5.10s, exit 0** —
the avatar column does not regress the owned inbox-geometry suite. No other suites run (siblings own the live
tree; full validation is Main's job after landing).

## 8. Retro

- Freezing the candidate *before* configuring (with a hash equality check) made every later number
  attributable; do it first, every gate.
- Reusing the production `--capture-script` path instead of a new harness paid off: real seeding
  (`applyRemoteCards`), real toggle, real offline banner — no fixture-specific test doubles to doubt.
- Counting pixels alone nearly produced a vacuous oracle (identical totals across sizes are *expected* for
  fixed-size avatars); connected-component bboxes (position + size + count) are what actually prove
  presence, size variant, shared-once, and member absence. Lead with blobs next time.
- The `!bundled` finding was visible in code within minutes but only became a *fact* after the expanded-shot
  crops + blob scan — keep the rule: code reading directs, renders decide.

Changed: this receipt only (`receipts/avatar-coverage-proof.md`). Evidence kept at
`/home/kom/proj/ai_pr/litterbox-avatar-coverage-proof-20261019/` (`source/`, `build/`, `profile/`,
`scripts/*.json`, `out-*/`, `crops/`, `run-*.log`, `scope-inboxlayout.log`). What remains: owner decision on
section 6 (member avatar vs text-only spec), native mobile cohort, Main's full validation after landing.
