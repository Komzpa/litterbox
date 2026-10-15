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
- **R2 Private journal** (decided, owner 2026-09-27; inbox-only flow clarified by the owner 2026-10-03). Journal notes are created and shown in the main inbox flow; no separate journal screen or menu entry. Add card creates an owner-written private journal note in one field, saves it offline like any other card, and shows it as an ordinary inbox card with a journal kind label. Journal cards can be archived like other cards. Litterbox never sends notes anywhere. The
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

- **R7 Archive and spam go to Gmail** (decided, owner 2026-09-27;
  spam added and reconnect failures clarified 2026-10-12). Archiving a mail
  card or a whole bundle archives its threads in their originating Gmail
  accounts. Mark as spam uses the same operation path and thread targets,
  adding SPAM and removing INBOX with the existing gmail.modify scope.
  Bundle actions skip pinned cards. Success leaves cards archived across
  refresh, history polling and full reconciliation and publishes the
  existing card changes to other devices; Gmail failure rejects the
  operation and rolls back local card state. Replaying the same operation
  ID makes no second Gmail call; a new ID may harmlessly repeat the label
  change. Before transmission, cancelling the client's queued operation
  makes no Gmail call. After acknowledgement, Undo inside its advertised window sends the registered `gmail.label_add` INBOX inverse for the same card, account and thread; only Gmail success commits `cards.state=open` with the normal card-change event, while rejection leaves the card archived and surfaces the error. A user can connect several Gmail accounts. An OAuth
  refresh `invalid_grant` rejects the action with HTTP 422 and JSON
  `code=gmail_reconnect_required` plus `account_address`; the existing account
  `status` becomes `reauth_required` and remains listed in Accounts (2026-10-12
  correction). The rejected operation records no receipt and restores every
  affected card. Qt derives expired-access messaging solely from the account
  list (R52). Other errors retain their generic message.
  Oracle: isolated fake-Gmail server tests exercise success, failure, replay,
  bundle members, tenant isolation and both resync paths; isolated invalid-grant
  token endpoint tests cover individual and bundle archive with non-grant
  negative controls. The Qt lane proves enqueue/Undo/flush, menu availability
  and transport/message mapping.
- **R52 Stale Gmail-expired banner clears after reconnect** (decided, owner 2026-10-12). The Inbox expired banner and Accounts Access expired / Reconnect controls derive only from `/v1/gmail/accounts` status `reauth_required`. Status `active`, or an omitted status on an older server, shows no expired warning even after a remembered `gmail_reconnect_required` operation failure. The list reloads on each refresh, returning to the app, and during the OAuth connect flow. Operation results are not stored as reconnect state; successful operations reload the list. Oracle: `UiPolish::test_reconnectLifecycle` at 1440×1000 and 900×1000, including active-after-connect, stale error, missing status, and fresh Inbox/Accounts PNGs.
- **R47 Qt mark as spam** (decided, owner 2026-10-12). Each mail card's More actions menu places **Mark as spam · <account address>** directly after Archive in Gmail, with a named icon, accessible name and a 48px minimum row. The bundle details menu offers the matching spam action for open, unpinned members. The existing durable outbox queues `spam` or `bundle_spam`, removes the selected cards immediately, and shows Undo. Undo before transmission cancels the queued operation and sends no HTTP request; HTTP 422 restores the cards and shows an error. Repeating the same spam click before acknowledgement queues only one operation per card. Oracle: `cardstore` spam tests and `markspam` real-store UI tests, plus open-menu screenshots at 1440×900 and 412×915; all network proofs use disposable local transport, never real Gmail.
- **R8 No reply in Litterbox** (decided, owner 2026-09-27; edited 2026-10-13: the
  composer exception for agent conversations is R54, because the owner asked to
  chat with every agent from inside Litterbox). Litterbox has no composer for
  mail. A mail card opens the thread in the regular Gmail app.
- **R54 Agent conversations in Litterbox** (decided, owner 2026-10-13). Litterbox shows one
  thread per a2a peer (`a2a_threads`, unique per tenant and `peer_id`) and a
  composer in that thread only. Messages (`a2a_messages`) are a tenant-scoped RLS
  projection of the canonical a2a journal keyed by `event_key`; re-ingesting an
  event creates no duplicate, and every insert or delivery-state change advances
  `seq`. Device-composed messages carry `client_msg_id` (uuid, null for journal
  rows), unique per thread, so a retry is idempotent. A commit fires
  `NOTIFY a2a` with only the tenant id. Oracle: `internal/a2a` store tests under
  pg_virtualenv with `CARD_TEST_POSTGRES=1`.
- **R18 Gmail app published unverified** (decided, owner 2026-09-27).
  Litterbox's Google Cloud app is published without Google verification.
  Connecting a Gmail account shows Google's unverified-app warning, and the
  app serves at most 100 users. A connected account keeps working without
  weekly reconnection.
- **R48 Gmail callback and account health** (decided, owner 2026-10-12). The Gmail account list returns id, address and status (`active` or `reauth_required`). Refresh invalid_grant in sync and operations persists `reauth_required`; OAuth callback reconnect restores `active`. Success and error callback pages reuse the homepage/privacy shell and CSS, title Litterbox, escaped account email, and readable reconnect guidance for denied authorization, expired state and exchange failure. Success says “Gmail connected: <email>. You can close this tab and return to Litterbox.” Oracle: Go callback/list/status-transition tests and invalid-grant operation/sync tests, with non-grant errors leaving status active.
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
- **R23 Bundles formed by Litterbox** (decided, owner 2026-09-28; topic isolation clarified 2026-10-12). Litterbox
  forms bundles from the start; using tags or labels like Simplify Gmail's
  bundles is not enough. Litterbox forms bundles from structured thread identity when the source provides one (GitHub: owner/repo from the subject; mailing lists: List-Id), stripping provider boilerplate such as unsubscribe footers; remaining mail is clustered across all accounts by embeddings with a local model on the home server, and the model can create new topics. LinkedIn network-update digests use their subject as the embedding input, not account statistics, recipient details or provider footers; topic matching retains the existing cosine threshold >0.82. Platform, sender and recipient alone never establish a shared topic. The mesh/OpenMVS, Sunbim training and NodeODM/ODM release subjects remain three separate cards with the deployed all-minilm model. Messages in one Gmail thread, including real replies, remain one card; GitHub per-repository routing remains unchanged. If the local model is unavailable, Litterbox retains structured routes, leaves remaining mail standalone and reports the missing model; unrelated threads are never merged into one bundle just because they share a sender. Gmail categories are not bundles. Every
  card has an action "take out of this bundle" for a wrong assignment, and
  the correction is kept: the card stays out, and similar later mail learns
  from it. Importance is detected separately from topic: an important message is not hidden inside a bundle, and a message that needs the human to answer or decide is shown as its own card. Bot and CI traffic on the human's own changes may form an agents-handle-this bundle; subscription-only traffic may form a quiet bundle. Bundles appear in the list
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
- **R10 Offline actions** (decided, owner 2026-09-27; spam added 2026-10-12). Archive, bundle
  archive, spam, bundle spam, snooze, pin, done and writing journal notes work with no network.
  They apply locally at once and sync when the connection returns. Gmail
  changes for archive, spam and snooze (R7, R47, R20), and star changes for pin (R21),
  run then; other source actions run on sync through R12.
  A rejected action never holds back later queued actions; it is shown as failed on its card.
- **R11 Network loss is not an error** (decided, owner 2026-09-27: "must not
  break in the elevator"). Losing the network mid-use never blocks the
  interface or shows an error screen.
- **R29 Live propagation across clients** (decided, owner 2026-09-28;
  spam added 2026-10-12). While online, an action on one client — archive,
  bundle archive, mark as spam, snooze, pin, done, take out of a bundle, or a
  new card arriving — shows on every other open client live, without refresh
  or reopen, within 2 s (owner accepted 2026-09-28).

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
- **R26 Assistant output as cards** (decided, owner 2026-09-27; clarified 2026-10-01 after source QA found an unreachable classifier and lost deliverables). The owner's
  assistant turns each of these into cards through the ingest API of R12:
  research results when the research task's result is ready (the full result
  and its deliverable files readable offline like any card, R9); reminders
  the assistant sets (behaving like reminders the owner sets and appearing on
  every client, R6); and proactive briefs, which today go to Telegram.
  Persisted semantic kind (`source_kind`) travels in card, snapshot, and change
  payloads; Qt displays assistant reminders as reminders even when their token
  belongs to `research`. Token-bound `source`, card identity, RLS, and actions
  remain unchanged; older cards without semantic metadata keep their source label.
  Classification must originate in the actual completion producer, not only
  consumer test metadata. Other assistant output does not become a card.
  Oracle: invoke the native completion tool with research/brief classification,
  observe persisted completion through the notifier and ingest receipt, and
  verify full content and file bytes remain accessible offline with no ordinary
  Telegram duplicate. Preserving Telegram for files while receiver storage is
  unavailable prevents loss but does not satisfy this requirement.
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
  entry point limits the request rate per device (the authenticated token's
  device; per client address only for requests without a valid token); limit
  60 requests per minute per key (owner accepted 2026-09-28; per device since
  2026-10-14, because every client reaches the server from 127.0.0.1).
- **R42 Android server configuration** (decided, owner 2026-10-02; the v6 build always showed "Offline · changes saved on this device" because Android has no LB_SERVER and the app defaulted to its own localhost with no way to change it). The installed Android build reaches its configured server with no environment variables: the server base URL resolves test profile > LB_SERVER env > saved `server_url` > compiled LB_DEFAULT_SERVER_URL default, and the compiled default is never 127.0.0.1. Per R17 the repository default stays empty; each release build passes its real URL with -DLB_DEFAULT_SERVER_URL=<url>. The enrollment screen shows an editable Server field prefilled with the current URL; enrolling saves the server URL and the device token to QSettings and takes the app online without a restart or hand-copied token. No clause above contradicts this. Oracle: the Qt serverurl precedence test plus the enrollment QML test; install the signed release APK with no LB_SERVER set and enroll it against a disposable server.

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

- **R33-V Visual review before every release** (decided, Darafei 2026-10-13:
  «каждый билд надо отсматривать визуально. на телефонном размере в том числе»;
  reason: the phone inbox was unacceptably sparse). Every build, both desktop
  deb and Android APK, must be reviewed visually before release. The lead must
  open screenshots of the exact release candidate at phone size (412x915)
  and at desktop size. Release is blocked if the phone inbox shows fewer than
  4 complete cards per screen, including a mixed mail/non-mail fixture.
  Review must also click a real mail card on the exact candidate, on the
  desktop build and at phone size 412x915, and keep a screenshot of the
  opened mail body (MailDetailPage, no error toast); a screenshot of the
  list alone does not pass review (0.1-24 shipped with a click that ended in
  «Action failed (HTTP 429)» because review looked only at the list). Oracle:
  `tst_cardstore` `bodyPrefetchPacesLargeSnapshotsAndForegroundBypassesQueue`
  plus the opened-mail screenshot. The `-V` suffix distinguishes this release
  gate from the existing R33 app-native capture contract.
  Phone inbox correction: below 640 px, use 8 px gutters and card padding,
  full-width sender → subject → preview text, an inline bundle-count chip,
  and one compact trailing action strip. Sender/account metadata must not
  compete with the subject or preview. Collapsed bundles use the same text
  hierarchy and density as single mail cards; desktop layout stays unchanged.
  Non-mail cards use the same compact metadata and trailing controls; phone
  subjects, non-mail summaries and notes are bounded to two elided lines.
  `tst_inboxlayout` scans rendered card pixels with agent, mail, manual, journal,
  reminder and alert cards at 412x915, 480x915 and 1280x800, and checks action bounds.
  This replaces conflicting phone-only spacing and chip placement in R36.

## Open questions

None.

## Live inbox

- **R30 Inbox timing, ordering, and text** (decided, owner 2026-09-28; replaces any conflicting inbox-feed ordering, time-display, or card-summary wording). `GET /v1/cards?now=<RFC3339 optional>` returns `now`, `later`, and `missed` arrays. `now` contains untimed open todos, current timed slots, and real agent results, ordered current timed slot first, untimed todos in note order, then agent results newest first. `later` contains future timed cards in ascending `at` order; `missed` contains superseded past timed cards in descending order. A timed card's displayed position is anchored to its actual `at`; untimed todos have `at=null` and are never assigned a fabricated time. The display uses the system locale and timezone. Every summary is plain text: it contains no raw JSON or XML, Markdown tables, or citation blocks. Oracle: automated server contract tests assert section membership and ordering at fixed `now`, null times for untimed todos, and rendered clock/date/timezone and summary exclusions for synthetic fixtures.
- **R31 Agent results are substantive** (decided, owner 2026-09-28). An agent card is shown only when it contains a real result; probe or acknowledgement sessions, including `CHANNEL OK`, never produce cards. A card must not repeat its title as its summary. Oracle: automated server contract tests feed synthetic probe/ack and real-result sessions, asserting only substantive results appear and no returned card duplicates title and summary.
- **R32 Card note reaches task generator** (decided, owner 2026-09-28). Every card exposes a per-card note that the owner can write and update; the stored note is delivered to the task generator as instruction/context for that card. Oracle: automated integration test writes and updates a synthetic card note through the card API, then verifies the task-generator input for that card contains the latest note and no other card's note.

- **R38 Manual card creation and order** (decided, owner 2026-09-29; grab-and-drag restated 2026-09-30; named move icon replaces the tiny `=` on 2026-10-03 to match R36 and the owner's restart-layout correction). The Qt desktop/Android inbox exposes an obvious Add card control that creates an open manual card with title and optional details; creation is saved locally and sent through the authenticated idempotent operation API. Each card exposes a visible named move-icon drag handle: the owner presses it and drags the card to the position where it must land, then releases. The drop moves the card to that position; a press with no movement changes nothing. The drag never overrides an order the server decides elsewhere: a pinned card moves only within the pinned block (R21), and a card whose position is anchored to its time (R30) does not move. Order changes persist offline and sync through the same operation API across clients. The server accepts reorder only for the complete current open-card set and updates `note_order`; manually created cards use a tenant-scoped manual identity and appear in the inbox. Oracle: Qt offscreen app capture opens the visible add-card dialog and records a frame of it, then drags the rendered move handle with real pointer events to a target row (both up and down), verifies the rendered new card and the card's changed position, verifies a pinned drag stays inside the pinned block and a time-anchored drag is refused, verifies a press on the handle with no movement leaves both the order and the queue unchanged, verifies the rendered Note and Done controls beside the drag surface still respond to real pointer clicks, and independently confirms local cache/outbox operations; server operation test verifies tenant-scoped create/reorder and rejects unknown/closed IDs.
  Create card and Card note dialogs retain the same explicit light palette under forced Material Dark, with English footer labels. Oracle: run `tst_uirequirements.qml` under forced Material Dark, then run `capture.sh <commit>` and inspect `material-dark-action-sheet.png` and `create-card-material-dark.png` for white dialog chrome, `#263b3a` ink, `#397d73` actions, and explicit English button labels. The R38 pointer/no-motion, pinned-clamp, timed-refusal, and cache/outbox checks remain required; use the capture scenario below for those behavioral proofs.
- **R39 Android self-update** (decided, owner 2026-09-30). Android users can explicitly check for updates from the app. The client uses the configured API origin and enrolled device Bearer token for the manifest and its fixed same-origin APK route; it rejects a package ID that differs from the running app, treats `version` as a decimal Android `versionCode`, and offers only strictly newer releases. It streams the complete APK to a temporary file and verifies SHA-256 against the authenticated manifest before showing Install/Later; invalid metadata, transport failure, or checksum mismatch must not launch an installer or replace the installed app. Install is always an explicit user action followed by Android's installer confirmation; if Android requires permission for this source, the user must grant it in system settings and explicitly retry. The app never grants installer permission or installs silently. Cancelling or denying either platform prompt leaves the old app and its data intact. Oracle: `tst_androidupdater` exercises the real updater with a loopback HTTP server and verifies authenticated fixed-route requests, valid newer-release readiness only after complete-byte digest match, no APK fetch for equal/older versions, and rejection of package/path/checksum mismatches. Device acceptance additionally requires a same-package, signature-compatible upgrade initiated from the in-app UI, observed Android installer/consent UI with no automated approval, then installed package/version readback and app-data preservation; an `adb install` result or toolbar screenshot alone does not satisfy this oracle. Amended 2026-10-15 by R64: the check and verified download now also run automatically; installation stays an explicit tap plus Android's confirmation.
  Release completion for shared Qt fixes requires ONE same-source cohort: server Debian package, ordinary desktop Debian package and signed Android APK, installed/published through canonical `deploy/release.py plan/apply/rollback`, not independent ad-hoc cutovers. Bind every artifact, full source SHA, exact binary hash, signing identity and accepted proof before any install; desktop provenance uses the normal build receipt, not an injected ELF note. Record the release CLI commit/hash separately from the frozen artifact source. Normal Android delivery requires authenticated APK publication and an older normally enrolled isolated client's actual **Check updates** offer of **Update available** for the published version; physical-phone ADB is never a release prerequisite. Preserve old packages/APK and exact environment/unit/executables in a root-private rollback packet, including every manifest-declared absolute path/SHA256-bound service drop-in and every existing environment source with ownership/modes. Read repeated systemd EnvironmentFiles entries independently and require the configured environment by exact path; require the effective DropInPaths set and each file hash to match the manifest before mutation, rejecting unlisted or changed drop-ins. Stage all three, install both Debian packages together, atomically change only the three Android release keys preserving other secrets and environment sources, and restart the canonical server once; recheck complete service composition after apply and rollback, restoring exact saved bytes/ownership/modes on rollback. Completion requires installed package/binary identities, passive desktop adoption without owner GUI actions, live server revision and authenticated same-origin manifest/download hashes plus anonymous protection. Optional standalone maintenance with an explicitly authorized `--install-mobile` selection additionally requires the canonical phone registry/lock, independent transport and hardware identity checks, Litterbox-only `install -r`, and installed package/version/hash readback; no default phone, pairing, debugging setup, unlock, app launch or permission approval. Without that option the phone stays untouched and the receipt must say `not_requested`, not claim all-three installation. Partial readback is failure. Explicit rollback restores and verifies old server/desktop/publication state; it preserves the old phone APK but never automatically downgrades/uninstalls Android or restores app data.
  Android acceptance still requires a signed candidate whose decimal versionCode exceeds served and recorded installed versions, with package/signing lineage, TLS runtimes and QML imports retained. Exact-candidate native normal enrollment without `LB_SERVER` or a pre-enrolled profile must reach Online on the intended deployed route; phone-size inbox/mail/topic review shows four complete collapsed/standalone cards, three expanded Techship previews with shared identity once, retained real mail body and neighboring Settings. An older normally enrolled isolated client must offer **Update available** through actual **Check updates**. Keep scoped enrollment requests and each run's script/log/capture identities. Explicit authorized release installation does not replace the in-app installer-consent oracle above, and an observed offer is not completed consent or upgrade. Disposable real-CLI plan/apply/rollback and forced-failure restoration must be proved, alongside source/hash/signer pre-install rejections, before production use.
## Developer tooling

- **R33 App-native Qt behavior capture** (decided, owner 2026-09-28). The Qt desktop app has an opt-in capture scenario that captures PNGs from its own rendered window and exercises the real QML snooze and pin-order actions against an isolated, pre-seeded fixture database. Capture mode must not access the normal application database or server. Oracle: run `litterbox-qt --capture-scenario <fixture-copy.sqlite> <empty-output-directory>` on the R20/R21 fixture; it exits successfully only after writing baseline, post-snooze, and post-pin-order window captures and independently reading the SQLite cache to verify the snoozed card disappeared, pinned order changed, and exactly the snooze and reorder operations were durably queued. Running with a non-fixture database must reject the scenario without changing it. A screenshot alone is not evidence that either action occurred.
- **R34 Debian package with desktop menu launcher** (decided, owner 2026-09-29). The Qt desktop app builds into a normal `.deb` package that installs via `apt install ./litterbox-qt_*.deb` or `dpkg -i`. After install, the app appears in the system application menu as "Litterbox" under Office/Utility, launches from the menu entry, and its binary is at `/usr/bin/litterbox-qt`. Oracle: build the package with `dpkg-buildpackage -us -uc -b` from the `qt/` directory, install the resulting `.deb`, then verify all of: `dpkg -l litterbox-qt` shows `ii`, `which litterbox-qt` returns `/usr/bin/litterbox-qt`, `desktop-file-validate /usr/share/applications/litterbox-qt.desktop` passes, `test -f /usr/share/icons/hicolor/scalable/apps/litterbox-qt.svg` succeeds, and `gtk-launch litterbox-qt.desktop` starts the app process within 5 seconds. Running `dpkg --verify litterbox-qt` reports no missing or altered files.
- **R35 Desktop inbox scrolling** (decided, owner 2026-09-29; source: "hi precision scroll ... скроллит его в час по чайной ложке; скролл с клавы не работает (home/end/pgup/pgdn/up/dn)"). The desktop inbox list takes focus and supports Home/End, PageUp/PageDown, and Up/Down with useful, bounded movement. High-precision wheel/trackpad pixel deltas move a useful distance rather than crawling; conventional wheel angle deltas also scroll. Oracle: run the Qt app-native capture scenario on an isolated scroll fixture and inject key and pixel/angle wheel events into the real rendered QML inbox; assert the scroll position changes in both directions, End renders the last card at the viewport's bottom edge and Home renders the first card at the top (a bare numeric contentY bound is not sufficient, because a jump on a list with variable-height cards can land on a blank, unrendered region), page/line keys move, and pixel-delta wheel movement is not negligible; inspect the resulting window capture. Inbox Home/End MUST move through the list's own positioner (`positionViewAtBeginning()`/`positionViewAtEnd()`), NOT by assigning `contentY` directly: `contentHeight` is only an estimate until every variable-height delegate has been created once, so a direct assignment can land past the rendered content and leave the viewport blank.
- **R46 Desktop scroll resilience** (decided, owner 2026-10-04; source: "ой оно теперь не скроллится даже"). The inbox, including an expanded bundle with hundreds of members, and the open-mail page always scroll by wheel/touchpad. Expanding or collapsing a bundle never freezes the window: a 300+ member expansion or collapse must finish in <100 ms in the offscreen oracle. Presentation ordering must not use per-item delegate moves or trigger its own regrouping, and must preserve R36's contiguous expanded members and zero hidden-member height. Oracle: the CMake-registered `scrollresilience` test (`tst_scrollresilience.qml`, real CardStore, 308 interleaved members in 1100 cards) checks expansion/collapse timing, source-order preservation, no self-triggered presentation resets, hidden-member height, and wheel scrolling on inbox and open-mail pages.
- **R36 Consistent light UI and touch targets** (decided, owner 2026-10-02; replaces the 2026-09-29 fullscreen-only width rule). The inbox, every page, and every dialog use one centered content column no wider than 1200px with gutters at least 18px. Fonts use Qt/Kirigami theme font roles, never literal point sizes. Actions are visible at rest and are either named icons or labelled buttons—never bare Unicode glyphs—with hit targets at least 48×48px. Every page and dialog pins one explicit light palette (white surfaces, ink `#263b3a`, teal `#397d73`) independently of the host, including under forced Material Dark; dialog buttons have explicit English labels. Oracle: run `tst_uirequirements.qml` via Qt 6 `qmltestrunner` with the real `litterbox` QML import path and forced Material Dark; it checks the real InboxView/CardActions action labels/icons and sizes and the action-sheet content palette. Run `capture.sh <commit>` and inspect `inbox-3840.png`, `inbox-1280.png`, `inbox-560.png`, `accounts-page.png`, `material-dark-action-sheet.png`, and `create-card-material-dark.png` in the capture output for centered gutters, theme typography, visible actions, and light surfaces.
  Bundle-summary correction (owner, 2026-10-03; replaces the sender/count row contract): a bundle is one distinct, focusable summary row with a human topic title or sender-domain fallback (never a raw `sender:` key), an open-member count chip, sender/account context, a collapsed latest-subject preview, and a visible labelled **Archive bundle** action that archives at once, with no confirmation (owner, 2026-10-03: "Сразу + Undo"). Clicking either its chevron or title area, or pressing Enter/Space on the focused summary, expands every ordinary member contiguously inside one enclosing bundle frame at its anchor; clicking again collapses them with zero hidden-row height or leftover spacing. Header title, account context, important/archive disclosure and member subjects share one text column; one continuous left rail spans every expanded member, with thin separators rather than individually bordered member cards. Expanded members appear once, with per-message archive and options (including Take out of bundle), and omit repeated account context unless it differs. Pinned and important cards remain standalone; whole-bundle archive still includes every unpinned card under R22, with important-related scope disclosed. Archive bundle removes those cards at once, queues one archive per email in its originating Gmail account (R7, R10), and shows an inline bar `Archived N emails · Undo` for about 8 s; Undo puts every card back where it was, cancels the archives still queued, and queues the existing Gmail INBOX re-add for any archive already sent. Single-mail Archive from the inbox, keyboard or reader uses the same full-window Undo (owner defect 2026-10-15: Undo hid the bar but left an acknowledged archive archived): it restores local order immediately, cancels an unsent archive without a request, and for a transmitted or acknowledged archive queues the INBOX re-add after it, keeps the card visible across archived snapshots, never lets a late archive ACK hide it again, and shows rejection truthfully without claiming restoration. Spam Undo stays queued-only. Oracle adds `tst_cardstore singleArchiveUndoAcrossAcknowledgement` (queued, in-flight, ACK callback, acknowledged, lost ACK, inverse rejected, expired; archived snapshots; untouched neighbour), server `TestArchiveUndo*` under pg_virtualenv, and `qt/tests/archive_undo_consumer.py` real Qt+server journey. Per-message archive is otherwise unchanged. Presentation grouping never reorders durable cards; status and section headings retain distinct breathing room. Tab focuses the opener, Right expands, and Left/Escape collapse and return focus to the summary. At window widths below 640 px, the bundle opener spans the card's content width, the count badge and archive/options actions sit below the title without overlapping it, and title wrapping occurs only at word boundaries. At a 412×915 px window, the title is at least 60% of the card width in both collapsed and expanded states. The Archive bundle action is icon-only with an accessible name and tooltip `Archive bundle`, retains a 48×48 px hit target, and bundle title and account text are not elided at 520 px or 598 px. At widths of 640 px and above, the existing single-row title/count/actions layout and labelled Archive bundle action are unchanged. Oracle: run `tst_bundleexpand`, including its 412 px title-width and badge non-overlap check, `tst_inboxlayout` and the `tst_cardstore` archive-Undo test (cards gone and ops queued; Undo restores order and cancels queued ops; an already-sent archive queues the INBOX re-add), then `qt/tests/bundlecapture_driver.py` with the real CardStore runner, a read-only copied LinkedIn cache (only its disposable temporary copy is writable), an owned Xvfb, and the installed org.kde.desktop style; each viewport uses a separate process. Inspect fresh collapsed and after-real-xdotool-click expanded captures at 520×900, 598×1200, and 1440×1000; also inspect mixed captures at 1440×1000 and 598×1200, the Undo bar after a real click on Archive bundle and the restored cards after a real click on Undo at 520×900 and 1440×1000, plus recorded xdotool Enter/Space activation. For the narrow-title regression, compare base/candidate X11 root captures at 412×915 and 1440×900, collapsed and expanded.
  Source/title oracle (owner, 2026-10-14): recorded generated seed titles and newly clustered titles use an independently named human topic or sender-domain presentation fallback, never an issue date inherited from the oldest member or latest subject. Persisted origin distinguishes generated seeds/structured topics from explicit or unknown-origin topics; subject equality never establishes provenance. Unknown and explicit topics survive List and the next Cluster pass, including when an archived member shares their wording. Legacy seed repair requires separately evidenced, expected-value-guarded metadata repair, never broad inferred backfill. `TestNewsletterPersistedList` rehearses the private provenance packet against the three immutable Techship cards; `TestPreviewConsumerPreservation` checks topic preservation through real Cluster and List. `tst_bundleexpand.qml::test_senderSummary` checks the actual visible group label, not only the accessible opener, against a distinct canonical topic/domain at 412, 520 and 1356 px, collapsed and expanded; the separate collapsed preview remains unchanged. Membership, thresholds, importance, exclusions and ordering do not change.
  Member-context correction (owner, 2026-10-14; replaces the expanded-member context clause above): expanded ordinary mail members omit sender and account context already represented by the bundle summary's single sender/account identity. Compare the existing bundle model's exact sender/account values, never address substrings. A mixed-sender or mixed-account member retains its own differing identity once; standalone, pinned and important mail retain their existing context. Shared account identity stays on the summary at desktop and compact widths. Subject, preview, date, grid/font/frame/actions and mail-body paths are unchanged. Oracle: `tst_inboxlayout.qml::test_expandedMemberContext` with copied sanitized Techship records, a differing-account/sender negative control and a standalone mail neighbor; inspect collapsed/expanded before/after captures at 1356×475 and 412×915 from an owned Xvfb. The baseline must fail the same-account member-context assertion and the candidate must pass.
  Bundle-frame regression checks (2026-10-04): expanded two- and three-email ordinary bundles paint continuous left and right edges from the summary header to the final ordinary member, with only the first top edge and the last closing edge. No expanded ordinary member paints a standalone rectangle border, including when its controls have focus; control-local focus indicators remain available. An important email presented last paints its own rounded standalone border, separated from the ordinary frame, and must not steal its closing edge. While the archive Undo bar is visible, the first delegate starts immediately after the header plus the normal ListView top margin (18 px), with no additional status-dependent band. The focused `tst_bundleexpand.qml` frame and archive-header geometry cases cover 520×900 and 598×1200; they inspect all painted edge items and standalone border widths rather than only the cached last-member ID or the bottommost closing edge.

- **R49 Desktop menu rhythm** (decided, owner 2026-10-12; replaces R36 menu sizing where contradictory). Inbox commands use content-sized rows at least 48px tall, 6px exterior padding, 12px icon/text spacing, 20px named icons and an inset rounded highlight. Unsupported actions contribute zero height: no empty tail below Refresh. Oracle: `uipolish` geometry assertions at 1440, 900 and 572px; inspect `overflow-1440.png` and `overflow-900.png` from the headless capture below.
- **R50 Desktop card grid polish** (decided, owner 2026-10-12). Mail and bundle archive/overflow roles use 20px icons inside existing 48px minimum hit targets. Desktop date/action columns are 120px, not 230px; text keeps the available column, account context is bounded, previews retain one clean elided plain-text line. Existing centered 1200px maximum cards, 18px gutters and continuous expanded-bundle framing stay intact. Oracle: `uipolish` icon/preview/bounds assertions and before/after inbox and expanded-bundle captures; retain `tst_inboxlayout.qml` and `tst_bundleexpand.qml` regressions.
  Preview/source oracle (owner, 2026-10-14): derive previews at read time from stored MIME alternatives, excluding structural decoration while retaining visible linked prose and meaningful candidates in source order. Preserve the document's distinction between a multi-column presentation-table label band and the body of a following heading-led article cell: the article's prose supplies the preview, not the template labels or article heading. Ordinary introductory text, data tables and unsectioned correspondence remain in source order. Length, CSS classes and promotional vocabulary are not meaning classifiers; existing subject/signature rejection must see the content candidates. Empty or unusable HTML projection must not discard a usable plain alternative. Do not rewrite full text/HTML. `TestNewsletterPersistedList` must pass exact article-paragraph expectations on disposable copies of the three immutable archived Techship inputs (October 14 cellular module prices, October 7 high-speed 5G modules, September 23 Standalone/Non-Standalone 5G), with unchanged full-body SHA-256 values. `TestPreviewConsumerPreservation`, `TestPreviewText`, `TestMailSnippetRules` and production-body controls retain short replies, link-only text, entities, punctuation and ordinary newsletters. Qt remains a plain-text, single-line right-elided consumer; no sender-specific filter, model/network call on List, or UI cleanup is allowed.
- **R51 Accounts and connection chrome** (decided, owner 2026-10-12). Accounts uses a centered column at most 720px wide, a normal content-width Connect account button, 84px account rows separated by 12px, email/status text, and consistent light secondary buttons. A `reconnect_required` status or known failed-account address exposes inline Reconnect; device enrollment has a page heading and 48px minimum fields. Gmail-access errors are warm tinted banners with a visible Reconnect action. This presentation does not prove successful OAuth or clear historical failures by itself. Oracle: accounts/settings/expired-banner screenshots at 1440 and 900px plus accounts/enrollment behavior tests. Server status transport and successful-reconnect acknowledgement remain separate from these visual requirements.
- **R53 Every pushed page has a visible exit** (decided, owner 2026-10-12; source: "пищдец, из этого стейта нет выхода" on the Accounts page, whose only exit was the mouse side button). Accounts, Enroll device, MCP tokens and the mail reader each show a labelled **Back to inbox** button (`go-previous` icon, 48px minimum, left edge of the centered content column, objectName `backToInbox`), and Escape or Alt+Left returns to the inbox from any pushed page; an open menu or dialog takes Escape first. The header Inbox commands menu stays visible and usable on the subpage. Oracle: `uipolish` `test_backExit` at 1440 and 900px (button, Escape, Alt+Left per page, stack depth 2 to 1), plus `accounts-*`/`enroll-*` captures in `<receipt>/back/`. Amended 2026-10-15 by R59 (owner msg 15890 «Кнопки говно»): the mail reader keeps the exit but as the back-arrow icon in its slim app bar (still objectName `backToInbox`, 48px hit target) instead of a labelled button, and the reader hides the inbox commands menu; Escape and Alt+Left are unchanged.

  Headless proof (never the user's display): `cmake -S qt -B build-ui -G Ninja && cmake --build build-ui --target tst_mailresize -j2`; create `<receipt>/before` and `<receipt>/after`, then run `xvfb-run -a env QT_QPA_PLATFORM=xcb QT_QUICK_CONTROLS_STYLE=org.kde.desktop QT_QUICK_BACKEND=software LB_UI_PROOF_DIR=<receipt> build-ui/tests/tst_mailresize -input qt/tests/uipolish.qml -import qt/tests/qml-imports UiPolish::test_after UiPolish::test_geometry`. `UiPolish::test_before` captures the same screens on the baseline checkout. Screens are inbox, overflow, bundle-expanded, mail, accounts, settings, offline, and empty; each is captured at 1440×1000 and 900×1000. The existing native Qt fixture and isolated cache are sufficient; no WebAssembly target is required.

- **R40 Isolated Qt test profile** (decided, owner 2026-09-30). The Qt client accepts an explicit `--test-profile <existing-directory>` startup option whose `profile.json` supplies `server_url` and a non-empty bearer `token`; its SQLite cache and QSettings are confined to that profile directory and never overlap the normal app-data database or normal settings file. Missing/malformed profile input exits before loading QML or opening the normal cache. The profile uses the same application code and UI as normal startup; this is not a fixture or mock mode. Oracle: launch the actual Qt UI with an isolated profile and a disposable local service, verify the process uses that profile's configured URL/token and creates `cards.sqlite`/`settings.ini` only there, verify normal settings and normal cache remain unchanged, then remove only the disposable profile after stopping the process. This startup option does not itself grant Android launch-extra, loopback routing, install, profile, or permission authority.

- **R41 Server Debian package with systemd redeploy** (decided, owner 2026-10-01). The home application server ships as a real `litterbox-server` `.deb` (packaging in `deploy/server/debian`, built from the pinned release binary) that owns `/opt/litterbox/server/litterbox` and the `litterbox-server.service` unit; database credentials and environment in `/etc/litterbox` are never bundled. Production cutover is `dpkg -i` of that package followed by an explicit unit restart, with the previous binary and unit preserved for rollback. Oracle: `dpkg -s litterbox-server` shows the release version, the installed executable's SHA-256 matches the pinned candidate, `systemctl is-active litterbox-server.service` is `active`, `GET /healthz` returns `200`, and unauthenticated `/v1/cards` returns `401`.

- **R44 Desktop self-update on package upgrade** (decided, owner 2026-10-04). When the `litterbox-qt` Debian package replaces `/usr/bin/litterbox-qt` while the app is running, the running app moves itself onto the newly installed binary with no restart by the user: it detects the replacement, waits until the new binary is stable and the package manager holds no dpkg lock, then starts the new binary in its own transient systemd user service with the same arguments and required desktop environment, and quits only after systemd confirms the replacement service started. If systemd is unavailable or cannot start the replacement, the old app stays running. The normal quit drains queued offline actions through `~CardStore` before the replacement opens the same SQLite database, so no queued action is lost; window placement is restored from settings and no open state is lost beyond the current page. The behavior works including when the app runs as a systemd user service (desktop launcher). Watching is armed only for an installed prefix and never for capture or build-tree runs. Oracle: `tst_updatewatcher` covers restart into an atomically replaced binary, the dpkg-lock wait, debounce of flapping replacements, pre-start replacement, a single restart per replacement, and installed-prefix arming; the CMake-registered `restart_service_survival` test launches a desktop-service test copy, atomically replaces its installed-style binary, and asserts that the generation+1 replacement is alive 10 seconds after the original service exits.

## Feed quality

- **R37 Ingest rejects noise and duplicate summaries** (decided, owner 2026-09-29, source: "перебери задачи которые туда попадают - там ща хуйня какая-то"). Ingest rejects summaries that duplicate the title after trimming surrounding whitespace and ignoring case, and summaries classified as acknowledgement/probe or process-status noise by R31's shared agent-result matcher (including `CHANNEL OK`, `still waiting`, `stopped, owned by`, `waiting for`, `still running`, `in progress`, and `working on it`). Empty and whitespace-only summaries remain allowed. Rejected cards are not stored and therefore cannot be returned by the cards endpoint. Oracle: `TestIngestRejectsNoiseCards` verifies HTTP 400 and no stored row for duplicate-title and representative shared-matcher noise, and HTTP 204 plus persistence for a useful summary and empty/whitespace-only summaries; `TestAgentResultNoiseFilters` continues to verify the R31 probe/ack contract.

- **R43 Production feed excludes proof/test cards** (decided, owner 2026-10-02). No proof, test, fixture, or smoke run may create a card in a production feed. Producer tick proofs must use a disposable Litterbox endpoint, tenant, and source token; the proof harness refuses production ingest targets and never loads production source credentials. Oracle: run the native producer tick against a disposable receiver and confirm it creates the expected card only there, then use the production device-token feed readback to verify zero proof/test/fixture/smoke cards while genuine mail and reminder cards remain visible.

- **R45 Open-mail page shows sender and arrival** (decided, owner 2026-10-04, source: the open-mail page showed only the subject and the owner's own address). The open-mail page shows, directly under the subject, the sender as name and address (`Cerebras Systems <welcome@cerebras.net>`) and when the mail arrived in the local zone: time only (`· 11:43`) for mail received today, day and month plus time (`· 4 Oct 11:43`) for mail from this year, and the full date (`· 4 Oct 2026 11:43`) otherwise. A card without an arrival time shows no separator and no time. The card API carries `received_at` (the card's latest `messages.received_at`, RFC3339 UTC, omitted for non-mail cards) and `sender_address`; the Qt card model and SQLite cache carry both. Oracle: `tst_maildetail.qml` loads the open-mail page with a fixed `received_at` and asserts the rendered line for all three date branches plus a no-stray-separator negative control, and `TestCardArrivalAndSenderAddress` asserts both API fields and their omission for non-mail cards.

## Mobile UX (owner reports 2026-10-14 and 2026-10-15)

The owner reported these on the installed Android app (0.1-29 on 2026-10-14,
0.1-34 on 2026-10-15) in Telegram. Quotes are verbatim, typos included.
Screenshots are kept privately on the assistant host; the path and SHA-256
bind later visual comparison to the exact evidence. On 2026-10-14 this set was
drafted only as uncommitted edits on a stale release branch (numbered R54-R60
there, colliding with R54 above) and never shipped. These ids replace that
draft. The shared visual oracle for R55-R69 is a headless phone-width
(549x1280) capture of one fixture before and after the change. The fixture
holds Polis.ge mail with an emoji preheader, a PostGIS bot-review bundle of 2
and of 4, single mails, agent cards with and without a source mail, the offline
state, and the reader on a long mail. Owner screen proof on the shipped APK is
the final acceptance; until he sees it each item is partial.

- **R55 Actionable offline status, silent when online** (decided, Darafei
  2026-10-14 msg 15785 «Можно литтербокск не срать мне ошибками в экран?
  Спасибо», screenshot `img_96882716abd0.jpg` sha256
  `1562577bdf78c1b4765dfcaca1688d071a1a18db328db0ce967faf78e0349ab1`;
  2026-10-15 msg 15880 «Broken layout still, useless errors», screenshot
  `img_bd2367fd722c.jpg` sha256
  `bc8fdfb680153a7a41551c3e90cc1ec9b4aa92fa76d96e455ec8caa8c4b5d134`; msg
  15884 item «Online · changes sync across devices» line wastes a row). The
  inbox never shows a raw socket/errno string such as «Software caused
  connection abort». When online and synced nothing is shown. When the server
  is unreachable, one slim line says what is unreachable, the last successful
  sync time, the number of changes waiting on this device, and offers Retry;
  the app keeps retrying on its own with backoff. Raw error text is only in an
  expandable Details view and in diagnostics. Auth and data-save failures
  stay actionable (R51, R52) and are never hidden behind this line. Oracle:
  QML test drives offline with reason «Software caused connection abort» and
  asserts the rendered text omits it, shows pending count and Retry; online
  renders no status row.
- **R56 Touch scrolling never activates** (decided, Darafei 2026-10-14 msg
  15786 «Еще надо починить скролл: когда происходит скролл тачем, не должно
  быть изменений на экране, все так должно оставаться чтобы скриншот тул от
  телефона мог снять приложение с прокруткой и по пути не происходило
  селектов», screenshot `img_eaa36e28f1ad.jpg`, same bytes as R55's). A touch
  drag or flick that starts on card text or an action icon only scrolls: no
  selection, open, action or pressed residue. Taps and long presses still
  work. Oracle: QML test flicks across a card and its controls and asserts no
  open, action or selection.
- **R57 Compact bottom Undo** (decided, Darafei 2026-10-14 msg 15790 «Вот эта
  штука про ю кен анду тоже очень плохо смотрится, надо целиком переработать
  ее юикс», screenshot `img_86c78dcd3196.jpg` sha256
  `43945a9996ed9705dc8345321841a1760231a74f4bc682c09e09b3f0eb7507b9`;
  2026-10-15 msg 15884 item 10). After Archive or Spam, one line overlays the
  bottom of the list above the system navigation, for example «Archived ·
  Undo» or «Archived 3 · Undo», with no explanatory sentence. It does not
  reflow the list, dismisses itself after a bounded time, and Undo restores
  exactly the indicated action with the semantics already on main (Undo kept
  after server acknowledgement and across stale snapshots). It never carries
  the reconnect or auth banner: those stay a separate persistent banner
  (resolves the 2026-10-14 draft's conflict with R51 and R52). Oracle:
  tst_bundleexpand and markspam Undo cases pass with the bar outside the list
  layout; list y of the first card is unchanged while the bar is shown.
- **R58 Agent results open in full** (decided, Darafei 2026-10-14 msg 15792
  «Сообщения от агентов нельзя раскрыть и что-то с ними сделать, очень плохо»,
  screenshot `img_919e62b26e8b.jpg` sha256
  `9862166bdd34e11eb547b9b1f908c0c5447355351d5105cbb324678b39731222`). Tapping
  an agent card opens its full text in the reader, with links and copy and the
  existing local Archive/Done/Undo actions; no invented replies or sends.
  Back returns to the same scroll position. Oracle: QML test opens a long
  agent card and finds its last paragraph rendered.
- **R59 Reader is one page scroll with a slim icon app bar** (decided,
  Darafei 2026-10-14 msg 15794 «Юай занимает пол экрана, письмо надо читать в
  скролле спичечной коробки, на переработку юикс отправьте», screenshot
  `img_d31a4badb7b2.jpg` sha256
  `e7bae710961723a040fdf712dbd9a7cca543d9035ff0948525d33abb2dbe433c`;
  re-reported 2026-10-15 msg 15890 «Проблема что письмо скроллится в
  половинке экрана все еще не решена. Кнопки говно. Отправь на перепроверку
  все требования от вчера, что они потерялись», screenshot
  `img_ffcd9f3b483f.jpg` sha256
  `fb4234433d55ec82b57b8e3c4be8ee0dacfc1ca01c68f6452e788db087749fec`). The
  mail reader has no inner bordered scroll box: subject, sender line and body
  scroll together as one page. The inbox chrome (Add card, menu, sync line) is
  absent in the reader. On top is one slim app bar that fits one row at 549px:
  a back arrow icon, the truncated title, and icon actions Archive, Open in
  Gmail and overflow, each a 48dp hit target with a 24px icon and no
  labelled pill buttons. This amends R53 for the mail reader only. Oracle: QML
  test at 549x1280 asserts one Flickable owns the body, the toolbar is one row
  under 64px, and the body text starts in the top quarter of the screen.
- **R60 Overflow is actions only and fits the screen** (decided, Darafei
  2026-10-14 msg 15796 «Я нажал многоточие вверху и попал в полностью
  разъебанный юикс», screenshot `img_4974f0d45498.jpg` sha256
  `02b2234fc1fc2ae2d06e03cde529eddaaab17949d4893b2ae2739f37598bcfa4`). The
  overflow menu lists actions only, never mail content, fits the phone
  viewport, and dismisses on outside tap or Back with the position kept.
  Spam is visually distinct. Oracle: QML test at 549x1280 opens overflow and
  asserts it is within the window and contains only action items.
- **R61 Preserve the exact original email body** (decided, Darafei
  2026-10-14 msg 15798 «Тут полностью сломано письмо», screenshot
  `img_c63f7a65bd36.jpg` sha256
  `c506409971e1d8bbcd7ffbd64540caece1b996eee99160da1d8a6006a15efd4e`, mail
  «Шаблон B2B-договора TwójStartup»). Establish the message's identity and
  source bytes from an authorized read-only source before judging rendering;
  never invent a body or substitute the subject. Open until that source is
  traced.
- **R62 The app collects its own crashes** (decided, Darafei 2026-10-14 msg
  15800 «Какой то креш литтербокса произошел. У тебя будет стектрейс? Если
  нет надо допилить коллектор», msg 15803 «Не надейся на адб, прилага должна
  сама»). The Android app records Java crashes, native crash and ANR exit
  information into a bounded private on-device spool with build identity,
  and uploads it on next start through the authenticated app API, with no
  message bodies, addresses or tokens. ADB is never part of the path. Owner:
  crash-collector work (wip commit f583489, to be rebased).
- **R63 Dense inbox cards** (decided, Darafei 2026-10-15 msg 15880 items
  2-6 and msg 15884 «Тут много лишнего на экране и занимает много визуально
  спейса 3оторый сог бы быть занят чем полезным», screenshot
  `img_d7c0bc99e309.jpg` sha256
  `24549ca15f2723e47282a631174e1b3fd6048db0dbe896aebcafc4f03a315ed8`). A
  single mail is a header line (sender, time on the right) plus at most two
  text lines: subject primary, preview one smaller secondary line. No resting
  row of date, move, mail and overflow icons and no blank gap; actions are
  swipe, long-press and one small overflow, and drag reorder starts with a
  long press. The inbox header is one slim app bar (title, compact add icon,
  menu). Bundle heads collapse an empty snippet line and show and sort by the
  newest member time; nested members align with the card padding and do not
  repeat a full action row. Previews drop leading invisible or unrenderable
  characters (zero-width, private-use, variation selectors). Target: at least
  10 single mails per 549x1280 screen at default font scale. This raises the
  R33-V floor for phone builds. Oracle: QML test counts fully visible single
  mail delegates on the fixture at 549x1280 (>=10) and asserts the stripped
  Polis.ge preview.
- **R64 Updater checks and predownloads by itself** (decided, Darafei
  2026-10-15 msg 15882 «Make updater predownload apk and not make me check for
  updates manually. Add animations where makes sense»). The Android app checks
  for updates on start, on resume and periodically while running (at most a
  few times a day, backoff on failure, no checks while offline). A newer build
  downloads in the background to app-private storage, is verified against
  the manifest SHA-256, size, package and strictly newer versionCode, only one
  pending APK is kept and the same version is not downloaded twice. Then one
  non-blocking «Update ready · Install» prompt appears; tapping it opens the
  Android installer directly. This amends R39: the check and download are
  automatic; installation is still the owner's tap plus Android's own
  confirmation, never silent. Oracle: tst_androidupdater covers the automatic
  trigger, predownload, hash mismatch rejection, offline backoff and no
  second download of the same version.
- **R65 Short purposeful animations** (decided, Darafei 2026-10-15 msg
  15882). Bundle expand and collapse, item removal on Archive/Spam/swipe,
  the Undo bar and the offline or update line animate for 150-250 ms. They
  never delay input or move the scroll anchor, and are off when Android's
  remove-animations setting is on.
- **R66 Agent cards sit with their sources** (decided, Darafei 2026-10-15
  msg 15886 «Агентские сообщения не группируются с их источниками и вообще
  организованы странно», screenshot `img_657995c87bc0.jpg` sha256
  `d33466a778007678eb547f592b078fee2e2d4c704ac1621704f2af10b4d4d9b9`). An agent
  card carries source references (mail thread or external issue key) from the
  server. When its source mail is in the inbox, the agent card renders as the
  head of that source group, never as a sibling elsewhere; when the source is
  archived it shows a compact source line. Agent cards without a source go to
  one collapsible Agent digest section instead of being interleaved by time.
  The label names the agent and source, not bare «Agent»; the card shows its
  time and any deadline; boilerplate such as «What happened:» is not the
  preview. Expired deadline items collapse with Undo. Oracle: server test for
  source refs in the cards API, and a QML fixture where agent card and source
  mail render as one group. Status 2026-10-15: the Agent digest bundle and
  boilerplate-free preview are implemented (`TestClusterCollectsAgentCardsIntoDigest`);
  nesting under a source mail stays open because agent producers do not yet
  report a source mail or issue key.
- **R67 Sender avatars** (decided, Darafei 2026-10-15 msg 15887 «Аватарок не
  хватает»). Every sender line (single cards, bundle heads, nested members,
  agent cards) shows a compact avatar (about 28dp) on the first text line
  without adding height. The fallback is a circle with initials in a stable
  per-sender colour; forge, domain and Gravatar images, when added, come only
  through the server and are cached on the device. Remote images inside mail
  bodies stay blocked.
- **R68 Scroll stays put after Archive** (decided, Darafei 2026-10-15 msg
  15893 «После архивации сообщений скролл еуда-то улетает постоянно»).
  Archive, Spam, swipe and Undo keep the visual position: the next card takes
  the removed card's place, also when a server snapshot arrives afterwards.
  Oracle: archive card N in a 200-card list scrolled to the middle, deliver a
  stale and a new snapshot; the first visible card id is unchanged except for
  the removed one.
- **R69 No freeze, no black screen on return** (decided, Darafei 2026-10-15
  msg 15894 «14:49 юай простотзалип и завис», msg 15895 «Переключился
  туда-назад - черный экран»). Archive and snapshot handling never block the
  GUI thread for more than 200 ms, and switching away and back on Android
  shows the content, not a black surface. Oracle: archive storm plus snapshot
  fixture with a frame-time log, and an emulator pause/resume journey.
