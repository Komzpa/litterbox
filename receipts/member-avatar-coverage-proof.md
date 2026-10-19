# Member Avatar Coverage Proof — bundled member sender circles (frozen candidate, real app)

Gate: MemberAvatarCoverageProof (RepairMemberAvatarCoverage row). Run 2026-10-19 ~17:26-17:44 Asia/Tbilisi.
No source edits, no Git mutation, no deploy/install, no owner desktop/bus/phone. All renders are the real
`litterbox-qt` desktop binary (system Qt) under Xvfb; **no cross-platform claim is made — the final native
mobile cohort runs later** (unchanged from the prior avatar gate; this receipt only extends the desktop
proof to the new member-band circles).

## 0. Gate record

- Command (this gate): prove that every *displayed* sender-identity line on a bundled inbox member now
  paints that sender's avatar circle in the existing leading gutter — Carol (differing-sender member)
  circle present and unclipped at 1280/1440/412 (and 28px at 900 wide-pane) — while identity omission,
  head/standalone/agent avatars, row height and row order are unchanged, and collapsed state leaves no
  residue.
- Working directory: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (ROOT; this receipt only —
  code untouched; `git status` on `qt/qml/InboxView.qml` still shows the lane's pre-existing `M` from the
  working tree, not from this gate).
- Artifact identity: new frozen candidate
  `/home/kom/proj/ai_pr/litterbox-member-avatar-coverage-proof-20261019/source` (rsync of ROOT minus `.git`,
  `receipts`, stray `qt/mobileux-*.png`); frozen `qt/qml/InboxView.qml` sha256
  `19deee88a91c2e1905fb809938bf192d9f6b45ff47919aeb1edad3cbcff44811` — byte-identical to the live file at
  freeze time; built binary `.../build/litterbox-qt` sha256
  `03c964b90c3b135deb05facd256c174ffdeb289b55930780679c6ca23d3a20c6` (cmake configure exit 0, build exit 0).
- Exclusive resources: private per-run Xvfb (`xvfb-run -a`), private `dbus-run-session` bus, fresh `mktemp`
  HOME/XDG per run, the kit's own test profile (`profile/profile.json`, dummy server
  `http://127.0.0.1:18099`, never contacted — store kept offline). Distinct tree/build from the prior
  `litterbox-avatar-coverage-proof-20261019` kit; **that kit's frozen build was used read-only once** as the
  pre-avatar A/B baseline (section 6). No old receipt or old run output was rewritten.
- Exit status: all 5 capture runs exit 0 (desktop-1280 3 shots, desktop-1440 4 shots incl. recollapsed
  negative control, phone-412 3, desktop-900 2, omit-1440 2); scoped `ctest -R tst_inboxlayout` exit 0,
  1/1 passed, 5.99s.
- Warnings: two harness-level retries happened *before* the recorded runs and are superseded: (1) passing
  the tmp-HOME as `--capture-script` profile dir fails ("Test profile requires an existing directory
  containing profile.json") — fixed by pointing at the kit's `profile/`; (2) `item.mapToItem(window,…)`
  inside `eval` steps is rejected by Qt 6 ("Passing incompatible arguments to C++ functions from JavaScript
  is not allowed") — scripts were regenerated with `mapToItem(null,…)`, which for this window is the same
  coordinate space (verified once: null-target `(180,140)` == manual parent-chain sum `(180,140)`, depth 10).
  Final runs carry neither error; app exit 0 with all shots written. Log chrome (portal/systemd/atspi noise)
  is the usual private-bus background.

## 1. Source identity (read-only; nothing edited by this gate)

- Live ROOT HEAD `e62b81c8cd3b541949fe0aa35fee290979f46563`; working tree carries the lane's change set
  (uncommitted): `M qt/qml/InboxView.qml` plus the lane's adjacent files.
- The member-avatar hunk in the frozen copy is exactly one contiguous insertion after `mailSenderLabel`
  (`qt/qml/InboxView.qml` ~L1766-1784, one added `Loader` block + comment; diff against the prior kit's
  frozen pre-avatar file shows only this hunk plus the lane's already-present archive-continuation edit
  ~L430-449, which does not touch list layout).
- Wiring under test (production code, not a harness double):
  - `Loader { objectName: "memberSenderAvatar-" + cardId; anchors.right: parent.left; anchors.rightMargin:
    Kirigami.Units.smallSpacing; anchors.verticalCenter: parent.verticalCenter;
    active: cardRow.visible && cardRow.bundled && mailSenderLabel.visible;
    sourceComponent: SenderAvatar { width: window.compactInbox ? 32 : 28; height: width;
    name: window.mailSender(card) } }`
  - It lives inside the sender label (so it can only ever accompany a *visible* sender text line), hangs
    left of the text in the row's existing leading gutter via anchors (no `Layout.` participation → cannot
    affect row geometry), and is a **separate object from `dragHandle`** (~L1649, whose `Loader ->
    SenderAvatar` stays `active: visible && !bundled`). R36 head behavior, R38 hidden member reorder
    handle, and the `showMemberSender` omission rule are the untouched priors.

## 2. Fixture note — mixed bundle, do not assume omission

The current fixture's bundle `sender:bundle@example.test` has **two** distinct senders (Bob Bundle, Carol
Diverge), so `showMemberSender` is true for *every* member, including same-sender `av-bundle-same`.
The acceptance oracle therefore is: **every displayed sender line carries its avatar** (leader, same,
diverge — three circles in the bundle band), and the single-identity omission is proven separately with a
new all-Solo-Sam bundle (run `omit-1440`), not assumed from the mixed fixture.

## 3. How it was proved (real consumer entry, isolated runtime)

Same production `--capture-script` path as the prior gate: seed via real `store.applyRemoteCards({now,
later:[], missed:[]})`, `store.online=false` for the whole render (offline banner visible), real
`bundleToggle` click, shots + `eval` geometry probes on the live item tree (rows/labels/loaders walked by
`objectName`).

| Run | Script | Exit | Shots |
|---|---|---|---|
| desktop-1280x800 (compact) | `scripts/desktop-1280.json` | 0 | 01-collapsed, 02-expanded, 03-top |
| desktop-1440x1000 (compact) | `scripts/desktop-1440.json` | 0 | same + **03-recollapsed** (negative control) |
| phone-412x915 | `scripts/phone-412.json` | 0 | 3 shots |
| desktop-900x800 (wide pane, `listWidth:720`, `compactInbox:false`) | `scripts/desktop-900.json` | 0 | 01-wide-collapsed, 02-wide-expanded |
| omit-1440: mixed bundle **+** single-identity Solo Sam bundle | `scripts/omit-1440.json` | 0 | 01-collapsed, 02-expanded-both |
| A/B baseline: same 1440 script against prior kit's pre-avatar binary | (read-only reuse) | 0 | `out-ab-oldbuild/` |

## 4. Wiring proof (eval oracles on the live item tree)

- Mixed fixture, expanded: all five rows visible in original order
  `["av-standalone-1","av-bundle-leader","av-bundle-same","av-bundle-diverge","av-agent-1"]`, count 5 —
  **no reorder**; identical on 1280/1440/412/900.
- Every displayed member sender line has an instantiated avatar (compact runs, window coords):
  leader `{label "Bob Bundle" (252,405), av active, item 32x32 @ (216,397)}`; same-sender Bob
  `{label (252,500), av 32x32 @ (216,492)}`; **Carol diverge `{label "Carol Diverge" (252,568),
  av active, item 32x32 @ (216,560)}`**. Phone-412: Carol label (72,512), av 32x32 @ (36,504).
  desktop-900 wide: avatars **28x28** (Carol @ (242,635)) — both size variants exercised.
- Identity omission preserved (omit-1440): the single-identity Solo Sam bundle's leader and member both
  report `showSender:false, labelVisible:false, avActive:false, avHasItem:false` — the R36 shared-identity
  rule still hides the redundant line, and the new Loader follows the label into inactivity (no orphan
  circle).
- Drag semantics untouched: prior `cardHandles` oracle still holds (standalone handle visible, bundled head
  handle hidden); the member avatar is a distinct object that never registers as a handle.

## 5. Pixel proof (Pillow connected components, tolerance 24, >200px)

Expanded 1440 avatar-color blobs `(x0,y0,x1,y1,px)`:
`(204,189,235,220,679)` rust AE standalone · `(196,255,227,286,658)` head BB ·
`(216,397,247,428,658)` leader-band BB · `(216,492,247,523,658)` same-band BB ·
**`(216,560,247,591,664)` Carol-band CD — inside the Carol row band y 540–612, full 32×31 disc, not
clipped** · `(204,653,235,684,708)` agent A. Expanded 1280 is identical (Carol `(216,560,247,591,664)`);
phone-412: Carol `(36,512,67,543,664)` inside band 484–556; desktop-900: Carol `(242,635,269,662,484)`
— 28×27 disc. Blob pixel counts (658-664) match the proven head/standalone circles of the prior gate, so
the member discs paint at full size, not a clipped sliver.

Negative controls — collapsed state shows exactly the three pre-existing circles and **zero** member-band
residue: `01-inbox-collapsed` (all densities) and `03-recollapsed` (1440, after a real expand→collapse
toggle) each contain only AE/BB-head/agent blobs; the member-band y-ranges hold nothing above 200px.
The omit run's `02-expanded-both` blob scan likewise finds no circle anywhere in the Solo band (y 711–981)
— omission leaves no painted residue.

Crops (certifier-opened): `crops/d1440-diverge-member-band.png`, `crops/d1280-diverge-member-band.png`,
`crops/p412-diverge-member-band.png`, `crops/d900-diverge-member-band-28.png`,
`crops/d1440-recollapsed-negative.png`, `crops/d1440-solo-omission.png`.

## 6. No added row height / no reorder (A/B against the pre-avatar build)

Same 1440 script run against the prior kit's frozen pre-avatar binary (`out-ab-oldbuild/`): row geometry
is **identical** — rows `{148,99},{247,198},{445,95},{540,72},{612,99}` (y,h) on both builds, same label
positions — while the old build reports `av:null` for every member and the new build reports 32x32 items
at the same label rows. The circles are purely additive paint in the existing gutter; row height, band
heights and row order are unchanged.

## 7. Scoped existing check (geometry question only)

`ctest --test-dir <proof>/build -R tst_inboxlayout` (Xvfb, private HOME/bus): **1/1 passed, 5.99s, exit 0**
(`scope-inboxlayout.log`) — the gutter-anchored Loader does not regress the owned inbox-geometry suite.
No other suites run (siblings own the live tree; full validation is Main's job after landing).

## 8. Retro

- The prior gate's fixture hid the interesting case behind `showMemberSender`: with 2 senders every member
  shows a line, so "all displayed lines have avatars" — not "Carol has one" — is the honest oracle; the
  single-identity omission needed its own bundle fixture. Read the visibility formula before naming the
  acceptance case.
- `mapToItem(window,…)` from capture `eval` steps dies in Qt 6's argument conversion; `mapToItem(null,…)`
  is the same window space here and worked. One tiny probe with a hand-rolled parent-chain sum confirmed
  equivalence before regenerating all five scripts — verify the coordinate-space assumption once, cheaply.
- Reusing the prior kit's pre-avatar binary as an A/B baseline turned "no added row height" from an
  argument about QML anchoring into a measured equality of row geometry across builds. Frozen binaries are
  evidence, not just artifacts — keep them.
- One failing script step aborts the remaining steps (shots taken before it survive), which made the exit-4
  runs diagnosable from `run-*.log` grep alone: "Capture script step N failed" + the TypeError is the whole
  story. Log the eval payload, not a screenshot, for wiring claims; pixels only where pixels decide
  (presence, size, clipping, residue).

Changed: this receipt only (`receipts/member-avatar-coverage-proof.md`). Evidence kept at
`/home/kom/proj/ai_pr/litterbox-member-avatar-coverage-proof-20261019/` (`source/`, `build/`, `profile/`,
`scripts/*.json`, `out-*/`, `crops/`, `run-*.log`, `scope-inboxlayout.log`, `build-*.log`). What remains:
final native mobile cohort, Main's full validation after landing.
