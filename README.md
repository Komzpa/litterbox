# Litterbox

A Google Inbox–style single inbox for everything that needs your attention.
The name is cats + letter box.

## What lands in it

- mail, grouped into bundles that can be archived in one go
- reminders set from the phone and visible on every device
- "meeting soon" cards
- results of research done by an AI assistant
- unanswered chats from messengers and dating apps
- system alerts

Create private journal notes in the Qt inbox with **Add card → Private journal
note**. The same composer accepts one note field; notes appear as ordinary
`journal` cards, save offline on the shared storage worker, and sync through the
existing card outbox. The owner's assistant may read the journal, not write it.
Archiving removes the card from the inbox without deleting the journal entry.

The goal is Inbox Zero that is actually reachable.

## Status

The server and Flutter client implement the inbox. Decisions remain numbered in
[REQUIREMENTS.md](REQUIREMENTS.md).

## Server tests: disposable databases only

From `server/`, run the complete Go suite with PostgreSQL integration enabled:

```sh
pg_virtualenv bash -c 'unset DATABASE_URL; createdb litterbox_test_suite && export PGDATABASE=litterbox_test_suite CARD_TEST_POSTGRES=1 BUNDLE_TEST_POSTGRES=1 AGENT_TEST_POSTGRES=1 && go test ./...'
```

`pg_virtualenv` exports `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD` and
`PGDATABASE=postgres`; it does not create a Litterbox test database. The command
creates one in its throw-away cluster and explicitly selects it. Never print
`PGPASSWORD` or use a production `DATABASE_URL` for tests.

Every DDL helper goes through `internal/testdb.Setup`: it refuses a missing
`PGDATABASE`, a resolved database without a nonempty `litterbox_test_` prefix
(including `kom`), or a connected database without that prefix before any DDL. Each test
then owns a unique schema and application role, removed on cleanup. Neither
test-enabling flags nor being inside `pg_virtualenv` bypass the database guard.


### Qt change — 2026-10-13

Callback-driven Qt GETs use high network priority; signal-only cache prefetches
use low priority. Opening an agent chat therefore no longer waits behind the
inbox's mail-body prefetch queue. The regression lists threads, queues body
prefetches, and opens a chat: `ctest --test-dir <qt-build> -R '^api$'
--output-on-failure`.

The Qt shell exposes `a2aStream` to agent pages. It consumes authenticated
`/v1/a2a/events` frames (`event: a2a`, `data: {"seq":N}`) and emits
`changed(seq)`. Pages refetch the thread list on invalidation; chat pages
refetch messages after their own saved cursor, not the SSE sequence. The
`reconnected()` signal fires on each accepted stream, including the first,
so pages can recover changes missed while disconnected. Retries back off
from 1 to 10 seconds; token/server changes cancel the old stream. Bearers
and server error bodies are never logged by the consumer. Focused proof:
`ctest --test-dir <qt-build> -R '^a2astream$' --output-on-failure`.

The Qt agent chat page renders incoming, outgoing and observed a2a messages in
sequence order. Sending posts a UUID `client_msg_id` and text to the thread's
messages endpoint, immediately showing **pending**. A busy recipient is shown
as **recipient_busy / not delivered**; **Retry** keeps the original message ID.
Live a2a events and reconnects reload changes after the page's saved cursor.
The scoped fake-API proof is `qt/tests/tst_agentchat.qml`; run it with
`qmltestrunner -input qt/tests/tst_agentchat.qml` on an owned Xvfb display.

## Disposable a2a integration server

The integration registers the source-token bridge routes alongside the device
API; see [the bridge contract](docs/a2a-bridge.md). For UI review, use only a
fresh `pg_virtualenv` cluster, never the production database. From the repository
root, enter `pg_virtualenv bash`, then run the following **inside that shell**.
Migration filenames intentionally have a gap at 014; the glob applies all
existing migrations through 018 in order. These public fixture credentials are
only for this disposable loopback server.

```sh
unset DATABASE_URL LITTERBOX_DEV_TENANT_ID
for migration in server/db/*.sql; do
    psql -v ON_ERROR_STOP=1 -f "$migration" || exit 1
done
psql -v ON_ERROR_STOP=1 <<'SQL'
ALTER ROLE litterbox_app PASSWORD 'a2a-local-only';
INSERT INTO tenants (id) VALUES ('11111111-1111-4111-8111-111111111111');
INSERT INTO devices (tenant_id,id,token_hash,device_name,platform)
VALUES ('11111111-1111-4111-8111-111111111111',
        '22222222-2222-4222-8222-222222222222',
        sha256(convert_to('a2a-local-device','UTF8')), 'UI review', 'desktop');
INSERT INTO source_tokens (tenant_id,source,token_hash)
VALUES ('11111111-1111-4111-8111-111111111111', 'a2a',
        sha256(convert_to('a2a-local-bridge','UTF8')));
INSERT INTO accounts (tenant_id,id,address,refresh_token)
VALUES ('11111111-1111-4111-8111-111111111111',
        '55555555-5555-4555-8555-555555555555','owner@example.test',''::bytea);
INSERT INTO cards
    (tenant_id,id,source,external_id,account_id,gmail_thread_id,title,subject,summary)
SELECT '11111111-1111-4111-8111-111111111111', gen_random_uuid(),
       CASE n WHEN 1 THEN 'agent' WHEN 2 THEN 'mail' WHEN 3 THEN 'manual'
              WHEN 4 THEN 'journal' WHEN 5 THEN 'reminder' WHEN 6 THEN 'alert'
              ELSE 'mail' END,
       CASE WHEN n IN (2,7) THEN NULL ELSE 'ui-review-' || n END,
       CASE WHEN n IN (2,7) THEN '55555555-5555-4555-8555-555555555555'::uuid END,
       CASE WHEN n IN (2,7) THEN 'ui-review-mail-' || n END,
       'Review the inbox update ' || n, 'Review the inbox update ' || n,
       'Disposable fixture card ' || n || ': existing phone inbox must stay compact.'
FROM generate_series(1,7) n;
INSERT INTO a2a_threads (id,tenant_id,peer_id,display_name,kind,availability)
VALUES ('33333333-3333-4333-8333-333333333333',
        '11111111-1111-4111-8111-111111111111','agent:mau','Mau','token_agent','online'),
       ('44444444-4444-4444-8444-444444444444',
        '11111111-1111-4111-8111-111111111111','omp:review','Review session','omp_session','busy');
INSERT INTO a2a_messages
    (tenant_id,thread_id,event_key,direction,sender,recipient,text,delivery_state,reason_code)
SELECT '11111111-1111-4111-8111-111111111111',
       CASE WHEN n <= 3 THEN '33333333-3333-4333-8333-333333333333'::uuid
            ELSE '44444444-4444-4444-8444-444444444444'::uuid END,
       'ui-review-message-' || n,
       CASE WHEN n IN (2,5) THEN 'in' WHEN n = 6 THEN 'observed' ELSE 'out' END,
       CASE WHEN n = 2 THEN 'agent:mau' WHEN n = 5 THEN 'omp:review'
            WHEN n = 6 THEN 'agent:mau' ELSE 'agent:darafei' END,
       CASE WHEN n IN (2,5) THEN 'agent:darafei' WHEN n <= 3 THEN 'agent:mau'
            ELSE 'omp:review' END,
       CASE n WHEN 1 THEN 'Can you review the integration?'
              WHEN 2 THEN 'Yes. Threads and replies are in the same conversation.'
              WHEN 3 THEN 'Thank you. This longer message exercises line wrapping in the narrow phone viewport without hiding the delivery label.'
              WHEN 4 THEN 'Please inspect the busy-recipient case.'
              WHEN 5 THEN 'The previous reply remains visible while a new request is pending.'
              ELSE 'Observed coordination between agents.' END,
       CASE WHEN n = 4 THEN 'not_delivered' WHEN n = 3 THEN 'pending' ELSE 'replied' END,
       CASE WHEN n = 4 THEN 'recipient_busy' ELSE NULL END
FROM generate_series(1,6) n;
SQL
export DATABASE_URL="postgresql://litterbox_app:a2a-local-only@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable"
export LISTEN_ADDR=127.0.0.1:18080
(cd server && go run ./cmd/litterbox)
```

In a separate terminal, create a fresh isolated client profile (no normal cache
or settings are reused), then launch the desktop build:

```sh
profile=$(mktemp -d /tmp/litterbox-a2a-profile.XXXXXX)
printf '%s\n' '{"server_url":"http://127.0.0.1:18080","token":"a2a-local-device"}' > "$profile/profile.json"
env -u WAYLAND_DISPLAY QT_QPA_PLATFORM=xcb xvfb-run -a \
    /home/kom/litterbox-wt/A2aIntegration-build/litterbox-qt --test-profile "$profile"
```

For scripted PNG capture, use `--capture-script "$profile" SCRIPT.json EMPTY_OUT_DIR`
instead of `--test-profile`; the script supports resize, click and shot steps.
Start the script with `{"eval":"store.online = true; store.refresh()"}` and
`{"wait":3000}`: scripted mode deliberately does not start normal live sync.
Review inbox, Agents and one chat at 412×915 and 1280×800 on the owned Xvfb
display; never use `DISPLAY=:0`. At least four complete mixed cards must fit
the phone screen. `tst_inboxlayout` also counts painted card interiors at 412,
480 and 1280 px and checks that the non-mail overflow sheet is reachable.
Stop the local server with Ctrl-C and exit the `pg_virtualenv` shell to remove
its cluster after review.

## Reading mail on desktop

Click a mail card's text or preview to read the message inside Litterbox.
**Back to inbox** returns to the saved list position; opening never archives
the message. Bodies load asynchronously through `GET /v1/cards/{id}/body` and
remain available in the device's existing offline cache.

**Archive** and the card's **⋮ More actions** are available while reading.
Archive, Done and Snooze use the same optimistic outbox as inbox cards and
return to the saved inbox position immediately, without waiting for sync.
Press **e** to archive the open message; the shortcut is disabled while a
dialog is open, so it does not interrupt note or snooze entry.

The detail page uses Qt rich text, not a JavaScript-capable web browser. The
existing server sanitizer removes scripts, event handlers, active embeds and
tracking pixels; permitted remote images are fetched by the server with its
existing SSRF protections and embedded as data URLs, never loaded directly by
the desktop. Plain-text messages retain line breaks and spacing while long
lines wrap. Only explicitly clicked HTTP(S)/mailto links open externally.

Focused UI regression checks use Xvfb so separate inbox windows can receive
keyboard focus. Configure with `-DLB_TEST_PLATFORM=xcb`, then run with an
isolated home/config directory and a session bus (never the live desktop):

```sh
cmake -S qt -B build/qt -DLB_TEST_PLATFORM=xcb
test_home=$(mktemp -d /tmp/litterbox-qt-test.XXXXXX)
env -u WAYLAND_DISPLAY HOME="$test_home" XDG_CONFIG_HOME="$test_home/config" \
    QT_QPA_PLATFORM=xcb xvfb-run -a dbus-run-session -- \
    ctest --test-dir build/qt -R '^tst_mail(open|detail)$' --output-on-failure
```

Omit `-R` for the complete Qt suite. UI screenshots default to a disposable
directory; set `LB_UI_PROOF_DIR` to retain them (nested directories are created).
The restart-survival fixture never reads normal Litterbox credentials. It and
the service-spawning UpdateWatcher cases explicitly skip when no systemd user
manager is reachable; a private session bus alone does not create that manager.


## Gmail account connection

Configure a Google **web** OAuth client with its exact authorized redirect URI at
`https://YOUR_HOST/v1/gmail/oauth/callback`. Set `DATABASE_URL`,
`LITTERBOX_DEV_TENANT_ID` (until device auth is configured),
`GMAIL_OAUTH_CREDENTIALS` (path to the downloaded web-client JSON),
`GMAIL_OAUTH_CALLBACK_URL` (that URI), and `GMAIL_OAUTH_STATE_SECRET` (at least
32 unpredictable bytes). Apply database migrations including `006_mail_sync.sql`
before startup. Open **Gmail accounts** in the app to request consent; copy the
shown URL to a browser, finish Google's consent screen, then refresh the account
list. The server stores the refresh token for that tenant and polls Gmail at
most five minutes apart; Disconnect revokes the token and removes the account.
Publishing the OAuth app is a separate Google Cloud Console action by its owner.

