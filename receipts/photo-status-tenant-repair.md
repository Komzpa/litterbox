# Photo status tenant-scoped visibility repair (R67)

Date: 2026-10-20. Worker: RepairPhotoStatusTenant. Scope: bounded source fix in
the assigned senderphoto owner, one scoped production-HTTP proof, normative R67
update. No Qt, no Git mutation, no deploy, no installer/production-DB work.

## 1. Cause (first checkpoint, confirmed on source)

`GET /v1/gmail/sender-photos/status` returned `200 {"accounts":[]}` while card
GETs and `GET /v1/cards/{id}/sender-photo` photo bytes served fine. Cause:
`StatusHandler.ServeHTTP` queried `accounts` through `h.DB.QueryContext` with no
request tenant transaction, so `litterbox.tenant_id` was unset and the 023
`tenant_isolation` RLS policy (`tenant_id = NULLIF(current_setting('litterbox.tenant_id', true), '')::uuid`)
hid every row. The canonical neighbor — `Handler.ServeHTTP` (photo bytes route,
same package) — and `gmail.WebHandler.dbTx` both begin a transaction and run
`SELECT set_config('litterbox.tenant_id',$1,true)` before touching tenant rows.
StatusHandler was the only photo path missing that convention.

## 2. Exact changed paths (with post-fix sha256)

| Path | sha256 |
| --- | --- |
| `server/internal/senderphoto/handler.go` | `8ee4deab7efa047aeafc7048a50389141ea172fecd7dcb66001ea330885487c9` |
| `server/internal/senderphoto/status_test.go` (new) | `d64ace80399d5dbf0d01e2a411d56c74011ea6099b9d62d9a0d47d135d72ae8b` |
| `REQUIREMENTS.md` | `0c7115e06b553eea1f6391df05d2f1594d6f1afe3008f65eafc9a7c1f09b7144` |

`git diff --stat`: `REQUIREMENTS.md +14/-2`, `handler.go +19/-1`
(`status_test.go` untracked new). Nothing else touched.

## 3. The fix (same convention, no fallback / no RLS disable / no validation extras)

`server/internal/senderphoto/handler.go` `StatusHandler.ServeHTTP` now mirrors
the photo route's request tenant transaction verbatim: `BeginTx` →
`SELECT set_config('litterbox.tenant_id',$1,true)` → `tx.QueryContext` →
`tx.Commit()`. Query text, response shape (`{"accounts":[…]}` with
`account_id`/`address`/`photo_ready`/`authorization_required`/`required_scopes`),
status codes (401 unauthenticated, 503 database unavailable) and the
`status IN ('active','reauth_required')` filter are unchanged. No RLS policy,
grant, migration, or validation was added or weakened.

## 4. Acceptance clause → production entry → wiring → behavior proof

**Consumer route (end to end):** `qt/pages/GmailAccountsPage.qml`
`reloadPhotoStatus()` → `api.get("/v1/gmail/sender-photos/status", …)` (rendered
rows: `photo_ready` / `authorization_required` + `required_scopes` + the
"Authorize sender photos…" action) → authenticated `Api` transport
(`qt/src/main.cpp`) → server `auth.Handler.Middleware` (device bearer) →
`cmd/litterbox/main.go:192` route → `senderphoto.StatusHandler.ServeHTTP`.
The repair sits at the server hop: the status list now answers with the caller
tenant's accounts instead of an RLS-emptied `[]`, which is what made the page
show no account rows despite healthy accounts.

| Clause | Production entry | Wiring | Behavior proof |
| --- | --- | --- | --- |
| StatusHandler uses the canonical request tenant transaction | `GET /v1/gmail/sender-photos/status` → `cmd/litterbox/main.go:192` `mux.Handle(..., devOnly(senderphoto.StatusHandler{DB: db}))` behind `auth.Handler.Middleware` | `StatusHandler.ServeHTTP` `BeginTx` + `set_config('litterbox.tenant_id',$1,true)` (same shape as `Handler.ServeHTTP` lines 35–44 and `gmail.WebHandler.dbTx`) | Negative control (pre-fix handler, same route/test): `own tenant accounts=[], want exactly 2`, exit 1 — exact reported baseline. Post-fix scoped run: PASS, exit 0 |
| Own tenant's account status visible in every grant state | same route | accounts query inside the tenant transaction; `gmail.HasPhotoScopes` derivation unchanged | App-role HTTP: tenant A device token → 200 with exactly 2 entries — granted+stored `photo_ready=true, authorization_required=false`; Gmail.modify-only `photo_ready=false, authorization_required=true` with `required_scopes=[contacts.readonly, contacts.other.readonly]` |
| Neighbor tenant invisible (cross not leaked) | same route | same tenant transaction scopes RLS to the caller's tenant | Own-tenant body contains no neighbor account id/address (`strings.Contains` leak check); neighbor's own device still gets its one `photo_ready` row (visibility scoped, not emptied) |
| Unset credential not leaked — 401 per canonical HTTP auth | same route | `auth.Handler.Middleware` bearer device-token gate (no dev bypass in test) | `Authorization` unset and bogus token → 401 before the handler runs |
| RLS hides rows when context unset | `litterbox_app` role at runtime | 023 `tenant_isolation` policy active; test queries as granted runtime role (`SET ROLE`) | In-test control: `SELECT count(*) FROM accounts` with unset `litterbox.tenant_id` → 0 rows |
| Normative R67 states the measurable tenant behavior | `REQUIREMENTS.md` R67 | status contract sentence now requires own-account visibility in every grant states, neighbor invisibility, 401 on unauthenticated; oracle names `TestStatusRouteTenantVisibility` | `REQUIREMENTS.md:451-462` and `:488-494` |

## 5. Scoped proof (one run after source edits frozen)

- Command (working dir `ROOT/server`):
  `env -u DATABASE_URL LITTERBOX_DEV_TENANT_ID= pg_virtualenv bash -c 'createdb -h $PGHOST -p $PGPORT -U $PGUSER litterbox_test_photostatus_20261020; DATABASE_URL=postgres://$PGHOST:$PGPORT/litterbox_test_photostatus_20261020?user=$PGUSER&password=$PGPASSWORD&sslmode=disable; CARD_TEST_POSTGRES=1; go test -count=1 -v -run "^TestStatusRouteTenantVisibility$" ./internal/senderphoto/'`
- Result: `--- PASS: TestStatusRouteTenantVisibility (0.05s)`, `ok … internal/senderphoto 0.053s`, **exit 0**.
  Log: `/tmp/photo-status-tenant-proof.log`. Disposable `18/regress` cluster on
  port 5434 created/dropped by its own trap (Android-3's 5435/18131 never touched).
- Negative control (same command, handler temporarily reverted to the pre-fix
  direct query): `--- FAIL … status_test.go:181: own tenant accounts=[], want exactly 2`,
  **exit 1** — reproducing the shipped failure (`200` + `accounts: []`) on the
  real route. Log: `/tmp/photo-status-tenant-negative.log`. Handler then restored
  to the fixed state (hash above).
- Earlier attempt with a mis-numbered seed parameter in the new test failed on
  `accounts_tenant_id_fkey` (test-harness bug, exit 1, log
  `/tmp/photo-status-tenant-proof.log` first run); seed fixed before the proof.
- Test mechanics (existing testdb patterns only): `testdb.Setup` per-test schema
  + role, canonical lexical `../../db/*.sql` set via `tdb.Migration`, seeds as
  schema owner then `SET ROLE` so the granted runtime role runs with 023 RLS
  active; production HTTP path is `httptest.NewServer(auth.NewHandler(auth.NewPostgresStore(conn), "").Middleware(mux))`
  with the route registered exactly as `cmd/litterbox/main.go` registers it —
  real device tokens (sha256 `token_hash` rows), no dev tenant bypass, no mock
  handler, no direct helper invocation as proof.

## 6. Inventory of same photo status/consent paths (missing-context audit)

Within the assigned senderphoto owner:
- `Handler.ServeHTTP` (`GET /v1/cards/{id}/sender-photo`): already sets the
  request tenant transaction — unchanged, healthy.
- `StatusHandler.ServeHTTP` (`GET /v1/gmail/sender-photos/status`): the defect —
  fixed above.

Cross-owner (reported, deliberately not edited — not scope creep):
- `POST /v1/gmail/photo-consent` (`gmail/http.go` `beginPhotoConsent`): uses
  `h.dbTx` which runs `set_config('litterbox.tenant_id',$1,true)` — has the
  convention, no defect found.
- `gmailsync.trySenderPhotos` (photo upsert): `s.beginTenant` sets the same
  config (`sync.go:545-547`) — has the convention, no defect found.
- `cards` list `markSenderPhotos`: runs inside the cards request tenant
  transaction (`cards/handler.go:85`) — has the convention, no defect found.
No identical missing-context path remains in the photo status/consent surface;
no cross-owner handoff required.

## 7. Retro (required)

1. **Why it shipped:** R67 shipped two routes from one feature branch; the
   photo-byte route copied the tenant-transaction boilerplate from the cards
   handlers, but the later status route was written as a plain read and no test
   ever queried it through RLS as the runtime role. Unit-level seeding (schema
   owner, RLS bypassed) masked the difference: the same query returns rows for
   the owner and zero rows for `litterbox_app` with the context unset.
2. **What would have caught it earlier:** any test that exercises a
   tenant-scoped handler as the granted app role — the empty-result signature
   is distinct from a 403/500 and easy to assert. The new test's in-test
   control (`unset context → 0 rows visible`) makes that failure mode
   impossible to miss again.
3. **Convention, not copy-paste:** every tenant-scoped request must open its
   work in a transaction with `set_config('litterbox.tenant_id',$1,true)`
   before the first tenant-row query — the repo now has this spelled out in R67
   text so the next handler is checked against the normative contract.
4. **Cheap fix, expensive discovery:** the repair is ~19 lines; the discovery
   came from a fixture observing `accounts: []` on a 200. Treat an empty list
   on a200 from a tenant-scoped route as a defect until proven otherwise.

## 8. Known out-of-scope risks — reported as NOT done

- **NOT done (owner handoff):** fresh source freeze/publish, server rebuild and
  git commit — git owner work after this yield, per the brief. Current
  `18facb4` builds remain untouched and are not re-validated here (not claimed
  universally accepted).
- **NOT done (cross-worker):** Android fixture 18131/5435 (BuildCorrectionAndroid-3)
  — untouched; my cluster was a throwaway port-5434 pg_virtualenv.
- **NOT done (pre-existing R67 gap, unchanged):** real Google grant / People API
  enablement proof; the status route still never touches Google and
  `authorization_required` remains driven by stored `granted_photo_scopes`.
- **NOT done:** full suite (CF58) — one scoped proof only, per the brief; the
  lead runs the full suite after the frozen merged candidate.
- **NOT done:** installed 0.1-40 `GET /v1/gmail/sender-photos/status` live gate
  (`receipts/google-photo-live-gate.md` GATE = UNKNOWN_ROUTE_MISSING) — needs
  the rebuilt/installed server, i.e. the owner rebuild above.
- **NOT done:** no shared installer diagnostics (even dry-run), no production
  tokens/grants, no production DB services, no deploy/Qt changes.

## 9. Publication checkpoint (Git owner, 2026-10-20) — current identity, retro, rollback

### 9.1 Current status delta identity (corrects the stale "immutable source SHA" reading of old freeze §11)

`receipts/correction-source-freeze.md` §11 recorded `f7845ee`/`06ac0f5` as the immutable source SHA;
the branch then advanced through `18facb4` (successor freeze) to this publication. Old history is left
untouched — not replaced, not rewritten. The current identity of this repair is the content delta
against `18facb4`:

| Path | sha256 |
| --- | --- |
| `server/internal/senderphoto/handler.go` | `8ee4deab7efa047aeafc7048a50389141ea172fecd7dcb66001ea330885487c9` |
| `server/internal/senderphoto/status_test.go` (new) | `d64ace80399d5dbf0d01e2a411d56c74011ea6099b9d62d9a0d47d135d72ae8b` |
| `REQUIREMENTS.md` | `0c7115e06b553eea1f6391df05d2f1594d6f1afe3008f65eafc9a7c1f09b7144` |

Staged with them: this receipt and the two durable logs below — six paths, nothing else. Qt provenance
for consumers: `git diff --stat 18facb4 -- qt/` is **empty** — the Qt source/build input tree,
including `qt/debian/changelog`, is byte-identical to `18facb4` and no Qt path is in this delta (the
Qt changelog is deliberately unchanged so the input tree stays equal).

### 9.2 Durable proof logs (copied out of /tmp)

| Log | sha256 |
| --- | --- |
| `receipts/photo-status-tenant-proof.log` | `02c6cf13720af0541ef44c5c4a9618c499bfedf795718cf70b4a6c2def243f6a` |
| `receipts/photo-status-tenant-negative.log` | `f9ddc019588d1233b0016c770936a1c889713232b0a909a5ade6e54516de1ded` |

Copy readback: each durable copy hashes equal to its `/tmp/photo-status-tenant-*.log` original, so the
evidence is no longer /tmp-only. Contents: scoped proof `--- PASS: TestStatusRouteTenantVisibility`
exit 0; negative control `own tenant accounts=[], want exactly 2` exit 1.

### 9.3 Publication tuple, push, readback, rollback

- Remote URL: `git@github.com:Komzpa/litterbox.git` (remote `origin`).
- Full destination ref: `refs/heads/ui-desktop-mobile-redesign-20261018`.
- Observed old OID (fresh `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018`
  immediately before the push): `18facb4e153d51586c8e4cf7875e7d3113191986`.
- Local branch, recorded separately: `ui-desktop-mobile-redesign-20261018` at `18facb4e…` pre-commit —
  equal OIDs never substitute for matching repo and ref.
- Push (exact): `git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:18facb4e153d51586c8e4cf7875e7d3113191986 git@github.com:Komzpa/litterbox.git ui-desktop-mobile-redesign-20261018:refs/heads/ui-desktop-mobile-redesign-20261018`
- Readback: `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018` after the push —
  published OID equals this commit's OID; the literal value is returned in the handoff and re-checkable
  with that same command.
- Rollback (same full ref, expecting the published OID): `git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<publishedOID> git@github.com:Komzpa/litterbox.git 18facb4e153d51586c8e4cf7875e7d3113191986:refs/heads/ui-desktop-mobile-redesign-20261018` —
  restores `18facb4` under a lease that fails if the publication is not the current tip; no
  delete/recreate, no bare force.
- Immutable export: `git archive --format=tar <fullSHA> | tar -x -C <outdir>` from this repository;
  `git rev-parse HEAD` at the publication commit yields `<fullSHA>`.
- Identity: author/committer `Darafei Praliaskouski <me@komzpa.net>` from
  `git var GIT_AUTHOR_IDENT` / `GIT_COMMITTER_IDENT`, config provenance
  `file:/home/kom/.gitconfig` (`user.name`/`user.email`); no injected identity, no `--author`.

### 9.4 Retro (publication, required)

1. Content was accepted and its three sha256 values re-verified against the worktree before any Git
   mutation; §2 hashes matched byte-for-byte, so nothing drifted between repair and publication.
2. Provenance was narrowed to the one claim builders need — Qt input tree unchanged from `18facb4`
   (`git diff --stat 18facb4 -- qt/` empty). An earlier attempt compared Git SHA-1 blob IDs against
   `sha256sum` output; that mixes hash families and was discarded — only git's own tree diff is cited.
3. One lease-bound push on the named full ref with the observed old OID: a concurrent writer fails
   the lease instead of being overwritten, and the mirror command above restores `18facb4`.

### 9.5 Residual gates (not closed by this publication)

- Server artifact rebuild/rebind for `deploy/release.py`: **Main's decision** — this row publishes
  source only; no build, no manifest or validator edit, no version change.
- Android fixture 18131/5435 (BuildCorrectionAndroid-3-2): untouched and ongoing, preserved.
- Real Google grant / People API enablement and the installed-server live status gate: still open
  (§8); installed release unchanged.
- No installer diagnostics (even dry-run), no production DB/grants, no deploy/Qt/device use; the
  three original waiters keep their state and were not dispatched from this row.
