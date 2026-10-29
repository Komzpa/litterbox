# Litterbox — agent rules

Read `REQUIREMENTS.md` and `docs/DEPLOYMENT.md` before changing shipped code.
This file adds one rule that overrides any "done" claim below it.

## A mobile fix is not done until it is released

Any change that reaches the shipped Qt/Android binary or the server that serves it
ends with a **published release**, in the same task, not in a later "publish"
task that can be dropped. Green tests, a built APK, an emulator install, a proof
receipt, a commit, or a desktop `.deb` are *candidates*, not a release.

"Released" means, all four observed on the real updater:

1. A signed APK whose decimal `QT_ANDROID_VERSION_CODE` in `qt/CMakeLists.txt`
   is **strictly greater** than the served version and every recorded installed
   version, built from accepted source with the existing package and signing
   lineage.
2. The APK is installed at a service-readable path and the running server has
   been restarted so its startup snapshot is that file
   (`docs/DEPLOYMENT.md#android-update-release`).
3. The authenticated manifest and the downloaded bytes match that candidate's
   versionCode, package and SHA-256; anonymous requests still get `401`.
4. An older enrolled isolated client presses **Check for app updates** and is
   offered **Update available** for the new version. Same-version client showing
   no offer is the negative control.

Only after all four does the work count as done. If publication is impossible,
keep the task open and name the exact blocker — never report the fix as done.

Oracle: `python3 tools/release_status.py --check` exits non-zero when the working
tree's versionCode, the served manifest, the served APK digest and
`receipts/published-releases.json` disagree, or when `qt/` changed after the
published source commit. Run it before claiming any mobile work complete.

Rollback is a publish of the previous known-good APK at its own versionCode —
never a silent downgrade of an installed phone.

## Minimum bar for other changes

- Server/desktop fixes: the task names which of the three cohort artifacts
  (server `.deb`, desktop `.deb`, Android APK) it publishes, or states that
  none is shipped code.
- Record the release identity (versionCode, versionName, package, SHA-256, source
  SHA, published time, receipt path) in `receipts/published-releases.json` in
  the same commit that publishes.
