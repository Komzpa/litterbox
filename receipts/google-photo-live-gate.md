# Google photo live gate — observed status (read-only, no grant)

Date: 2026-10-20 (Asia/Tbilisi, ~14:35–14:40). Worker: InspectGooglePhotoGrant.
ROOT=/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018.
No production mutation, no new grant, no contact mutation, no private photos exported.
No shared installer/upgrader (not even dry-run), no Git metadata write, no server restart/deploy,
no browser, no owner desktop/phone/TV/session. CF39 holds.

## Canonical prerequisites (docs, not probed console)

`docs/DEPLOYMENT.md` R67 (fresh-read, line 34): real sender avatars need
(1) Google Cloud project with People API enabled, AND (2) the incremental photo grant —
both `https://www.googleapis.com/auth/contacts.readonly` and
`https://www.googleapis.com/auth/contacts.other.readonly` **actually granted**
(never the requested string; `gmail.modify` alone carries no photo field).
Operator path on a build with photo routes: `POST /v1/gmail/photo-consent`
(authenticated, same tenant) returns `authorization_url`; the shared OAuth callback
persists granted scopes. `GET /v1/gmail/sender-photos/status` reports per-account
`photo_ready` / `authorization_required` + `required_scopes` without touching Google.
Coverage is contact photos only (saved contacts + other contacts, exact mailbox match,
non-default); senders absent from both stay uncovered even when Gmail renders a photo.
Server code matches: `server/internal/gmail/oauth.go` PhotoScopes/HasPhotoScopes,
`server/internal/people/people.go` connections.list + otherContacts.list only,
`server/internal/senderphoto/handler.go` StatusHandler, `server/cmd/litterbox/main.go:192`.

## Last publication (read-only citation)

`receipts/ui-corrections-publication.md`: PUBLISHED 2026-10-20 ~04:52 +04,
source `df9b13e5513ed1f1739149a3d6048c27eee3c725`, server 0.1-40 binary
`e8d911d0…` / deb `00d6285b…`, desktop `56d975e1…`, Android v27 `83ad492b…`,
installed/served on all three origins. CF26 identity carried from that receipt;
this lane did no re-hash of installed binaries.

## Live probes (redacting loader, loopback only)

Loader: `/home/kom/.local/share/tasks-loop/state/secrets/litterbox-desktop-credentials.json`
— keys only: `device_id, server_url, tenant_id, token` (all str, UUID/token lengths
verified; CF9: contents never printed, token used in-process only).
Origin forced to `http://127.0.0.1:8081` (documented loopback bind). Results:

| Path | Status | Meaning |
|---|---|---|
| `GET /v1/version` (auth header sent; endpoint itself unauthenticated) | 200, `server_build=df9b13e5513ed1f1739149a3d6048c27eee3c725` | Installed build confirmed = published 0.1-40 cohort |
| `GET /v1/gmail/sender-photos/status` | **404 `404 page not found`** | Route absent on installed build — **not** a People API disabled signal |
| `GET /v1/gmail/accounts` | 200, count 2, `status_counts={active: 2}` (ids len 36 each; address lengths only, values redacted) | Tenant scope verified + positive known accounts (2 active) |

Tenant scope: the 200 on the tenant-RLS `listAccounts` proves the loader token's
tenant is valid. Positive known accounts: 2 active rows observed (no zero-conclusion
without this check — satisfied). Addresses redacted to lengths; no PII recorded.

## Granted scopes / photo evidence: UNKNOWN on live

- `listAccounts` on this build returns only `id/address/status` — no scope fields.
  `granted_photo_scopes` (migration 022) is not exposable through any live route
  (status route missing). **Granted OAuth scopes on live: UNKNOWN** — not claimed as
  gmail-only, not claimed as photo-granted.
- Stored photo bytes: not probed (no card id, no photo route, no export). **UNKNOWN.**
- People API enablement on the Cloud project: console never inspected (out of scope).
  **UNKNOWN** — no "owner must enable it" homework is assigned from this lane.
- Direct Google People read-only request: **NOT performed**. It would require a Google
  access token derived from production refresh secrets, which this lane does not extract
  (CF9). No existing-grant access token was proven on live, so no safe in-grant People
  call existed. Hence no insufficient-scope / API-disabled / usable distinction is
  possible from Google's side today. The 404 must not be read as any of those three.

## Gate (exact)

**GATE = UNKNOWN_ROUTE_MISSING** (installed 0.1-40 `df9b13e` predates the photo routes;
missing route is not disabled People API, not denied scope, not zero-photo proof).

## User action (verified only)

**None verified — no user homework is assigned.** The consent URL can only come from
`POST /v1/gmail/photo-consent` on a server build that registers the photo routes, and
POST was deliberately not called (it writes PKCE state = production mutation).
When a build with the routes is live, the unavoidable action — only if
`sender-photos/status` then reports `authorization_required` — is: call
`POST /v1/gmail/photo-consent` (authenticated, same tenant) and approve the returned
`authorization_url` in a browser for Gmail.modify + both contacts scopes. That step is
conditional future work, not a current instruction. No assumption is made that the new
scopes grant universal Gmail-photo parity; coverage stays contact+other-contacts only.

## What was NOT done (explicit)

No `POST /v1/gmail/photo-consent` (would mutate state), no OAuth callback, no Gmail/People
mutation, no photo byte fetch/export, no DB write, no credential/token printing, no full
secret file read, no console inspection, no installer/upgrader, no Git operation, no
service restart/deploy, no browser launch, no owner desktop/phone/TV, no sibling file edits
(`CompleteSenderPhotoNotification` owns Qt source/proof; this lane wrote only this receipt).

## Retro (required)

- The first probe script died on a Python `urllib` shadowing bug (`import` inside the
  function signature scope); fixed by top-level `urllib.error` import. Lesson: keep
  redacting-probe scripts flat and re-runnable.
- Checking `/v1/version` first was correct for build identity but proves nothing about
  tenant — the tenant proof came only from the authenticated `listAccounts` 200.
  Lesson preserved: version ≠ auth; always pair with a tenant-RLS read before any
  zero/unknown conclusion.
- Not probing POST consent on live was the right call even though it leaves the user-action
  cell as "none": a grant URL from a mutating call would have violated the read-only
  contract. The gate stays UNKNOWN rather than manufacturing AUTHORIZATION_REQUIRED.

## Carry-forwards

- CF9: no secret fullreads — loader keys + string lengths only; token in-process; addresses
  as lengths; db.env never opened.
- CF26: observed identity — live `server_build=df9b13e…` (fresh 200); artifact hashes
  cited from `ui-corrections-publication.md`, not re-hashed here.
- CF39: no owner session — loopback API only; no desktop/phone/TV/browser/GUI.
