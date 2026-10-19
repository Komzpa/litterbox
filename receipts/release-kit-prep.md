# Cached release kit preparation receipt — ReleaseKitPrep, 2026-10-19

Owner: `ReleaseKitPrep` (gate row "Prepare cached desktop and native release
kits"). Start 2026-10-19T16:56+04:00, finish 2026-10-19T17:18+04:00.
ROOT: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`.

Status: **kit identities verified and separated output paths prepared; nothing
built, installed, deployed, published, signed or run.** No build, install,
deploy, publication, Git fetch/metadata command, shared-installer use (even
`--dry-run`), owner-desktop interaction, or touch of the current
archive/avatar gate copies or builds occurred in this row.

## 1. Separated kit output paths (created 2026-10-19T17:09+04:00, exit 0)

- Desktop: `OUT=/home/kom/proj/ai_pr/litterbox-desktop-proof-20261018` →
  `release-kit-20261019/` (new empty dir; nothing else in OUT modified).
- Native: `NATIVE=/home/kom/proj/ai_pr/litterbox-native-proof-20261018` →
  `release-kit-20261019/` (new empty dir; nothing else in NATIVE modified).

## 2. Cached desktop kit identity (OUT; read-only `file`/`sha256sum`/`grep`, all exit 0)

| Field | Value |
|---|---|
| Artifact | `OUT/build/litterbox-qt` |
| Type | ELF 64-bit LSB pie executable, x86-64, dynamically linked, BuildID `a877fb9f00c7597bd54b95699824e417888a3587` |
| sha256 | `ead409d51ded0d01f40c7e7831b01e725ead83fcff1c493a0b2c6ebad63de981` |
| Qt | **6.11.2 system** (`libQt6Core.so.6` → `/usr/lib/x86_64-linux-gnu`; `Qt6_DIR=/usr/lib/x86_64-linux-gnu/cmake/Qt6`; dpkg `libqt6core6t64 6.11.2+dfsg-5`) — matches DEPLOYMENT.md desktop kit |
| Built | 2026-10-19 01:14 (ninja, per `.ninja_log`) |
| Source binding | **not established by this row** — desktop artifact `source_sha` binding belongs to the accepted three-way proof set (preflight §10.3) |

## 3. Cached native kit identity (NATIVE; read-only, all exit 0)

| Field | Value |
|---|---|
| Build dir | `NATIVE/build-repaired-3eb373fe/` (name as cached; reused across later builds — dir name ≠ source binding) |
| Qt | **6.7.3 android_arm64_v8a** (`Qt6Core_DIR=/home/kom/Qt/6.7.3/android_arm64_v8a/lib/cmake/Qt6Core`) |
| ABI | `ANDROID_ABI=arm64-v8a`, `QT_ANDROID_ABIS=arm64-v8a`; APK `lib/arm64-v8a/` only with bundled OpenSSL 3 [DOC: mobile-ui-proof §1] |
| JDK | build logs `build-repaired-jdk17-3eb373fe.log`, `build-repaired-jdk17-99a0086.log` present and `java-17-openjdk-amd64` installed; no inline `org.gradle.java.home` in cached `gradle.properties` — the exact `JAVA_HOME` export of those builds is **not re-derivable from cache** (§7.3) |
| APKs in `NATIVE/artifacts/` (sha256, mtime) | `litterbox-qt-v23-3eb373fe.apk` `fcdf98fca1cba0427dced9e9cb06eba0d95e190c90e98cc2f22bc04d7ca18a26` 22:36 (run-2, **superseded** per mobile-ui-proof §1); `litterbox-qt-v23-99a00861.apk` `70bf404008e84f56787f762c2b0b07eec7c40da9ad124b3005f932a5e250da77` 23:31; `litterbox-qt-v23-a6cb742c.apk` `b186bf5e77d58dd9476fb2e23ca5b1464a89ef59260e09ff72bd68a2c5125f70` 00:52 (latest; matches `build-app-a6cb742c.log` tail "Android package built successfully … Built target litterbox-qt_make_apk"); `litterbox-qt-v23.apk` `3c58b90eb416103e5a3c56052fd5fcf63ea616c055da6ad1db50b754dade658d` (v1, superseded) |
| Package / versionCode / versionName | `org.qtproject.example.litterbox_qt` / **23** / **0.1-36** [DOC: mobile-ui-proof §1] |
| Signer cert | `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37` (CN=Android Debug, O=Android, C=US — canonical lineage) [DOC: mobile-ui-proof §1] |

## 4. Signing locator (paths and modes only; no secret printed, none loaded)

- Canonical redacted loader: `NATIVE/sign-apk-redacted.py`
  (`-rwxrwxr-x`, 2699 B, 2026-10-18) — password is read **at runtime** from
  the canonical loader source and passed via `env:LB_KEYSTORE_PASSWORD`; this
  row executed no signing and printed no credential.
- Keystore locator: `/home/kom/.local/share/litterbox/android-debug.keystore`
  (exists, `kom:kom`, 2666 B).
- Password loader source: `/home/kom/litterbox-releases/android-update-v6/build-config/build-sign.sh`
  (exists, executable).
- Tooling: `/usr/lib/android-sdk/build-tools/35.0.1/apksigner` and `aapt`
  (paths per loader constants; not invoked).
- Loader guardrail observed: versionCode must be decimal ≥ 23 (served = 21,
  22 consumed); cert must match the lineage SHA-256 above.

## 5. Owned emulator / ADB mapping (read-only probes)

- Owned AVD: `NATIVE/avd/litterbox-native-proof.ini` → target `android-34`;
  launch params (`emu-launch-params.txt`): `-avd litterbox-native-proof
  -no-window -no-audio -no-snapshot -port 5590 -gpu host`.
- Listeners: `127.0.0.1:5590` and `127.0.0.1:5591` (console + ADB bridge for
  the owned emulator), plus `[::1]` twins — `ss -ltn` exit 0.
- `adb devices -l` (exit 0): **`emulator-5590` = `device`, product
  `sdk_gphone64_x86_64`, transport_id 169** — the owned emulator mapping is
  live. The TV `192.168.100.39:5555` also appears in the adb list
  (`transport_id:153`) — it is **not** kit-owned and must never be selected;
  physical `R5CX80MZ1SN` is **absent** from the list. No `adb` mutation
  (install/uninstall/forward/reverse/shell) was run by this row; the required
  two fresh mappings before the journey ADB session remain the journey
  owner's first step against `emulator-5590` only.

## 6. Version-ordering and enrollment prerequisites (preserved measured facts)

- Candidate **23**/0.1-36 strictly above served **21**/0.1-34
  [DOC: handoff §2] — ordering precondition holds as-of those measurements.
- Update-offer proof (existing, untouched):
  `ROOT/receipts/update-offer-proof/` — `00-emulator-https-origin-reachable.png`,
  `01-enrolled-online-v21.png`, `02-update23-offer.png`,
  `03-update23-banner-later.png`: normal **Online** enrollment of an older
  v21 client and its real **Check updates** offer for 23 against the HTTPS
  origin; auth route exercised through the normal UI, no `LB_SERVER`
  [DOC: mobile-ui-proof §3; preflight §7].
- **Current live check not re-run here by design**: the fresh before-receipt
  live capture (served `signer_sha256`, `apk_sha256`, old manifest
  version/package, current offered version) is operator plan-time work —
  already named MISSING in preflight §6/§10.5. This row neither repeated nor
  replaced it.

## 7. Remaining blockers (named, finite — none owned by this row)

1. **Live before-receipt capture** (served signer/apk hash + current offered
   version) at plan time — owner: operator plan run [preflight §10.5].
2. **Accepted three-way proof set** binding `source_sha` + artifact `sha256`
   for server/desktop/android (as-of production source `a6cb742c` per
   preflight §1; desktop binary §2 has no cache-derivable source binding) —
   owner: proof lanes [preflight §10.3].
3. **Exact `JAVA_HOME` of the cached JDK-17 native builds** is evidenced only
   by log filenames and installed JDK 17; a future rebuild must re-export
   `JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64` per DEPLOYMENT.md rather
   than infer it from cache.
4. **Publication tuple**: named ref `refs/heads/ui-desktop-mobile-redesign-20261018`
   was absent at the last fresh readback [preflight §3] — Git owner (Main)
   decides; no Git command run here.
5. **Two fresh ADB mappings** on `emulator-5590` before the journey session —
   journey owner's step (§5); TV and physical phone excluded.

## 8. Retro (required; existing incident reviews referenced, not repeated)

Prior retros in `release-handoff-prep.md` §5 (drift hidden from the operator
runbook) and `release-install-preflight.md` §11 (unexecuted tool text,
recovered with bounded readbacks) already cover those incidents; nothing here
repeats them. New, kit-scoped lesson: the native build dir name
`build-repaired-3eb373fe` outlived its source — three different APKs
(3eb373fe/99a00861/a6cb742c) were produced inside it, so a cached dir name is
not a source binding and kit receipts must record artifact hashes and log
tails, not directory names. This row therefore lists all four APK hashes (§3)
and marks the desktop source binding as unestablished (§7.2) rather than
assuming the folder label. No requirement was weakened; no code, config, test,
Git or runtime state was touched by this row.
