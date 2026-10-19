# Release plan readiness — receipts/release-plan-readiness.md

Gate: `ReleasePlanReadiness`, 2026-10-19 (Asia/Tbilisi). ROOT =
`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`, HEAD
`fa01568ad3c4fff5ac61cf3354b6e9f4a100c367`. Lane prefix (sole output
location): `/home/kom/proj/ai_pr/litterbox-release-plan-20261019`.

Status: **PREPARATION ONLY — NOT plan-ready, NOT published, NOT installed, NOT
accepted.** Rollback prerequisites completed; remaining LEFT items are explicit
below. `deploy/release.py` was read only (`--help`); no `plan`/`apply`/
`rollback` invocation, no installer even `--dry-run`, no source/Git/build-tree/
shared-installer mutation, no DB write, no owner-desktop drive, no ADB, no API
mutating request (all live reads were `GET` only), no Git command of any kind.

## 0. First finite check — identities (2026-10-19 ~04:11 Asia/Tbilisi)

- ROOT exists.
- `receipts/frozen-release-cohort.md` sha256
  `7336c3c24d23ee11b675ff9b2a53d134a153a1c612c536fb875019d2e25c57d2` — **MATCHES** the
  brief's named SHA; read in full (§§1–6).
- `docs/DEPLOYMENT.md` sha256
  `92b2d277a8d7bc365e1e4014e43c68a24c8cbadb37b4ee4faeae50464e44872a` — matches the
  hash recorded in frozen-release-cohort.md §1 and cohort-hash-manifest.json;
  read in full (cohort.json contract, publisher-inputs/before-receipt contract,
  unified release/rollback, Android update contract).
- `/home/kom/proj/ai_pr/litterbox-release-cohort-20261019/cohort-hash-manifest.json`
  sha256 `fef8e33f39747b2a22947d16d1127f7e488bbd6e632ad84a5c949ad4faa7f1fd` —
  **MATCHES** the brief's named SHA; valid JSON; all three frozen artifacts
  present under that prefix.

## 1. Two current live mappings re-verified before any host-target use

- `systemctl cat litterbox-server.service`: `User=litterbox`;
  `ExecStart=/opt/litterbox/server/litterbox -listen 127.0.0.1:8081`;
  `EnvironmentFile=/etc/litterbox/db.env` (×2 as separate ordered sources),
  `/etc/litterbox/server.env.d/ollama-embeddings.conf`,
  `/etc/litterbox/gmail.env`. Service `active`; listeners observed
  `127.0.0.1:8081`, `192.168.100.74:8081` (LAN proxy), `127.0.0.1:5435`.
- `/etc/litterbox/db.env` DSN shape (private guard, credentials never printed):
  `postgresql://<redacted>@127.0.0.1:5435/litterbox` — **both mappings confirmed**:
  server `127.0.0.1:8081`, DB `litterbox@127.0.0.1:5435`.
- Effective drop-ins re-verified read-only:
  `DropInPaths=/etc/systemd/system/litterbox-server.service.d/embeddings.conf
  /etc/systemd/system/litterbox-server.service.d/gmail.conf` exactly; hashes
  `bb554e2f7deb0f2c5efcb5ff58fe2379ded174f0d71800d7008e0cede26f3eee` and
  `0c0440e3e058835a3db49c405d8b5caf33d6536bc572257b8a504e0835ebf8c4`; unit
  `/etc/systemd/system/litterbox-server.service`
  `dd3edbb60dc053479725c857a026bfa5630fdb60a2a52314cdb72fa11b66dc3a` — all three
  match the frozen cohort manifest.

## 2. Server rollback deb 0.1-34 — MISSING, then RECONSTRUCTED (not the original deb)

**Search (finite, first):** bounded `find` over `/home/kom/proj/ai_pr`
(maxdepth 4), `/home/kom/proj`, `/var/cache/apt/archives`, `/var/backups/litterbox`,
`/tmp`, `/home/kom/Downloads`, `/opt/litterbox`, plus `locate` for
`litterbox-server_0.1-34` — **no `litterbox-server_0.1-34*.deb` exists**. Only
`0.1-26` variants under `DeployA2aLitterbox-server-out/` and the frozen
`0.1-36` in the cohort prefix; `litterbox-undo-release-cohort/` holds the
0.1-34 *desktop* deb and a `server/` source tree, no server deb. (An earlier
unbounded broad find timed out at 120 s — superseded by the bounded search;
no result lost.)

**Reconstruction (canonical recipe, own prefix only):**
`deploy/server/debian/build.sh /opt/litterbox/server/litterbox 0.1-34 /home/kom/proj/ai_pr/litterbox-release-plan-20261019`
→ `lane/litterbox-server_0.1-34_amd64.deb` sha256
`45c1587586c5a2407edf0b133703e3a639a8c92822f7707c1d02599aa0ee864b`.
Input was the exact readonly installed binary (never copied out of `/opt` for
mutation, never recompiled).

**Readback verification (all pass):**

| Check | Result |
|---|---|
| dpkg fields | `Package: litterbox-server` / `Version: 0.1-34` / `Architecture: amd64` == installed (`dpkg-query`: `litterbox-server 0.1-34 amd64`) |
| inner `/opt/litterbox/server/litterbox` | `8dfe79fc8f601c97367f525a97942e37ad0a29781cd336032171b9303784d8ea` == installed binary |
| packaged unit | `dd3edbb60dc053479725c857a026bfa5630fdb60a2a52314cdb72fa11b66dc3a`, `cmp` == live `/etc/systemd/system/litterbox-server.service` |
| `postinst` | `cmp` == canonical `deploy/server/debian/postinst` (`f535b065b31ae5db3308dea642bb5605a34b95eb8b2960872e29221404d0b599`) |
| maintainer scripts | exactly `control`, `postinst` (no preinst/prerm/postrm) |

**Documented as reconstruction, not the original 0.1-34 deb:** the original
package file was never found; this deb was freshly packaged from the installed
binary with the canonical recipe. It satisfies `release.py validate()`'s
rollback contract as read in source (rollback ref: version/architecture ==
installed, inner binary digest == installed digest). The CLI was **not**
modified; no validator weakening (only `--help` was executed).

## 3. Desktop rollback deb — VERIFIED (known artifact, no rebuild)

`/home/kom/proj/ai_pr/litterbox-undo-release-cohort/litterbox-qt_0.1-34_amd64.deb`
sha256 `eb739199162fb50420ded3712edefea9cfadf58ed2c97d0573c170031ee5fe73`;
control `litterbox-qt 0.1-34 amd64` == installed; inner `/usr/bin/litterbox-qt`
sha256 `d8ef906f335a1dff19ba64f10e25bd59c3beb0ec2777e4881d4e99173e086904` ==
installed desktop binary; scripts: `control`, `md5sums` only.

## 4. Candidate artifact internals re-read in this lane (identity, not acceptance)

- **server** `lane cohort/litterbox-server_0.1-36_amd64.deb`: fields
  `litterbox-server 0.1-36 amd64`; inner binary
  `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e`;
  `go version -m` → `main.buildSHA=a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75` (correct
  frozen stamp, not the rejected `99a00861…`); packaged unit `cmp` == live unit;
  `postinst` `cmp` == canonical; scripts exactly `control`, `postinst`.
- **desktop** `litterbox-qt_0.1-36_amd64.deb`: fields `litterbox-qt 0.1-36 amd64`;
  inner `/usr/bin/litterbox-qt`
  `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` == the exact
  production binary proven in `receipts/repaired-consumer-proof.md` §2.
- **android** `litterbox-qt-v23-a6cb742c.apk`: `aapt` badging
  `org.qtproject.example.litterbox_qt` versionCode **23** versionName 0.1-36;
  `apksigner` signer SHA-256
  `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37`; the three
  required `lib/arm64-v8a/` TLS members present.

## 5. Live before-capture (safe anon + authenticated GET; intended origins, as-of run)

All three origins from the canonical publisher-inputs shape:
`http://127.0.0.1:8081`, `http://192.168.100.74:8081`, `https://komzpa.net:9443`.
Identical results at every origin:

| Read | Result |
|---|---|
| `GET /v1/version` (anon) | 200, `server_build=323bd022b6f4fc0ff00d379075ec58bacd4b506c` |
| `GET /v1/android/update` (anon) | **401** (protected) |
| `GET /v1/android/update.apk` (anon) | **401** (protected) |
| `GET /v1/android/update` (enrolled-device token) | 200, manifest `{version: "21", package: "org.qtproject.example.litterbox_qt", sha256: "16b5873f675a6c6d54a5280b7d022638b4aefd0058b40b9b5e7cb046aeaf0b75", download_path: "/v1/android/update.apk"}` |
| `GET /v1/android/update.apk` (token) | 200, bytes sha256 `16b5873f…`, size 21 073 792 — matches manifest and the served file `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk` on disk |

**Version ordering re-read now:** candidate versionCode **23** > served
manifest `version` **21**, freshly re-read at plan-preparation time on all
three origins (not the stale handoff value).

**Credential loader (reference only):**
`/home/kom/.local/share/tasks-loop/state/secrets/litterbox-desktop-credentials.json`
mode `600`, uid 1000 — used programmatically to attach the `Authorization`
header; **contents/token never printed** and never written to ROOT/receipts.
Served-APK signer-cert readback (`apksigner --print-certs` over the served
bytes copy `lane/.served-apk-octetcheck.bin`, mode 0600) was still in flight
when this receipt was frozen — pending item 7b.

**Private lane inputs (0600, by reference only):**
- `lane/.env-capture.json` — the three current `LITTERBOX_ANDROID_*` values
  (key names: `LITTERBOX_ANDROID_APK_PATH`, `LITTERBOX_ANDROID_VERSION`,
  `LITTERBOX_ANDROID_PACKAGE`); values deliberately not copied into this
  receipt.
- Current `/etc/litterbox/db.env` sha256
  `5cd8efeb492ef657e397ad07649254f7495f6d2a249f6e6f2b7b938176bba0f0` (differs from
  the 2026-10-15 value because the v21 release replaced the three keys).
- `lane/.served-apk-octetcheck.bin` — byte copy of the currently served APK.

## 6. DONE (concrete, final as of 2026-10-19T05:30+04:00)

1. **Identity gate:** frozen-release-cohort.md SHA `7336c3c2…` and
   cohort-hash-manifest.json SHA `fef8e33f…` match the brief; DEPLOYMENT.md
   read (SHA `92b2d277…`, matching both records); CLI `--help` + `validate()` +
   `proof_gates()` source read — schema identical across `--help`, source and
   DEPLOYMENT.md, no conflict.
2. **Both live mappings re-verified before host-target use:** service
   `-listen 127.0.0.1:8081`, `ExecStart path=/opt/litterbox/server/litterbox ;`
   (raw `systemctl show`), DB DSN shape `<redacted>@127.0.0.1:5435/litterbox`;
   service active, MainPID 1364707 `/proc/1364707/exe` sha256
   `8dfe79fc…` == installed binary; exact drop-in set (2) with hashes
   `bb554e2f…`/`0c0440e3…`; unit `dd3edbb6…`.
3. **Server rollback deb 0.1-34:** absent everywhere searched (bounded find +
   locate over lane/home/proj caches, apt cache, /var/backups, /opt) →
   **reconstructed** with canonical `deploy/server/debian/build.sh
   /opt/litterbox/server/litterbox 0.1-34 <lane>` →
   `lane/litterbox-server_0.1-34_amd64.deb` sha256
   `45c1587586c5a2407edf0b133703e3a639a8c92822f7707c1d02599aa0ee864b`;
   readback: fields `litterbox-server 0.1-34 amd64` == installed, inner binary
   `8dfe79fc…` == installed, packaged unit `cmp` == live unit, `postinst` `cmp`
   == canonical, scripts exactly `control`+`postinst`. **Documented as
   reconstruction, not the original deb.** Canonical CLI permits it (rollback
   ref rules read in `validate()`/`package()`); validator untouched.
4. **Desktop rollback deb verified:** `litterbox-undo-release-cohort/litterbox-qt_0.1-34_amd64.deb`
   sha256 `eb739199…`, fields `litterbox-qt 0.1-34 amd64`, inner binary
   `d8ef906f…` == installed desktop binary.
5. **Candidate internals re-read in this lane:** server deb inner binary
   `6c23d73e…` with stamp `main.buildSHA=a6cb742c…`, unit `cmp` == live,
   postinst `cmp` == canonical, scripts `control`+`postinst`; desktop deb inner
   binary `ead409d5…`; APK `aapt` v23/0.1-36 `org.qtproject.example.litterbox_qt`,
   signer `a36ea8f6…`, 3 TLS members present. File hashes re-verified:
   `9b3bb149…` / `cdf94087…` / `b186bf5e…`.
6. **Live before-capture at all three canonical origins** (safe anon +
   authenticated `GET` only): `/v1/version` 200 `server_build=323bd022…`;
   anon `/v1/android/update`+`update.apk` **401** at every origin; auth manifest
   `{version:"21", package:"org.qtproject.example.litterbox_qt",
   sha256:"16b5873f…", download_path:"/v1/android/update.apk"}`; auth APK bytes
   sha256 `16b5873f…` (21 073 792) == served file on disk. **Version ordering
   re-read now: candidate 23 > served 21.**
7. **Served signer-cert readback COMPLETE:** `apksigner verify --print-certs`
   on both the private served-bytes copy `lane/.served-apk-octetcheck.bin` and
   the live served file `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk`
   → `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37`
   (canonical lineage; equals candidate signer → signing lineage unchanged).
8. **Private inputs written (0600, kom-owned, reference-only):**
   - `lane/before-receipt.json` sha256
     `3faf9436415a22a96cc2fc2aecb43459fea5acf608f93e8535d67a285954c74c` —
     signer `a36ea8f6…`, apk `16b5873f…`, manifest `21`/`org.qtproject…`,
     per-origin status table.
   - `lane/publisher-inputs.json` sha256
     `05f9c176fecbb03edec5f63c632f2200c6594809517451fbc0b33536fe73aef1` —
     all contract keys present; `environment_sha256 5cd8efeb…`,
     `server_binary_sha256 8dfe79fc…`, `server_source 323bd022…`, 3 origins,
     credential_loader path (contents never read into any receipt),
     candidate fields (source `a6cb742c…`, apk `b186bf5e…`, version `23`,
     signer `a36ea8f6…`), `rollback_apk_preserve`
     `/opt/litterbox/server/litterbox-qt-v21-16b5873f.apk` (digest == before
     apk), `proposed_immutable_apk_path /opt/litterbox/server/litterbox-qt-v23-b186bf5e.apk`
     (verified absent), `environment_release_values` = the three current keys
     (shape-checked against the live env file line-for-line).
   - `lane/.env-capture.json` (0600) — three current `LITTERBOX_ANDROID_*`
     values, never printed.
9. **Cohort JSON assembled with explicit pending proofrefs (NOT plan-ready):**
   `lane/cohort.json` sha256
   `e7289fe64887ca78fbe2445cfcb60ba6e8d46478aac774f9c09e393d4f4cd1aa`
   (0600) — full contract: source_sha, three artifacts with package/version/
   architecture/binary_sha256/version_code/version_name/signer_sha256,
   proofs ×2 **both deliberately non-acceptance** (`accepted` key absent —
   `proof_gates()` will correctly fail until ReleaseProof supplies the
   accepted refs), target block (host/service/environment/binaries/unit_path/
   origins/credential_loader/aapt/apksigner/publisher_inputs+before_receipt
   hash refs/rollback_debs ×2 hash refs/drop_ins ×2 hash refs). Every ref hash
   and every dotted proof binding was re-resolved programmatically after
   writing (all OK).
10. **Hash-binding receipt for ReleaseProof:**
    `lane/proof-cohort-bindings.json` sha256
    `ac0e6abe1af8a61a146148e4fa6daffa5b1cf99b825fb49fc5c1b336d7b11c76` —
    exact deb/APK hash → accepted-consumer-receipt chains (cited by path+SHA),
    `status: hash_bindings_verified_pending_releaseproof`,
    `acceptance.state: pending` owner ReleaseProof (per applied plan-doctor);
    creates no new human-approval prerequisite.
11. **Consumer acceptance evidence cited (already existing, not authored here):**
    `mobile-final-consumer-proof.md` (`5463512a…`, exact-candidate native 18/18
    PASS on `b186bf5e…`), `named-b2b-native-proof.md` (`b006de4b…`, PASS —
    named B2B original renders in real native reader; closes the former
    OriginalMessageConsumerProof gap), `repaired-consumer-proof.md`
    (`6d44f947…`, desktop `ead409d5…` consumer + server note SSE at a6cb742c).
12. Canonical CLI: only `--help` executed; **no `plan`/`apply`/`rollback`**, no
    installer (including `--dry-run`), no service operation, no DB write, no
    owner-desktop/ADB, no mutating API request, **zero Git commands**.

## 7. LEFT (explicit)

1. **ReleaseProof: final accepted three-way proof refs** — `cohort.json`'s two
   proof refs are pending non-acceptance by design; `proof_gates()` fails until
   ReleaseProof adds the accepted `{path,sha256,artifacts,accepted,bindings}`
   receipt bound to `source_sha a6cb742c…` and deb/APK hashes
   `9b3bb149…`/`cdf94087…`/`b186bf5e…` (bindings ready in
   `lane/proof-cohort-bindings.json`). Acceptance itself is supplied by
   rowQA/receipts (incl. `named-b2b-native-proof.md` PASS) — no new human
   approval invented here.
2. **Git-only tuple gate (Main, Git owner):** publication tuple = exact URL
   `git@github.com:Komzpa/litterbox.git` + full ref
   `refs/heads/ui-desktop-mobile-redesign-20261018` + oldOID from **ONE fresh
   forge head read**. Named ref was ABSENT at the 02:58 read; no OID
   substituted, no remote HEAD guessed, zero Git commands by this lane; local
   branch separate. Publication is the true remaining waiter.
3. **plan/apply (Main, canonical CLI only, after item 1 + tuple):**
   `sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py plan
   <lane/cohort.json>` then `apply … --receipt /var/backups/litterbox/release-ID`;
   CLI sha256 `e2b1c060cdd60b81723dc68b44d7f0d62bf3350df5328985c0dc162cd0b57b5a`.
   Cohort targets no `--install-mobile` (phone untouched; a mobile block would
   be a Main decision + separate gather).

## 8. Retro (required)

- **Late-receipt discipline:** the first draft landed in the initial finite
  check, but DONE/LEFT finalization was pushed by the supervisor three times
  (~30–45 min vs a 20 min plan); from outside it read as "no receipt".
  Correct pattern: stamp DONE/LEFT into the ROOT receipt after **every**
  milestone, then incrementally.
- **Wrong blocker claim, corrected:** I once wrote that private inputs were
  "blocked on acceptance + signer" — Main corrected: acceptance is not a
  dependency of gathering private inputs, and final proof binding belongs to
  ReleaseProof. The signer readback and both private JSONs then closed in
  minutes. Lesson: distinguish *gathering* (always do) from *acceptance*
  (owner-gated).
- **Dead-end discovery:** searching for a "lead visual approval" record after
  actualQA/native proof already existed wasted a checkpoint; stop looking for
  a prerequisite once its successor receipt exists.
- **Time sinks:** one unbounded find burned 120 s; several short reads were
  backgrounded by incoming messages and their results arrived after the
  decision they fed. Bounded searches first; keep handoff-critical readbacks
  in the foreground.
- **Honesty held:** no original-deb claim (reconstruction documented with the
  recipe + five-way verification); no acceptance forged (pending proofrefs
  deliberately fail `proof_gates()`); signer cert read twice before recording,
  never assumed; no validator weakening; no Git/install/deploy/ADB/SQL; secrets
  never printed (token used in-process only, DSN redacted, env values in 0600
  lane file); plan-readiness explicitly withheld.
- All prior acceptance receipts, docs, criteria and the authorized Git handoff
  are unchanged; this lane touched only its lane prefix and this receipt.
