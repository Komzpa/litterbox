# Named B2B original — native Android proof (FINAL, 2026-10-19)

Gate: NamedB2B original in native Android. ROOT=`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`;
RUNTIME sibling=`/home/kom/proj/ai_pr/litterbox-native-proof-20261018/named-b2b-proof` (abbrev **NBR**).
Verdict: **PASS** — the exact archived 107-byte named original renders in the real native reader
with byte equality fixture→API→app-SQLite→actual-WebView-load, empty senders preserved, no
card-sender substitution, one-byte negative control detected. Generic touch/note-save/disclosure
proofs are reused from §11d/§11e/§11f/§11g, not re-executed. No production source, build, install,
Git, or shared-DB write occurred in this lane.

## 1. Exact input (immutable, re-hashed by this lane)

| Identity | Value |
|---|---|
| `fixtures/named-b2b-original.html` | **107 bytes**, SHA-256 `fec8152d95fb7035d39cd0856a7eab0db6ecd168ca3486958e998dc5336c83c1` (== §11k archive) |
| Card / message / thread | `f508a4fc-92ef-49b2-a06b-5ec11bb72bc0` / `da3aeae7-406a-4a0b-90cb-34cf87d1d2f3` / `1a139ec80dd50597`; single-message card |
| Card sender (card level, kept) | Aliaksandr Baranau `<abaranau@maumap.com>` |
| Original `sender_name` / `sender_address` | **both empty** — copied verbatim, never filled from card sender |
| `received_at` / `body_hash` | `2026-10-14 14:19:11+04` / `0375bbb171057b52a029265767143c70fcd00c31d13f5bcf63bf3cf79bc0c559` |
| Identity copies in NBR | `fixtures/{identities,card-row,msg-bodyhash,named-source-sha256,named-b2b-original.txt}.txt` |

## 2. Criterion → production entry / wiring → real consumer (named scope)

| # | Named criterion | Production entry + wiring | Real consumer evidence | Verdict |
|---|---|---|---|---|
| N1 | Exact named bytes reach the native reader | Server `/v1/cards/{id}/body` + snapshot (§11e pipeline, same frozen binary) → app `mail_bodies` SQLite → `InboxView` bundle row → `MailDetailPage` `bodyLoader` → `MailBodyNative.qml` → `MailView.loadMail` → `WebView.loadDataWithBaseURL` (`MailView.java:64–65`; wiring carried from §11 on the identical installed artifact, not re-derived) | `logs/named-b2b-compare-result.json` **pass:true** (exit 0), 16/16 checks: fixture 107B sha fec8152d… == API html == SQLite top/message html == `<section class='lb-original'>` slice of actual load doc; Qt doc == WebView doc (2284B, `eb33898961c52a758b3581120007ed49e2a429e969567113d91e89f830b76bd5`); single message; ids preserved; source_url equal with thread | **PASS** |
| N2 | Empty original senders preserved; no card-sender substitution | `messages.sender_name/sender_address` columns (021_message_sender) through API serializer → SQLite → load-doc header path | API `sender_name:"" sender_address:""` and SQLite identical (`body-before.json`, `cache-named.sqlite`); load doc contains **zero** occurrences of `abaranau` / `Aliaksandr Baranau` (checked in full 2284B doc and in original slice); inbox avatar `AB` is card-level attribution, not message From | **PASS** |
| N3 | Real UI opens the exact named mail at 412 requested viewport | Owned AVD `wm size 1133x2516` (override; density stays 2.75 → **412×915** logical) with app foregrounded in user 11 | Real inbox row tap at (511,461) on `Open Шаблон B2B-договора TwójStartup` [198,396][824,527] → reader; `screenshots/34-named-reader.png` (1133×2516, **inspected directly**): header subject + `14 Oct 14:19`, body title + body text, account-address fallback line; AX reader nodes corroborate | **PASS** |
| N4 | Rendered freshness, not fixture replay | Frida 16.7.19 observer called the originals with identical args and archived the **actual** UTF-8 load documents; screenshot is the live render of that same load | `dumps/webview-document-01.html` (MailView.loadMail) == `-02.html` (WebView.loadDataWithBaseURL) byte-for-byte; metrics logged (density 2.75/440); screenshot 34 shows the same document rendered | **PASS** |
| N5 | Changed-byte negative control | Same comparator, one-byte mutation `TwójStartup→TwójStartuQ` | Mutation hash differs from actual original slice; reported `negative_one_byte_change_detected:true`; comparator exits nonzero on any mismatch (helpers alone never claimed) | **PASS** |
| N6 | Unchanged APK / frozen server / no build-install-Git | APK `artifacts/litterbox-qt-v23-a6cb742c.apk` b186bf5e… v23/0.1-36 (re-hashed this lane; `install-existing --user 11` only, no reinstall/shared install); server `build/litterbox-server-a6cb742c` 6c23d73e… (re-hashed; run only) | `dumpsys package` versionCode 23/versionName 0.1-36 readback; server launch log pins binary hash; journal records zero build/test-suite/Git/installer/upgrader/`android-build.sh`/`--dry-run` invocations | **PASS** |
| N7 | Owned mappings before ADB; phone/TV untouched | qemu `litterbox-native-proof -port 5590` ↔ `emulator-5590` checked first; every ADB via `scripts/native.py -s emulator-5590`, journaled in `logs/commands.jsonl` | `192.168.100.39:5555` (TV) and `R5CX80MZ1SN` (phone) appear only in the read-only `adb devices -l` mapping line; zero targeted commands to either | **PASS** |
| N8 | Own isolated user/profile; user0 + display preserved and restored | Full secondary user **11** `NamedB2BProof20261019`; user-10 archived context untouched; normal UI enrollment (typed URL+invite, visible Enroll tap — §3 of incremental receipt for the entry recipe); `wm size reset`; `am switch-user 0`; user-0 app relaunch `Status: ok` | `screenshots/01-preserved-primary.png` (before) and `38-user0-ready2.png` (**inspected**: real Beta-Maps/Google/Bumble/Play/gody01/Konsole rows, 1080×2340, user 0); first post-restore capture was an empty QML-loading tree and was discarded, not used as oracle; user 11 retained as offline archived context | **PASS** |
| N9 | Fixture-only SQL; no production/real-card mutation | All rows live in private DB `litterbox_test_named_b2b_20261019`, tenant `c9200000-…0019`; sole card/message pair | Fixture SHA `52a92fdf…`; server serves only the isolated tenant; user-0 enrollment/cards never written (read-only inbox evidence); no note/archive/task op anywhere in journal | **PASS** |

Reused without re-execution (still valid on the same installed artifact): actual ≥48dp hit spans (§11d),
native note Save/persistence (§11e), generic 5-original byte/disclosure mechanism (§11f/§11g).

## 3. Outputs (hashes)

- Input: `fixtures/named-b2b-original.html`, 107B, `fec8152d…c1`.
- `logs/body-before.json` (API), `dumps/cache-named.sqlite` (`cc228633…`, app DB copy),
  `dumps/webview-document-01/02.html` (2284B, `eb338989…`), `logs/webview-observer.jsonl`
  (metrics+ready+2 load events), `logs/named-b2b-compare-result.json` (pass:true, exit 0),
  `screenshots/30-enroll-result.png`, `32-synthetic-inbox.png`, `33-inbox-412.png`,
  `34-named-reader.png` (inspected), `38-user0-ready2.png` (inspected), `logs/commands.jsonl`
  (every targeted ADB command + rc), `scripts/{fixture.sql,server.sh,compare-named.py,native.py,
  observe-webview.js,observe-webview.py}`.
- Reader display note (honest): with the original From empty, the reader header falls back to the
  account address line `darafei@maumap.com` plus `14 Oct 14:19`; the card sender is **not** injected
  anywhere in the load document (N2). No tolerance, padding, or source change was made for this.

## 4. Restoration (verified readbacks)

`wm size reset` → `Physical size: 1080x2340`; `am switch-user 0` → current user `0`;
`am start -W --user 0` → `Status: ok`; ready inbox (88 AX nodes, live rows, inspected screenshot);
only user-11 app force-stopped; owned `reverse tcp:18128` and `forward tcp:27045` removed
(both lists empty); observer exited 0; owned `lb-named-b2b-server` stopped; user 11 retained
offline; prior lane's inert device frida daemon (same pinned bytes `d27e9eba…`) left running,
untouched; real phone/TV never addressed.

## 5. Retro (required)

1. **The 93-minute source crawl taught the right lesson the hard way, and this lane redeems its
   tail.** The named R61 source was declared unavailable after a KB/filesystem-only search, then
   found by a bounded canonical DB lookup (§11k). This lane proves the newly exposed tail — the
   actual named bytes rendering natively — instead of relitigating the crawl. The correction path
   (lookup → archive → freeze → native render) is now fully closed.
2. **Pixel-mapped coordinates beat eyeballing.** ~30 min went to enrollment taps missing boxes
   (gaps, dropdown overlap, keyboard-up scroll shift). Detecting the white box spans from
   screenshot pixels gave exact geometry (server 800–930, invite 960–1090, device 1120–1250,
   platform 1280–1410) and unblocked the run; the y≈1300 tap that opened the Platform dropdown
   (then Back → inbox → form reset) is the mechanism behind the earlier "form reset" mystery.
3. **Gboard autocomplete is the enemy of URL entry.** Bulk `input text` + trailing SPACE lets the
   IME rewrite the field (`…18128tpht`, `…18128http`). The working recipe: select-all via
   `input keycombination 113 29` + DEL + `input text` + immediate Enter, no trailing space, TAB
   (never Next) to advance, Back only while the keyboard is visibly up.
4. **Kept intact:** all 18 native criteria (§7/§11h) and docs; no production source/Qt/APK change,
   build, test suite, installer, upgrader, deploy, Git mutation, sibling ADB use, or shared-state
   write. Only this receipt plus owned NBR evidence changed. No new campaign started; the
   isolated user 11 stays as an offline archived proof context.
