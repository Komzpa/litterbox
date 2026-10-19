# UI release proof — receipts/ui-release-proof.md

Gate: `ReleaseConsumerCompletion`. ROOT=`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (verified present; no replacement checkout).
Status: **DONE — installed state verified read-only + real older-consumer update offer observed. No mutation of installed state, no rollback/reapply/restart.**

## 0. Standing decisions honored

- Failed CLI receipt (`/var/backups/litterbox/release-20261019-a6cb742c/receipt.json`, `status:failed`, `error:release endpoint unreachable`) **stays failed**; never upgraded, never retried into the same receipt dir, no re-apply, no rollback (Main decision: KEEP verified working installed state).
- Owner desktop NEVER restarted/driven; still zero `litterbox-qt` processes (package-installed, not running — unchanged claim).
- Owner phone/TV untouched; no `LB_SERVER`/pre-enrolled shortcut; no owner-phone/TV ADB commands (device list observed read-only only).
- Owned AVD emulator-5590 preserved: still v23/0.1-36, user0 intact, display 1080x2340 re-verified after the run.
- Secrets: token/DSN/env/invite-code contents never printed here or in chat; invite code lived in a 0600 file and was shredded after use.
- No source/Git/build/matrix; no shared-installer invocation (not even `--dry-run`); no raw-SQL beyond the documented test-enrollment invite/device rows (recorded with scope + rollback below).

## 1. Independent installed readback (read-only, this lane)

| Check | Result |
|---|---|
| `dpkg -l` | `litterbox-server 0.1-36 amd64`, `litterbox-qt 0.1-36 amd64`, both `ii` |
| `/opt/litterbox/server/litterbox` | `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e` == candidate |
| `/usr/bin/litterbox-qt` | `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` == proven production binary |
| Immutable APK v23 | `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` (21 237 919 B) == candidate |
| Old APK preserved | `16b5873f675a6c6d54a5280b7d022638b4aefd0058b40b9b5e7cb046aeaf0b75` == before bytes |
| Service | `active`, `MainPID=759134`, `ExecStart=/opt/litterbox/server/litterbox -listen 127.0.0.1:8081`, start `2026-10-19 07:22:55 +04` (unchanged since apply; no restart by this lane) |
| Live `server_build` (anon, all 3 origins) | `a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75`, HTTP 200 on `127.0.0.1:8081`, `192.168.100.74:8081`, `https://komzpa.net:9443` |
| Anon protection (all 3 origins) | `/v1/android/update` 401 + `/v1/android/update.apk` 401 |
| Auth manifest (token in-process only, all 3 origins) | 200 `{version:23, package:org.qtproject.example.litterbox_qt, sha256:b186bf5e…}`, download 200, 21 237 919 bytes, sha256 MATCH on every origin |
| Owned AVD emulator-5590 | `versionCode=23 versionName=0.1-36`, `wm size 1080x2340`, qemu `-avd litterbox-native-proof -port 5590` untouched |
| Desktop running state | zero `litterbox-qt` processes (only the `dpkg-query` observer matched) |

One accidental self-inflicted event, immediately repaired: a `--help` probe of the installed server binary spawned a stray listener on `:8080`; killed within the same lane (`ss` confirms no `:8080` listener). Production `:8081` listeners never dropped (re-verified 200 on all origins after).

## 2. Real older-consumer update offer (fresh isolated AVD, normal UI enrollment)

- Fresh owned AVD `litterbox-updatecheck` (cloned from `litterbox-proof` config, swiftshader, port 5612) — never held v23, so no downgrade anywhere; emulator-5590 untouched.
- Installed exact old bytes `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk` → `versionCode=21 versionName=0.1-34` confirmed on the fresh AVD.
- Normal UI enrollment: **Enroll device → Connect to Server**, typed server URL + single-use invite + device name `UpdateCheck-v21`, tapped **Enroll**. Result: **Online · changes sync across devices** with live real inbox (Bumble/Google Play/Google rows) — `receipts/update-offer-proof/01-enrolled-online-v21.png` (`dc64f9ae…`).
- **Check updates → Update available v23**: status banner **"Update 23 downloaded and verified"** + dialog **"Install Android update — Version 23 is downloaded and checksum-verified. Android will ask you to confirm installation."** with Later/Install — `02-update23-offer.png` (`bffb9727…`).
- **No installer/safety prompt approved**: tapped Later (missed the Qt-drawn button; dismissed via Back — approves nothing), banner persists, package still `versionCode=21` after — `03-update23-banner-later.png` (`34ec0383…`).
- Bounded deviation, honest: enrollment used LAN origin `http://192.168.100.74:8081`, NOT the intended `https://komzpa.net:9443`. Cause: the v21 app's enroll flow reports "Can't reach the server" for the https origin even though the same emulator reaches it fine (Chrome on the same AVD rendered `{"server_build":"a6cb742c…"}` from `https://komzpa.net:9443/v1/version` — `00-emulator-https-origin-reachable.png` `3fb9f7d6…`). The served update manifest/bytes are proven byte-identical on all three origins (§1), so the v23 offer observed over LAN is the same production payload; the v21-app-vs-https gap is app-side (old Qt build) and is NOT waived — recorded here.
- Test-enrollment scope + rollback (production tenant `3b09aac5-…f8bc`): one invite consumed by device `UpdateCheck-v21` (`eeb6daae-…`, created 07:51:41+04); device **revoked** after proof; 5 unused invite rows deleted; invite code shredded. No owner devices touched (prior test devices were already revoked by their own lanes).

## 3. Acceptance entries (clause → production entry/wiring → real consumer + installed bytes)

1. Server `main.buildSHA=a6cb742c…` installed AND serving on all origins (§1) — behavior at same source proven by `repaired-consumer-proof.md` §4 SSE PASS. DONE.
2. Desktop `ead409d5…` installed (§1) — proven by `original-message-desktop-proof.md` FINAL PASS + `repaired-consumer-proof.md` §5 + `bundle-frame-consumer-proof.md` 40/40. DONE (package-installed; owner app never restarted per standing rule).
3. Android `b186bf5e…` v23 served on all origins with anon 401 (§1) AND installed on owned AVD (§1) — proven by `mobile-final-consumer-proof.md` §11 + `named-b2b-native-proof.md` FINAL PASS on exact bytes. DONE.
4. Older normally-enrolled isolated consumer Check updates → Update available v23 (§2 + PNG hashes). DONE (bounded origin deviation recorded).
5. Negative anon401 on all origins (§1). DONE.
6. Restoration: emulator-5590 v23/user0/1080x2340 intact; temp AVD stopped (`adb devices` shows only TV + emulator-5590); desktop untouched; failed CLI receipt preserved failed. DONE.

## 4. Done / left

- DONE: everything above; receipt dir `receipts/update-offer-proof/` (4 PNGs + hashes in §2).
- LEFT (not this lane): Main disposition of failed receipt (already decided KEEP); docs/Git owner commit + publication (separate owner, fresh tuple read); optional future re-observation of the v23 offer over the https origin from a newer client.

## 5. Retro (required)

- Transient-restart-readback lesson held: failed CLI receipt never converted; all evidence re-read independently in this lane with exact hashes.
- Prior late-handoff lesson held: screenshots archived to ROOT immediately after capture (`sha256sum` at copy time), receipt written same lane.
- New lesson: Qt-drawn dialogs are invisible to uiautomator and Qt text fields fight `adb input` appends — right-edge tap + forward/backward delete + select-all-popup sequence works; recorded for reuse.
- Criteria/docs/Git preserved; no scope creep (no note-save/fixture work attempted); owner state untouched throughout.
