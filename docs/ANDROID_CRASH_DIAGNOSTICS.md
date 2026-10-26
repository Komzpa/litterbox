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
`422` invalid sanitized format, `503` failed persistence. Authentication can also
return its existing `429` rate limit; this is not an acknowledgement.

## Storage and operator readback

Apply `server/db/026_crash_reports.sql` through normal deployment migration
procedures **before** serving uploads. Historical WIP migration number 019 is not
usable: the current migration 019 owns card embeddings. No migration or release
is implied by adding these sources. Missing storage makes uploads fail rather
than ACKing undurable evidence.

The PostgreSQL store rechecks the enrolled device/tenant pair and revocation,
sets transaction-local `litterbox.tenant_id` and `litterbox.device_id`, inserts,
and commits. The primary key is `(tenant_id, device_id, report_id)`; another
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

- `server/internal/diagnostics/diagnostics_test.go`: real auth middleware plus
  registered route, exact ACK, failed-save no-ACK, method/path negative controls,
  malformed JSON, sensitive-body rejection and honest historical metadata.
- `server/internal/diagnostics/postgres_test.go`: disposable database migrations,
  real token lookup and HTTP consumer, non-owner/no-BYPASSRLS persistence,
  immutable duplicate ACK across reconstructed consumers, exact operator row
  readback, same-ID device/tenant isolation, unscoped/scoped RLS, forged device
  rejection, revocation and privacy negative controls.

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

At the single frozen verification phase, run `go test ./internal/diagnostics`
from `server`, then the same package with `CARD_TEST_POSTGRES=1` and a disposable
PostgreSQL `DATABASE_URL` (the repository `testdb` harness isolates schemas and
roles). These tests must never target a product database. Client lifecycle and
Android runtime proof belong to their owning tests; passing server tests does
not prove installed phone behavior, an existing incident's cause, or deployed
route existence.
The exact Android retained exit row, Java spool contents and authenticated
server receipt must also be checked together in disposable runtime proof.
History availability alone cannot satisfy that exact-row acceptance gate.
