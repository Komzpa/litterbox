# Android crash diagnostics

The app owns private crash collection and retries delivery to its existing server
using its enrolled device bearer token. No USB connection, manual log export or
third-party telemetry is part of this path. Source implementation is not proof
that a collector or migration has been installed on any particular device/server.

## Evidence and privacy boundary

The Android application records sanitized Java exception classes and
`  at Class.method:line` frames, never arbitrary exception messages, thread names,
mail bodies, addresses, credentials, request payloads or raw logcat. Metadata
identifies the package, app version/build, Android API, observation time, kind,
reason, status and whether frames are available. Reports remain in app-private
storage until the authenticated server acknowledges their stable report ID.
Offline operation and server failures must not erase queued evidence or change
mail, outbox, payload or Undo state. See the Android collector and Qt uploader
sources for local durability and lifecycle behavior.

The local queue holds at most 64 unacknowledged reports. At capacity it refuses
new reports rather than evicting undelivered evidence; capture is therefore
best effort under saturation, low storage, memory exhaustion or abrupt death.
Acknowledged-ID tombstones are bounded to 256, so very old Android history can
eventually be rediscovered after its tombstone is pruned; server identity-based
deduplication still preserves the original receipt. Upload uses the existing
authenticated API/network manager on startup, token availability, reconnect,
resume and periodic retry, not a separate unauthenticated transport.

On Android API 30+, the collector can query retained package process-exit
history on next startup. Native process crashes and ANRs can therefore be
reported as typed reason/status/time facts with `stack=unavailable`, not invented
Java exceptions. Raw tombstones/ANR traces are not retained because they may
contain sensitive data. This path does not produce symbolicated native frames.
Android can evict exit records; older Android versions do not provide this API.
An empty, unsupported or failed history query is availability evidence, **not a
crash** and not proof that no crash happened. A historical record uses
`version=unknown`, `build=unknown` and
`version_scope=unknown-historical-process`: the current recovery build must not
be attributed to the past process. Installing a collector cannot recreate
exception text, frames or build identity Android has already lost. The cause of
a past phone failure remains unknown unless retained evidence establishes it.

## Authenticated receipt contract

`server/cmd/litterbox/main.go` registers `diagnostics.Register` inside the database
configuration branch. Its mux is wrapped in the existing device-auth middleware;
this is not a public diagnostics endpoint. The handler additionally requires a
real `auth.DeviceFromContext` identity (the synthetic development identity is
rejected). Tenant and device are derived from that identity, never JSON fields.
There is no HTTP report-list/read endpoint.

`POST /v1/diagnostics/crashes` keeps the historical JSON envelope:

```json
{"report_id":"java-1793000000000-123","body":"<sanitized lines>","uploader_build":"abcd123-dirty"}
```

The report ID is 1–96 lowercase letters, digits or hyphens. `uploader_build` is
1–64 ASCII letters/digits, dots, underscores, plus or hyphens. The body is at most
65536 bytes. Unknown envelope fields and trailing JSON are rejected. Body lines
are unique allowlisted metadata, qualified exception class names or sanitized
frames. Required metadata is:

| Key | Accepted values |
| --- | --- |
| schema | `1` |
| version | 1–96 ASCII letters/digits, dots, underscores, plus or hyphens |
| build | unsigned decimal or `unknown` |
| package | `org.qtproject.example.litterbox_qt` |
| android, timestamp | unsigned decimal; timestamp is client epoch milliseconds |
| kind | `java` or `exit` |
| reason | unsigned decimal Android exit reason, `uncaught` or `unavailable` |
| status | signed decimal Android status or `unknown` |
| stack | `frames` or `unavailable` |
| version_scope | `current-process` or `unknown-historical-process` |

Optional `history=available|empty|unavailable|unsupported` is accepted only for
exit evidence. Java reports require `reason=uncaught`, `status=unknown`,
`stack=frames`, `version_scope=current-process`. Exit evidence requires
`stack=unavailable`, unknown version/build and historical-process version scope.
Exit evidence cannot contain class/frame lines. Its reason is numeric or
`unavailable`, never `uncaught`. `reason=unavailable` requires `status=unknown`
and a history availability value; numeric reasons allow history omitted or
`available`, not `empty`, `unsupported` or `unavailable`.
An exit with `reason=unavailable` and a history availability value is not an
incident row that establishes a process crash. Do not count it as one in analysis.
Metadata allowlisting is a format/privacy boundary, not attestation that an
authenticated client's reported facts are true.

Only a committed save yields HTTP 200 with `{"ack":"<exact report_id>"}`.
The client may remove pending evidence only after that exact authenticated ACK.
A retry after ACK loss receives the same ACK without modifying the original row
or its first `received_at`. All other responses, including validation rejection,
leave evidence pending: `400` malformed JSON, `401` missing/invalid/revoked device,
`422` invalid sanitized format, `503` failed persistence (including a device
revoked after middleware authentication). Authenticated `POST` requests to this
exact diagnostics route have an independent per-device budget of 64 requests
per rolling minute, including rejected reports. They never debit the ordinary
60 requests/device/minute budget used by mail and owner operations. Both budgets
return `429` on exhaustion, never an acknowledgement. Token/revocation checks
still precede the diagnostic limiter; unauthenticated traffic retains the
existing IP limit.

## Storage and operator readback

Apply `server/db/026_crash_reports.sql` through normal deployment migration
procedures **before** serving uploads. Historical WIP migration number 019 is not
usable: the current migration 019 owns card embeddings. No migration or release
is implied by adding these sources. Missing storage makes uploads fail rather
than ACKing undurable evidence.

The PostgreSQL store rechecks the enrolled device/tenant pair and revocation
under a device-row `FOR UPDATE` lock held through insertion and commit. This
serializes persistence with the existing revocation `UPDATE`: if revocation
commits before the store obtains the row lock, the store rejects without ACK;
if persistence obtains the lock first, its commit precedes completed revocation.
An ACK for that earlier commit may still arrive over the network after revocation
completes; this is not a network-delivery cutoff guarantee. The store sets
transaction-local `litterbox.tenant_id` and `litterbox.device_id`, inserts, and
commits before ACK. The primary key is `(tenant_id, device_id, report_id)`; another
device's colliding ID cannot overwrite or suppress this device's report. A
composite device foreign key prevents cross-tenant attribution. Forced RLS
restricts SELECT/INSERT to both transaction-local identities. The runtime role
has SELECT/INSERT only, not UPDATE/DELETE. There is no automatic server expiry.
Use authorized database-owner access for operator readback; never expose this
query as an unauthenticated API:

```sql
SELECT tenant_id, device_id, report_id, kind, app_version, uploader_build,
       received_at, body
FROM crash_reports
WHERE tenant_id = '<tenant UUID>'::uuid
  AND device_id = '<device UUID>'::uuid
ORDER BY received_at DESC, report_id;
```

`received_at` is server receipt time, distinct from the client-reported event
`timestamp`. For non-owner inspection set **both** identity settings inside a
transaction; unset scope intentionally exposes zero rows. Database-owner
readback bypasses RLS and must remain an operator privilege.

## Frozen verification gate

No test/build/deployment result is claimed here. Owning server behavioral tests:

- `server/internal/auth/auth_test.go`: exact-route quota selection, per-device
  independence, unchanged ordinary 60/minute limit and continued token checks.
- `server/internal/diagnostics/diagnostics_test.go`: real auth middleware plus
  registered route, exact ACK, failed-save no-ACK, method/path negative controls,
  malformed JSON, sensitive-body rejection and honest historical metadata;
  complete 64-request valid/422/503 batches interleaved with authenticated mail
  and the registered invite owner operation, diagnostic request 65 rejected,
  ordinary requests 1–60 accepted and 61 rejected. The mail handler here is a
  test consumer of auth context, not proof of mail storage or Qt CardStore state.
- `server/internal/diagnostics/postgres_test.go`: disposable database migrations,
  real token lookup and HTTP consumer, non-owner/no-BYPASSRLS persistence,
  immutable duplicate ACK across reconstructed consumers, exact operator row
  readback, same-ID device/tenant isolation, unscoped/scoped RLS, forged device
  rejection, revocation and privacy negative controls; deterministic revocation
  after middleware/before store and locked persistence versus blocked revocation.

Client-owned frozen-gate inventory (authored, not execution evidence):

- `qt/tests/tst_crashuploader.cpp` exercises the real API HTTP consumer with
  same-spool restart, offline/503/422/wrong-ACK retention, reconnect, exact-200
  ACK, bearer authentication, no enrollment and the full wire-schema fixture.
- `qt/tests/CrashStoreTest.java` with `qt/tests/crashstore_oracle.cmake` exercises
  the production Java spool/scrubber: privacy, stable bytes across restart,
  acknowledged tombstones and capacity refusal.
- CTest names are `crashuploader` and `crashstore`; Java oracle registration
  requires Java Development discovery. Neighboring regression owners are
  `qt/tests/tst_api.cpp`, `qt/tests/tst_serverurl.cpp` and
  `qt/tests/tst_androidupdater.cpp`.

At the single frozen verification phase, run `go test ./...` from `server`,
then `CARD_TEST_POSTGRES=1 go test ./...` with a disposable PostgreSQL
`DATABASE_URL` (the repository `testdb` harness isolates schemas and roles).
For focused diagnosis use `go test ./internal/auth ./internal/diagnostics` and
`CARD_TEST_POSTGRES=1 go test ./internal/auth ./internal/diagnostics` with the same
disposable URL. These tests must never target a product database. Client lifecycle and
Android runtime proof belong to their owning tests; passing server tests does
not prove installed phone behavior, an existing incident's cause, or deployed
route existence.
The exact Android retained exit row, Java spool contents and authenticated
server receipt must also be checked together in disposable runtime proof.
History availability alone cannot satisfy that exact-row acceptance gate.

## Archive-storm registry crash (2026-10-31)

`fix-archive-storm` changes inbox control-registry removal to assignment of
`undefined`, including drag rekeying. Every consumer already treats that value
as absent; the drag scan skips empty entries. Removed controls are not retained.
This avoids repeatedly deleting/re-adding dynamic card-id properties on the same
JavaScript objects during `registerControls` on presentation changes.

On the isolated real-mail v40 emulator, the original APK crashed twice in two
fresh trials: 18 taps / 28 queued operations (fault `0x40`, uptime 232 s), then
33 taps / 90 operations (fault `0x38`, uptime 46 s). The earlier acceptance run
also crashed at 97 operations. These are operation-row counts, not tap counts.
A test-only interpreter APK reproduced the same GUI-thread SIGSEGV after
54 taps / 78 operations (fault `0x50`, uptime 89 s). Its last QML console stack
was `unregisterControls <- registerControls <- onCardIdChanged <-
rebuildPresentation` at the `rows = ordered` notification. ARM64 translation
still yielded only an anonymous host-exec native frame; it did **not** expose
an Android guest C++ caller.

A small isolated Qt 6.7.3 desktop registry-churn input reproduced SIGSEGV
without Litterbox, mail, delegates, networking or Android translation. GDB
resolved `QV4::Object::insertMember <- QV4::Object::internalPut <-
QV4::Runtime::StoreElement::call`. The same input with clear-to-undefined
completed 10,000 rounds and exited 0; delete/re-add exited 139. These are
desktop diagnostic frames, not fabricated Android frames. The native run
receipts and retained probe are in `publish/cohort/archive-*.json`,
`archive-interp-1-logcat.txt`, `registry-churn.qml` and `registry-churn-gdb.txt`.

Retro: `console.trace()` in Qt 6.7 takes no arguments; log a label separately.
Interpreter mode alone does not bypass this engine object-property failure.
Recover caller context at the small registry owner rather than adding generic
delegate-lifetime guards. The first fixed-build trial did not start because
all three transient setup units were externally stopped together; those
attempts are setup failures, never passing load trials. Recovery reused the
same AVD/user, existing DB clone and ports and re-added the in-guest DNAT.
No owner phone, owner checkout, accessibility dump or live 8081 service was used.

The exact non-instrumented candidate from source
`2ec8d7797d641043f6e578ea863e2992e77f61df` is
`artifacts/litterbox-v40-fix-archive-storm.apk`, SHA-256
`8ea188b27e6a09f39fa1dcb4e006432044fcdb1de8d6047c10ba5223a5ee8c77`.
Three consecutive native trials queued **130, 130 and 131 operations**, drained
in **7,805, 7,794 and 9,751 ms**, retained PID **6726** throughout and had empty
crash buffers. All three started with an empty outbox (the first command used
`resume`, but the observed initial count was zero). No interpreter or trace
instrumentation is in this candidate. v38 under the same real-mail trigger
has **not** been run by this bounded lane; its historical synthetic control is
not a substitute. The original rejected release acceptance is not overwritten.
This is a proven crash-fix candidate, not a published release.
