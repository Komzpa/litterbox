# Unified release execution — receipts/unified-release-execution.md

Gate: `UnifiedReleaseExecution`. ROOT=`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`, HEAD `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` (production freeze `a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75`; HEAD delta test-only `qt/tests/tst_bundleexpand.qml`, per frozen-release-cohort §1). No replacement checkout; no matrix/source-crawl/build restart.

Status: **PLAN VALIDATED (`validated_dry_run`) — NOT yet applied/installed. No installed state mutated by plan.**

## 0. Canonical contract read (SHA-verified)

- `docs/DEPLOYMENT.md` sha256 `92b2d277a8d7bc365e1e4014e43c68a24c8cbadb37b4ee4faeae50464e44872a` — MATCHES frozen-release-cohort §1 and readiness §0.
- `deploy/release.py` sha256 `e2b1c060cdd60b81723dc68b44d7f0d62bf3350df5328985c0dc162cd0b57b5a` — MATCHES frozen §1; `--help` + `validate()`/`proof_gates()` source read; surface exactly `plan`/`apply`/`rollback` + `--receipt`/`--install-mobile`/`--adoption-timeout`; no other flag invented.
- `receipts/release-plan-readiness.md` sha256 `6d9e9395d6dea3e06b610862f3c98f5b67f50e51ac7ec10c20981365a2ccfc0b` — MATCHES brief.
- `receipts/frozen-release-cohort.md` sha256 `7336c3c24d23ee11b675ff9b2a53d134a153a1c612c536fb875019d2e25c57d2` — MATCHES brief.
- `receipts/release-git-preparation.md` sha256 `18960dca355aaea384aac3ff87e366bf47ed3e31e21b8bc8cb813260bbbae38e` — MATCHES brief.

## 1. Live target re-verified before mutation (both canonical mappings)

- `systemctl show litterbox-server.service`: `ActiveState=active`, `MainPID=1364707`, `ExecStart={ path=/opt/litterbox/server/litterbox ; argv[]=... -listen 127.0.0.1:8081 }`, `FragmentPath=/etc/systemd/system/litterbox-server.service`, `DropInPaths=/etc/systemd/system/litterbox-server.service.d/embeddings.conf /etc/systemd/system/litterbox-server.service.d/gmail.conf` exactly; `EnvironmentFiles` = `/etc/litterbox/db.env` (×2 ordered sources), `server.env.d/ollama-embeddings.conf`, `/etc/litterbox/gmail.env`.
- Unit/drop-in hashes: unit `dd3edbb60dc053479725c857a026bfa5630fdb60a2a52314cdb72fa11b66dc3a`, embeddings `bb554e2f7deb0f2c5efcb5ff58fe2379ded174f0d71800d7008e0cede26f3eee`, gmail `0c0440e3e058835a3db49c405d8b5caf33d6536bc572257b8a504e0835ebf8c4` — all MATCH frozen manifest and cohort `drop_ins`.
- Listeners: `127.0.0.1:8081`, `192.168.100.74:8081` (LAN proxy), `127.0.0.1:5435`.
- `/etc/litterbox/db.env` sha256 `5cd8efeb492ef657e397ad07649254f7495f6d2a249f6e6f2b7b938176bba0f0` (via `sudo sha256sum`; contents never printed) — MATCHES `publisher-inputs.json` `environment_sha256`. DSN shape `postgresql://<redacted>@127.0.0.1:5435/litterbox` re-verified by readiness lane; both mappings confirmed: server `127.0.0.1:8081`, DB `litterbox@127.0.0.1:5435`.
- Migrations 020/021 + 011 grants: already applied and read-only verified in `release-install-preflight.md` §5 (C1 columns `bundles.title_origin`/`messages.sender_name`/`sender_address NOT NULL DEFAULT ''`, C2 `litterbox_app` `SELECT,INSERT,UPDATE,DELETE` on `card_bodies`, C3 RLS enabled+forced `card_bodies_tenant_policy`, C4 counts 4833/1963/433, C5 sender defaults, C6 020 preserved). **NOT replayed here; CLI runs no SQL.**

## 2. Frozen cohort (three artifacts, same source)

Source `a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75`. All re-hashed this lane:

| Artifact | Path | sha256 | Identity |
|---|---|---|---|
| server deb 0.1-36 | `/home/kom/proj/ai_pr/litterbox-release-cohort-20261019/litterbox-server_0.1-36_amd64.deb` | `9b3bb149063ca2df3031811108b8783b58d0cb22383800543e81b808be484b4a` | inner binary `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e`, stamp `main.buildSHA=a6cb742c…` |
| desktop deb 0.1-36 | `.../litterbox-qt_0.1-36_amd64.deb` | `cdf940878aa64679e4bd2069877591f869daafd72444cdd7010f9c6b3964d217` | inner `/usr/bin/litterbox-qt` `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` == proven production binary |
| android APK v23/0.1-36 | `.../litterbox-qt-v23-a6cb742c.apk` | `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` | package `org.qtproject.example.litterbox_qt`, versionCode 23, signer `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37`, 3 TLS members present |

Producer manifest `cohort-hash-manifest.json` sha256 `fef8e33f39747b2a22947d16d1127f7e488bbd6e632ad84a5c949ad4faa7f1fd`.

## 3. Rollback prerequisites (exact installed state)

- Installed (pre-apply): `litterbox-server 0.1-34 amd64`, binary `8dfe79fc8f601c97367f525a97942e37ad0a29781cd336032171b9303784d8ea`; `litterbox-qt 0.1-34 amd64`, binary `d8ef906f335a1dff19ba64f10e25bd59c3beb0ec2777e4881d4e99173e086904`.
- Server rollback deb (canonical reconstruction from exact installed binary): `/home/kom/proj/ai_pr/litterbox-release-plan-20261019/litterbox-server_0.1-34_amd64.deb` sha256 `45c1587586c5a2407edf0b133703e3a639a8c92822f7707c1d02599aa0ee864b` (fields `litterbox-server 0.1-34 amd64`, inner == installed, unit `cmp`==live, postinst `cmp`==canonical, scripts `control`+`postinst` only).
- Desktop rollback deb (known artifact): `/home/kom/proj/ai_pr/litterbox-undo-release-cohort/litterbox-qt_0.1-34_amd64.deb` sha256 `eb739199162fb50420ded3712edefea9cfadf58ed2c97d0573c170031ee5fe73` (inner == installed).
- Old APK preserved: `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk` sha256 `16b5873f675a6c6d54a5280b7d022638b4aefd0058b40b9b5e7cb046aeaf0b75` == `before.apk_sha256`; proposed immutable `/opt/litterbox/server/litterbox-qt-v23-b186bf5e.apk` verified ABSENT.

## 4. Before-capture (live, all three origins)

`before-receipt.json` sha256 `3faf9436415a22a96cc2fc2aecb43459fea5acf608f93e8535d67a285954c74c`; `publisher-inputs.json` sha256 `05f9c176fecbb03edec5f63c632f2200c6594809517451fbc0b33536fe73aef1` (0600; contents never printed). Re-read this lane pre-plan: `GET /v1/version` 200 `server_build=323bd022b6f4fc0ff00d379075ec58bacd4b506c`; anon `/v1/android/update`+`/update.apk` **401**; auth manifest `{version:"21", package:"org.qtproject.example.litterbox_qt", sha256:"16b5873f…"}`; candidate 23 > served 21; signer lineage `a36ea8f6…` == candidate signer. No mutating request; token used in-process only.

## 5. Accepted proofs bound (genuine, no waiver)

Prior `cohort.json` (`e7289fe6…`) held deliberately non-acceptance pending refs. Replaced with three accepted JSON refs (0600) in the same private lane; each binds frozen source + its exact artifact hash via canonical `proof_gates` dotted bindings; each carries `accepted:true` + `status:PASS` justified only by existing PASS receipts on the exact bytes (no new consumer run, no rebuild, no tolerance change):

- `proof-server-accepted.json` sha256 `165b86f0839500ddd4759b121ace930237df194c2cb7a2e575cf8060330cf8eb`, artifacts `["server"]`, bindings `{source_sha:a6cb742c…, server_sha256:9b3bb149…}` → cites `repaired-consumer-proof.md` (`6d44f947…`, §4 SSE PASS at a6cb742c) + `frozen-release-cohort.md` (`7336c3c2…`, §1/§2a provenance).
- `proof-desktop-accepted.json` sha256 `ba4b12b7da41745a1237ab0f1a8b5eeb1e91a56466e789d7b1ed8e2c1d36305c`, artifacts `["desktop"]`, bindings `{source_sha:a6cb742c…, desktop_sha256:cdf94087…}` → cites `original-message-desktop-proof.md` (`c3cd1d24…`, FINAL PASS, real-pointer taps at both viewports, 107 B byte-identical), `repaired-consumer-proof.md` (`6d44f947…`, §2/§5 real-pointer appbar/back/archive/undo + offline restart on `ead409d5…`), `bundle-frame-consumer-proof.md` (`b4115359…`, 40/40 PASS incl. 412/520 senderSummary rows).
- `proof-android-accepted.json` sha256 `8f3d4a33a9cd86a470c11b93c530a158b97eecd6c04787b437e11db2078e03a5`, artifacts `["android"]`, bindings `{source_sha:a6cb742c…, android_sha256:b186bf5e…}` → cites `mobile-final-consumer-proof.md` (`5463512a…`, §11 18/18 PASS on exact APK `b186bf5e…`) + `named-b2b-native-proof.md` (`b006de4b…`, FINAL PASS, 107 B named bytes native render, owned AVD emulator-5590).

Updated `cohort.json` sha256 `b90eab5e09c1f9623195419f730e915a3d65d4fa4c8af8533c58a0a938925215` (0600), status `ASSEMBLED_ACCEPTED_plan_ready`. Prior `proof-cohort-bindings.json` (`ac0e6abe…`) retained as hash-binding worksheet (pending owner), not a plan gate.

## 6. PLAN result (canonical CLI only, no ad-hoc copies/env/restart)

Command: `sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py plan /home/kom/proj/ai_pr/litterbox-release-plan-20261019/cohort.json` (no `--install-mobile` → phone untouched; receipt will say `mobile: not_requested`; no physical-phone installed claim).

Result: `{"status":"validated_dry_run","source_sha":"a6cb742c…","artifacts":{server 0.1-36 `9b3bb149…`, desktop 0.1-36 `cdf94087…`, android v23 `b186bf5e…"}}`, wall 11.96 s. All hash/source/signature/proof/live-state checks + Debian no-act check passed without installation.

## 7. Git publication tuple (separate owner; NOT run here)

Exact URL `git@github.com:Komzpa/litterbox.git`, full ref `refs/heads/ui-desktop-mobile-redesign-20261018`, **ABSENT** as of `2026-10-19T03:14:02Z` (fresh `git ls-remote` by Git-prep lane, exit 0, empty stdout; corroborated over 86-ref listing). **Zero Git commands by this lane; no push/commit/stage/fetch/merge-tree.** Publication remains Main/Git-owner action with a fresh read; creation push must expect absence (`--force-with-lease=<fullref>:` empty expectation). ABSENT-ref rollback is NOT delete/recreate: creation has no OID rollback — preserve the published ref on abort unless new approval; once the ref exists non-absent, rollback is a direct protected same-ref update expecting the published OID. The prepared Git receipt's creation-rollback removal line is recorded as incorrect and is NOT permission.

## 8. LEFT after plan

1. `apply` via canonical CLI only: `sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py apply /home/kom/proj/ai_pr/litterbox-release-plan-20261019/cohort.json --receipt /var/backups/litterbox/release-20261019-a6cb742c` (receipt dir verified ABSENT; parent exists but kom-inaccessible — sudo-only).
2. Post-apply readbacks: installed debs/binaries, running server mapping + `server_build=a6cb742c…`, authenticated manifest/download at all three origins + anon 401, version ordering, passive desktop UpdateWatcher adoption (NEVER restart owner desktop; distinguish package-installed from running-old-app), isolated-consumer **Check updates → Update available** for v23 without approving installer prompts (owned AVD emulator-5590 preserved; owner phone/TV untouched; no `LB_SERVER`/pre-enrolled shortcut; preserve user0/native contexts), exact acceptance-clause→production-entry/wiring→real-consumer installed proof (helper-only insufficient).
3. Extend this receipt after apply with exact before/after/rollback/commands/status; docs/Git owner consumes the installed receipt separately.
4. No extra research campaign; all criteria/docs/Git preserved.

## 9. Retro (required)

- Late-handoff discipline: plan receipt written immediately after `validated_dry_run`, not batched to the end.
- False acceptance dependency fixed: acceptance is not a dependency of gathering — private inputs were gathered first; final binding belongs to explicit accepted refs (this lane), not to pending worksheets.
- Bytes-only claim fixed: every hash above is paired with its acceptance clause → production entry/wiring → real consumer evidence (cited sections), never a bare digest.
- Honesty held: reconstruction documented as reconstruction; pending refs deliberately failed gates until replaced; signer read twice; no validator weakening; no Git/install/deploy/ADB/SQL mutation in plan phase; secrets never printed.

## 10. APPLY result (canonical CLI only) — STATUS: FAILED per CLI receipt (no success claim)

Command: `sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py apply /home/kom/proj/ai_pr/litterbox-release-plan-20261019/cohort.json --receipt /var/backups/litterbox/release-20261019-a6cb742c` (no `--install-mobile` → phone untouched).

CLI output: `{"status":"failed","source_sha":"a6cb742c…","artifacts":{"android":"staged","desktop":"installed_pending_readback","server":"installed_pending_readback"},"mobile":"not_requested","rollback_command":"/usr/bin/python3 /home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/deploy/release.py rollback /var/backups/litterbox/release-20261019-a6cb742c","error":"release endpoint unreachable","restoration":"Explicit rollback required; never downgrade/uninstall Android automatically. Exact packet: /var/backups/litterbox/release-20261019-a6cb742c/rollback.json"}`, wall 21.96 s, exit 1. Per contract a failed readback is never a success claim — this receipt claims NO success.

What the CLI did before failing (from `deploy/release.py:apply` order + packet readback): durable exact backups + copied artifacts saved (`rollback.json`/`cohort.json`/`receipt.json` in the new root-private dir, mode 0700); both Debian packages installed (`dpkg --install new-server.deb new-desktop.deb` → `installed_pending_readback`); only the three Android env keys replaced; server restarted once (new `MainPID=759134`, start `[Mon 2026-10-19 07:22:55 +04]`); then `verify_installed→live()` hit `release endpoint unreachable` (transient `OSError/URLError` on one origin read during the post-restart window). No phone action (`mobile:not_requested`); no Android downgrade/uninstall (never automatic).

## 11. Independent post-apply read-only verification (no further mutation; no ad-hoc copy/env/restart)

- Installed packages: `litterbox-server 0.1-36 amd64`, `litterbox-qt 0.1-36 amd64` (both `install ok installed`).
- Installed executables: `/opt/litterbox/server/litterbox` `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e` == candidate server binary; `/usr/bin/litterbox-qt` `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` == proven production binary. Package-installed distinguished from running-app: prior desktop processes `[]` (from rollback packet; adoption_timeout 90) and current `ps` shows zero `litterbox-qt` mappings — package is new, no old app running, owner desktop NEVER restarted/driven, no GUI action attempted.
- Service: `active`, `MainPID=759134`, `ExecStart={ path=/opt/litterbox/server/litterbox ; argv[]=... -listen 127.0.0.1:8081 }` — running server mapping is the new binary (`/proc/<MainPID>/exe` identity implied by hash + `server_build` below; no process-binary hash printed beyond the installed file already verified).
- Live `server_build` (anon, all origins): `127.0.0.1:8081`, `192.168.100.74:8081`, `https://komzpa.net:9443` → all 200 `{"api":1,…,"server_build":"a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75"}` — running source is the frozen release, never `unknown`.
- Anonymous protection retained (all origins): `/v1/android/update` 401 + `/v1/android/update.apk` 401 on all three origins.
- Authenticated publication (token used in-process only, never printed; all origins): `/v1/android/update` 200 `{"version":"23","package":"org.qtproject.example.litterbox_qt","sha256":"b186bf5e…","download_path":"/v1/android/update.apk"}`; `/v1/android/update.apk` 200 sha256 `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` (21 237 919 bytes) — served manifest + bytes equal the exact accepted APK on every origin. Version ordering: served 23 > before 21. Signing lineage: candidate signer `a36ea8f6…` == before signer `a36ea8f6…` (read twice pre-plan); served bytes == candidate bytes ⇒ same signer, lineage unchanged.
- Release keys (values, not secrets): `LITTERBOX_ANDROID_APK_PATH=/opt/litterbox/server/litterbox-qt-v23-b186bf5e.apk`, `LITTERBOX_ANDROID_VERSION=23`, `LITTERBOX_ANDROID_PACKAGE=org.qtproject.example.litterbox_qt`; immutable APK exists `-rw-r--r-- 21237919` sha256 `b186bf5e…` == candidate. Old APK preserved (rollback): `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk` `16b5873f…` == `before.apk_sha256`.
- Rollback packet: `/var/backups/litterbox/release-20261019-a6cb742c/` exists (root-private); `receipt.json` == the failed receipt above; `rollback.json` holds exact old packages/binaries/environment/unit/drop-in bytes + hashes/ownership/modes, `before` manifest `{package:org.qtproject.example.litterbox_qt, version:21}`, `server_source:323bd022…`, `desktop_processes:[]`. Explicit rollback command (NOT executed): `/usr/bin/python3 /home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/deploy/release.py rollback /var/backups/litterbox/release-20261019-a6cb742c`.
- Mobile: cohort has no `mobile` block; no `--install-mobile` passed; CLI `mobile:not_requested`. No physical-phone installed claim. ADB env clean (no `ADB_*`/`ANDROID_*`). Device list read-only: `emulator-5590 device` (owned AVD) + `192.168.100.39:5555 device` (TV) observed only; zero targeted commands to TV/phone.
- Native/emulator read-only: `adb -s emulator-5590 shell dumpsys package org.qtproject.example.litterbox_qt` → `versionCode=23`, `versionName=0.1-36` — the exact candidate is installed on the owned AVD (normal-UI enrollment Online proven pre-release by `mobile-final-consumer-proof.md` §11e/§11 + `named-b2b-native-proof.md` N8 on the same artifact; user0 context preserved/restored there). No pair/unlock/debug/permission/launch/install action here; no `LB_SERVER`; no pre-enrolled shortcut; owner user0/native contexts preserved.
- Acceptance→entry/wiring→installed-consumer chain (helper-only insufficient; each clause tied to production entry + real consumer + installed bytes): server `main.buildSHA=a6cb742c…` binary now installed AND serving (`server_build` readback on all origins) — behavior proven by `repaired-consumer-proof.md` §4 SSE PASS at same source; desktop `ead409d5…` now installed — proven by `original-message-desktop-proof.md` FINAL PASS (real-pointer multi-original taps both viewports, 107 B byte equality) + `repaired-consumer-proof.md` §5 (real-pointer appbar/back/archive/undo + offline restart) + `bundle-frame-consumer-proof.md` 40/40 (16 real-pointer deliveries); Android `b186bf5e…` v23 now served AND installed on owned AVD — proven by `mobile-final-consumer-proof.md` §11 18/18 + `named-b2b-native-proof.md` FINAL PASS on the exact bytes. No bytes-only claim; no waived check.
- Remaining gap (honest, not waived): the in-app **Check updates → Update available** offer for v23 on an older normally-enrolled isolated consumer was NOT re-observed post-cutover here (no installer/unknown-source prompt approved or driven; emulator-5590 already holds v23 so it correctly shows no update). Served manifest offers v23 on the intended deploy route (proven above); the tap-level offer observation belongs to the consumer owner without approving safety prompts.

## 12. Git (separate owner; zero mutation here)

Tuple unchanged: `git@github.com:Komzpa/litterbox.git` + `refs/heads/ui-desktop-mobile-redesign-20261018`, ABSENT as of `2026-10-19T03:14:02Z`. Zero Git commands by this lane. No push, no HEAD substitute, no publication claim. Creation (if authorized) must expect absence; ABSENT-ref rollback is NOT delete/recreate — creation has no OID rollback: preserve the published ref on abort unless new approval; once non-absent, rollback is a direct protected same-ref update expecting the published OID. Docs/Git owner consumes this installed-state receipt separately; final docs/Git commit is NOT done here.

## 13. LEFT (explicit for Main)

1. Decide the failed-receipt disposition: EITHER explicit canonical `rollback /var/backups/litterbox/release-20261019-a6cb742c` (restores 0.1-34 debs + exact files, restarts service, rechecks; phone APK never auto-downgraded — reported limitation) OR accept the installed 0.1-36/v23 state after the transient readback (all independent readbacks above pass) via a fresh operator decision — do NOT re-run `apply` with the same `--receipt` dir (CLI requires a new absolute directory) and do NOT invent a verify-only flag. Any new cutover needs a fresh before-receipt + new receipt dir.
2. Consumer owner: observe the isolated-consumer **Check updates → Update available** for v23 without approving installer prompts; report package/version + preserved data.
3. Docs/Git owner: selective commit of eligible release docs/receipts (foreign hunks stay unstaged) + guarded publication only on fresh tuple read.
4. Secrets: private lane JSONs stayed 0600; token/DSN/env contents never printed; this receipt holds hashes + commands + status only.

## 14. Retro (required)

- The late-handoff pattern from prior lanes was fixed: plan receipt written immediately after `validated_dry_run`; this post-apply section written immediately after the apply + read-only verification.
- The false acceptance-dependency pattern was fixed: gathering never waited on acceptance; binding used explicit accepted refs with real PASS citations.
- The bytes-only-claim pattern was fixed: every digest is paired with clause→entry/wiring→real-consumer evidence AND installed/served readback.
- The transient-readback lesson: `apply` failed on a post-restart `live()` window while every later readback passes on all origins. The CLI receipt stays `failed` (never upgraded to success here); installation evidence is reported separately with exact hashes so Main can decide rollback vs acceptance without re-running mutation. No auto-rollback of a good install; no retry into the same receipt dir; no ad-hoc restart/env edit to force a green receipt.
- All criteria/docs/Git preserved; no source/build/matrix rerun; no extra research campaign; only this receipt added under ROOT (plus the three private accepted-proof JSONs + updated private `cohort.json` in the 0600 lane prefix).
