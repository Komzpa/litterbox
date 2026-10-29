# Self-hosted deployment operations

Production PostgreSQL and the VPS/second backup host are external deployment prerequisites; this repository does not contain real endpoints, keys, credentials, or deployment proof. Run server operations only after provisioning dedicated hosts and restricting service accounts.

## Production application service

The application server runs as a dedicated systemd unit `litterbox-server.service` (template in `deploy/systemd/litterbox-server.service`). Install the release-built binary at `/opt/litterbox/server/litterbox` (root-owned, mode 0755, directory mode 0755 so the service account can traverse it) and enable the unit. The unit runs as the least-privileged `litterbox` account and reads the production DSN from `/etc/litterbox/db.env`, which selects the dedicated production cluster through the application role:

```sh
DATABASE_URL='postgresql://litterbox_app:...@127.0.0.1:5435/litterbox?sslmode=prefer'
```

The unit passes no `-dev-tenant-id` and no `LITTERBOX_DEV_TENANT_ID`, so protected `/v1` routes require a valid bearer token and unauthenticated requests fail closed with `401`. `GET /healthz`, `GET /v1/version`, the homepage at `GET /`, and the privacy policy at `GET /privacy` remain unauthenticated. The homepage identifies Litterbox and its owner; the privacy policy includes the Google API Services User Data Policy Limited Use disclosure. Use `https://komzpa.net:9443/` and `https://komzpa.net:9443/privacy` as the Google OAuth Branding homepage and privacy policy URLs. The server listens on `127.0.0.1:8081` (loopback only), distinct from the development server's `8080`, and never binds a public address. Verify with `systemctl is-active litterbox-server.service`, the installed executable's SHA-256, `GET /healthz`, `GET /`, and `GET /privacy` returning `200`, and an unauthenticated `/v1/cards` returning `401`.

The public nginx proxy also needs to allow these exact page paths. On `komzpa.net`, the `litterbox-https-proxy` container mounts `/etc/litterbox-remote/nginx.conf`; its proxied location is `location ~ ^/($|privacy$|healthz$|v1/)`, forwarding to the existing reverse tunnel on `127.0.0.1:18080`. Other paths continue to return `404`. Back up this file before changing it, then validate and reload with `docker exec litterbox-https-proxy nginx -t && docker exec litterbox-https-proxy nginx -s reload`. For rollback, restore the saved configuration into the existing mounted file and validate/reload again.

Release binaries support a source stamp through the existing `main.buildSHA`
variable. Build from the committed release checkout with
`go build -ldflags="-X main.buildSHA=$(git rev-parse HEAD)" -o <binary> ./cmd/litterbox`
in `server/`, then pass that exact binary and release version to
`deploy/server/debian/build.sh`. Read back `GET /v1/version` after cutover:
`server_build` must equal the recorded release commit, not `unknown`.

GitHub notifications are routed by repository and reaction before semantic clustering; the semantic pass must process only the remaining non-GitHub cards, even when Ollama is healthy. On the first Gmail synchronization pass after startup (and idempotently on subsequent account polls), `SyncAccount` calls `bundles.ReassignGitHub` inside its tenant-scoped transaction before any Gmail network access. This reuses the ingestion router and take-out exclusions to repair legacy sender-bundled notifications, including archived, done, and snoozed cards, without changing their state or requiring manual SQL or embeddings. Unchanged memberships write no card changes. This repair requires the existing Gmail sync worker to be configured and the tenant to have a connected account; restart the packaged server normally, then verify card membership keys and preserved states through the application or a read-only tenant-scoped query.

LinkedIn topic isolation is repaired by the ordinary `AfterIngest`/`Cluster` pass over open mail cards on the next successful Gmail thread save (or another existing ingest trigger), not by the GitHub-only startup repair. That pass replaces existing memberships and clears unmatched cards; no one-off SQL migration is needed for an open wrong bundle. Empty historical bundle rows may remain, but without member cards they do not group anything. Embedding failure also detaches non-structured open mail rather than sender-bundling it. Archived/done/snoozed non-GitHub cards are outside this pass and are reconsidered when reopened; restarting with no subsequent ingestion alone does not run this repair.

`AfterIngest`/`Cluster` owns its tenant-scoped transactions: it reads committed mail in a short transaction, commits before calling Ollama, and opens a separate short transaction to apply bundle memberships. Gmail thread saves commit their card, messages, and body before invoking this hook synchronously. Non-mail `POST /v1/ingest` commits its card upsert and returns `204` without waiting for clustering: the hook runs in a background job detached from the HTTP request, bounded to 60 seconds, with errors logged. Each Ollama embedding call has its own 15-second deadline, including with a custom HTTP client; a failed call uses the existing structured-routing fallback while the parent cluster context remains valid. Slow or canceled embeddings therefore hold no database transaction or row locks, and errors roll back the active database phase. The hook accepts a pool or connection, not an existing transaction. On SIGTERM, the server first shuts down HTTP (up to 10 seconds), then drains in-flight ingest cluster jobs for up to 60 seconds before closing database pools. Clustering interrupted by the bound or process termination re-runs on the next successful ingest or Gmail thread save; committed producer cards do not depend on completion of that pass.

Apply `server/db/019_card_embeddings.sql` before running the embedding-cache release. Semantic clustering persists vectors in tenant-scoped `card_embeddings`, keyed by card id and `LITTERBOX_EMBED_MODEL` (default `nomic-embed-text`), with SHA-256 of the exact normalized input sent to Ollama. The first run warms the cache; unchanged cards make no embedding calls on subsequent runs, including after restarts. A new card or changed embedding input costs one call; changing models requires a new vector for each candidate. Cache hits do not bypass re-clustering or membership repair. Cache reads share the short mail-read transaction, and each successful miss is written in its own short transaction committed before the next embedding call. Embedder failures are not cached and still use structured fallback. No transaction spans an Ollama call.

## Gmail sender photos (R67)

Real sender avatars need the Google Cloud project's People API enabled and the incremental photo grant. `gmail.modify` alone carries no sender-photo field: the photo sync gates on both `contacts.readonly` and `contacts.other.readonly` actually granted, never on the requested string. To grant, call `POST /v1/gmail/photo-consent` (authenticated, same tenant) and open the returned `authorization_url` in a browser; the shared OAuth callback persists the granted scopes. `GET /v1/gmail/sender-photos/status` reports per-account `photo_ready` and explicit `authorization_required` with `required_scopes` without touching Google. Accounts stay fully usable for Gmail while photos wait for consent. Coverage is contact photos only (saved contacts plus other contacts, exact mailbox match, non-default): senders absent from both datasets stay explicitly uncovered even when Gmail renders a photo. No credentials enter the repository; proof runs use disposable fake People servers, never production grants.

The loopback-only listener is kept as the server's only bind. To reach clients on a trusted LAN (for example `192.168.100.0/24`), install `deploy/systemd/litterbox-lan-proxy.socket` and `deploy/systemd/litterbox-lan-proxy.service`: the socket listens on the host's LAN address (`192.168.100.74:8081`) and `systemd-socket-proxyd` forwards accepted connections to `127.0.0.1:8081`. Enable with `systemctl enable --now litterbox-lan-proxy.socket`. This exposes the API only on the specified LAN address; docker bridges and other interfaces stay closed. Authentication is unchanged, so every `/v1` route still requires a bearer token. Rollback with `systemctl disable --now litterbox-lan-proxy.socket litterbox-lan-proxy.service` and remove the two unit files.

The socket uses `FreeBind=yes` so the LAN address may be assigned after the listener is created. Do not add `After=network-online.target` or `Wants=network-online.target` to this socket: network startup can depend on `sockets.target`, creating an ordering cycle that deletes the socket's boot start job. Existing installations can retain `/etc/systemd/system/litterbox-lan-proxy.socket.d/10-freebind.conf` with `[Socket]` and `FreeBind=yes`.

## Unified release installation and rollback

Publish one frozen-source cohort: the server Debian package, ordinary desktop
Debian package and signed Android APK. Build and accept all three before changing
any installed or served state. `deploy/release.py` is the sole cutover entrypoint;
do not reconstruct a release from ad-hoc copy, environment-edit or restart commands.
Apply the operator-owned SQL migrations before release: `server/db/020_bundle_title_origin.sql`
for explicit title origin and `server/db/021_message_sender.sql`. Migration 021 is
additive only — `messages.sender_name` and `messages.sender_address` are
`NOT NULL DEFAULT ''`, so legacy rows keep empty sender fields; migration 020's
historical rows stay untouched. The release CLI runs no SQL: migrations and grant
changes are completed by the operator before the CLI plan/apply/rollback flow.
Read-time preview/title projections leave archived cards and stored bodies intact.
An unknown legacy title may be marked as a generated seed only from independently
recorded provenance, using an exact expected-value guard and preserved rollback;
never infer provenance from title/subject equality or run a broad backfill.

Source publication, package preparation, server publication and installed-client
adoption are different facts. Record the full artifact source SHA separately
from the commit and SHA-256 of the release CLI: documentation/CLI-only commits
do not require rebuilding unchanged accepted application artifacts. All three
artifacts of one release must be frozen from the same production source SHA;
later test-only or documentation-only commits are bound separately in the
handoff receipt and never relabel the already-frozen artifacts.

Build packages out of tree; preserve installed binaries, unit and packages.
The true current build kits differ per platform and must not be substituted for
each other. Desktop: Qt 6.11 with the system Kirigami (`libkirigami-dev`,
`qml6-module-org-kde-kirigami` from `qt/debian/control`). Android: Qt 6.7.3
`android_arm64_v8a` with the Kirigami Android prefix (`KIRIGAMI_ROOT`), the
bundled ABI-matched OpenSSL 3 runtimes, and Gradle bounded with
`-Dorg.gradle.daemon=false -Dorg.gradle.workers.max=2
-XX:ActiveProcessorCount=2`. The Android recipe must pin
`JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64`: the host default JDK 21
breaks R8/D8 dexing deterministically (see `receipts/mobile-ui-proof.md`).
`qt/android-build.sh` still defaults `JAVA_HOME` to JDK 21, so the release
recipe must override it explicitly. Sign the APK only through the canonical
redacted loader `RUNTIME/sign-apk-redacted.py`; the keystore password is loaded
at runtime from the private release record and is never printed, and no
signing secret may enter this document, the cohort manifest, receipts or the
repository.

Prove the frozen server through real CardStore/QML in an isolated profile before
publication or installation. Installed desktop clients use the existing
UpdateWatcher: after the package replaces the executable and the replacement is
stable with package-manager locks released, it starts a replacement user service
and quits normally. Do not drive or restart the owner desktop. Observe executable
mappings to confirm adoption; otherwise report the exact remaining serving gap.

Build the server package with `deploy/server/debian/build.sh` from the pinned
binary. It installs `/opt/litterbox/server/litterbox` and the canonical unit at
`/lib/systemd/system/litterbox-server.service`; postinst reloads systemd but does
not restart it. Build the desktop with the existing Debian recipe, without adding
a custom ELF note: its exact executable hash and source are bound by the build
receipt. Configuration and credentials under `/etc/litterbox` never enter packages.
Complete pending operator-owned SQL migrations before release; `tools/release_status.py --check` verifies them.
The check probes created tables and effective table grants, not every migration
statement. The CLI neither migrates databases nor changes grants. Migration `011`
requires `SELECT, INSERT, UPDATE, DELETE` on `card_bodies` for `litterbox_app`,
with RLS retained.

Prepare a private absolute-path `cohort.json` with this contract (no credentials):

- `source_sha`: full frozen Git SHA. `artifacts` has exactly `server`, `desktop`,
  `android`; each has `path`, `sha256`, `source_sha` equal to that frozen SHA.
  Debian entries add `package`, `version`, `architecture`, `binary_sha256`.
  Android adds `package`, integer `version_code`, `version_name`, `signer_sha256`.
- `proofs`: `{path, sha256, artifacts: [names], accepted, bindings}` references.
  Every reference binds the source and each listed artifact hash through dotted
  JSON keys, for example `source_sha` and `artifacts.desktop.sha256`. All three
  need an accepted receipt (`status` PASS/ACCEPTED/accepted or `accepted: true`,
  never `accepted: false`). Include the producer manifest/build receipts as
  non-acceptance references; retain exact-candidate native and lead visual approval.
- `target`: `host`, `service`, `environment`, `server_binary`, `desktop_binary`,
  `unit_path`, `origins`, `credential_loader`, `aapt`, `apksigner`, plus hash-bound
  `publisher_inputs`, `before_receipt`, `rollback_debs.server/desktop` references.
  `drop_ins` is an explicit list of `{path, sha256}` references for every effective
  absolute systemd drop-in path (use `[]` for none). The live `DropInPaths` set must
  match exactly, and every declared file must match its hash before mutation.
  Repeated `EnvironmentFiles` properties are separate ordered sources with their
  `ignore_errors` flags, not a dictionary's last value; the configured environment
  must be an exact path entry. The CLI snapshots every environment source and
  declared drop-in, without printing their contents.
  The credential loader is a private JSON file with an enrolled-device `token`;
  neither its contents nor child-command output is printed.
- Publisher inputs bind `host`, `service`, `environment_path`,
  `environment_sha256`, `server_binary`, `server_binary_sha256`, `server_source`,
  `origins`, `credential_loader`, `candidate_source`, `candidate_apk`,
  `candidate_sha256`, `candidate_version` (decimal string), `candidate_package`,
  `candidate_signer`, `rollback_apk_preserve`, `proposed_immutable_apk_path`, and
  `environment_release_values` containing the three current Android keys below.
  The before receipt contains `signer_sha256`, `apk_sha256`, and `manifest`
  with the old `version` and `package`.
- Explicit mobile installation additionally requires `mobile`: hash-bound
  `helper` (`phone_adb.py`), canonical `registry` and `lock_path`, `adb`, verified
  `endpoint`, hardware `serial`, `model`, and `old_apk` with Android metadata.
  Use the public helper's registered identity/discovery first; no default device.
  Preserve the operator's `HOME` and `XDG_STATE_HOME` as below so root uses the
  same canonical phone lock and already-paired ADB key, not a second root identity.

Run the same manifest through the real CLI (backup parent must already exist).
The CLI's complete accepted surface is exactly the three actions `plan`, `apply`
and `rollback` with the optional `--receipt` (required for `apply`),
`--install-mobile DEVICE_KEY` and `--adoption-timeout` options defined by the
parser in `deploy/release.py`; do not invent or pass any other flag.

```sh
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py plan /private/cohort.json --install-mobile mainphone
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py apply /private/cohort.json --install-mobile mainphone --receipt /var/backups/litterbox/release-ID
sudo --preserve-env=HOME,XDG_STATE_HOME python3 deploy/release.py rollback /var/backups/litterbox/release-ID
```

`plan` performs all hash/source/signature/proof/live-state checks and the Debian
no-act check without installation. `apply` repeats every preflight before any
installation, preserves old packages/APK, executable/environment/unit bytes and
all declared drop-ins and existing environment sources (deduplicated by path) in
a root-private rollback packet with hashes, ownership and modes. It stages all
three candidates, installs both Debian packages together, atomically replaces
only the three Android keys, and restarts the server once. Other environment
bytes and ownership/mode are preserved. Composition is checked again before
installation and after restart, including repeated environment entries and flags,
drop-in paths/hashes, and unchanged source bytes/ownership/modes (except the three
intentional release-key changes). Unlisted or changed drop-ins fail preflight.
It verifies installed package/executable identities, the running server mapping
and source revision, authenticated manifests/download hashes at every origin,
anonymous rejection, and passive desktop adoption. It never drives the owner GUI.
With explicit mobile selection it uses the canonical phone lock and independently
verified transport/hardware identity for only `adb -s <endpoint> install -r <APK>`
and narrow package/version/hash readback. It never pairs, unlocks, enables debugging,
approves permissions, launches apps, or changes other packages.

Without `--install-mobile`, the phone is untouched and the receipt says
`mobile: not_requested`; publication success is not all-three installed completion.
Any failed readback leaves `status: failed`, never a success claim. Keep the
printed receipt and exact CLI hash. Explicit rollback verifies its saved hashes,
reinstalls both old Debian packages, restores exact old files including declared
drop-ins and all saved environment sources with ownership/modes, restarts the same
service and rechecks complete composition, old package/process/publication identities
and desktop adoption. The old phone APK is preserved, but Android downgrade/uninstall/data
restoration is never automatic; rollback explicitly reports that phone limitation.

## Android update release

The server exposes `GET /v1/android/update` (JSON manifest) and `GET /v1/android/update.apk` (APK bytes). Both routes are behind the existing device-token authentication middleware when the database-backed API is enabled; the client requests the manifest's fixed relative download path on the same configured origin with the same enrolled-device Bearer token. The server snapshots the configured APK at startup (maximum 256 MiB), computes SHA-256 over those exact served bytes, and serves that same snapshot for its lifetime.

Qt Android HTTPS requires bundled OpenSSL 3 runtimes: Android's system TLS libraries are not an OpenSSL ABI provider. Configure every Android build with `-DQT_ANDROID_EXTRA_LIBS='/path/to/arm64-v8a/libcrypto_3.so;/path/to/arm64-v8a/libssl_3.so'` (or set `ANDROID_OPENSSL_ROOT` for `qt/android-build.sh`). CMake fails closed if either runtime is missing. Use the same verified KDAB `android_openssl` arm64 libraries as the TLS-proven release; do not omit them when copying a release recipe. Before publishing, `unzip -Z1 <signed.apk>` must contain `lib/arm64-v8a/libcrypto_3.so`, `lib/arm64-v8a/libssl_3.so`, and the Qt OpenSSL TLS backend. Offline status now includes the actual transport/HTTP reason, cleared when the cards stream reconnects. A saved enrollment `server_url` still overrides the build default; changing the default does not migrate existing devices away from a dead LAN origin.

Configure the litterbox service environment with all three values. `VERSION` is the signed APK's decimal Android `versionCode`, not a marketing version; `PACKAGE` is its exact application ID:

```sh
LITTERBOX_ANDROID_APK_PATH=/opt/litterbox/server/litterbox-qt-v20-<sha256-prefix>.apk
LITTERBOX_ANDROID_VERSION=20
LITTERBOX_ANDROID_PACKAGE=org.qtproject.example.litterbox_qt
```

Every shared Qt release uses the unified cohort above; a desktop package, source push or local APK alone is incomplete. Freeze a signed candidate whose decimal versionCode is strictly above both served and recorded installed releases; record its full source and APK SHA-256, badging and certificate, retaining package/signing lineage, OpenSSL runtimes and QML imports. Inspect `qt/android-build.sh` before use: its default selects the real phone and installs/launches. Release builds must use an explicit out-of-tree build-only recipe with the canonical redacted signing loader, not that default workflow.

Prove R42 on that exact signed file first: a fresh owned native emulator enrolls through the normal UI with no `LB_SERVER` and no pre-enrolled profile and reaches Online against the intended deployed origin. Record any dedicated test-device production enrollment API requests and their tenant scope/rollback. Retain exact-candidate phone-size inbox/mail/topic captures: at least four complete collapsed/standalone cards, the real mail card's retained body, and neighboring Settings. Separately, the member-preview gate uses version 1 of the deterministic synthetic three-member newsletter projection in `qt/tests/techshipcontext.js`, with no owner mailbox/token/content or live message/date/archive-state dependency. Run the full `tst_inboxlayout.qml` unfiltered and an isolated production `InboxView` consumer smoke; retain collapsed/expanded 1356×475 and 412×915 captures showing all three original previews, shared sender/account identity once, retained bodies/order, and differing sender/account plus standalone-neighbor controls. Reuse `tst_maildetail -input qt/tests/tst_inboxlayout.qml -import qt/tests/qml-imports` with `LB_UI_PROOF_DIR` pointing to the owned evidence directory for screenshots. This fixture gate cannot replace native enrollment/Online, real-mail rendering, Settings or update proof. Test/docs-only changes do not rebuild the accepted APK: record the tooling commit separately from frozen artifact source/hash. A desktop Qt wrapper, launch-only or disposable-server-only receipt does not pass
the native gate, and neither does a rebuild: only the enrolled owned-emulator
native proof run on the exact signed candidate counts, never a rebuild or new
build receipt by itself. Use the existing owned-runtime/proof recipes after contract and readiness checks, preserve each run's logs, and stop after at most two full acceptance attempts rather than rerunning a matrix blindly.

Correction retrospective (2026-10-26): “make sure tests are not bound to ephemeral user data, rework so it is not required but tested regardless”. The former gate required a particular live three-message Techship bundle; archived owner messages made a rendering regression depend on mailbox state. The canonical R39/R36 clauses now require a versioned synthetic projection through the existing real QML consumer, with unchanged native and publication gates. No SMTP, archived-mail reopening, production token or copied owner identifiers belong in this fixture lane. `deploy/release.py::proof_gates` binds accepted receipts to cohort source/artifact hashes; it has no sender/message-specific selector and must not be bypassed or given a fabricated aggregate acceptance.

After native and lead acceptance, publish and install the entire cohort through `deploy/release.py` above. The server source/binary must become the accepted cohort identity, not remain the previous release; its startup-bound APK snapshot must match the same accepted cohort. Read back the authenticated same-origin manifest and downloaded bytes, with anonymous protection retained. Normal Android delivery requires authenticated APK publication and an older normally enrolled isolated consumer's actual **Check updates** offer of **Update ready** for the published version, retained without approving installer/unknown-source safety prompts; physical-phone ADB is never a release prerequisite. Optional standalone maintenance with explicit `--install-mobile` remains s…

Install each immutable release at a fixed, service-readable path and restart the server to select it. If any value is absent, the file cannot be read, or its size is empty or exceeds the bound, both endpoints fail closed with `503 Service Unavailable`. The server never accepts a request-provided file path. Keep APKs private; do not place signing keys or credentials in this configuration or repository.

The client offers only a strictly newer `versionCode`, requires the manifest package to match the running app, and downloads only the fixed same-origin APK route. It verifies SHA-256 over the complete downloaded APK before showing Install/Later. Before publishing a release, confirm the APK's badging has the same package ID and versionCode configured above and that its signing certificate matches the installed release lineage. Android refuses same-package upgrades signed by a different certificate. In particular, a custom Qt Android manifest must retain the full application ID (here `org.qtproject.example.litterbox_qt`); copying the default template's shorter `org.qtproject.example` value produces a different app, even if `adb install -r` reports success. Keep the FileProvider authority derived from the full application ID (`${applicationId}.qtprovider`).

Installation is not automatic: the user taps Install, Android presents its package installer, and any Android 8+ “install unknown apps” source permission must be granted by the user in system settings before they explicitly retry. The app never grants that permission or installs silently. Cancelling or denying either system prompt leaves the old app and its data intact. An `adb install -r` build/developer workflow is not evidence that the in-app updater or user-consent path works. Acceptance requires an in-app-triggered, package/signature-compatible upgrade, observed system installer/consent UI without automated approval, then installed package/version and preserved-data readback. For screenshot evidence, first confirm the app activity is resumed and its update control is visible; an immediate post-launch capture can show a transient surface or another app and does not count.

## Daily PostgreSQL backup

Install `deploy/backup/daily-pg-dump.sh` on the application host. Create a root-owned, mode-0600 `/etc/litterbox/backup.env` with:

```sh
DATABASE_URL='postgresql://backup_user:...@db-host/litterbox?sslmode=verify-full'
BACKUP_DIR=/var/backups/litterbox
BACKUP_SECONDARY=backup-user@second-host:/srv/backups/litterbox/
```

The service account needs source-database read/backup privileges and SSH key access to the second host. Pin the second host key in that account's `known_hosts`; do not enable password login or host-key bypass. Enable the daily job with `systemctl enable --now litterbox-backup.timer`. The script writes a custom-format `pg_dump`, removes dumps at least fourteen days old from the local and dedicated remote backup directories, and copies the new dump to the parameterized secondary host. The remote path must be dedicated to Litterbox dumps and use the restricted path characters validated by the script. Protect both backup locations as sensitive data and monitor timer/exit status. Test access and capacity during provisioning; no production copy is configured here.

## Weekly restore verification

Provision a separate disposable PostgreSQL instance/database account for restore tests. Add `RESTORE_TEST_ADMIN_URL` and `RESTORE_TEST_URL` to the same private environment file; the latter MUST end in `/litterbox_restoretest` (optionally followed by URL query parameters). The script drops/recreates only that specifically named database, restores the newest local dump, checks required tables, and drops the disposable database on exit. Never point either URL at production. Enable `litterbox-restore-test.timer` and alert on failure; a successful restore test proves the dump is readable, not that production recovery objectives have been met.

The disposable instance is not interchangeable with an empty cluster. `pg_dump` does not carry cluster-wide roles, so the dump's `GRANT` and `ALTER DEFAULT PRIVILEGES` statements fail unless the roles they name already exist on the restore instance: create the application and backup roles there before enabling the timer. The restore command must also run as a role allowed to replay dump-wide `ALTER DEFAULT PRIVILEGES FOR ROLE ...` statements, i.e. the restore instance's superuser. Both ways of getting this wrong fail the timer with `role ... does not exist` or `permission denied to change default privileges`; neither is a bad backup.

Keep the required-table check as the `DO` block it is. Rewriting it as `CASE WHEN <constant predicate> THEN 1 ELSE 1/0 END` makes the planner fold the whole expression and raise `division by zero` on a perfectly good restore, so the timer can never pass again.

## Reverse SSH tunnel

`deploy/systemd/litterbox-reverse-ssh@.service` is a unit template for scanner-resistant access through a VPS you control. For instance `litterbox-reverse-ssh@prod.service`, provide root-owned `/etc/litterbox/prod-tunnel.env`:

```sh
SSH_USER=tunnel
SSH_HOST=vps.example.net
REMOTE_PORT=18080
LOCAL_PORT=8080
```

Provision the VPS account with key-only auth and a forced reverse-forward policy bound to loopback; firewall its listener and expose it only through an authenticated TLS reverse proxy. Configure that public proxy to limit each client IP to 60 requests per minute: the reverse tunnel hides the original client address from the home server, whose middleware can only apply the same limit to its observed remote address. Install and enable the instance after configuring pinned host keys: `systemctl enable --now litterbox-reverse-ssh@prod.service`. Values and VPS are intentionally placeholders; this repository does not open firewall ports or create remote accounts.
