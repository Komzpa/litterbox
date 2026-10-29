# Litterbox — agent rules

Read `REQUIREMENTS.md` and `docs/DEPLOYMENT.md` before changing shipped code.
This file adds one rule that overrides any "done" claim below it.

## A mobile fix is not done until it is released

Any change that reaches the shipped Qt/Android binary or the server that serves it
ends with a **published release**, in the same task, not in a later "publish"
task that can be dropped. Green tests, a built APK, an emulator install, a proof
receipt, a commit, or a desktop `.deb` are *candidates*, not a release.

"Released" means, all five observed on the real updater:

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
   offered the new version — the banner reads **Update ready** on older builds
   and **Update available** on newer ones; either counts, and the file it
   downloaded must hash to the served candidate. A same-version client shows no
   offer and downloads no `update-*.apk`; that is the negative control.
5. The **server artifact** from the same cohort is deployed and running: its
   `/v1/version` build SHA contains the published client source, so every route
   the shipped client calls already exists. v36 shipped its APK while the server
   stayed on the previous release, and the crash uploader and journal delete
   met `404`/`422` against it. An APK-only release is not a release.

Only after all five does the work count as done. If publication is impossible,
keep the task open and name the exact blocker — never report the fix as done.

Oracle: `python3 tools/release_status.py --check` exits non-zero when the working
tree's versionCode, the served manifest, the served APK digest and
`receipts/published-releases.json` disagree, when `qt/` changed after the
published source commit, when a migration's objects are absent from production,
or when the deployed server's `/v1/version` build does not contain the published
source. Run it before claiming any mobile work complete.

Rollback is a publish of the previous known-good APK at its own versionCode —
never a silent downgrade of an installed phone.

## Minimum bar for other changes

- Any release of shipped code publishes **all three** cohort artifacts from one
  source SHA — server `.deb`, ordinary desktop `.deb`, signed Android APK —
  through `deploy/release.py plan/apply/rollback` (REQUIREMENTS R39). Naming a
  subset is not a release: the v36 cutover published the APK alone and left both
  the server and the desktop package on the previous cohort.
- Record the release identity in `receipts/published-releases.json` in the same
  commit that publishes: versionCode/versionName/package/SHA-256/source/published
  time/receipt for the APK, plus an `artifacts` map with the same fields for the
  server and desktop packages. An entry without `artifacts` fails the oracle.
