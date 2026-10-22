# Release 0.1-50 receipt: real Gmail sender avatars (Gravatar + domain icon)

Date: 2026-10-22. Server only. Desktop stays 0.1-46 (Qt not changed).
Code commit: f28135c9e6ae2b112d4d4e9a87cd24313c81cde5
("gmailsync: Gravatar + domain-icon sender photo fallback").
User choice: Gravatar + sender-domain icon. Gmail photos of non-contacts
are not available to this app (see receipts/gmail-client-avatar-trace.md);
when People/contacts, directory, and BIMI all miss, sync now tries
Gravatar, then the sender-domain icon (favicon pass skips freemail domains).

## 1. Installed package

Raw `dpkg -s litterbox-server` output:

```
Package: litterbox-server
Status: install ok installed
Priority: optional
Section: utils
Maintainer: Litterbox <litterbox@example.com>
Architecture: amd64
Version: 0.1-50
Description: Litterbox server
```

## 2. Binary and deb sha256

```
f725b5a9dbb816edd1b9c327f29a49b9a66a838b460016735d33c82634e3b2ac  /opt/litterbox/server/litterbox
9132370aa7021fbc69234d429c4be360508a17da3c227d532ca3a17fbf02c2ee  /tmp/lb050/litterbox-server_0.1-50_amd64.deb
420a23575b5bd49d546066f03da7afa149a2db1f7d418fd1c4eaa20417c12fe5  /tmp/lb-049/litterbox-server_0.1-49_amd64.deb
```

## 3. Live version endpoint

`curl -s http://127.0.0.1:8081/v1/version` raw output:

```
{"api":1,"min_client_api":1,"server_build":"f28135c9e6ae2b112d4d4e9a87cd24313c81cde5"}
```

server_build matches the code commit f28135c9. (Port 8081 is the
production listener per /etc/systemd/system/litterbox-server.service.)

## 4. Backups and rollback

`sudo ls -la /var/backups/litterbox/` (recent entries only):

```
-rw-------  1 litterbox litterbox 2187476446 кас 22 00:19 litterbox-20261021T201739Z.dump
drwxr-xr-x  2 root      root            4096 кас 22 17:28 release-049-20261022
drwxr-xr-x  2 root      root            4096 кас 22 18:35 release-050-20261022
```

`sudo ls -la /var/backups/litterbox/release-050-20261022/`:

```
-rw-r--r--  1 root root  3853312 кас 22 18:35 litterbox-server_0.1-50_amd64.deb
-rwxr-xr-x  1 root root 13459616 кас 22 18:35 old-server-binary-0.1-49
```

Rollback deb (0.1-49):

```
-rw-r--r-- 1 kom kom 3855160 Oct 22 17:28 /tmp/lb-049/litterbox-server_0.1-49_amd64.deb
sha256 420a23575b5bd49d546066f03da7afa149a2db1f7d418fd1c4eaa20417c12fe5
```

Exact rollback command:

```
sudo dpkg -i /tmp/lb-049/litterbox-server_0.1-49_amd64.deb && sudo systemctl restart litterbox-server && curl -s http://127.0.0.1:8081/v1/version
```

Nightly DB dump: /var/backups/litterbox/litterbox-20261021T201739Z.dump
(2187476446 bytes). Secondary: litterbox-backup@smallcat:/srv/backups/litterbox/.

## 5. Photo counts per account (tenant 3b09aac5-be4b-43d8-abca-f7b1db66f8bc)

Exact count SQL (run as postgres superuser via peer socket; the
litterbox_app role is denied reads on accounts in this DB):

```sql
SELECT left(a.id::text,8), a.address,
 (SELECT count(*) FROM cards c WHERE c.tenant_id=a.tenant_id
   AND c.account_id=a.id AND c.state='open') AS open_cards,
 (SELECT count(DISTINCT c.sender_address) FROM cards c
   WHERE c.tenant_id=a.tenant_id AND c.account_id=a.id AND c.state='open'
   AND EXISTS (SELECT 1 FROM sender_photos s WHERE s.tenant_id=c.tenant_id
    AND s.account_id=c.account_id AND s.sender_address=c.sender_address)
 ) AS senders_with_photo,
 (SELECT count(*) FROM sender_photos s WHERE s.tenant_id=a.tenant_id
   AND s.account_id=a.id) AS photos
FROM accounts a;
```

Raw psql output AFTER 0.1-50 sync:

```
   left   |      address       | open_cards | senders_with_photo | photos
----------+--------------------+------------+--------------------+--------
 4a08d440 | darafei@sunbim.io  |        593 |                 32 |     34
 8ce5c0f7 | darafei@maumap.com |        407 |                  7 |     14
 e09758a2 | komzpa@gmail.com   |        504 |                 19 |     43
(3 rows)
```

BEFORE (from receipts/gmail-client-avatar-trace.md, open cards with photo):

```
4a08d440 darafei@sunbim.io:  27/593
8ce5c0f7 darafei@maumap.com:  1/406
e09758a2 komzpa@gmail.com:    8/502
```

After: 32/593, 7/407, 19/504 distinct open-card senders with a stored
photo (raw totals: 34 + 14 + 43 = 91 rows in sender_photos). Every gain
is a Gravatar or domain-icon fetch; zero placeholders (all rows pass
the octet_length(photo) > 0 CHECK, all decode as PNG/JPEG below).

## 6. Ten sample senders (byte size + sha256 of stored photo)

Exported with `SELECT encode(photo,'base64') ...` per sender_address,
decoded locally, then `stat` + `sha256sum` + `file`:

```
alexei.ivasechko@pvfarm.io 2042 bytes 071fae85b384ba09a8d0fb5b0a83ccea69f9aaf8e46e7a7b655849dfe110e6fe PNG image data, 32 x 32
aliaksandra@sunbim.io 1600 bytes 6f26fd1f23a267b582e147499b512a14bcbf84d83adedd91bb5f2c7b285820ab PNG image data, 128 x 128
analytics-noreply@google.com 2326 bytes 07e659b8dad50a8dcd5cedea1f521c94530286826207524937337fb47f337085 PNG image data, 128 x 128
announce2@myheritage.com 1661 bytes 1c26590480286edb0d0f1ac83f6b0d99884348856cdb3c3afcf333a6ca29faed PNG image data, 48 x 48
announcement@hetzner.com 495 bytes 868bf7a7f62a1ff737c80f7e7bf044139bc9babbafec97af4856065c8cf16818 PNG image data, 128 x 128
businessprofile-noreply@google.com 2326 bytes 07e659b8dad50a8dcd5cedea1f521c94530286826207524937337fb47f337085 PNG image data, 128 x 128
chat-noreply@google.com 2326 bytes 07e659b8dad50a8dcd5cedea1f521c94530286826207524937337fb47f337085 PNG image data, 128 x 128
comments-noreply@docs.google.com 1339 bytes 63efcfe9eeda4cc58965c7587a485886612cbf878ecd8fc3e4dd594db31a67fd PNG image data, 32 x 32
darafei@maumap.com 5186 bytes 1a888c5310c4624d13bee4cec915498c7fea0d621b110229f2edc8cf75806145 JPEG image data, JFIF standard 1.01
darya.yermashkevich@gmail.com 44133 bytes ba6f6651c799758bb7d08c172aeb7fc156b33e7b0aec3fcb0bf1cf7d7a95357e PNG image data, 128 x 128
```

Sources: google.com / docs.google.com / myheritage.com / hetzner.com /
pvfarm.io rows are sender-domain icons (favicon pass); sunbim.io /
maumap.com rows are directory/Gravatar photos; the gmail.com row is a
Gravatar photo (freemail domains skip the favicon pass by design).
`file` confirms every blob is a real PNG/JPEG image, not a placeholder.

## 7. Tests

Fail-before: server/internal/gmailsync/photosync_avatar_test.go and
server/internal/people/avatar.go are new in f28135c9, so before the
change there was no Gravatar/domain-icon test to run (the fallback did
not exist). Pass-after raw output:

```
$ go test ./internal/people/ ./internal/gmailsync/ -run "Avatar|Gravatar|Favicon|Photo" -count=1
ok  	github.com/Komzpa/litterbox/server/internal/people	0.007s
ok  	github.com/Komzpa/litterbox/server/internal/gmailsync	0.011s
```

## 8. Git

- Code: f28135c9e6ae2b112d4d4e9a87cd24313c81cde5 (server only, 5 files).
- No code changed for this receipt. No stash, no rebase in progress
  (pid 613348 not present; no .git/rebase-*).
- This receipt committed separately; push without force.
