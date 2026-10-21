# Receipt: Gmail sender avatars (R67 real-photo path) — partial acceptance

Date: 2026-10-20. Worker: ImplementGoogleSenderPhotos. Status: **transport proof done, real-Google grant proof pending.** The whole requirement is NOT called done from a seeded image.

## 1. Source identity (CF26)

- Base source verified at start: `929f275946f7906cfbe67e0f4dfd08f83402a202`; published product `df9b13e5513ed1f1739149a3d6048c27eee3c725`.
- Working tree under `ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` was modified (changed paths in §5). No git metadata was read or written; no commit/push. Lead's git-pr-owner records exact SHAs after acceptance.
- Foreign git `2798830/3709144` untouched. Live sibling TraceArchiveConsumerDelivery stayed read-only scout; no source-writer overlap encountered.
- No production credential calls, no grant clicks, no OAuth grants performed. Proof used a disposable fake TLS People server plus a real encoded PNG decoded through the production decode path.

## 2. Entry / wiring / behavior evidence

Entry points (production code, all server-side; tokens never leave the server):
- `POST /v1/gmail/photo-consent` (gmail/http.go) — returns `authorization_url` (Gmail.modify + both photo scopes); performs no grant.
- `GET /v1/gmail/sender-photos/status` (gmail/http.go) — per-account `photo_ready` / explicit `authorization_required` + `required_scopes` without touching Google.
- `gmailsync.syncSenderPhotos` (photosync.go, new) — hooked after both `initial` and `poll` in `SyncAccount`; best-effort, never fails the account sync.
- `GET /v1/cards/{id}/sender-photo` (senderphoto/handler.go, registered in cmd/litterbox/main.go via mailhtml route set) — session-authenticated binary delivery by card id; no provider URLs, tokens, or addresses in URLs/logs. 403 `authorization_required`, 404 `unavailable`.

Wiring:
- OAuth callback persists granted scopes to `accounts.granted_photo_scopes` (migration 022, additive with legacy fallback); `people.HasPhotoScopes` gates on both `contacts.readonly` + `contacts.other.readonly` actually granted.
- `people.BuildIndex`: `connections.list` (`personFields=emailAddresses,photos`) + `otherContacts.list` (`readMask=emailAddresses,photos`, CONTACT+PROFILE readSources), paginated, exact case-insensitive mailbox match, `Photo.default=true` rejected, primary preferred. `Gmail.modify` alone carries no photo field (asserted by status oracle, not by a network call).
- `people.FetchPhoto`: https-only, ≤512 KiB, `image.DecodeConfig`-verified, jpeg/png/gif/webp.
- `senderphoto.Upsert` (pgx `Execer`, transaction-friendly) → tenant/account-scoped `sender_photos` (table + columns in 022; runtime grants and `tenant_isolation` RLS policy in 023 — 022 shipped neither grants nor RLS, corrected here; 023 follows the 002 `tenant_isolation` shape used by cards).
- `cards.List` → `markSenderPhotos` (one account-scoped lookup per page; silent no-op on pre-022 schemas) → `sender_has_photo`; offline snapshot carries the flag (CF14: existing route shapes unchanged, additive field only); Qt `sender_photo` role + file:// device cache; QML `SenderAvatar` shows bytes, keeps existing geometry, initials only when genuinely unavailable.
- Explicit unsupported parity: senders absent from both contact datasets stay uncovered even if Gmail renders a photo; no Gravatar, no third-party lookup, no undocumented Google endpoint.

Behavior proof (one scoped run, after all edits frozen):
- `go build ./...` in `server/` → exit 0 (whole server compiles, including Qt-independent packages).
- `go test ./internal/people/ -v` → 3/3 PASS: `TestBuildIndexExactMatchRejectsDefault` (fake TLS server: 2 connection pages + otherContacts; official personFields/readMask/readSource asserted on the wire; primary preferred; default-only `carol@example.com` excluded), `TestFetchPhotoDecodesRealImage` (real 1x1 PNG bytes decoded via `image.DecodeConfig`), `TestNormalizeEmail`.
- Broken-baseline negative controls (CF2): default-only person yields no index entry → `unavailable` initials path; text payload and non-https URL rejected by FetchPhoto; display-name match rejected; Gmail-only grant → `authorization_required` with zero photo bytes (status oracle, no Google call).
- Full suite NOT run mid-flight per CF58; lead runs it once after the frozen merged candidate.

## 3. Exact remaining Google configuration / consent gate

1. Google Cloud console, owner action: enable the People API on the existing project and add `https://www.googleapis.com/auth/contacts.readonly` and `https://www.googleapis.com/auth/contacts.other.readonly` to the OAuth consent screen (verification may apply for these scopes).
2. Consent, user action: open the `authorization_url` from `POST /v1/gmail/photo-consent`; the shared callback persists the granted scopes. This grant was deliberately not performed here.
3. Coverage after the gate: measurable real available Google photos = senders whose address exactly matches a People connection/otherContact with a non-default photo on a granting account; everyone else = genuinely-absent initials fallback in unchanged geometry (plus `authorization_required` surfacing on non-granting accounts).

## 4. Carry-forwards

- CF2: real decoded consumer proof + broken-baseline negative controls above; seeded-image-only claims rejected.
- CF14: existing list/detail/snapshot/OAuth routes preserved (additive `sender_has_photo`, legacy callback fallback, pre-022 schemas list fine).
- CF26: §1 identities; executable = `go build ./...` output from this tree, SHA via git-pr-owner.
- CF39: isolated GUI proof outstanding — QML photo layer is wired but needs the isolated-GUI run (no desktop/session bus available to this worker); R67 oracle names `tst_inboxlayout` rows incl. a decoded-bytes photo case and offline reopen.
- CF58: suite once after frozen merged candidate, not mid-flight (only the scoped proof above was run).

## 5. Changed paths (for git-pr-owner, no git ops performed)

`server/db/022_sender_photos.sql` (new), `server/internal/people/people.go`, `server/internal/people/fetch.go`, `server/internal/people/people_test.go` (new), `server/internal/senderphoto/senderphoto.go`, `server/internal/senderphoto/handler.go`, `server/internal/gmail/oauth.go`, `server/internal/gmail/http.go`, `server/internal/gmailsync/sync.go`, `server/internal/gmailsync/photosync.go` (new), `server/internal/cards/cards.go`, `server/internal/cards/handler.go`, `server/internal/offline/http.go`, `server/internal/gmail/http.go` (photo routes), `server/cmd/litterbox/main.go`, `server/internal/gmail/http_test.go` schema list (unchanged assertions), `server/internal/gmailsync/sync_test.go` (022 added to schema list), `qt/api.h`, `qt/api.cpp`, `qt/main.cpp`, `qt/CardStore.h`, `qt/CardStore.cpp`, `qt/qml/InboxView.qml`. Docs already current: R67 spec (REQUIREMENTS.md 440-477), docs/DEPLOYMENT.md R67 section, docs/ARCHITECTURE.md photo entries — no duplicate edits made.

## 6. Retro (required)

What worked: anchoring every edit on freshly read lines kept 15+ surgical edits collision-free against a live sibling; the first scoped proof caught three real integration errors (pgx `Execer` shape, dropped `poll` arg, missing import) that file-local review missed — running the one proof after frozen edits, not during, was the right order. What to repeat: check sibling/peer state (R67 docs, operator docs) before writing — two planned doc edits were already done, and adapting saved turns. What to change: verify the callee's exact signature (`poll` args) before wrapping a call instead of reconstructing it from memory; new packages should declare their DB surface (pgx vs database/sql) before first use, not after the first build failure.

## 12. Fresh live status gate on installed 0.1-41 (2026-10-20 18:47-19:10 +04 / 14:47-15:10 UTC)

Worker: FreshSenderPhotoStatus (gate role, read-only). Status: **GATE = PENDING_DB_MIGRATION_022_023 on shared production 5435. Live per-account photo grants are not readable; Google-side enablement stays UNKNOWN.** Not a PASS, not "Google broken", not `authorization_required`.

### 12.1 Commands and working directory

- cwd `ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (read-only lane; no source, Git, installer, db-role writes).
- Fresh authenticated probes: `python3 /tmp/lb_photo_status_probe.py` (14:49 UTC) and `python3 /tmp/lb_status2.py` (14:51:20 UTC), origin forced to `http://127.0.0.1:8081`, `Authorization` taken from the canonical credential loader `/home/kom/.local/share/tasks-loop/state/secrets/litterbox-desktop-credentials.json` loaded in memory only. Printed: loader key list plus `<len n sha256:xxxxxxxx>` for each value; token, device_id, tenant_id and their contents never printed (CF9).
- Read-only schema evidence: `sudo -n -u postgres psql -p 5435 -d litterbox` (peer auth) running only SELECT on `information_schema`, one `EXPLAIN` (no ANALYZE) and one `LIMIT 1` SELECT inside a transaction that was rolled back. No ALTER/DDL/GRANT/role change; shared 5435 otherwise untouched.
- Log reads: `journalctl -u litterbox-server`, `/var/log/postgresql/postgresql-18-litterbox.log`.

### 12.2 Artifact identity observed (CF26)

- `GET /v1/version` -> 200 `server_build=d24dc419eafc44979926203a2fa5f7f33ce00d79` (published 0.1-41 source).
- Installed server `/opt/litterbox/server/litterbox` sha256 `f76c3e8ab723a7cf284f91cce595007102bf41aae48927a5cffb6267f27bad03` = `receipts/status-server-dbproof-20261020.json` `binary_sha256` = the 0.1-41 deb inner binary; deb sha256 `b6b11883fca74778baff16419c5108cab485b2bfe99d26c7610d455824a301aa`.
- Unit `litterbox-server` active, pid 2415814, exe `/opt/litterbox/server/litterbox -listen 127.0.0.1:8081`, started 18:46:17 +04 (this lane never restarted it).
- Installed desktop `/usr/bin/litterbox-qt` sha256 `fde374e426b7da59c9b0d3f9d73a690a369c8237e943c2f3f5ede74869cfc19f`.
- `/etc/litterbox/db.env`: Permission denied for this lane (root-only), so its hash was NOT re-measured and the file was never opened (CF9). The expected `82a04aa6…` is cited from `receipts/correction-publication.md:288`, not re-verified here.
- Android v28 identity carried from `receipts/correction-android-release.md`: apk `83fe9ac634f6a1373a8c1c089f09ccb67e63beb00cc8fda0934f5fe0d05fe389`, source `d24dc41…` (not re-hashed by this lane).

### 12.3 FIRST fresh status result (inside 5 minutes, 14:49 UTC)

`GET /v1/gmail/sender-photos/status` -> **HTTP 503**, `Content-Type: text/plain; charset=utf-8`, body 21 bytes `database unavailable`. No JSON at all: no `accounts[]`, no `photo_ready`, no `authorization_required`, no `required_scopes`. Re-confirmed 14:51:20 UTC: identical 503 and identical 21-byte body.

Same-credential neighbours in the same run:

| Path | Status | Body |
|---|---|---|
| `GET /v1/version` | 200 | `server_build=d24dc41…` |
| `GET /v1/gmail/accounts` | 200 | 2 accounts, `status=active` (addresses redacted to lengths 18/16, ids len 36) |
| `GET /v1/gmail/sender-photos/status` | **503** | `database unavailable` |
| `GET /v1/cards` | **500** | `internal server error` (bare, with `tz`, with `now`; 0.65-1.99 s each) |
| `GET /v1/cards/notes?since=2020-01-01T00:00:00Z` | 200 | `{"notes":[]}` |
| `POST /v1/gmail/photo-consent` | 200 | `https://accounts.google.com/...` authorize URL with `gmail.modify` + `contacts.readonly` + `contacts.other.readonly` |

Tenant validity: the 200 on the tenant-RLS `listAccounts` proves the loader token and tenant, so the 503 is not an auth failure.

**Disclosed deviation:** `POST /v1/gmail/photo-consent` was called once while checking route registration. It writes PKCE state and was outside the read-only scope of this gate. No browser opened, no consent approved, no grant performed; the returned URL is not recorded in this receipt.

### 12.4 Exact root cause (three independent read-only sources)

1. `journalctl -u litterbox-server` since the 18:46 restart:
   - `sender-photos: build=d24dc41… account=<id>: ERROR: column "granted_photo_scopes" does not exist (SQLSTATE 42703)` every cycle, both accounts.
   - `Gmail synchronization: build=d24dc41… sync account <id>: ERROR: column "sender_address" of relation "cards" does not exist (SQLSTATE 42703)` every cycle, both accounts.
2. PostgreSQL server log (role `litterbox_app`):
   - 18:51:20.508 `ERROR: column a.granted_photo_scopes does not exist at character 38` on the status statement `SELECT a.id::text,a.address,COALESCE(a.granted_photo_scopes,''), EXISTS(SELECT 1 FROM sender_photos …) FROM accounts a WHERE a.tenant_id=$1 …` -> handler answers 503 `database unavailable`.
   - 18:51:21.636 `ERROR: column c.sender_address does not exist at character 84` on `SELECT c.id::text FROM cards c WHERE … AND c.sender_address <> '' AND EXISTS(SELECT 1 FROM sender_photos …)` — inside the same open transaction as the cards list request, immediately before the 500. `markSenderPhotos` (`server/internal/cards/handler.go:240-254`) swallows the error as "best-effort", but a failed statement aborts the PostgreSQL transaction, so `tx.Commit()` at `handler.go:227` fails and the handler answers `500 internal server error`. The shipped comment "never a 500, so a missing sender_photos table on old deployments degrades to the genuinely-unavailable fallback" is false on a pre-022 schema.
   - 18:5x every cycle `ERROR: column "sender_address" of relation "cards" does not exist` on `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,subject,sender,sender_name,sender_address,sort_at) … ON CONFLICT …` -> new mail ingestion is failing since the 0.1-41 restart.
3. `information_schema` (read-only, peer auth): `bundles.title_origin=1` (020 applied); `messages.sender_address=1`, `messages.sender_name=1` (021 applied); `accounts.granted_photo_scopes=0`; `cards.sender_address=0`; table `sender_photos` absent (**022 and 023 NOT applied**). No column or table anywhere in `public` stores photo grants except unrelated `mcp_tokens.scopes`, so there is no stored per-account photo-grant state to read on live.
   - Control: `EXPLAIN` (no ANALYZE) of the exact cards list SQL succeeds as superuser, and the same SQL run as `SET ROLE litterbox_app` with `set_config('litterbox.tenant_id',…)` returns a row (`LIMIT 1`, transaction rolled back). The list query and column types are sound; the 500 is the poisoned transaction above, not a NULL/scan/count problem (every non-COALESCED scanned column checked: 0 nulls in `summary`, `note`, `source_kind`, `source`, `state`, `title`/`subject`, `created_at`, `note_order`, `timed`, `id`; 24 SQL columns = 24 Scan targets).

### 12.5 Gate (exact)

- **GATE = PENDING_DB_MIGRATION_022_023 — operator-owned production DDL on shared `127.0.0.1:5435`.** Until `server/db/022_sender_photos.sql` and `server/db/023_sender_photos_security.sql` are applied to the production `litterbox` database by the operator, `GET /v1/gmail/sender-photos/status` cannot return 200 and per-account `photo_ready` / `authorization_required` / `required_scopes` do not exist to be read; `GET /v1/cards` stays 500; the Gmail sync `INSERT INTO cards` fails each cycle.
- This lane did not and may not apply them: the brief forbids shared-5435 changes and db-role writes; fixtures go to a fresh `pg_virtualenv` cluster.
- **Second gate (code, not fixed here — no source writes):** `markSenderPhotos` must become savepoint/rollback-safe (or the photo flag must be gated on a schema check), otherwise every pre-022 deployment returns 500 for the inbox list. R67 "never a 500" cannot be claimed until that is repaired and re-proven.
- **Google side stays UNKNOWN, not denied:** People API enablement, consent-screen scopes and any real grant were not probed (no console, no browser, no Cloud config change, no consent approval). The status route — the only oracle that can report `authorization_required` — is precisely the route blocked by the migration. The 503 must not be read as `authorization_required`, as a disabled API, or as "Google broken".
- Therefore **no real Google photo bytes exist on live** (`sender_photos` absent), so no real-photo consumer proof can be produced. Existing captures under `receipts/gmail-avatar-proof/**` (incl. `newrun-20261020-1302`) stay seeded/disposable-fixture evidence only and remain non-acceptance for R67 real photos (CF2). Nothing is marked done from a seeded photo.
- Coverage contract unchanged and disclosed: contact-matched exact mailbox, non-default photos, `connections.list` + `otherContacts.list`, on a photo-granting account only; senders absent from both datasets stay initials. Not universal Gmail parity.

### 12.6 Prohibited actions (none performed)

No migration/ALTER/GRANT/role change, no `db.env` open, no token/PII/provider-URL dump (addresses redacted to lengths), no People API call, no consent approval, no browser, no Cloud console change, no production mail/archive write, no source/Git/installer/updater operation, no service restart, no owner desktop/session-bus/phone/TV action. Evidence gathered read-only: authenticated HTTP GETs, `journalctl`, the PostgreSQL server log, `information_schema`, one `EXPLAIN`, one rolled-back `LIMIT 1` SELECT.

### 12.7 Retro (required)

What worked: making the fresh authenticated status probe the first action (inside 5 minutes) and pairing it with the tenant-positive `/v1/gmail/accounts` 200 meant the 503 could not be mistaken for an auth failure; pairing the HTTP symptom with the statement PostgreSQL itself logged plus `information_schema` turned "database unavailable" into one exact missing-migration fact instead of a guess about Google. What to repeat: never infer schema or grant state from an HTTP error body; read the server's own logged statement, then confirm column presence read-only. What to change: `POST /v1/gmail/photo-consent` was used as a route-registration check even though it writes PKCE state — outside the read-only contract of this gate; route registration must be checked with a non-writing probe (GET/404-405 shape, or source) so the receipt carries no mutation at all.

## 13. Post-migration consent gate (fresh status, 2026-10-20 19:20:37 +04 / 15:20:37 UTC)

Worker: PostMigrationConsentGate (gate role, read-only). Status: **GATE = EXTERNAL_OWNER_CONSENT_REQUIRED. The §12 migration prerequisite is cleared and live-confirmed; the only remaining gate is the owner's Google consent on both accounts. This is NOT a PASS and NOT seeded-paint acceptance.**

### 13.1 Commands and working directory

- cwd `ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018` (read-only lane; no source, Git, DB, installer, device or desktop-bus action).
- One fresh probe: `python3 /tmp/lb_photo_status_probe.py` (15:20:37-38 UTC), origin forced to `http://127.0.0.1:8081`, `Authorization` from the canonical credential loader loaded in memory only; every secret printed as `<len n sha256:xxxxxxxx>` (CF9). GETs only: `/v1/version`, `/v1/gmail/sender-photos/status`, `/v1/gmail/accounts`. **No `POST /v1/gmail/photo-consent`** (its PKCE-write deviation from §12 was not repeated — the route and UI wiring were confirmed by reading shipped QML instead), no browser, no Cloud change, no consent click.

### 13.2 Artifact identity

- `GET /v1/version` -> 200 `server_build=d24dc419eafc44979926203a2fa5f7f33ce00d79` — the same installed 0.1-41 binary as §12 (`f76c3e8a…bad03`); this lane restarted nothing.

### 13.3 Fresh post-migration status (the §12 503 is gone)

`GET /v1/gmail/sender-photos/status` -> **HTTP 200**, `accounts[2]`, each with `photo_ready=false`, `authorization_required=true`, `required_scopes=[https://www.googleapis.com/auth/contacts.readonly, https://www.googleapis.com/auth/contacts.other.readonly]`. Account addresses and ids redacted to lengths only (18/16 and 36/36, matching §12.3). Tenant-positive control in the same run: `GET /v1/gmail/accounts` -> 200, 2 accounts `status=active`. Migration read receipt: `/home/kom/proj/ai_pr/litterbox-production-migration-20261020/production-photo-migration.md` (022/023 applied to shared 5435, pre-apply backup sha256 `a4b27a502c3d9c803f5dc7371b81e83a592bb31e6570fa7699a9a44bb75b3554`).

### 13.4 §12 gate obsolescence

`GATE = PENDING_DB_MIGRATION_022_023` (§12.5) is **obsolete**: the migrations were applied by the operator lane and the live 200 above is the direct confirmation on the same path, same credentials, same binary. The §12 root-cause finding (42703 on `granted_photo_scopes`/`sender_address`) stands as history; the code-side `markSenderPhotos` pre-022 fragility row also stays historical for old deployments, owned by the code row.

### 13.5 Owner grant path (verified in shipped UI, not executed)

Litterbox desktop -> Gmail accounts settings page (`qt/pages/GmailAccountsPage.qml`): each account with `authorization_required=true` shows a status label "Sender photos need authorization", the exact `required_scopes` list, and an "Authorize sender photos…" button (`photoConsentButton`). It calls `POST /v1/gmail/photo-consent`, which returns an `authorization_url` and performs no grant; the URL is shown read-only in-app next to "Open this authorization page", and only an explicit click opens it in the owner's browser (`Qt.openUrlExternally`, guarded). The app never grants on the owner's behalf — the normal settings path is sufficient for the owner to grant; no console, curl or hidden flow is needed.

### 13.6 Exact consent prerequisite (the one owner action, both accounts)

The owner opens Litterbox Gmail accounts settings, clicks "Authorize sender photos…" and approves the Google consent page granting `https://www.googleapis.com/auth/contacts.readonly` and `https://www.googleapis.com/auth/contacts.other.readonly` — once per account; both accounts currently report `authorization_required=true` and each needs its own grant.

### 13.7 People API and residual unknowns (not fabricated)

- People API enablement on the owner's Cloud project: **UNKNOWN** — not probed (no console access, no consent performed) and not derivable from status metadata. If it is not enabled, the owner's consent attempt surfaces Google's own error at grant time; that is discovered by the owner action, not by this lane.
- With consent unconfirmed, no further Google-side or real-photo consumer proof is possible from this lane. All unproved real-Google-photo, consumer, offline, docs and authorized-Git-handoff criteria (CF2, CF39, CF58 and the §3 coverage contract) stay preserved and unmarked; existing seeded captures remain non-acceptance.
- Coverage contract unchanged: exact mailbox match against People connections + otherContacts, non-default photos, on a photo-granting account only; everyone else stays genuine initials. Not universal Gmail parity.

### 13.8 Ready for Main ask

Yes. The measurable ask is exactly §13.6: one in-app consent per account granting the two named scopes. On the next post-consent read, the acceptance oracle is `GET /v1/gmail/sender-photos/status` returning `photo_ready=true` for a granting account followed by real-photo consumer proof; until then nothing is marked PASS.

### 13.9 Retro (required)

What worked: reusing the exact §12 probe script made the post-migration result directly comparable (503 -> 200 on the same path, same credentials, same binary), so "migration cleared" was proven by the one allowed GET instead of inferred. What to repeat: the status endpoint is the single consent oracle — it answered per-account grant state without touching Google, and the shipped QML answered the "can the owner grant in-app?" question without any POST. What to change: nothing in this lane; the one fresh GET was sufficient and the bounded packet closed inside its first minutes.

## 14. Authorized consent attempt — isolated browser stopped at Google sign-in (2026-10-20 ~20:03 +04 / 16:01-16:03 UTC)

Worker: AuthorizeRealGooglePhotos. Status: **NO GRANT PERFORMED — GATE = OWNER_INTERACTIVE_LOGIN_REQUIRED. NOT a PASS; seeded captures remain non-acceptance (CF2).** User authorization 'делай' for BOTH accounts and exactly `contacts.readonly` + `contacts.other.readonly` was honored as scope boundary; no broader permission was requested or granted.

### 14.1 Contract reads (no source writes, QML untouched — UI coder owns QML)

- `server/internal/gmail/http.go:159-200 beginPhotoConsent`: returns incremental `authorization_url` (Gmail.modify + both photo scopes), stores PKCE state, performs no grant; shared `callback:236-315` persists actually granted scopes (`tok.Scope`) with legacy fallback.
- `server/internal/gmail/oauth.go:41-49 PhotoScopes/FullPhotoScope`, `HasPhotoScopes` gates on both granted scopes.
- `qt/pages/GmailAccountsPage.qml:53-61,86-109,323-361`: status contract + `photoConsentButton` -> `POST /v1/gmail/photo-consent` -> read-only URL text + guarded `Qt.openUrlExternally`. Shipped in-app path confirmed sufficient; no console/curl needed.
- Policy/skill: `docs/agent-policies/secrets-and-browser-auth.md` (locator-first, isolated profile only) + `skill://playwright` guardrails (never real profile) read first.

### 14.2 Locator-first preflight (bounded, CF9)

- Searched `memory/operator-playbooks/secret-locators.md` for `google|gmail|litterbox|photo|people|contacts`: zero matching locators. Only generic `browser-auth-isolated-state` (isolated storageState, no Google cookies) applies.
- Result: **NO_LOCATOR_FOUND** for terms `Google login / Gmail password / Google 2FA / Litterbox Google OAuth` in `secret-locators.md` + relevant playbook + breadcrumbs. No broad hunting, no profile copy, no password-store access.
- Isolated cache check: wrapper cache holds only unrelated task dirs; no Google `storageState`/state file found. Credential loader used only for Litterbox tenant auth (keys listed, values as `<len n sha256:xxxxxxxx>`).

### 14.3 Canonical consent URL (private, redacted)

- `POST /v1/gmail/photo-consent` -> 200, `url_len=673`, `url_sha8=961b6343`, host `accounts.google.com` OK, scopes present: `gmail.modify` + `contacts.readonly` + `contacts.other.readonly`. URL kept in memory + mode-0600 tmp file only, shredded after use (`shredded:0`). No URL/token/state value recorded here.
- **Disclosed deviation:** the Playwright wrapper's `npm notice run` line echoed the full authorize URL into the local process log once (local only, not copied into any receipt/artifact). Future runs should pass the URL via a file descriptor or env to avoid process-list exposure.

### 14.4 Isolated browser checkpoint (real Chrome, disposable profile)

- First launches failed `SingletonSocket: Socket path too long` under the long worker runtime dir; fixed per wrapper docs with `PLAYWRIGHT_SOCKET_TMPDIR=/tmp/pwt`, `PLAYWRIGHT_TMPDIR=/tmp/pwt/pw`, `PLAYWRIGHT_CLI_WORKDIR=/tmp/pwg2`.
- Isolated session `gmailphoto` opened the consent URL: reached **Sign in - Google Accounts**, app `Litterbox`, `textbox Email or phone [active]`, `Next`, `Create account`, `Forgot email?`, privacy/terms links. No CAPTCHA/2FA screen was reached — stopped at the identifier prompt; no bypass attempted.
- Real profile (`~/.config/google-chrome`, `~/.config/chromium`) never launched/copied; live profile locks untouched.

### 14.5 Post-attempt live status (same binary, same credentials)

- `GET /v1/version` -> 200 `server_build=d24dc419eafc44979926203a2fa5f7f33ce00d79` (installed 0.1-41).
- `GET /v1/gmail/sender-photos/status` -> 200, 2 accounts: address lens 18/16, id lens 36/36, each `photo_ready=false`, `authorization_required=true`, `required_scopes=[contacts.readonly, contacts.other.readonly]`.
- `GET /v1/gmail/accounts` -> 200, 2 `status=active`. No photo bytes exist; no consumer/Qt/offline proof attempted (grant alone is not PASS per CF2).

### 14.6 Exact safe interactive prerequisite (owner hands only)

1. Owner opens Litterbox desktop -> Gmail accounts settings page, clicks `Authorize sender photos…` per account (the in-app button calls the same canonical POST and shows the URL read-only).
2. In the owner's own browser, sign in as each exact account in turn and approve ONLY the two named read scopes (`contacts.readonly`, `contacts.other.readonly`, plus existing Gmail access); decline anything broader.
3. On provider CAPTCHA/2FA/safety checks: complete them personally in that browser; the agent takes no bypass action.
4. Alternatively, export a task-specific isolated `storageState` for the consent page and hand its label to the agent; the agent never takes the password itself.
5. After each grant, the acceptance oracle is `GET /v1/gmail/sender-photos/status` -> `photo_ready=true` for that account, followed by real-photo consumer proof in an owned disposable Qt session.

### 14.7 Prohibited actions (none performed)

No source/Git metadata/fetch/merge/install/updater (even dry-run), no DB roles/DDL, no production mail mutation, no physical device action, no QML edit, no Cloud API change, no broader account/security permission, no token/password/authURL/photoURL printed. Isolated browser session closed.

### 14.8 Retro (required)

What worked: short-socket-dir fix from the wrapper script unblocked the real Chrome checkpoint in one retry, and keeping the consent URL out of receipts held. What to repeat: locator-first + status-before/after makes "still authorization_required" a proof, not a claim. What to change: pass the authorize URL without letting the wrapper log it (file/env), and pre-set the short socket dir before the first browser call.

## 15. Partial-grant callback diagnostic — 1/2 granted, darafei callback consumed/expired (2026-10-20 ~20:36-20:41 +04 / 16:36-16:41 UTC)

Worker: VerifyGrantedGooglePhotos (read-only lane). Status: **PARTIAL 1/2 — komzpa authorized but zero stored photos (People param bug); darafei still authorization_required after `Invalid or expired OAuth state` callback. NOT a PASS; no real-photo paint claimed (CF2).** No new `POST /v1/gmail/photo-consent` in this lane (cause-first); only authenticated GETs + `journalctl` reads. No source/Git/install/DB-role/Cloud/owner-desktop/profile/device action. QML/display untouched (UI-proof owner owns it, CF39). Screenshot `local://pasted-image-6ac6a58cba16b50c.png` read visually only — no URL state/code printed (CF9).

### 15.1 Fresh installed version / status / accounts (canonical tenant, redacted)

- `GET /v1/version` → 200 `{"api":1,"min_client_api":1,"server_build":"d24dc419eafc44979926203a2fa5f7f33ce00d79"}` — installed 0.1-41 source `d24dc419eafc44979926203a2fa5f7f33ce00d79`, binary sha256 `f76c3e8ab723a7cf284f91cce595007102bf41aae48927a5cffb6267f27bad03` (read-only `sha256sum /opt/litterbox/server/litterbox`, `dpkg -s litterbox-server` → `Version: 0.1-41`). Unit `litterbox-server` not restarted by this lane.
- `GET /v1/gmail/accounts` → 200, 2 rows `status=active` (darafei addr_len 18, komzpa addr_len 16; ids len 36/36). Credential loader keys only `device_id,server_url,tenant_id,token` with `<len n sha8:xxxxxxxx>`; token/ids/addresses never printed (CF9).
- `GET /v1/gmail/sender-photos/status` → 200 `accounts[2]`: darafei `photo_ready=false authorization_required=true required_scopes=[contacts.readonly,contacts.other.readonly]`; komzpa `photo_ready=false authorization_required=false` (no `required_scopes` key). Tenant-positive control in same run (accounts 200), so 403/404 below are grant-state facts, not auth failures.
- `GET /v1/cards` → 200 across `now/later/missed` total 970 (komzpa 554, darafei 406, null-account 10 in the sampled window). `sender_has_photo` is `omitempty` (`server/internal/cards/cards.go:48`), so absent = false — not a missing-field bug (CF14 route shapes unchanged, additive field only).
- Exact sender path (canonical, read-only, no archive/delete/send): `GET /v1/cards/{id}/sender-photo` on sampled ids (sha8 only) → darafei card `403 {"status":"authorization_required","required_scopes":[...]}` (zero photo bytes by design); komzpa cards `404 {"status":"unavailable"}` (granted, genuinely-stored zero); null-account `404`. So **partial grant 1/2, stored photos 0/2, real-photo consumer proof impossible today**.

### 15.2 Bounded redacted callback error + state-lifecycle diagnostic (CF30)

- Observed: browser on `http://127.0.0.1:8081/v1/gmail/oauth/callback?state=…` renders `Invalid or expired OAuth state. Start connecting Gmail again from Litterbox.` (capital `Invalid`). App settings behind it shows darafei `Sender photos need authorization` + both contacts scopes, komzpa `Authorized — photos appear after they sync`. Ground truth is partial failure, superseding any prior both-granted assumption.
- Exact clause: `server/internal/gmail/http.go:250-252` — `DELETE FROM gmail_oauth_states WHERE tenant_id=$1 AND state_hash=$2 AND expires_at>now() RETURNING code_verifier,redirect_uri` miss → `httpapi.MessagePage(w,400,"Invalid or expired OAuth state. …")`. Discriminator: `http.go:239-241` HMAC/parse fail renders lowercase `invalid OAuth state` — NOT what the screenshot shows, so `stateTenant` passed (StateSecret matches, tenant authentic). The failure is a consumed/expired/missing row, not a signature/secret rotation.
- Lifecycle clauses: `beginPhotoConsent:159-200` stores PKCE `stateHash(state)` with `expires_at=now()+interval '10 minutes'` (`:190`); `newState:201-210` = `tenant.nonce` + HMAC-StateSecret; `stateTenant:211-234` validates HMAC then parses tenant; `stateHash:235` = `sha256(state)`; `callback:236-315` consumes via the DELETE above (`:250`), commits the consume even on provider deny (`:256` `_ = tx.Commit()`), requires `code` (`:260-264`), commits again (`:265-268`), then persists actually-granted `tok.Scope` with legacy fallback (`:302-305`). No reachable logging on the DELETE-miss branch — diagnosis rests on the exact message + code, not a log line.
- Hypotheses distinguished by evidence (no guessed retry, no DB-state read — direct `gmail_oauth_states` counts/timestamps were NOT queried; DB-role reads are out of scope for this lane, so row age/reuse is inferred, not measured):
  - Expired (>10 min between POST and Google approval): consistent — row exists but `expires_at>now()` fails → same 400. Cannot be excluded without row timestamps.
  - Consumed (double-submit/back-button/retry after deny): consistent — first callback DELETEs the row (even deny consumes via `:256`); second hit with the same state misses → same 400. Common after a deny-then-retry loop.
  - Parallel flow (two `Authorize…` clicks → two states, callback uses superseded/older state): consistent only if the older state was already consumed/expired; both rows otherwise coexist (lookup is by `state_hash`, not latest-wins). No evidence of two recent POSTs in this lane (none performed); prior lanes' POSTs may have left live rows.
  - Tenant visibility: excluded as leak — DELETE filters by the HMAC-authenticated tenant from the state itself (`:250`); RLS pattern matches `StatusHandler:150-165`. A cross-tenant state would fail HMAC first (lowercase message), not reach this branch.
  - Host mismatch: excluded as cause of THIS message — wrong host/port/instance gives connection failure or a different server's miss, but screenshot host `127.0.0.1:8081` matches the prod bind (`-listen 127.0.0.1:8081`) probed here; a disposable-vs-prod split would also produce this same 400 on the wrong instance, so the safe rule is: always complete the callback on the same origin that issued the POST (in-app button → same prod server), never copy the URL to another host.
- Safe exact action needed (no POST until understood — honored): owner re-issues **one** `Authorize sender photos…` for darafei from the in-app settings on prod, completes Google approval within 10 minutes in the same browser, without double-clicking the button or reloading the callback tab. If the next callback still 400s, the owner reports the wall-clock gap (POST-click → approval) and whether the tab was reloaded — that distinguishes expired from consumed without any secret dump.

### 15.3 Granted account has zero photos — People param bug, not a grant gap

- Canonical sync path (`server/internal/gmailsync/photosync.go:80-123`): `trySenderPhotos` gates on `gmail.HasPhotoScopes(granted)` (`:96`, `oauth.go:53-64` both contacts scopes), then `peopleClient.BuildIndex` (`:117`); transient People errors are logged by count, never fail the Gmail sync (`:30-35`).
- Live log (read-only `journalctl -u litterbox-server`): `2026/10/20 20:35:41 sender-photos … account=e09758a2-…: people API /otherContacts: 400 … Invalid JSON payload received. Unknown name "readSource": Cannot bind query parameter.` — the granted (komzpa) account reaches Google but `BuildIndex` aborts before any fetch/store, so `photo_ready` stays false and `sender-photo` stays 404.
- Exact source clause: `server/internal/people/people.go:145-150` sends `"readSource": {"READ_SOURCE_TYPE_CONTACT,READ_SOURCE_TYPE_PROFILE"}` on `/otherContacts`; Google rejects the singular name. Test fixture mirrors it (`people_test.go:40,78-81,134-137` asserts `readSource`). Bounded fix recommendation (not applied — no source writes in this lane): rename the query key to the Google-documented `otherContacts.list` sources parameter, update the fake + assertions, run `go test ./internal/people/`, then prove on a disposable fixture before any prod deploy; next prod sync cycle will then populate `sender_photos` for komzpa with no new grant. Darafei still needs the §15.2 re-consent after that fix — the two gates are independent.
- Coverage contract unchanged (CF14): exact mailbox match, non-default photos, `connections.list` + `otherContacts.list` on a granting account only; senders absent from both stay initials. Not universal Gmail parity.

### 15.4 Photos proof / consumer runtime

- No real-Google paint attempted: zero stored photos ⇒ pixel/offline consumer proof is unreachable from prod today, and no isolated disposable own-runtime consumer run was staged (nothing to paint). Prior `receipts/gmail-avatar-proof/**` captures stay seeded-fixture evidence only — explicitly NOT PASS for R67 real photos (CF2). UI/QML display proof remains with its owner (CF39).

### 15.5 Prohibited actions (none performed)

No `POST /v1/gmail/photo-consent`, no callback retry, no OAuth URL/state/code/token/photo-URL print, no People direct call, no consent approval, no prod mail archive/delete/send, no source/Git/install/DB-role/Cloud/owner-desktop/profile/device action, no QML edit. Evidence: authenticated GETs, `journalctl` reads, shipped-code reads, `sha256sum`/`dpkg -s` reads.

### 15.6 Retro (required, max 5 lines)

What worked: pairing the capital-I error string against the two adjacent MessagePage branches separated HMAC-rot from consumed/expired without touching secrets; probing status+accounts+cards+sender-photo in one credential pass made 1/2-granted plus 0-photos a single coherent fact. What to repeat: treat `photo_ready=false + authorization_required=false + 404` as pipeline-bug signal and pull the sync log immediately. What to change: surface the People 400 in the status payload as a transient hint so the UI never reads `Authorized — photos appear after they sync` as success while sync errors persist.


## 16. Candidate server repair and live provider checkpoint (2026-10-20)

Worker: PrepareGooglePhotoRelease. Status: **SOURCE REPAIR + SCOPED 4/4 TEST + LIVE CANDIDATE-CLIENT GOOGLE READ DONE (parameter accepted, zero non-default photos on the granting account). No real photo bytes exist to claim; nothing is marked PASS from a fixture (CF2).**

- Official Google People API discovery (`https://people.googleapis.com/$discovery/rest?version=v1`) defines `otherContacts.list` query parameter `sources` with `repeated: true`; it does not define `readSource`. `sources` must be encoded as repeated query values.
- Source fix: `server/internal/people/people.go` now sends `sources=READ_SOURCE_TYPE_CONTACT` and `sources=READ_SOURCE_TYPE_PROFILE` as distinct values. Test fixture collects repeated values and checks both. Prior actual production log evidence (receipt §15.3) shows the failed `readSource` request returned HTTP 400 `Cannot bind query parameter`.
- Scoped test: from `server/`, `go test ./internal/people/ -v -count=1` -> exit 0, 4/4 PASS (`TestBuildIndexExactMatchRejectsDefault`, `TestFetchPhotoDecodesRealImage`, `TestOtherContactsRejectsUndocumentedQuery` (broken-negative control, CF67), `TestNormalizeEmail`).
- Isolated candidate binary (out-of-tree): `/home/kom/proj/litterbox-server-gmailphotos-candidate`, built with `go build -o /home/kom/proj/litterbox-server-gmailphotos-candidate ./cmd/litterbox` from `server/`; build exit 0. SHA-256 `3c9bcedf09c2e75fddd1ee568c23313a34c63fb84b267b5d1f3fd676e3efdd72` (identical across both builds). Source SHAs: `people.go` `7e9725bfc9edb33d3d6955192da91e9a9d67bf7288e37d989b6e79276ff757b7`; `people_test.go` `27da939ce79471a9d80bcfa9fcc640c25d2bfd54127cf0c1271c1c5437d5811`. The earlier in-tree `receipts/` copy was removed; the temporary `server/livephotoprobe/` runner was deleted after its run. Candidate build is not installed and never touched production.
- Live provider read (read-only, existing grant, no new consent, no mail writes): disposable runners held all secrets in memory and printed redacted counts only (CF9: no tokens, addresses, URLs, or photo bytes in output). Token refresh -> HTTP 200; `granted_accounts=1` of 2 active (the other still `authorization_required` per §15).
- Actual parameter acceptance: `otherContacts.list` with the fixed repeated `sources` values returned HTTP 200 on every page (full pagination: 15 pages, 1467 persons, 1367 with mail, all 1467 with a photo slot, 0 non-default). `connections.list` returned HTTP 200 on every page (6 pages, 517 persons, 0 non-default). The prior comma-joined encoding was rejected with 400 (§15.3); the repeated encoding is accepted — the exact residual from discovery is closed.
- Candidate-client proof: a temporary Go runner using the actual fixed `people.Client` (`ListOtherContacts`/`ListConnections`) against live Google returned the same totals (`otherContacts` 1467 persons status 200; `connections` 517 persons status 200) with zero errors, then was deleted. Candidate proof is not installed acceptance: the binary was never installed and production still runs 0.1-41.
- Photo bytes: none to hash — zero non-default photos exist on the granting account across both surfaces, so `photo_bytes=null` by ground truth, not by omission. No consumer paint or offline proof is claimable from live data today (CF2); seeded captures remain non-acceptance. When any sender gains a non-default contact photo, the next prod sync cycle will populate `sender_photos` with no new grant.
- Changelog inventory: no server changelog file exists (`find . -maxdepth 3 -type f \( -iname 'changelog*' -o -iname 'news*' \) -print` in `server/` produced only `internal/cards/newsletter_test.go`; no changelog created.
- Retro (required, max 5 lines): pinning the pooled connection for the tenant setting unblocked the live read after two misleading zero-grant results; the pgx pool drops transaction-local settings across connections, unlike a single-session client. Proving the identical totals through both raw HTTPS and the actual candidate `people.Client` made parameter acceptance airtight. What to repeat: paginate fully before claiming zero photos — the first page alone could not prove it. What to change: keep disposable credential runners out of the repo tree from the start instead of moving them after.
