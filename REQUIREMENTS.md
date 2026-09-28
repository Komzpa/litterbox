# Litterbox requirements

Normative list of what must hold. When a decision changes, edit its
requirement in place and name the date and the reason; never add a second
line that contradicts an earlier one. Git history keeps the old wording.

Each requirement is **decided** or **open** and names its source.

## Scope

- **R1 One inbox** (decided, owner 2026-09-27). Everything that needs the
  owner's attention lands in one list: mail, reminders, "meeting soon" cards,
  AI research results, unanswered chats from messengers and dating apps,
  system alerts.
- **R2 Private journal** (decided, owner 2026-09-27). The owner can write
  journal notes in Litterbox. Litterbox never sends notes anywhere. The
  owner's AI assistant reads them through Litterbox's own MCP server, with
  its own token that the owner can revoke. That token can read journal notes
  and create cards, and nothing else; after revocation every call with it
  fails. Cards it creates enter through the ingest API of R12. The owner's
  own backup copy of the database (R13) is not sending.
- **R3 Reachable Inbox Zero** (decided, owner 2026-09-27). The open list can
  be emptied in normal use; bundles clear many cards with one action.
- **R4 No daily pages** (decided, from the owner's failed daily-note setup,
  2026-09-27). Nothing depends on a per-day page or a daily run. Skipping
  days of use loses nothing and breaks nothing; the next open shows what
  still needs attention.
- **R5 Fast regardless of history** (decided, from the same setup, which
  grew slow on the phone, 2026-09-27). Opening the app shows the inbox from
  the local copy without waiting for the network; the amount of archived
  history does not slow it down.
- **R6 Qt desktop and Android client** (decided, owner 2026-09-27). The
  main client is a Qt/Kirigami app, shipped for Android first. The Linux
  desktop client uses the same Qt code. Reminders set on the phone show up on
  every client. On the desktop the owner uses an installed Linux app.

## Mail

- **R7 Archive goes to Gmail** (decided, owner 2026-09-27). Archiving a mail
  card or a whole bundle archives those messages in the Gmail account they
  came from. A user can connect several Gmail accounts.
- **R8 No reply in Litterbox** (decided, owner 2026-09-27). Litterbox has no
  composer. A mail card opens the thread in the regular Gmail app.
- **R18 Gmail app published unverified** (decided, owner 2026-09-27).
  Litterbox's Google Cloud app is published without Google verification.
  Connecting a Gmail account shows Google's unverified-app warning, and the
  app serves at most 100 users. A connected account keeps working without
  weekly reconnection.
- **R19 Rich mail readable offline** (decided, owner 2026-09-28). Synced mail
  renders its full HTML and images properly, including when read offline.
  The home server fetches remote images during sync, so mail reads offline.
  Before fetching remote images at sync, the home server removes known
  tracking pixels and links by a maintained list (e.g. MailTrackerBlocker
  patterns, BSD-3; EasyPrivacy email-tracker section) plus a heuristic for
  1x1 and invisible images; removed trackers are never requested, and other
  images are fetched as before.

## Card lifecycle

- **R20 Snooze** (decided, owner 2026-09-27). The owner can snooze a card
  until a chosen date and time. A snoozed mail card leaves the open list at
  once, and its thread leaves the Gmail Inbox. At the chosen time the server
  brings the card back to the open list on every client and moves the thread
  back to the Gmail Inbox, within 1 minute (owner accepted 2026-09-28), with no
  client running. Gmail shows this as archived and later moved to Inbox, not
  as Gmail's own Snoozed. A new incoming message in a snoozed thread brings
  the card back early (owner accepted 2026-09-28).
- **R21 Pin** (decided, owner 2026-09-27). The owner can pin a card. Pinned
  cards stay at the top of the open list (owner accepted 2026-09-28) in the order
  the owner sets, on every client; that order is kept only in Litterbox.
  Pinning a mail card stars its thread in Gmail; unpinning removes the star.
- **R22 Bundle archive skips pinned cards** (decided, owner 2026-09-27).
  Archiving a bundle archives every card in it except pinned ones. Pinned
  cards stay open, and their mail stays in the Gmail Inbox.
- **R23 Bundles formed by Litterbox** (decided, owner 2026-09-28). Litterbox
  forms bundles from the start; using tags or labels like Simplify Gmail's
  bundles is not enough. Litterbox forms bundles by clustering messages from
  all accounts (e.g. by embeddings) with a local model on the home server;
  the model can create new topics. Gmail categories are not bundles. Every
  card has an action "take out of this bundle" for a wrong assignment, and
  the correction is kept: the card stays out, and similar later mail learns
  from it. Importance is detected separately from topic: an important
  message is not hidden inside a bundle. Bundles appear in the list
  immediately; there is no daily or weekly schedule for forming them.
- **R24 Mail cards close on archive elsewhere** (decided, owner 2026-09-27).
  A mail card closes by itself on every client when, in another Gmail client,
  its thread was archived (no message of it is in the Inbox any more), within
  5 minutes (owner accepted 2026-09-28) while the server is online. Closing by
  itself changes nothing in Gmail. Changes that Litterbox made itself do not
  count. A new message arriving later in the Gmail Inbox opens the card again
  (owner accepted 2026-09-28).

## Offline

- **R9 Full local copy** (decided, owner 2026-09-27). The phone keeps every
  open card with its content, not only titles. Reading needs no network.
- **R10 Offline actions** (decided, owner 2026-09-27). Archive, bundle
  archive, snooze, pin, done and writing journal notes work with no network.
  They apply locally at once and sync when the connection returns. Gmail
  changes for archive and snooze (R7, R20), and star changes for pin (R21),
  run then; other source actions run on sync through R12.
- **R11 Network loss is not an error** (decided, owner 2026-09-27: "must not
  break in the elevator"). Losing the network mid-use never blocks the
  interface or shows an error screen.
- **R29 Live propagation across clients** (decided, owner 2026-09-28).
  While online, an action on one client — archive, bundle archive, snooze,
  pin, done, take out of a bundle, or a new card arriving — shows on every
  other open client live, without refresh or reopen, within 2 s (owner
  accepted 2026-09-28).

## Sources

- **R12 Own Gmail connector, one ingest API for the rest** (decided, owner
  2026-09-27). Litterbox talks to Gmail itself, both ways. Every other source
  (phone notifications, messengers, calendar, alerts, AI research) pushes
  cards, and closes them, through one ingest API. When the owner marks such a
  card done and R25 names a source action for it, the same API hands the done
  back to that source's collector, which performs the action. Collectors for
  private sources live outside this repository.
- **R25 Done on non-mail cards** (decided, owner 2026-09-27). Done removes the
  card from the open list on every client. Done never means "answered".
  Home Assistant: done dismisses the card's notification in Home Assistant.
  This does not fix the cause; if an automation creates the notification
  again, it arrives as a new open card. Meeting-soon cards close themselves
  when the meeting's scheduled end time passes. Done before that closes the
  card at once. Neither changes anything in the calendar. Chat cards close
  themselves when the owner replies in that chat, in the source app, after
  the card's message, for sources where the reply is visible (Telegram,
  Slack). Reading alone does not close a card. For Telegram, Slack,
  WhatsApp, Instagram, dating apps, and agent results, done only dismisses
  the card in Litterbox on every client (decided, owner 2026-09-28);
  nothing is sent or marked in the source.
- **R26 Assistant output as cards** (decided, owner 2026-09-27). The owner's
  assistant turns each of these into cards through the ingest API of R12:
  research results when the research task's result is ready (readable offline
  like any card, R9); reminders the assistant sets (behaving like reminders
  the owner sets and appearing on every client, R6); and proactive briefs,
  which today go to Telegram. Other assistant output does not become a card.
- **R27 Telegram after cards** (decided, owner 2026-09-27). Once an assistant
  output arrives as a Litterbox card (R26), the assistant no longer sends it
  to Telegram. Telegram keeps urgent messages and approval requests only.

## Server

- **R13 Self-hosted PostgreSQL** (decided, owner 2026-09-27, source confirmed 2026-09-28). The server runs
  on the owner's home server with PostgreSQL, in its own production cluster
  built from a release version with no development extensions, separate from
  the owner's development databases. The Litterbox database is dumped once a
  day; each dump is kept 14 days, and a restore of the latest dump is tested
  once a week. At most one day of changes can be lost. Every dump is also
  copied, unencrypted, to a second machine the owner runs.
- **R14 Multi-tenant** (decided, owner 2026-09-27). Every stored row belongs
  to one tenant, and no request can read or change another tenant's rows.
  Load sizing: 1, then 2, then about 10 users (family, colleagues).
- **R15 Remote access resists scanners** (decided, owner 2026-09-27). The
  phone reaches the home server from mobile networks over HTTPS through the
  owner's VPS using a reverse SSH tunnel initiated by the home server. No
  inbound port is open at home. Every device has its own token, received by
  entering a one-time invite code; a code works once. Each token can be
  revoked on its own, and a revoked token is refused on its next request. A
  request without a valid token gets HTTP 401 and no card data. The public
  entry point limits the request rate per client address; limit 60 requests
  per minute (owner accepted 2026-09-28).

## Project

- **R16 Personal first** (decided, owner 2026-09-27). Built for one user
  first; cleaned up for others only if it proves useful. The architecture
  still follows R14.
- **R17 Public repository without private data** (decided, owner
  2026-09-27). The code is public. Credentials, personal data and deployment
  details of the owner's machines never enter the repository.
- **R28 First slice: all connected Gmail accounts** (decided, owner
  2026-09-27, source confirmed 2026-09-28). The first thing built is triage of all the owner's Gmail
  accounts (at least three) on the phone, working with no network. Archiving
  a card or a Litterbox-formed bundle archives those messages in their
  originating Gmail accounts once the phone reconnects. Bundles are formed
  by Litterbox; Gmail categories alone are not bundles. The slice counts as
  done only if the owner opens it every day instead of what he uses now.
  In the acceptance scenario, connect all the owner's accounts and send
  `LB1-A`, `LB1-B`, `LB1-C <run-id>` and `LB1-P1..P3 <run-id>` fixtures to
  them, identified by account and Gmail message id. Include a rich HTML mail
  with inline and remote images. After sync, turn on airplane mode, kill and
  reopen the app: the last synced inbox and full bodies, including that
  message's HTML and images, are readable without a spinner or error. Archive
  `LB1-A`, then archive one Litterbox bundle containing `LB1-P1..P3`. Kill and
  reopen: the archived items remain gone locally, while Gmail still shows
  them in Inbox until reconnection. After reconnect, within 60 s (proposed,
  no source), Gmail shows the selected messages archived in their originating
  accounts, not in Trash and not deleted. Turning on airplane mode during
  bundle archive shows no error text, dialog or blocked screen; after
  reconnect the archive completes and the server operation log has exactly
  one archive operation per message. Untouched `LB1-B` stays in both inboxes
  and opens its thread in the Gmail app. `LB1-C`, archived in Gmail web while
  the phone is offline and archived again on the phone, is archived once
  after reconnect, with no error and no return to Inbox. Mail sent while the
  phone is offline appears after reconnect. With 50,000 archived cards
  seeded, an airplane-mode cold start shows the first list frame in under
  1 s (owner accepted 2026-09-28). A request with another tenant's device token
  cannot read any `LB1-*` card belonging to the first tenant.

## Open questions

None.

## Live inbox

- **R30 Inbox timing, ordering, and text** (decided, owner 2026-09-28; replaces any conflicting inbox-feed ordering, time-display, or card-summary wording). `GET /v1/cards?now=<RFC3339 optional>` returns `now`, `later`, and `missed` arrays. `now` contains untimed open todos, current timed slots, and real agent results, ordered current timed slot first, untimed todos in note order, then agent results newest first. `later` contains future timed cards in ascending `at` order; `missed` contains superseded past timed cards in descending order. A timed card's displayed position is anchored to its actual `at`; untimed todos have `at=null` and are never assigned a fabricated time. The display uses the system locale and timezone. Every summary is plain text: it contains no raw JSON or XML, Markdown tables, or citation blocks. Oracle: automated server contract tests assert section membership and ordering at fixed `now`, null times for untimed todos, and rendered clock/date/timezone and summary exclusions for synthetic fixtures.
- **R31 Agent results are substantive** (decided, owner 2026-09-28). An agent card is shown only when it contains a real result; probe or acknowledgement sessions, including `CHANNEL OK`, never produce cards. A card must not repeat its title as its summary. Oracle: automated server contract tests feed synthetic probe/ack and real-result sessions, asserting only substantive results appear and no returned card duplicates title and summary.
- **R32 Card note reaches task generator** (decided, owner 2026-09-28). Every card exposes a per-card note that the owner can write and update; the stored note is delivered to the task generator as instruction/context for that card. Oracle: automated integration test writes and updates a synthetic card note through the card API, then verifies the task-generator input for that card contains the latest note and no other card's note.

## Developer tooling

- **R33 App-native Qt behavior capture** (decided, owner 2026-09-28). The Qt desktop app has an opt-in capture scenario that captures PNGs from its own rendered window and exercises the real QML snooze and pin-order actions against an isolated, pre-seeded fixture database. Capture mode must not access the normal application database or server. Oracle: run `litterbox-qt --capture-scenario <fixture-copy.sqlite> <empty-output-directory>` on the R20/R21 fixture; it exits successfully only after writing baseline, post-snooze, and post-pin-order window captures and independently reading the SQLite cache to verify the snoozed card disappeared, pinned order changed, and exactly the snooze and reorder operations were durably queued. Running with a non-fixture database must reject the scenario without changing it. A screenshot alone is not evidence that either action occurred.
