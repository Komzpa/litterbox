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
- `senderphoto.Upsert` (pgx `Execer`, transaction-friendly) → tenant/account-scoped `sender_photos` (022, RLS same shape as cards).
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
