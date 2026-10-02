#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?set DATABASE_URL for the source PostgreSQL database}"
: "${BACKUP_DIR:?set BACKUP_DIR to the local backup directory}"
: "${BACKUP_SECONDARY:?set BACKUP_SECONDARY as user@host:/absolute/path}"

if [[ ! "$BACKUP_SECONDARY" =~ ^([A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+):(/[A-Za-z0-9_./-]+)$ ]]; then
  echo "BACKUP_SECONDARY must be user@host:/absolute/path (path characters limited to letters, digits, _, ., /, and -)" >&2
  exit 2
fi
remote_host=${BASH_REMATCH[1]#*@}
remote_user=${BASH_REMATCH[1]%%@*}
remote_dir=${BASH_REMATCH[2]}
if [[ "$remote_dir" == *"/../"* || "$remote_dir" == */.. ]]; then echo "BACKUP_SECONDARY path must not contain .." >&2; exit 2; fi

umask 077
mkdir -p "$BACKUP_DIR"
now=$(date -u +%Y%m%dT%H%M%SZ)
file="$BACKUP_DIR/litterbox-$now.dump"
tmp_file="$file.part"
trap 'rm -f "$tmp_file"' EXIT
pg_dump --format=custom --no-owner --file="$tmp_file" "$DATABASE_URL"
mv -- "$tmp_file" "$file"
# Keep fourteen daily dumps on both the local and dedicated remote directories.
find "$BACKUP_DIR" -maxdepth 1 -type f -name 'litterbox-*.dump' -mtime +13 -delete
rsync --archive --protect-args "$file" "$BACKUP_SECONDARY/"
# The whole remote command is one quoted string: passing find's arguments
# unquoted lets the remote shell glob 'litterbox-*.dump' in the login
# directory, and with two or more dumps present find then fails with
# "paths must precede expression" and the cleanup never runs. remote_dir is
# validated above against a charset without quotes, so this is safe.
ssh -o BatchMode=yes -o ServerAliveInterval=30 "$remote_user@$remote_host" "find '$remote_dir' -maxdepth 1 -type f -name 'litterbox-*.dump' -mtime +13 -delete"
