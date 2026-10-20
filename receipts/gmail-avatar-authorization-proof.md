# Receipt: Gmail sender avatars — photo consent/status UI (R67 authorization surface) — PARTIAL

Date: 2026-10-20. Worker: ExposeGooglePhotoAuthorization. Status: **consent/status UI edit landed and compiled; isolated GUI smoke staged but NOT executed (Main ordered settlement at 12:53 before the run). Real-Google grant gate unresolved.** No screenshot of the new UI exists — nothing below is claimed proven beyond build + entry/wiring evidence.

## 1. Source identity (CF26)

- Base source per `gmail-avatar-failed-acceptance.md` §1: base `929f275946f7906cfbe67e0f4dfd08f83402a202`, published product `df9b13e5513ed1f1739149a3d6048c27eee3c725`. No git metadata was read or written by this worker; no commit/push. git-pr-owner records SHAs.
- Foreign hunks preserved. `qt/qml/InboxView.qml` NOT touched (peers ConnectLocalArchiveSuccessors + PreservePinnedArchiveReader own the archive hunks; both acked in writing, zero overlap). Server source untouched. No installer/upgrader, no production credential/grant/mail writes, no Google call, no browser opened.

## 2. Changed paths (this worker only)

- `qt/pages/GmailAccountsPage.qml` — three additions in the existing account workflow (page reachable via nav "Accounts" / `openPage("GmailAccountsPage")`):
  1. Properties: `photoStatus`, `photoStatusError`, `photoConsentUrl`, `photoConsentError`, `requestingPhotoConsent` + contract comment.
  2. Functions: `reloadPhotoStatus()` (GET `/v1/gmail/sender-photos/status`) wired into `reload()`; `requestPhotoConsent()` (POST `/v1/gmail/photo-consent`) — stores `authorization_url` only, never opens a browser.
  3. "Sender photos" section below the account list: one row per server entry with exactly three server-derived states — `photo_ready` → "Sender photos ready"; not ready + not required → "Authorized — photos appear after they sync"; `authorization_required` → "Sender photos need authorization" + the server's `required_scopes` list + `MailActionButton` "Authorize sender photos…" (`photoConsentButton`). Below the rows: `photoConsentPrompt`, read-only `photoConsentUrlText` showing the returned URL, and `photoConsentOpenButton` "Open authorization page" (calls `Qt.openUrlExternally` only on the user's click, gated by `openLinks`). No scope/Google-config check is invented client-side: every state and scope string is the server's (`photo_ready` / `authorization_required` / `required_scopes`).
- `receipts/gmail-avatar-proof/**` (new) — durable copies of prior + staged proof artifacts (69 files): `captures/` (prior `offline-01-reopen.png`, `online-02-photo.png`), `logs/` (prior server/photo/status/403/404/people-test logs + this worker's 22 migration logs `lb-gphoto-mig-*.log`), `scripts/` (prior `online.json`, `offline.json`; this worker's `seed_photo-consent` fixtures: `seed_photo.sql`-derived `/tmp/lb-gphoto-seed.sql`, disposable `credentials.json`, `profile.json`).
- `receipts/gmail-avatar-authorization-proof.md` — this receipt.
- NOT run: `qt/i18n/screens_*.ts` regeneration (new `qsTr` strings fall back to source; `lupdate` is release-flow tooling).

## 3. Entry / wiring evidence (observed)

- Contracts used exactly as shipped: `GET /v1/gmail/sender-photos/status` → `{"accounts":[{account_id,address,photo_ready,authorization_required,required_scopes?}]}` (`server/internal/senderphoto/handler.go`); `POST /v1/gmail/photo-consent` → `{"authorization_url"}` with `FullPhotoScope()` = Gmail.modify + `contacts.readonly` + `contacts.other.readonly`, performs no grant (`server/internal/gmail/http.go` `beginPhotoConsent`).
- Build after all edits frozen: `cmake --build /tmp/lb-avatar-qt-build` → exit 0 (`rcc` recompiled resource `lb_pages` which embeds `pages/GmailAccountsPage.qml`, relinked `litterbox-qt`). The binary `/tmp/lb-avatar-qt-build/litterbox-qt` contains this UI. QML runtime behavior is NOT claimed (no run).

## 4. Disposable fixture stack staged (created, verified, not yet exercised against the UI)

- Fresh disposable Postgres 18 cluster `lb-gphoto-pg`: `initdb` → `pg_ctl start` was reaped after the shell call; relaunched as broker-owned named service `lb-gphoto-pg` (unix socket `/tmp/lb-gphoto-pg/.s.PGSQL.5447`) — **still running, left for the bounded successor**.
- DB `lb_gphoto_proof_20261020`: all 21 migration files (001–022) applied, exit 0 each (logs `receipts/gmail-avatar-proof/logs/lb-gphoto-mig-*.log`). Seed applied (log-level verified): tenant `11111111-…`, device token `disposable-gphoto-token-20261020` (sha256 `b1b63655…` seeded), three accounts covering the exact three UI states — `granted@example.test` (both photo scopes + stored photo), `default@example.test` (both scopes, no photo yet), `missing@example.test` (Gmail.modify only → `authorization_required` + `required_scopes`); three cards (`aaaaaaaa-…` photo card, `bbbbbbbb-…` default, `cccccccc-…` missing-grant) and one real 152-byte PNG row in `sender_photos`.
- Server binary built: `go build -o /tmp/lb-gphoto-server ./cmd/litterbox` → exit 0. **Not started** (settlement order). Start env for the successor: `DATABASE_URL` → keyword form `user=kom dbname=lb_gphoto_proof_20261020 host=/tmp/lb-gphoto-pg port=5447`, `LISTEN_ADDR=127.0.0.1:18083`, `LITTERBOX_DEV_TENANT_ID=11111111-1111-4111-8111-111111111111`, `GMAIL_OAUTH_CREDENTIALS=/tmp/lb-gphoto-run/credentials.json` (disposable fake client — required for the photo-consent route to be registered), `GMAIL_OAUTH_STATE_SECRET` ≥32 bytes, `GMAIL_OAUTH_CALLBACK_URL=http://127.0.0.1:18083/v1/gmail/oauth/callback`. Recommended: `HTTPS_PROXY=http://127.0.0.1:1 NO_PROXY=127.0.0.1,localhost` so the gmailsync worker's token refreshes (fake refresh tokens) cannot reach Google at all.
- Qt app fixtures: `/tmp/lb-gphoto-run/profile/profile.json` (`server_url http://127.0.0.1:18083`, token above), private `HOME/XDG_*`/runtime dirs staged.

## 5. Durable captures (CF65)

- `receipts/gmail-avatar-proof/{captures,scripts,logs}` — 69 files, copied from `/tmp/lb-avatar-proof` + `/tmp/lb-avatar-run/scripts` + this worker's staging. All receipt references are project-relative; nothing depends on `/tmp`. Prior consumer captures (`captures/offline-01-reopen.png`, `captures/online-02-photo.png`) are the only photo-paint artifacts that exist — they are the previous worker's (ProveGooglePhotoConsumer), not this worker's UI proof.

## 6. What remains for the bounded successor (all criteria preserved, none cut)

1. Start the disposable server (env above) and curl the two contracts into `receipts/gmail-avatar-proof/logs/` (status JSON + photo-consent `authorization_url`).
2. Isolated GUI smoke, private Xvfb + `dbus-run-session` + private `HOME/XDG` (`/tmp/lb-gphoto-run`), capture-script mode `litterbox-qt --capture-script /tmp/lb-gphoto-run/profile <script> <out>`:
   - Online script: `store.online=true` → wait → inbox shot with painted photo → click `navAccounts` → wait → `eval JSON.stringify(stack.currentItem.photoStatus)` → shot of the three rows → click `photoConsentButton` (index of the missing-grant row) → wait → `eval stack.currentItem.photoConsentUrl` → shot showing `photoConsentUrlText` + `photoConsentOpenButton`. **Do NOT click `photoConsentOpenButton`** (would open a browser; the agent must never launch the grant) — the visible, enabled button plus the displayed URL is the user-triggered action. Resize to 412×915 → shot (neighboring viewport).
   - Offline reopen script (same profile + cache, server stopped, script never sets online): shot of the inbox with the photo painted from device cache + `eval store.cachedSenderPhoto('aaaaaaaa-…')`.
3. Pixel-check both photo captures for the red/green half pattern (`220,30,30` with `30,160,60` 4–40px right); the paint appearing after the Image settles is the `Image.Ready` consumer proof.
4. One scoped test run on frozen edits: `tst_gmailaccounts.qml` (single test file). Full suite stays with the lead (CF58).

**Failing prerequisite (why this is partial):** no visual oracle — the smoke never ran, so there is no screenshot of the consent UI and no new paint artifact; "authorization-required visibly actionable", "granted/default/missing states accurate", and "photo bytes paint online after Image.Ready then offline reopen" are wired but **unproven by artifact**.

**One discriminator:** SUCCESS = (a) an online 1440×1000 capture of GmailAccountsPage shows rows "Sender photos ready" / "Authorized — photos appear after they sync" / "Sender photos need authorization" with the server's `required_scopes` and the "Authorize sender photos…" button; (b) after clicking it, `stack.currentItem.photoConsentUrl` is an `https://accounts.google.com/…` URL containing both `contacts.readonly` and `contacts.other.readonly`, with the Open button visible and unclicked; (c) the online and offline inbox captures each contain `220,30,30` pixels with `30,160,60` to their right. FAILURE = `photoConsentUrl` empty, or `photoStatusError`/`photoConsentError` label visible in the shot, or zero red pixels in the offline capture.

## 7. Unresolved live-consent gate (unchanged, NOT declared resolved)

1. Owner console action: enable People API on the existing Google project; add both photo scopes to the OAuth consent screen (verification may apply).
2. User action: open the `authorization_url` from `POST /v1/gmail/photo-consent` in their own browser and approve; the shared OAuth callback persists granted scopes.
3. Real Google enablement/grant remains **unknown** — not falsifiable from this box (no lane credentials present); the API is NOT declared disabled. No grant was launched, no browser opened, no Google endpoint contacted by this worker (the disposable server was never started).

## 8. Live handles left behind

- Named service `lb-gphoto-pg` (Postgres 18.6, socket `/tmp/lb-gphoto-pg/.s.PGSQL.5447`) — running, disposable, safe to drop after the smoke.
- Binaries: `/tmp/lb-gphoto-server` (built, not running), `/tmp/lb-avatar-qt-build/litterbox-qt` (contains the UI).
- Fixtures: `/tmp/lb-gphoto-run/**`, seed `/tmp/lb-gphoto-seed.sql` (durable copy at `receipts/gmail-avatar-proof/scripts/`).

## 9. Carry-forwards

- CF2: seeded-image-only claims rejected; the paint criteria above need the real bytes pattern check in the successor's captures (prior artifacts alone are the previous worker's evidence).
- CF26: identities per failed-acceptance §1; executables from this tree as named in §8.
- CF39: isolated GUI proof for the consent/status UI **outstanding** — oracle is §6/§6-discriminator; prior avatar paint proof stands at `receipts/gmail-avatar-proof/captures/`.
- CF58: no suite run; no tests run at all here (per "no tests mid-edit").
- CF65: durable copies complete (§5).

## 10. Retro (required)

What worked: staging the entire disposable fixture stack (fresh cluster → 22 migrations → seed with one account per UI state → fake OAuth file) before touching the run meant the successor can go straight to the visual proof with zero rediscovery; the ownership ping to InboxView-owning siblings before editing got a written zero-overlap ack in one round-trip, so the settings-page edit stayed collision-free. What to repeat: fixture-first batching and sibling acks before shared-surface edits. What to change: the proof window was spent on plumbing after the 15-minute first-edit gate — the disposable server start and first capture run should have followed the build immediately, because a compiled UI without a screenshot is not evidence; and progress should have been checkpointed to Main at the 15-minute mark instead of at the 41-minute intervention.

## 11. Successor addendum — staged smoke EXECUTED 2026-10-20 (RunStagedGooglePhotoSmoke, newrun-20261020-1302)

Status changes: consent/status UI smoke **executed and proven**; prior §6 "NOT executed" block is superseded by this section. Full evidence: `receipts/gmail-avatar-proof/newrun-20261020-1302/PROOF.md` (28 files: 9 logs, 8 PNGs, scripts, status/photo-consent JSON).

- Preflight: disposable PG service was down (restarted same data dir as own service); staged port 18083 foreign-occupied (pid 3067928, preserved) → server ran on 127.0.0.1:18085 with all other §4 env exact; dead proxy kept (zero Google contact). Staged seed PNG (152 B) is malformed (length-less IEND; undecodable) → repaired the disposable DB row only with a valid 64x64 red/green PNG (162 B). No source/migration/durable-seed edits.
- Proven: (a) accounts page shows all three server-derived rows + required_scopes + Authorize button (`online-03-accounts.png` + `photoStatus` eval); (b) clicking Authorize returns `https://accounts.google.com/…` URL containing gmail.modify + both contacts scopes, Open button visible and never clicked (`online-04-consent.png` + `photoConsentUrl` eval); (c) photo paints online after one `refresh()` post-prefetch (512 exact red-with-green-right pixels, `online-07-photo-refresh.png`) and offline from device cache with server stopped (512 pixels, `offline-01-reopen.png`, `Can't reach the server` banner); default/missing remain "?" fallback. Scoped `tst_gmailaccounts.qml`: 6 passed, 0 failed.
- Live-Google consent gate (§7) unchanged and NOT resolved: no grant launched, no browser opened, API not declared disabled.
- Own services stopped after proof (server, Xvfb :195, disposable PG); foreign processes/ports and prior 69 files preserved.
