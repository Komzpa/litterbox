# Original thread messages — desktop Qt consumer proof (FINAL, 2026-10-19)

Gate: OriginalMessage desktop consumer. ROOT=`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`;
RUNTIME sibling=`/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/original-message-proof` (abbrev **OMP**).
Verdict: **PASS** — the real frozen desktop app opens the multi-original thread at 1440×900 and
412×915 with sender metadata truthful per original; **each of the 3 older disclosure rows was really
tapped with the X11 pointer at both viewports and visually verified open (▶→▼ + own original body)**;
latest rendered once; single-message card shows zero disclosures; named B2B original renders
byte-identical (107 B, `fec8152d…c1`); fixture→API→app-SQLite→rendered bytes all equal; one-byte
negative detected. No source/build/Git/production-DB write/real-card mutation/shared installer occurred.

## 1. Frozen inputs (re-hashed this lane)

| Identity | Value |
|---|---|
| Server binary | `build/litterbox-server-a6cb742c`, SHA-256 `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e` (run only, port 18124, pid 1194 — pre-existing, reused after `ss`/`/v1/version` check) |
| Qt binary | `OUT/build/litterbox-qt`, SHA-256 `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` (run only, `--capture-script` to reach the reader + private HOME, Xvfb+dbus) |
| Fixture DB | private `litterbox_test_origmsg_20261019`, tenant `3b9d7e51-…`, token `litterbox-origmsg-proof-token` |
| Named source | `OMP/b2b-source/named-b2b-original.html`, **107 bytes**, SHA-256 `fec8152d95fb7035d39cd0856a7eab0db6ecd168ca3486958e998dc5336c83c1`; identity in `b2b-source/identities.txt` beside it |
| HEAD | `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` (production `a6` same blobs, test-only delta) |

## 2. Criterion → production entry / wiring → real consumer

| # | Criterion | Production entry + wiring | Real consumer evidence | Verdict |
|---|---|---|---|---|
| O1 | Multi-original thread opens with sender metadata at 1440 | Server `/v1/cards/{id}/body` → app `mail_bodies` SQLite → `MailDetailPage` `messages`/`documentHtml` → `MailBodyDesktop` WebEngine | `captures/1440/02-multi-collapsed-1440.png` (**inspected**): disclosure rows `▶ Ada Lovelace <ada@example.test> · 12 Oct 13:00`, `▶ Earlier message · 13 Oct 14:30` (empty-From truthful, no invented sender), `▶ <grace@example.test> · 14 Oct 16:00` (address-only truthful); latest `Linus Torvalds <linus@example.test> · 15 Oct 17:45` + `orig_code_latest = "ok"` rendered once (`latestOnce:1`); eval `details:3, pageMessages:4, documentChars:2039` | **PASS** |
| O2 | Same at 412 requested viewport | Same pipeline, `resize [412,915]` | `captures/412/02-multi-412.png` (**inspected**): same 3 disclosure rows + latest block with `orig_code_latest = "ok"` and `the ORIGM4 source` link; eval `details:3, latestOnce:1` | **PASS** |
| O3 | Each older disclosure really tapped open at 1440 AND 412 (real X11 pointer, screenshot-derived coords, visual before/after per row) | `threadHtml()` `<details class='lb-previous'>` rows in the live WebEngine document; taps via XTEST pointer at absolute content coords (own Xvfb :197, `xdotool mousemove … click 1` on the only visible `Litterbox` window) | 1440 (`captures/1440-tap/`, run exit 0): baseline pixel-scan of `#dce5e3` separator lines → rows y=[278,331,383], x=1091. Tap1 → `Y-tap1-check.png` (**inspected**): `▼ Ada Lovelace` + `First original from Ada` + `orig_code_ada`. Rescan after shift (LINES [155,252,423,475,527]) → tap2 (1091,449) → `Y-tap2-check.png` (**inspected**): `▼ Earlier message` + `Legacy original without From metadata: the ORIGM2 source`. Tap3 (1091,573) → `Y-tap3-check.png` (**inspected**): all three `▼`, grace body + `orig_code_grace`. App's own `T1/T2/T3-after-tap*.png` corroborate (T2==T3 md5, `fc103b08…`, row1 open). 412 (`captures/412-tap/`, run exit 0; `captures/412-row3/`, row-3 run exit 1 on harness tail only): U0 scan LINES [55,148,224,276,328]; tap (206,250) → `V-tap1-check.png` (**inspected**): `▼ Earlier message` + legacy body (mapping off-by-one learned from the observed effect, not assumed); tap (206,186) → `V-tap2-check.png` (**inspected**): `▼ Ada` + `orig_code_ada`, row2 still open; fresh run tap (206,302) → `W-tap3-check.png` (**inspected**) + app's own `W1-after-tap3.png` (**inspected**): `▼ <grace@example.test>` + `Address-only original from Grace` + `orig_code_grace`. Attempt-1 (`captures/1440-tap-attempt1/`) preserved, superseded. Missed taps honestly recorded: attempt-1 tap1 (unfocused window consumed the click), 412 estimated-y tap (573); both corrected with scanned coords and re-verified. The 412-row3 run's tail (Back + W2 shot) was lost when this lane stopped its owned Xvfb before the run exited; the tap verdict rests on W-tap3-check + W1, both secured beforehand. | **PASS** |
| O4 | Single-message card: zero disclosures, unknown senders truthful | Same wiring, `messages.length==1` → no `<details>` emitted | `captures/1440/04-single-1440.png` (**inspected**): `Marie Curie <marie@example.test> · 16 Oct 10:30`, body + `orig_code_single = "ok"`, **no** `▶` rows; eval `single: msgCount:1, details:0`; 412 identical (`details:0`) | **PASS** |
| O5 | Named B2B original renders exactly; empty senders preserved | Card `91000000-…000003`, message `da3aeae7-…` html = archived 107 B | `captures/1440/05-named-1440.png` (**inspected**): `Шаблон B2B-договора TwójStartup`, `14 Oct 14:19`, body title; eval `named: msgCount:1, sender0:" <>", details:0, hasNamed:true`, `namedBytes` == archived 107 B (`fec8152d…c1`); card sender `Aliaksandr Baranau` nowhere substituted into the message | **PASS** |
| O6 | Fixture→API→app-SQLite→rendered byte equality | `OMP/scripts/fixture.sql` → server serializer → `profile/cards.sqlite` `mail_bodies` → `cachedMailBody` → `documentHtml` | API==fixture True (4/1/1 msgs), SQLite==fixture True (4/1/1), rendered m0/m1/m2/latest strings == fixture; named SHA re-hash `fec8152d…c1`, 107 B | **PASS** |
| O7 | Changed-byte negative control | Same comparator, 1-byte mutation `orig_code_ada→orig_code_adb` | `mutated==fixture: False` (`dd7e2e39…` vs `1dce0ed3…`) — comparator detects a single changed byte | **PASS** |
| O8 | No source/build/Git/production-DB/shared-installer writes | Frozen binaries run only; private PG DB; private HOME/XDG per run; owned Xvfb :197 stopped afterwards | Capture runs exit 0 except `capture-412-row3.log` exit 1 (harness tail only — owned Xvfb stopped early after W-tap3-check + W1 secured; tap evidence complete); journal = only this receipt + OMP evidence (scripts, captures incl. `*-tap*`, logs); temp HOMEs removed; pre-existing server (pid 1194) and pre-existing Xvfb :193 left untouched; no ADB | **PASS** |

## 3. Outputs (hashes)

- Captures: `captures/1440/` 6 PNG, `captures/412/` 6 PNG (byte-proof runs) + `captures/1440-tap/` (T0–T4 + Y-tap1/2/3-check) + `captures/1440-tap-attempt1/` (preserved superseded attempt) + `captures/412-tap/` (U0–U4 + V-tap1/2/3-check) + `captures/412-row3/` (W0 baseline + W1-after-tap3 + W-tap3-check; W2 tail lost, see O3). Every quoted PNG inspected directly.
- Scripts: `scripts/origmsg-{1440,412}.json`, `scripts/origmsg-{1440,412}-tap.json`, `scripts/origmsg-412-row3.json`, `scripts/tap_driver.py` (lane-owned, heredoc-created).
- Logs: `logs/capture-{1440,412}.log`, `logs/capture-{1440,412}-tap*.log`, `logs/version.json` (`a6cb742c…`), `profile/cards.sqlite` (`mail_bodies` 3 rows).
- Byte proof: API==fixture True ×3, SQLite==fixture True ×3, named `fec8152d95fb7035d39cd0856a7eab0db6ecd168ca3486958e998dc5336c83c1` (107 B), negative mismatch `False` as required.

## 4. Restoration

Temp HOME dirs removed; owned Xvfb :197 stopped; no Qt processes left; pre-existing Xvfb :193 (pid 3680645)
and server pid 1194 (both not owned) untouched; user session/display/config untouched; no ADB.

## 5. Retro (required)

1. **The harness cannot click WebEngine content — so the taps went around it, truthfully.** `--capture-script` clicks resolve QML `objectName`s only; `mailDetailPage.bodyLoader` is a QML `id`, unreachable from eval (first 1440 run failed exit 4). Correction: capture-script drives navigation/screenshots only; a second channel (owned Xvfb + `xdotool` XTEST pointer at screenshot-derived separator coords) performs the taps, `import` screenshots verify each toggle. Both channels' evidence is kept; neither passes alone.
2. **Coords go stale after every expansion.** Opening a disclosure shifts all rows below it. Each tap re-scanned the separator lines from the latest screenshot; the one estimated (non-scanned) tap missed and was redone scanned. The 412 off-by-one (scan block started one separator too low) was caught by reading the check shot — row 2 opened instead of row 1 — and corrected, not hidden.
3. **Kept intact:** all native criteria + docs; no production source/Qt/server change, build, test suite, installer, deploy, Git mutation, ADB use, or shared-state write. Only this receipt plus owned OMP evidence changed.
