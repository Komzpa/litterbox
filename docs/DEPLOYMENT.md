# Self-hosted deployment operations

Production PostgreSQL and the VPS/second backup host are external deployment prerequisites; this repository does not contain real endpoints, keys, credentials, or deployment proof. Run server operations only after provisioning dedicated hosts and restricting service accounts.

## Production application service

The application server runs as a dedicated systemd unit `litterbox-server.service` (template in `deploy/systemd/litterbox-server.service`). Install the release-built binary at `/opt/litterbox/server/litterbox` (root-owned, mode 0755, directory mode 0755 so the service account can traverse it) and enable the unit. The unit runs as the least-privileged `litterbox` account and reads the production DSN from `/etc/litterbox/db.env`, which selects the dedicated production cluster through the application role:

```sh
DATABASE_URL='postgresql://litterbox_app:...@127.0.0.1:5435/litterbox?sslmode=prefer'
```

The unit passes no `-dev-tenant-id` and no `LITTERBOX_DEV_TENANT_ID`, so every `/v1` route requires a valid bearer token and unauthenticated requests fail closed with `401`; `GET /healthz` and `GET /v1/version` remain unauthenticated. The server listens on `127.0.0.1:8081` (loopback only), distinct from the development server's `8080`, and never binds a public address. Verify with `systemctl is-active litterbox-server.service`, the installed executable's SHA-256, `GET /healthz` returning `200`, and an unauthenticated `/v1/cards` returning `401`.

## Android update release

The server exposes `GET /v1/android/update` (JSON manifest) and `GET /v1/android/update.apk` (APK bytes). Both routes are behind the existing device-token authentication middleware when the database-backed API is enabled; the client requests the manifest's fixed relative download path on the same configured origin with the same enrolled-device Bearer token. The server snapshots the configured APK at startup (maximum 256 MiB), computes SHA-256 over those exact served bytes, and serves that same snapshot for its lifetime.

Configure the litterbox service environment with all three values. `VERSION` is the signed APK's decimal Android `versionCode`, not a marketing version; `PACKAGE` is its exact application ID:

```sh
LITTERBOX_ANDROID_APK_PATH=/srv/litterbox/releases/litterbox-v4.apk
LITTERBOX_ANDROID_VERSION=4
LITTERBOX_ANDROID_PACKAGE=org.qtproject.example.litterbox_qt
```

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
