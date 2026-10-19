# Release handoff preparation receipt — ReleaseHandoffPrep, 2026-10-18

Status: **preparation documented; release NOT published, NOT installed, NOT accepted.**
This receipt records documentation work and the finite pending handoff fields.
No `deploy/release.py` invocation, no build, no deploy, no Git write, no
installer or shared-installer use (including `--dry-run`), no production DB or
card mutation, no owner-desktop or physical-phone/TV interaction occurred.

## 1. Source freeze

- Production + test source freeze commit:
  `99a0086113305af72205c92326f93d44e873bf72`, parent
  `3eb373fe30291d1b1f232fd6e0716e2c5d914764`
  (`standaloneown.git`, ROOT `litterbox-ui-desktop-mobile-20261018`).
- Commit `99a0086` contains exactly seven files: production
  `qt/qml/InboxView.qml`, `qt/pages/MailDetailPage.qml`,
  `server/internal/gmailsync/sync.go`, plus tests
  `qt/tests/tst_mailopen.qml`, `qt/tests/tst_bundleexpand.qml`,
  `qt/tests/tst_inboxlayout.qml`, `qt/tests/tst_uirequirements.qml`.
- The final cohort's frozen production source SHA is now fixed at
  `99a0086113305af72205c92326f93d44e873bf72`; the server Go build stamp check
  (`main.buildSHA=99a0086113305af72205c92326f93d44e873bf72`,
  `deploy/release.py` validate) must be built from this commit. The earlier
  `3eb373fe`-based artifacts (including the §2 APK) predate the fix and are
  **not** the final cohort; all native/desktop/server artifacts must be
  rebuilt from `99a0086` (native build of the new candidate in progress).
- All three cohort artifacts (server Debian, desktop Debian, signed Android APK)
  must freeze from one and the same production source SHA. Later test-only or
  documentation-only commits — including this receipt and the
  `docs/DEPLOYMENT.md` edits below, which Main will commit separately — are
  bound separately and never relabel the frozen artifacts.

## 2. Current candidate and served state (read-only facts)

| Item | Value | Evidence |
|---|---|---|
| Candidate Android package | `org.qtproject.example.litterbox_qt` | `receipts/mobile-ui-proof.md` §1 (aapt badging) |
| Candidate versionCode / versionName | **23 / 0.1-36** (no bump) | `receipts/mobile-ui-proof.md` §1 |
| Candidate signer cert SHA-256 | `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37` (CN=Android Debug, O=Android, C=US) — canonical lineage | `receipts/mobile-ui-proof.md` §1 |
| Repaired candidate APK | `RUNTIME/artifacts/litterbox-qt-v23-3eb373fe.apk`, sha256 `fcdf98fca1cba0427dced9e9cb06eba0d95e190c90e98cc2f22bc04d7ca18a26` | `receipts/mobile-ui-proof.md` §1 |
| Served release | versionCode **21** / **0.1-34** | release brief; owner-reported installed 0.1-34 (`REQUIREMENTS.md`, 2026-10-15) |
| Publication / installation | **none for this cohort** — no `plan`, `apply` or `rollback` run; served and installed state unchanged | this run performed no CLI invocation |

The candidate's versionCode 23 is strictly above the served 21, satisfying the
version ordering rule; that is a preflight precondition, not an acceptance.

## 3. Finite handoff fields still PENDING (no placeholders, no completion claim)

1. **Accepted proofs** — `deploy/release.py` requires accepted identity proofs
   binding `source_sha` and each artifact `sha256` for all three of
   `server`, `desktop`, `android` (`proof_gates`). No such accepted three-way
   proof set exists yet for the final cohort, and no native/desktop/server
   proof has been accepted for `99a0086`. The §2 APK
   (`RUNTIME/artifacts/litterbox-qt-v23-3eb373fe.apk`, 3eb373fe-based) is
   **not** the final-cohort candidate; a new native candidate built from
   `99a0086` is in progress. The owned-emulator native proof must run on the
   exact signed candidate built from the freeze SHA (enrolled through the
   normal UI, no `LB_SERVER`, no pre-enrolled profile, Online against the
   intended origin); a rebuild or new build receipt alone does not pass that
   gate.
2. **Full frozen artifact source SHA** — resolved: the source freeze is
   `99a0086113305af72205c92326f93d44e873bf72` (§1). Every cohort artifact's
   `source_sha` must equal it; artifacts built from `3eb373fe` are superseded
   and never the final cohort.
3. **Artifact hashes** — server Debian (`litterbox-server`, binary sha256),
   desktop Debian (`litterbox-qt`, binary sha256) are not built/frozen in this
   handoff; only the Android candidate APK hash in §2 is recorded. The exact
   release-CLI commit and SHA-256 of `deploy/release.py` must also be recorded
   separately from the artifact source SHA.
4. **Version certificate** — APK badging (package/versionCode) and signer
   certificate lineage are recorded for the candidate (§2); server and desktop
   package versions for the final cohort are not yet pinned, and the before
   receipt (`signer_sha256`, `apk_sha256`, old `manifest.version`/`package`)
   must be captured from the live served state at plan time.
5. **Installed readback** — none exists. Completion requires the full
   `apply` readback: installed package/executable identities, running server
   mapping and source revision, authenticated same-origin manifest/download
   hashes at every origin, anonymous rejection, passive desktop adoption via
   UpdateWatcher executable mappings, and (with explicit `--install-mobile`)
   narrow phone package/version/hash readback. Any failed readback leaves
   `status: failed`; publication success alone is not all-three completion.

## 4. Operator runbook updates made (docs/DEPLOYMENT.md, additive; Main commits separately)

- Migration section: operator-owned `server/db/020_bundle_title_origin.sql`
  **and** `server/db/021_message_sender.sql` required before release; 021 is
  additive (`sender_name`/`sender_address` `NOT NULL DEFAULT ''`, legacy rows
  keep empty sender), 020's historical rows untouched; the CLI runs no SQL.
- Frozen-SHA section: all three artifacts bind one production source SHA;
  later test/doc-only commits bound separately, never relabeling artifacts.
- Build-kit section: desktop Qt 6.11 + system Kirigami vs Android Qt 6.7.3 /
  Kirigami Android prefix / bundled OpenSSL 3 / `JAVA_HOME` pinned to JDK 17
  (host JDK 21 breaks R8/D8; `qt/android-build.sh` still defaults to 21) /
  Gradle bounded to two workers; APK signed only via canonical redacted loader
  `RUNTIME/sign-apk-redacted.py` (password loaded at runtime, never printed,
  no secret in docs/cohort/receipts).
- CLI surface: exactly `plan`/`apply`/`rollback` with `--receipt`,
  `--install-mobile`, `--adoption-timeout` as parsed in `deploy/release.py`;
  no invented flags.
- Native gate: enrolled owned-emulator proof on the exact signed candidate is
  the only pass; rebuild/wrapper/launch-only/disposable-server receipts do not.

No requirement was weakened; no UI copy or test oracle was changed.

## 5. Retro

The previous release tail hid concrete preparation: build-kit drift (JDK pin,
Gradle workers), migration 021, and the frozen-SHA binding rule were known in
lanes but not written into the operator runbook, so a later operator could have
rebuilt with the broken host JDK 21 or skipped migration 021. This handoff makes
that preparation explicit in `docs/DEPLOYMENT.md` and records the pending
fields in this receipt. All original criteria, documents and the Git handoff
remain preserved; nothing was committed, deployed or installed by this lane.

Freshness update (2026-10-18, ReleaseHandoffRefresh): this receipt's §1/§3
were originally written while the visibility/upsert-fix source was still
uncommitted at `3eb373fe`; the commit then landed as `99a0086` (parent
`3eb373fe`) while the handoff still described it as pending — a stale-source
read. The §1/§3 clauses were refreshed to the freeze commit `99a0086`; the
§2 candidate APK (3eb373fe-based) is superseded, not the final cohort.
Documentation remains uncommitted and will be committed separately; that later
doc commit never relabels the frozen artifacts. No commands, no Git writes, no
source edits were made by this refresh.
