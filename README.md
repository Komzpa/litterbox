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

Focused UI regression checks:

```sh
ctest --test-dir build/qt -R '^tst_mail(open|detail)$' --output-on-failure
```


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

