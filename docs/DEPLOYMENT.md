# Self-hosted deployment operations

Production PostgreSQL and the VPS/second backup host are external deployment prerequisites; this repository does not contain real endpoints, keys, credentials, or deployment proof. Run server operations only after provisioning dedicated hosts and restricting service accounts.

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

## Reverse SSH tunnel

`deploy/systemd/litterbox-reverse-ssh@.service` is a unit template for scanner-resistant access through a VPS you control. For instance `litterbox-reverse-ssh@prod.service`, provide root-owned `/etc/litterbox/prod-tunnel.env`:

```sh
SSH_USER=tunnel
SSH_HOST=vps.example.net
REMOTE_PORT=18080
LOCAL_PORT=8080
```

Provision the VPS account with key-only auth and a forced reverse-forward policy bound to loopback; firewall its listener and expose it only through an authenticated TLS reverse proxy. Install and enable the instance after configuring pinned host keys: `systemctl enable --now litterbox-reverse-ssh@prod.service`. Values and VPS are intentionally placeholders; this repository does not open firewall ports or create remote accounts.
