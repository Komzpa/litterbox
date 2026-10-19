# Unified release installation preflight — receipts/release-install-preflight.md

Owner: `ReleaseInstallPreflight`, written 2026-10-19 (Asia/Tbilisi).
ROOT: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`, checkout HEAD
`fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` as of this receipt (local branch,
separate from any remote publication ref).

Status: **PREPARATION ONLY. Release is NOT verified, NOT published, NOT
installed, NOT accepted — release stays BLOCKED until the remaining actual
prerequisites in §10 are verified by their owners.**

This receipt covers the original pass (one timestamped named-ref query and one read-only pre-apply schema query) plus the continuation pass that completed §5: canonical target re-verified through both documented mappings first, then read-only 020/021/011/RLS/row-count readbacks — every command `rc=0` except one read-only query syntax error that was corrected immediately (failure disclosed in §5). No fetch, merge-tree, Git metadata mutation, push, installer/upgrader/`release.py` command (including plan or `--dry-run`), service operation, owner-desktop mutation, or card mutation was run. **No SQL mutation was run in the continuation pass**: migration 021 was applied by the approved predecessor operator step (provenance in §5) and was never reapplied here. `/etc/litterbox/db.env` was read privately; credentials and DSN were never printed; no default `kom` database was used. The earlier `ReleaseInstallPreflight` crawl was cancelled after ~28 minutes; recovered unknowns with bounded readbacks, not another research campaign. No secret, token, password or unredacted DSN appears here.

## 1. Canonical operator flow [DOC]

`docs/DEPLOYMENT.md` ("Unified release installation and rollback") is
canonical and unchanged by this receipt:

- `deploy/release.py` is the **sole** cutover entrypoint; no ad-hoc copy,
  environment-edit or restart reconstruction. The release CLI runs no SQL.
- Migrations `server/db/020_bundle_title_origin.sql` and
  `server/db/021_message_sender.sql` are **operator-owned and must complete
  before** the CLI plan/apply/rollback flow. Migration 021 is additive only
  (`messages.sender_name`, `messages.sender_address` are
  `NOT NULL DEFAULT ''`); migration 020's historical rows stay untouched.
- One cohort contains exactly three artifacts (server Debian, ordinary desktop Debian, signed Android APK), all frozen from one production source SHA. `a6cb742c` is the production source; `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` is test-only, not production. Native source may change; these are as-of identities, not a claim of final cohort readiness. Later test/documentation commits are bound separately and never relabel artifacts.
- Installed desktop clients adopt passively through the existing UpdateWatcher:
  after the package replaces the executable and package-manager locks are
  released, a replacement user service starts and the app quits normally. The
  operator never drives or restarts the owner desktop.
- The earlier `99a0086113305af72205c92326f93d44e873bf72` production-freeze statement is stale. As-of this receipt, `a6cb742c` is the production source and `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` is test-only; native source may still change, so these do not establish the final accepted cohort. Acceptance receipts retain their own artifact-proof binding.

## 2. Exact CLI surface and commands [VERBATIM/DOC]

The accepted parser surface (documented; not invoked here) is three actions with the optional
`--receipt` (required for `apply`), `--install-mobile DEVICE_KEY` and
`--adoption-timeout` options, as read from `deploy/release.py` in the old run
and documented in `docs/DEPLOYMENT.md`. No other flag may be invented or
passed. The exact operator commands (placeholders in `<…>`):

```sh
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py plan /private/cohort.json --install-mobile mainphone
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py apply /private/cohort.json --install-mobile mainphone --receipt /var/backups/litterbox/release-ID
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py rollback /var/backups/litterbox/release-ID
```

`plan` performs all hash/source/signature/proof/live-state checks and the
Debian no-act check without installation. `apply` repeats every preflight
before any installation, preserves old packages/APK, executable/environment/unit
bytes and all declared drop-ins and environment sources in a root-private
rollback packet, installs both Debian packages together, atomically replaces
only the three Android keys, and restarts the server once. Without
`--install-mobile` the phone is untouched and the receipt says
`mobile: not_requested`. Any failed readback leaves `status: failed`, never a
success claim. The receipt must record the exact SHA-256 of
`deploy/release.py` alongside the artifact identities.

## 3. Publication tuple — **NAMED REF ABSENT**

The publication tuple uses one fresh exact named-ref readback, not stale observations:

- remote URL: `git@github.com:Komzpa/litterbox.git`
- full ref: `refs/heads/ui-desktop-mobile-redesign-20261018`
- old OID: **the current remote value of that exact full ref**

Historical old-run `HEAD` output `323bd022b6f4fc0ff00d379075ec58bacd4b506c HEAD` is not the named ref and is not substituted.

> Exact named-ref readback, executed once 2026-10-19 (Asia/Tbilisi):
> ```sh
> cd /home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018
> printf 'START '; date -Is
> printf 'URL '; git remote get-url origin
> printf 'LOCAL_BRANCH '; git branch --show-current
> printf 'NAMED_REF\n'; git ls-remote git@github.com:Komzpa/litterbox.git refs/heads/ui-desktop-mobile-redesign-20261018
> printf 'FINISH '; date -Is
> ```
> Start `2026-10-19T02:58:27+04:00`; finish `2026-10-19T02:58:29+04:00`; exit 0. URL exactly `git@github.com:Komzpa/litterbox.git`; fullref exactly `refs/heads/ui-desktop-mobile-redesign-20261018`; result **absent** (successful query, no output line). Local branch separately: `ui-desktop-mobile-redesign-20261018`. There is no old OID in this result. No remote HEAD or local OID substituted and no publication occurred.

## 4. Named-ref publication and rollback plan (Git owner executes; not run here)

For an existing named ref, push with a lease bound to the full ref and exact old OID from §3; local branch stays separate. §3 found absence, so do not substitute an OID or use this example until a fresh readback establishes the publication tuple:

```sh
git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<OLD_OID_FROM_FRESH_READBACK> \
  git@github.com:Komzpa/litterbox.git \
  refs/heads/ui-desktop-mobile-redesign-20261018:refs/heads/ui-desktop-mobile-redesign-20261018
```

Publication rollback is a **direct protected push to the SAME full ref**,
expecting the published OID, never delete/recreate of the ref:

```sh
git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<PUBLISHED_OID> \
  git@github.com:Komzpa/litterbox.git \
  <OLD_OID_FROM_FRESH_READBACK>:refs/heads/ui-desktop-mobile-redesign-20261018
```

Because §3 found the named ref absent, the shown old-OID lease is not executable with an invented value. Main is sole Git owner. Any future publication must explicitly handle absent-ref creation under the exact fullref; rollback remains protected against the published OID and never deletes/recreates the ref. This lane performed only the readback, no Git metadata mutation or publication.

## 5. Operator migration state (020/021/011) — applied by predecessor; re-verified read-only here

**Provenance (recovered by readback, never replayed).** The predecessor gate `ReleaseSchemaPreparation` ran the approved operator step. `history://ReleaseSchemaPreparation` lines 89–97 record, each with **exit 0**: the owner-connection target mapping, the exact `server/db/021_message_sender.sql` read, the transactional apply (`psql --single-transaction -v ON_ERROR_STOP=1`), and the post-apply schema readback. Its final 011 grant/RLS verification was emitted as malformed tool-call text and **was not executed**. This continuation pass ran only that missing verification — bounded read-only SELECT/catalog queries. **No 021 reapply, no SQL mutation, no DDL/DML of any kind occurred in this pass.**

**Target re-verified first — both canonical mappings (as-of `2026-10-19T03:30:14+04:00`; DSN/credentials never printed):**

- `/etc/litterbox/db.env`: `sudo -n stat -c 'ENVFILE mode=%a owner=%U:%G' /etc/litterbox/db.env` → `ENVFILE mode=640 owner=root:litterbox` — **rc=0**; private shape guard `case "$DATABASE_URL" in postgresql://litterbox:*@127.0.0.1:5435/litterbox*|"postgresql://litterbox_app:"*@127.0.0.1:5435/litterbox*)` → `DSN_SHAPE_ACCEPTED` — **rc=0**.
- Owner socket path: `sudo -n -u postgres psql -h /var/run/postgresql -p 5435 -d litterbox -X -v ON_ERROR_STOP=1 -At -c "SELECT 'target'||chr(47)||current_database()||chr(47)||current_user||chr(47)||current_schema()||chr(47)||version()"` → `target/litterbox/postgres/public/PostgreSQL 18.6 (Debian 18.6-3) … 64-bit` — **rc=0**.
- Auth app-role connection, guarded by the exact pattern `postgresql://litterbox_app:*@127.0.0.1:5435/litterbox\?sslmode=prefer` (mismatch → `REFUSED`, exit 40): → `auth/litterbox_app/litterbox/public/127.0.0.1/32:5435/18.6` — **rc=0**.

**Readbacks (owner socket path `-h /var/run/postgresql -p 5435 -d litterbox`, `-X -v ON_ERROR_STOP=1 -At`, all rc=0 unless noted):**

- **C1 — 020/021 columns** (`information_schema.columns`): `col/bundles/title_origin/text/NO/'unknown'::text`, `col/messages/sender_name/text/NO/''::text`, `col/messages/sender_address/text/NO/''::text`. Both 021 columns are `text NOT NULL DEFAULT ''`; `bundles.title_origin` is exactly the 020 expectation.
- **C2 — 011 grant** (`role_table_grants`, grantee `litterbox_app`, table `public.card_bodies`): `grant011/litterbox_app/DELETE,INSERT,SELECT,UPDATE` — all four privileges `SELECT`, `INSERT`, `UPDATE`, `DELETE` granted.
- **C3 — RLS/policies** (`pg_class` + `pg_policy`): `rls/true/true/1` and `policy/card_bodies_tenant_policy` — row-level security enabled **and** forced, exactly one policy, the tenant policy name retained.
- **C4 — row counts**: `count/messages/4833`, `count/card_bodies/1963`, `count/bundles/433`. Consistent with the prior post-apply readback: no mutating statement ran between the predecessor's apply and this read (predecessor history after apply = readbacks only; this pass = SELECT only).
- **C5 — sender default invariant**: `sender/0/0/0` — 0 NULL `sender_name`, 0 NULL `sender_address`, 0 rows with either column non-empty; every pre-existing row carries the applied default and the data is unchanged.
- **C6 — 020 preserved**: `title_origin/seed/39`, `title_origin/topic/15`, `title_origin/unknown/379` (433 = the bundles count) — migration 020's historical titles untouched.
- **App-role cross-checks (guarded auth connection, rc=0)**: `appcols/sender_address,sender_name` (app role sees both 021 columns); `self_grant/true` (`has_table_privilege(current_user,'card_bodies','SELECT,INSERT,UPDATE,DELETE')`).

**Failure disclosure:** the first formulation of the count query failed with `ERROR: ORDER BY position 2 is not in select list` — **rc=1**, a read-only query syntax error with zero database-state effect; the corrected query returned rc=0 (C4). No other command in this pass failed.

**Rollback / back-compatibility:** migration 021 is additive. On application rollback the columns remain in place — `text NOT NULL DEFAULT ''` is ignored by the old binary — so rollback is **no DROP, no column removal, no backfill, ever**; rollback never mutates this schema.

## 6. Installed and served identities — historical as-of, **not** current claims

Recorded from the prior run (2026-10-18, ROOT host) and the handoff receipt;
these are as-of facts, deliberately **not** restated as present state:

| Fact | Value | As-of / evidence |
|---|---|---|
| Installed desktop + server packages | both Debian packages `0.1-34`, `amd64` (binary SHA-256s captured in the old run; values not preserved in the transcript) [OLD-RUN] | 2026-10-18 old run; re-readback required at plan time |
| Served server build | `GET /v1/version` reported `server_build=323bd022…` [OLD-RUN] | ~02:32, 2026-10-18 |
| Anonymous rejection | unauthenticated `/v1` route returned `401` on both origins [OLD-RUN] | 2026-10-18 old run |
| Served Android release | versionCode **21** / versionName **0.1-34** [DOC: `receipts/release-handoff-prep.md` §2] | handoff receipt, 2026-10-18 |
| Candidate Android package | `org.qtproject.example.litterbox_qt` [DOC: handoff §2] | handoff receipt |
| Candidate signer certificate SHA-256 | `a36ea8f6c4aa164adbbfb07ccbd4161ef2756b9bbdeef37123a7fe30d9e76a37` (CN=Android Debug, O=Android, C=US — canonical lineage) [DOC: handoff §2] | handoff receipt |
| Superseded candidate APK | `RUNTIME/artifacts/litterbox-qt-v23-3eb373fe.apk`, sha256 `fcdf98fca1cba0427dced9e9cb06eba0d95e190c90e98cc2f22bc04d7ca18a26`, versionCode 23 — built from `3eb373fe`, **not** the final cohort [DOC: handoff §1/§2] | handoff receipt |
| Served signer/apk identity for the before-receipt | **MISSING** — must be captured from live served state at plan time (`signer_sha256`, `apk_sha256`, old `manifest.version`/`package`) | owner: operator plan run |
| Served environment key names | `LITTERBOX_ANDROID_APK_PATH`, `LITTERBOX_ANDROID_VERSION`, `LITTERBOX_ANDROID_PACKAGE` (exact current values are private operator state; never printed here) [DOC] | DEPLOYMENT.md |

## 7. Install/adoption readback set required for completion [DOC]

Completion of the release (not of this receipt) requires the full `apply`
readback: installed package/executable identities; running server mapping and
source revision (`server_build` must equal the frozen release commit, never
`unknown`); authenticated same-origin Android manifest (`GET
/v1/android/update`, with the enrolled-device Bearer token) and downloaded APK
SHA-256 at every configured origin; anonymous `401` retained; passive desktop
adoption through UpdateWatcher executable mappings (no owner-GUI drive); and,
only with explicit `--install-mobile`, narrow phone package/version/hash
readback through the canonical phone lock and verified transport/hardware
identity. Normal Android delivery additionally needs an older normally
enrolled isolated consumer's real **Check updates** offer for the published
version, retained without approving installer/unknown-source prompts;
physical-phone ADB is never a release prerequisite. Any failed readback leaves
`status: failed`.

## 8. Rollback (installed state) [DOC]

`sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py rollback
/var/backups/litterbox/release-ID` verifies its saved hashes, reinstalls both
old Debian packages, restores exact old files including declared drop-ins and
all saved environment sources with ownership/modes, restarts the same service,
and rechecks complete composition, old package/process/publication identities
and desktop adoption. The old phone APK is preserved, but Android
downgrade/uninstall/data restoration is never automatic — the rollback receipt
must explicitly report that phone limitation. Publication rollback is the
§4 same-ref direct protected push. The nginx proxy file
(`/etc/litterbox-remote/nginx.conf` on `komzpa.net`) is backed up and
restored-in-place per `docs/DEPLOYMENT.md`, never reconstructed.

## 9. Secrets

No secret is printed in this receipt. APK signing goes only through the
canonical redacted loader `RUNTIME/sign-apk-redacted.py`; the phone/Android
credential loader is a private JSON enrolled-device token; the CLI never prints
loader contents or child-command output. `/etc/litterbox` contents never enter
packages, cohort manifests or receipts.

## 10. Release remains BLOCKED — remaining actual prerequisites

1. **Publication is not readied**: the exact named-ref is absent at fresh read. Git owner must determine the intended publication action; any future push lease must bind exact fullref and expected OID, and rollback must be a protected push to that same fullref expecting the published OID, never delete/recreate. No publication here.
2. **Migration 021 + 011 grant — SATISFIED (evidence in §5)**: 021 was applied by the approved predecessor operator step (history lines 89–97, exit 0) and re-verified read-only here — both columns `text NOT NULL DEFAULT ''` with all existing rows at the default (`sender/0/0/0`), `bundles.title_origin` exactly as 020 expects, `litterbox_app` holds `SELECT,INSERT,UPDATE,DELETE` on `card_bodies`, RLS enabled+forced with `card_bodies_tenant_policy` retained, counts 4833/1963/433 consistent with the prior readback. No longer a blocker; the remaining blockers are items 1 and 3–6 below.
3. **Accepted three-way proof set** binding `source_sha` and each artifact
   `sha256` for `server`, `desktop`, `android` at the final accepted source, plus
   server/desktop Debian builds and signed APK from that same source. As-of
   identities above are not final-cohort acceptance; owners: native/desktop/
   server proof lanes.
4. **Cohort manifest** (`cohort.json`, absolute private path) with the exact
   §-contract fields including `drop_ins` list, publisher inputs, before
   receipt, and rollback debs; exact CLI SHA-256 recorded separately.
5. **Before-receipt live capture** at plan time: served `signer_sha256`,
   `apk_sha256`, old manifest version/package (§6 row "MISSING").
6. **Version ordering**: signed candidate decimal `versionCode` strictly above
   both served (21 as-of handoff) and recorded installed (21/0.1-34 as-of)
   releases, re-read at plan time.

All acceptance/docs/authorized Git handoff from prior lanes are retained. Only this receipt was edited; no code, config, tests or other documents were touched.

## 11. Retro (required — prior owner failed this row)

The earlier preflight took ~28 minutes on a broad crawl and was recovered with bounded readbacks, not another research campaign: the exact named-ref result (§3) and the migration state (§5) come from real executed commands with recorded exit codes. The predecessor operator row **did apply the actual SQL** and failed only at its final 011 grant/RLS verification, which was emitted as malformed, unexecuted tool-call text; this pass recovered exactly that gap with read-only readbacks and **never replayed the apply**. One read-only query formulation failed (`rc=1`, ORDER BY syntax) and was corrected immediately — disclosed in §5 rather than hidden. Remaining gaps are explicitly listed in §10; earlier broad-discovery claims do not substitute for these readbacks.
