# Frozen unified release cohort preparation — receipts/frozen-release-cohort.md

Gate: `FrozenReleaseCohort`, 2026-10-19 (Asia/Tbilisi). ROOT =
`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`, HEAD
`fa01568ad3c4fff5ac61cf3354b6e9f4a100c367`. Lane output prefix (sole build
output location): `/home/kom/proj/ai_pr/litterbox-release-cohort-20261019`.

Status: **PREPARATION ONLY — three frozen artifacts built/staged and hash-bound;
the cohort is NOT plan-ready, NOT published, NOT installed, NOT accepted.** The
canonical `deploy/release.py` remains the sole later publication entrypoint under
Main; this lane invoked no installer/upgrader (including `--dry-run`), no
`qt/android-build.sh`, no `release.py` action, no install/restart/push/deploy,
no owner-desktop drive, no phone/TV/ADB mutation, no SQL of any kind, and no
Git command at all (no fetch, no merge-tree, no object writes). The only
mutations are: three artifacts + `cohort-hash-manifest.json` under the lane
prefix, and this receipt in ROOT.

## 1. Source identity (relevant-input blob rule, not global HEAD rebuild)

- Frozen production source: `a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75`.
- `git diff --stat a6cb742c HEAD -- qt server deploy` → **empty** for
  `qt server deploy` except test-only file; `git diff --name-status a6cb742c HEAD`
  is exactly `M qt/tests/tst_bundleexpand.qml` (11+/3−, test-only). Every
  production input path relevant to all three artifacts — `server/**`,
  `deploy/**`, `qt/src`, `qt/qml`, `qt/pages`, `qt/debian`, `qt/CMakeLists.txt`
  — is blob-identical between the freeze and HEAD, so building/packaging from
  this checkout is the a6 source input (per-path rule, not a HEAD rebuild).
- Canonical docs read and hash-checked: `docs/DEPLOYMENT.md` sha256
  `92b2d277a8d7bc365e1e4014e43c68a24c8cbadb37b4ee4faeae50464e44872a`;
  `receipts/release-install-preflight.md` sha256
  `b77f57bb32ae281f4ba449f8d8c30b5c149cc896fc4e2e0586644778f1088ed5` (matches the
  brief's named SHA).
- `deploy/release.py` sha256
  `e2b1c060cdd60b81723dc68b44d7f0d62bf3350df5328985c0dc162cd0b57b5a`.

## 2. The three frozen artifacts (all under the lane prefix)

| Artifact | Path | sha256 | Size |
|---|---|---|---|
| server deb | `litterbox-server_0.1-36_amd64.deb` | `9b3bb149063ca2df3031811108b8783b58d0cb22383800543e81b808be484b4a` | 8 309 828 |
| desktop deb | `litterbox-qt_0.1-36_amd64.deb` | `cdf940878aa64679e4bd2069877591f869daafd72444cdd7010f9c6b3964d217` | 698 100 |
| Android APK | `litterbox-qt-v23-a6cb742c.apk` | `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` | 21 237 919 |

Hash manifest with full bindings:
`/home/kom/proj/ai_pr/litterbox-release-cohort-20261019/cohort-hash-manifest.json`
sha256 `fef8e33f39747b2a22947d16d1127f7e488bbd6e632ad84a5c949ad4faa7f1fd`
(valid JSON; preparation manifest, not a `release.py`-ready `cohort.json`).

### 2a. Server deb provenance

- Existing stamped proof binary `remaining-r2/litterbox-server-stamped` was
  **rejected**: `go version -m` shows `-X main.buildSHA=99a0086113305af72205c92326f93d44e873bf72`
  (vcs.revision a6cb742c) — its stamp is 99a00861, not the frozen source; a
  relabel or reuse would fail `release.py`'s
  `main.buildSHA=<source_sha>` check. Never faked.
- Built fresh in the lane prefix (no recompilation of desktop; server rebuild was
  required by the stamp contract): `go build -ldflags="-X main.buildSHA=a6cb742c09b4c42bc178e3b01ffd0c5d50fa7d75" -o <lane>/build/litterbox-server-a6cb742c ./cmd/litterbox`
  from `server/` (blob-identical to the freeze); binary sha256
  `6c23d73ed708c219c3efb25aca383e8ed8bd071bf8e4c8d32b0b644d893a5f2e`,
  `go version -m` readback confirms `main.buildSHA=a6cb742c…`.
- Packaged with the canonical recipe
  `deploy/server/debian/build.sh <stamped-binary> 0.1-36 <lane>`. Readback:
  packaged binary sha matches; packaged unit
  `/lib/systemd/system/litterbox-server.service` sha256
  `dd3edbb60dc053479725c857a026bfa5630fdb60a2a52314cdb72fa11b66dc3a` ==
  live `/etc/systemd/system/litterbox-server.service`; `postinst` byte-identical
  to canonical `deploy/server/debian/postinst`; no preinst/prerm/postrm.

### 2b. Desktop deb provenance (no recompilation)

- Reused the existing production binary exactly as briefed:
  `/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018/build/litterbox-qt`
  sha256 `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981`
  (system Qt 6.11.2 + system Kirigami, built at a6cb742c per
  `receipts/repaired-consumer-proof.md` §2; read-only reuse, never rebuilt).
- Staged with `qt/debian` assets (`litterbox-qt.desktop` sha `ea712cf5…`,
  `litterbox-qt.svg` sha `f16be08a…`, `copyright` sha `1ac084c5…`, gzipped
  `changelog` from `qt/debian/changelog` 0.1-36) and built with
  `dpkg-deb --root-owner-group`. Readback: inner `/usr/bin/litterbox-qt`
  sha256 `ead409d51ded…` == the reused binary; metadata
  `litterbox-qt 0.1-36 amd64`; Depends derived from the real binary via
  `dpkg-shlibdeps` plus the canonical QML-module list from `qt/debian/control`.

**dpkg-shlibdeps failure/recovery (exact):** first attempt ran
`dpkg-shlibdeps -O -e <stage>/usr/bin/litterbox-qt` from the lane build dir
without `debian/control` →
`dpkg-shlibdeps: error: cannot read debian/control: No such file or directory`,
empty `SHLIBS`, and the subsequent `dpkg-deb --build` failed with
`parsing file '…/DEBIAN/control' near line 6 … 'Depends' field, missing package name, or garbage`
(exit 2). Recovery: copied the canonical `qt/debian/control` into
`<lane>/build/debian/control` (lane-local; no source mutation) and reran;
`dpkg-shlibdeps` then emitted the full library Depends (one benign warning:
`binaries to analyze should already be installed in their package's directory`)
and the deb built; the final Depends line is recorded in the manifest.

### 2c. Android APK (exact reuse, no relabel)

- Byte-identical copy of
  `/home/kom/proj/ai_pr/litterbox-native-proof-20261018/artifacts/litterbox-qt-v23-a6cb742c.apk`
  into the lane prefix; sha256
  `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` re-derived
  here (matches the brief's named hash).
- Fresh `aapt dump badging` readback in this lane:
  `org.qtproject.example.litterbox_qt` versionCode **23** versionName
  **0.1-36**; `apksigner verify --print-certs` signer SHA-256
  `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37`
  (canonical lineage); `unzip -Z1` confirms
  `lib/arm64-v8a/libcrypto_3.so`, `libssl_3.so`,
  `libplugins_tls_qopensslbackend_arm64-v8a.so`. No rebuild, no signing, no
  relabel of source.

## 3. Live/target facts captured read-only (as-of this run)

- Service `litterbox-server.service` active; `ExecStart=/opt/litterbox/server/litterbox -listen 127.0.0.1:8081`;
  `EnvironmentFiles=/etc/litterbox/db.env` (×2), `server.env.d/ollama-embeddings.conf`,
  `/etc/litterbox/gmail.env`; unit path `/etc/systemd/system/litterbox-server.service`
  sha `dd3edbb6…`; effective drop-ins exactly
  `/etc/systemd/system/litterbox-server.service.d/embeddings.conf` sha
  `bb554e2f7deb0f2c5efcb5ff58fe2379ded174f0d71800d7008e0cede26f3eee` and
  `.../gmail.conf` sha
  `0c0440e3e058835a3db49c405d8b5caf33d6536bc572257b8a504e0835ebf8c4` (drop_ins list
  for the future cohort manifest; no environment contents read or printed).
- Installed: `litterbox-server 0.1-34 amd64` (binary sha256
  `8dfe79fc8f601c97367f525a97942e37ad0a29781cd336032171b9303784d8ea`), `litterbox-qt 0.1-34 amd64`
  (binary sha256 `d8ef906f335a1dff19ba64f10e25bd59c3beb0ec2777e4881d4e99173e086904`,
  matching the 0.1-34 desktop deb's inner binary).

## 4. Production entry/wiring/consumer proof bindings carried from existing receipts

- Android/native: `receipts/mobile-final-consumer-proof.md` §11 (18/18 native
  checklist PASS on this exact APK; 48dp actual-hit and native note Save closed;
  §11f generic original-HTML byte preservation; §11e isolated user-10 normal-UI
  enrollment Online) — source/entry wiring
  `main.cpp`→`androidmailview.cpp`→`MailBodyNative.qml`→`MailView.java`→`MailDetailPage.qml`.
- Desktop/server: `receipts/repaired-consumer-proof.md` §2/§5 — same binary
  `ead409d5…` proven on a6cb742c with real-pointer appbar/back/archive/undo
  smoke and cross-restart offline cached reader; server note SSE PASS §4.
- Operator schema/DB state: `receipts/release-install-preflight.md` §5 (021
  applied by predecessor, 020/011 grants/RLS re-verified read-only here per its
  record), §10 remaining blockers.
- Named R61 B2B original remains **unavailable** — the
  `OriginalMessageConsumerProof` owner's explicit gap; no TECHSHIP/synthetic
  substitute.

## 5. LEFT — precise prerequisites before a plan-ready `cohort.json`

1. **Server rollback deb missing**: installed `litterbox-server 0.1-34` deb not
   found anywhere searched (lane/home/proj caches hold only 0.1-26/30/31
   variants). `release.py validate` requires `target.rollback_debs.server`
   with matching version and inner binary ==
   `8dfe79fc8f601c97367f525a97942e37ad0a29781cd336032171b9303784d8ea`. Desktop rollback deb
   is available: `/home/kom/proj/ai_pr/litterbox-undo-release-cohort/litterbox-qt_0.1-34_amd64.deb`
   (inner binary == installed, verified).
2. **Accepted three-way proof receipts** binding `source_sha` and the exact
   new deb/APK sha256 values above (existing receipts bind source and APK/
   desktop-binary identities, not these deb file hashes).
3. **before_receipt live capture at plan time**: served `signer_sha256`,
   `apk_sha256`, old manifest `version`/`package` (still MISSING per preflight §6).
4. **publisher_inputs** private file: credential loader (enrolled-device token),
   `environment_sha256`, `server_binary_sha256`, origins, proposed immutable APK
   path `/opt/litterbox/server/litterbox-qt-v<code>-<sha8>.apk`.
5. **Publication tuple**: one fresh exact remote URL + fullref + oldOID read by
   the Git owner (named ref was ABSENT at the 02:58 read; no OID substituted).
6. **Version ordering re-read at plan time**: candidate versionCode 23 vs
   served/installed 21 as-of handoff.
7. Cohort manifest finalization + `plan`/`apply` = Main via canonical
   `deploy/release.py` only.

## 6. Retro (required)

- **Done late vs the sizing checkpoint**: the first ~25 minutes were consumed by
  canonical-doc/contract reading and identity discovery with no artifact on
  disk; both Debian artifacts and the staged APK landed only after the P95
  call. The early useful fact — the 99a00861 stamp mismatch in the existing
  server proof binary — should have been surfaced with an ETA immediately
  instead of inside the full run.
- **Honesty held**: the stamped-99a00861 binary was rejected rather than
  relabeled; APK reuse is byte-identical reuse with fresh badging/cert readback,
  not a rebuild claim; the desktop deb packages the exact proven `ead409d5…`
  binary; HEAD-vs-freeze was proved per relevant path, not assumed.
- **One recoverable tooling failure** (dpkg-shlibdeps without `debian/control`)
  is documented verbatim in §2b with its lane-local recovery.
- All mobile/desktop acceptance receipts, docs, criteria and the authorized Git
  handoff are unchanged; this lane touched only the lane prefix and this
  receipt.
