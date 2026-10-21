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
- **R6 Qt desktop and Android client** (decided, owner 2026-09-27; adaptive layout approved 2026-10-18 to implement the desktop plan without losing mobile). The main client is a Qt/Kirigami app shipped for Android and Linux desktop from the same Qt code, not a prototype. Reminders set on the phone show up on every client. On the desktop the owner uses an installed Linux app. Wide windows show an adjustable inbox-list/real-reader split; narrow windows and phones show one pane using the existing StackView. Selection remains visibly highlighted, list order is unchanged, and opening, closing or resizing never archives. The same reader document moves across the breakpoint without reloading the body. Back/close restores the inbox selection, visual anchor and keyboard focus; external archive closes a removed selected card without making a second Gmail change. Local archive follows R68's external-archive close semantics unchanged: when a local archive removes the selected card the reader presents the next surviving openable card after the selected one, preserving filter, expansion and anchor, skipping hidden headers, or closes to the list with no wrap when none survives; Undo restores order and keeps the reader on that successor. Acceptance: at 1440×1000 and 1280×800 open a middle-list mail, scroll the list independently, drag the divider, resize through the breakpoint and back, close and reopen; observe stable selected id/order and visible-anchor offset within 3 px after layout settles, with no queued operation. Repeat open/Back at 412×915 and 480×915, including Android system Back and 150% platform text scale. The adaptive breakpoint must yield one pane before text/actions lose their minimum usable widths.
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
- **R8 No reply in Litterbox** (decided, owner 2026-09-27; agent composer exception R54 added 2026-10-13; actual reader clarified 2026-10-18). Litterbox has no mail composer. A mail card opens its cached original in Litterbox's reader; the distinct Open in Gmail action opens the regular Gmail app or source URL. Agent conversations alone retain the composer of R54.
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

- **R20 Snooze** (decided, owner 2026-09-27; snooze menu corrected
  2026-10-18). The owner can snooze a card until a chosen date and time.
  A snoozed mail card leaves the open list at once, and its thread leaves the
  Gmail Inbox. At the chosen time the server brings the card back to the open
  list on every client and moves the thread back to the Gmail Inbox, within
  1 minute (owner accepted 2026-09-28), with no client running. Gmail shows
  this as archived and later moved to Inbox, not as Gmail's own Snoozed. A
  new incoming message in a snoozed thread brings the card back early (owner
  accepted 2026-09-28). The snooze target is chosen through a preset menu of
  resolved local samples — one hour ahead, this evening when it is still ahead,
  tomorrow morning, and next Monday morning — plus a custom structured picker
  for an exact date and time. The menu is local, every preset resolves to a
  real date in the device timezone, and cancelling leaves the payload unchanged
  (a cancel is a no-op). DST transitions and future dates use the resolved
  local time rather than a guessed offset. On desktop and native Android the
  picker row is at least 48dp tall with actual contrast and is keyboard
  reachable. The custom Year, Month and Day values remain visible beside their
  step controls at phone widths and large text, rather than being squeezed to
  zero content width. Oracle: actual preset menu and custom picker captures at
  1440×1000, 412×915 and neighboring 480×915 (including native Android at
  150% font scale and its actual density), plus cancel/no-op, DST-forward
  and weekend-rollover cases. A visible/open accessibility node is not render
  proof: the menu must paint with nonzero width inside the viewport, and a
  physical preset/OK tap must persist the displayed instant in the payload.
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
  Phone inbox correction (updated 2026-10-21 from the owner's phone screenshot): below 640px of list-pane width, use 8px gutters and compact padding, subject → sender/account → preview hierarchy, a compact bundle-count chip, and a compact trailing action column. Sender/account metadata must not compete with subject/preview. Bundle and single-mail subjects share the available text column, with the actual time on the subject line and archive/options beside the content, never on a separate bottom row (R36). Collapsed bundles use the same hierarchy as single mail cards. Desktop uses R6's adaptive panes rather than one full-window column. Phone subjects, non-mail summaries and notes are bounded to two elided lines at default scale; expanded mail children may use three lines. Accessibility-scale text reflows rather than clipping controls.
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
- **R36 Consistent light UI and touch targets** (decided, owner 2026-10-02; updated 2026-10-18 because the approved adaptive desktop layout replaces the single-column rule). Desktop has useful navigation plus an adjustable list/reader split when space permits (R6); each reading/content pane has a comfortable maximum text width, not a shared 1200px cap over the entire desktop shell. Narrow/phone views use one pane, 8px gutters below 640px of pane width, and no forced desktop sidebar. Other standalone pages retain centered columns up to 1200px and desktop gutters at least 18px; Accounts retains R51's 720px cap. Fonts use Qt/Kirigami theme roles rather than fixed point sizes. Actions are visible at rest as named icons or labelled buttons, never bare Unicode glyphs, with 48×48px minimum hit targets on phone; accessibility text scale must reflow labels and menus without clipping or overlap. Every page/dialog retains the explicit light palette (white, ink `#263b3a`, teal `#397d73`) even under forced Material Dark; dialog footer labels remain explicit English. Oracle: existing UI requirements and palette captures remain required, plus R70's desktop/mobile acceptance journeys. Pane widths rather than full-window width determine compact row geometry.
  Bundle-summary correction (owner 2026-10-03; updated 2026-10-21 to suppress one-message count noise): a bundle is one distinct focusable summary row with the title selected by the Source/title oracle below (never a raw `sender:` key), an open-member count chip only when the count is 2 or more, sender/account context, collapsed latest-subject preview, and visible Archive bundle action. Archive is immediate with no confirmation (“Сразу + Undo”), separate from expand/open. Clicking the chevron/title or Enter/Space expands ordinary members contiguously inside one enclosing frame at its anchor; clicking again collapses with zero hidden-row height/spacing. Header title, account context, important/archive disclosure and member subjects share a text column; continuous rails span expanded members with thin separators, never separate member-card rectangles. Members appear once, with per-message archive/options including Take out of bundle; shared identity is omitted as specified below. Pinned and important cards remain standalone; whole-bundle archive includes every unpinned card under R22, with important-related scope disclosed.
  Archive bundle removes affected cards at once, queues the existing originating-account Gmail actions (R7/R10), and shows a compact bottom-overlay `Archived N emails · Undo` for about 8s, not an inline layout row. Undo restores order, cancels queued archives and queues the existing INBOX re-add for already-sent archives. Single-mail Archive from inbox, keyboard or reader uses the same supported Undo: restore local order immediately, cancel unsent changes without a request, keep the card visible across stale archived snapshots and late ACK, and show inverse rejection truthfully. Spam Undo stays queued-only. Existing `tst_cardstore singleArchiveUndoAcrossAcknowledgement` (queued, in-flight, acknowledged/lost ACK, inverse rejected/expired, stale snapshots and untouched neighbor), server `TestArchiveUndo*` and `archive_undo_consumer.py` remain required. Presentation grouping never reorders durable cards; section/status landmarks keep distinct breathing room. Tab focuses the opener, Right expands, Left/Escape collapse and return focus to summary.
  Geometry follows list-pane width, not window width (updated 2026-10-21 because stacked bundle actions wasted phone space): below 640px the bundle and single-mail subject columns align, with a shared 96px trailing date/action column. A count chip appears only for 2+ messages and stays below the title without overlap; a one-message thread reserves no chip row. Title wraps at word boundaries and fills the space between avatar and actions (at least card width minus 176px at 412×915), collapsed and expanded. Compact Archive bundle is icon-only with accessible name and hover/focus tooltip, >=48×48px target. Wider panes retain their title/count/actions layout, with the same 2+ count threshold. Existing narrow-title, badge overlap, bundle frame and Undo tests remain required with pane-relative geometry. Retain actual collapsed/expanded captures at 520×900, 598×1200, 1440×1000, and base/candidate 412×915/1440×900 comparisons, including real click Archive/Undo and Enter/Space activation, using the real CardStore, a disposable copy of authorized read-only source fixtures and an owned Xvfb. No production cache is writable.
  Source/title oracle (owner 2026-10-14; updated 2026-10-18 for meaningful sender-bundle presentation): explicit or unknown-origin topic titles remain verbatim and survive List and Cluster, including when archived members share their wording. Persisted provenance still distinguishes generated seeds/structured topics; subject equality never establishes provenance, and legacy repair requires separately evidenced expected-value guards, never broad inferred backfill. For an existing raw `sender:` bundle only, Qt uses the newest original member's subject as the primary title, then the original card title, then real sender/domain; it never displays the raw key or a synthetic summary in place of the original. The newest subject is selected alongside latestAt from existing source entries without regrouping or changing durable order. `BundlePresentation.title(card, summary)` is the single shared presentation rule. `TestNewsletterPersistedList` and `TestPreviewConsumerPreservation` retain producer/provenance coverage. The visible-title oracle at 412, 520 and 1356px, collapsed and expanded, must distinguish explicit topic from sender-bundle latest subject and include missing-title fallback plus a later original subject arriving in the same bundle. Membership, thresholds, importance, exclusions, account/sender context and ordering remain unchanged. GitLab pipeline notifications form their own bundles on meaningful project/ref identity — bundle key `gitlab:<project>/<ref>` with a readable `<project> · pipeline <status> · <ref>` title (for example `evince · pipeline failed · komzpa/evince-unlink-compressed-temp-upstream`) — and never a bare sender-domain bundle: unrelated projects, refs and accounts stay separate, including identical subjects across projects and across tenants. Oracle: `server/internal/bundles/cluster_gitlab_test.go` (`TestClusterGitLabPipelineSplitWithoutEmbedder`, disposable pg_virtualenv: split keys/titles, same-ref membership, tenant separation). Known gap under review: the worker route key `gitlab:project/ref` does not distinguish source host and account, and the oracle proves tenant separation only; route-key refinement is pending review.
  Member-context correction (owner 2026-10-14; identity rule retained under the 2026-10-18 adaptive hierarchy): expanded ordinary members omit sender/account context already represented by the summary's single identity. Compare exact existing sender/account values, never address substrings. Mixed-sender/account members retain their differing identity once; standalone, pinned and important mail retain context. Shared account stays on the summary at desktop and compact widths. Original subject/preview/date sources, theme fonts, continuous frame and action semantics remain intact while R50 changes presentation hierarchy/spacing. Oracle: `tst_inboxlayout.qml::test_expandedMemberContext` with copied sanitized Techship records, differing sender/account negative controls and a standalone neighbor; retain collapsed/expanded captures at 1356×475 and 412×915 from an owned Xvfb and baseline/candidate assertion evidence.
  Bundle-frame regression checks (2026-10-04; pane-relative margin clarified 2026-10-18): expanded two-/three-email ordinary bundles paint continuous side edges from summary to final ordinary member, with only the first top edge and final closing edge. No ordinary expanded member paints a standalone border even with controls focused; control-local focus remains visible. An important email presented last has its own separated rounded border and cannot steal the ordinary frame's closing edge. Showing Undo leaves the first delegate immediately after the header plus the NORMAL ListView top margin (18px comfortable desktop, 8px compact pane), with no status-dependent band. Existing `tst_bundleexpand.qml` frame/archive-header cases at 520×900 and 598×1200 inspect every painted edge and standalone border width, not just last-member id or bottom edge.

- **R49 Desktop menu rhythm** (decided, owner 2026-10-12; replaces R36 menu sizing where contradictory). Inbox commands use content-sized rows at least 48px tall, 6px exterior padding, 12px icon/text spacing, 20px named icons and an inset rounded highlight. Unsupported actions contribute zero height: no empty tail below Refresh. Density indicators occupy their own trailing space and never overlap option labels at narrow viewport widths. Empty-inbox content stays below popup paint and pointer-input layers: at 323×429 its envelope must not cover menu rows, and a real Settings-row click must close the menu and navigate to Settings. Oracle: `uipolish` geometry assertions at 1440, 900 and 572px; inspect `overflow-1440.png` and `overflow-900.png` from the headless capture below, plus checkable-row label/indicator bounds and `InboxLayout::test_emptyInboxMenuSettingsUsesPointerHitTest` at 323×429. Retain an empty-profile pointer-away menu screenshot and actual XTEST Settings click, nonempty-profile real keyboard search/filter/clear at 250×430, and neighboring 480px drawer/1280px desktop captures. Popup anchoring: the bundle-details Menu must open adjacent to its triggering dots in common window coordinates, remain fully viewport-visible in split reader, scrolled bundle, narrow and edge cases, and keep its painted actions pointer-reachable; no window-coordinate arithmetic against a trigger-local parent (the menu is parented to its trigger and positioned in trigger-local coordinates). Oracle: `tst_inboxlayout.qml::test_bundleMenuAnchorsToItsTrigger` (split 1440×900 scrolled bundle, narrow 412×915 viewport edge, retired-formula negative control, header-menu control) plus the isolated-Xvfb real-click surface proof in `receipts/bundle-popup-20261021` (popup box adjacent to its trigger in root-mapped space and inside the viewport; failing before, passing after).
- **R50 Desktop card grid polish** (decided, owner 2026-10-12; updated 2026-10-18 for subject-first rows and density in adaptive panes). Mail/bundle archive and overflow retain 20px icons and touch-safe targets. Row hierarchy is subject first, secondary sender/account and one clean plain-text preview; an explicit topic remains the bundle subject and sender-only bundles use R36's original-subject title. Comfortable and Compact density choices change spacing, not content/order or action semantics. Date/source landmarks make scan position and origin visible without a per-card wall of repeated metadata and never fabricate a time (R30). Date/action geometry responds to the list pane rather than reserving a fixed 120px column in a narrow desktop split. Expanded bundles retain continuous framing, shared context once and standalone pinned/important cards. Oracle: inspect both densities at 1440×1000, 1280×800, 412×915 and 480×915 with mixed mail, agent, manual, journal and reminder cards; assert no subject/action overlap, distinct date/source landmarks, >=4 complete mixed cards at 412×915 at default scale, and retained R63 single-mail density. Existing icon, preview, bounds and bundle-frame regressions remain required with adaptive geometry expectations.
  Android inbox polish clauses (decided, owner screenshot «сделай красиво», 2026-10-21; tightened after the 549px phone review to remove one-message chip noise and permit no unexplained thread height). Oracle for each: `/usr/lib/qt6/bin/qmltestrunner -input qt/tests/tst_androidinboxpolish.qml -import qt/tests/qml-imports`, `AndroidInboxPolish::test_capture` and `test_headerOrder`, at 549×1280 and neighboring 480×1280 in mobile form factor; retain and open before/after native PNGs on owned Xvfb with the sanitized screenshot-shaped today/yesterday/18-Oct fixture. This is a sanitized reproduction, not a production inbox capture.
  1. Ordinary mail in each source-owned section is newest first, even across intervening non-mail slots, with each date landmark Today, Yesterday, Earlier occurring once in that order. Bundles move as a unit using their newest actual arrival; expanded children never introduce their own landmark. Pinned, timed and non-mail priority slots stay source-owned. Oracle: exact ordered `dateStarts` values, no child landmark, and an intervening non-mail slot that keeps its index.
  2. Bundle summary time shares its title line; archive and overflow occupy the same trailing column and order as single-mail actions. No empty bottom action row. A count chip is visible only for 2+ messages; a one-message thread reserves no chip row. Oracle: title/date y difference <=6px and equal archive x; matched one-message thread and single-mail card bounding-box heights differ by <=2px. A collapsed 2+ message thread may be taller only by its measured count-chip height plus row spacing (<=6px including rounding), not by an extra action row.
  3. Expanded children read as compact sub-rows: their subjects wrap rather than clipping, their text starts at the parent/single-mail text column without a second indent, and they have 4px top/bottom padding, zero individual border and zero corner radius. Their enclosing left rail remains continuous at one x coordinate; shared sender/account stays on the summary once. Oracle: child height <= matched single-mail height, exact padding/border/radius and text-x assertions, untruncated long child fixture, aligned head/child rails, existing member-context tests and inspected PNGs.
  4. Narrow headers have exactly one navigation opener; Inbox commands uses a visually distinct overflow-menu icon, not a second hamburger. Oracle: navigation visibility at phone/desktop widths and exact commands icon identity.
  5. Single-mail and bundle subjects, arrival times and actions share stable column edges; date text has the same trailing inset as the visible action icon, and bundle/single avatars share one leading x. Archive glyphs have equal width/height. All controls remain inside cards with >=48×48px targets. Oracle: exact geometry assertions (no +1px allowance) for date inset, avatar x and icon dimensions at phone and desktop sizes, plus existing title/date/archive alignment and bounds assertions.
  Preview/source oracle (owner, 2026-10-14): derive previews at read time from stored MIME alternatives, excluding structural decoration while retaining visible linked prose and meaningful candidates in source order. Preserve the document's distinction between a multi-column presentation-table label band and the body of a following heading-led article cell: the article's prose supplies the preview, not the template labels or article heading. Ordinary introductory text, data tables and unsectioned correspondence remain in source order. Length, CSS classes and promotional vocabulary are not meaning classifiers; existing subject/signature rejection must see the content candidates. Empty or unusable HTML projection must not discard a usable plain alternative. Do not rewrite full text/HTML. `TestNewsletterPersistedList` must pass exact article-paragraph expectations on disposable copies of the three immutable archived Techship inputs (October 14 cellular module prices, October 7 high-speed 5G modules, September 23 Standalone/Non-Standalone 5G), with unchanged full-body SHA-256 values. `TestPreviewConsumerPreservation`, `TestPreviewText`, `TestMailSnippetRules` and production-body controls retain short replies, link-only text, entities, punctuation and ordinary newsletters. Qt remains a plain-text, single-line right-elided consumer; no sender-specific filter, model/network call on List, or UI cleanup is allowed. In an expanded bundle a substantive human review comment stays visibly distinct from summary-scaffold rows through the same structural selection (presentation-table band excluded, heading-led prose kept); no bot-identity, length or vocabulary classifier is allowed and `snippet.go` stays canonical. Oracle: `receipts/comment-preview-screen-20261021.md` on-screen expanded-bundle proof (1440x900 and 412x915 positive screenshots show the substantive row distinct, negative plain-text-precedence control shows identical rows) plus `receipts/comment-essence-20261021.md` structural distinction (`presentationTable()` vs heading-led prose, source order kept, plain-alternative precedence kept).
- **R51 Accounts and connection chrome** (decided, owner 2026-10-12). Accounts uses a centered column at most 720px wide, a normal content-width Connect account button, 84px account rows separated by 12px, email/status text, and consistent light secondary buttons. A `reconnect_required` status or known failed-account address exposes inline Reconnect; device enrollment has a page heading and 48px minimum fields. Gmail-access errors are warm tinted banners with a visible Reconnect action. This presentation does not prove successful OAuth or clear historical failures by itself. Oracle: accounts/settings/expired-banner screenshots at 1440 and 900px plus accounts/enrollment behavior tests. Server status transport and successful-reconnect acknowledgement remain separate from these visual requirements.
- **R53 Every page and embedded reader has a visible exit** (decided, owner 2026-10-12; updated 2026-10-18 for actual embedded reader). Accounts, Enroll device and MCP tokens keep the labelled Back to inbox button (`go-previous`, 48px minimum, objectName `backToInbox`) and existing header commands. A standalone narrow reader uses the R59 back arrow, hides inbox chrome, and pops its own StackView. An embedded desktop reader exits through `requestClose`, never by popping an unrelated page; `embeddedMode` defaults off for standalone callers. Back/Escape/Alt+Left return selection, visual anchor and focus to the inbox; open menus/dialogs consume Escape/Back first. Closing the reader and externally archived-card disappearance do not archive. Existing `requestNote`, `requestArchiveUndo` and Open in Gmail callbacks remain wired. Oracle: actual Back button, Escape, Alt+Left and Android Back journeys at 1440×1000, 900×1000 and 412×915, including breakpoint changes while open and external archive of the selected card; neighboring Accounts/Settings still exit normally.

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
  existing local Archive/Done actions and only actually supported Undo; no invented replies, sends or non-mail post-transmission inverse. This 2026-10-18 clarification forbids an Undo promise when the existing store cannot restore the action.
  Back returns to the same scroll position. Oracle: QML test opens a long
  agent card and finds its last paragraph rendered.
- **R59 Reader is one page scroll with an adaptive slim app bar** (decided,
  Darafei 2026-10-14 msg 15794 «Юай занимает пол экрана, письмо надо читать в
  скролле спичечной коробки, на переработку юикс отправьте», screenshot
  `img_d31a4badb7b2.jpg` sha256
  `e7bae710961723a040fdf712dbd9a7cca543d9035ff0948525d33abb2dbe433c`;
  re-reported 2026-10-15 msg 15890 «Проблема что письмо скроллится в
  половинке экрана все еще не решена. Кнопки говно. Отправь на перепроверку
  все требования от вчера, что они потерялись», screenshot
  `img_ffcd9f3b483f.jpg` sha256
  `fb4234433d55ec82b57b8e3c4be8ee0dacfc1ca01c68f6452e788db087749fec`). The
  mail reader has no inner bordered scroll box: subject, sender line, original body and expanded earlier messages scroll together in one document. On phone/narrow one-pane views, inbox Add card/menu/sync chrome is absent and the reader has one slim app bar under 64px: back arrow, truncated title, Archive, Open in Gmail and overflow as named icons with accessible names, hover/focus tooltips and 48dp targets. At comfortable desktop reader widths the primary Archive/Open in Gmail actions use discoverable labels, with overflow retaining secondary actions; embedded reader chrome must not duplicate the desktop inbox navigation. This updates R53 and the old all-width icon-only rule (approved desktop/mobile plan 2026-10-18). Oracle: inspect the actual reader at 1440×1000, 1280×800, 549×1280 and 412×915, including 150% text scale. Metadata and body share one scroll, the phone toolbar is one row under 64px at default scale, body starts in the top quarter, and long code/links/original formatting remain selectable and readable without half-screen nested scroll chrome. Newsletter reading has exactly one vertical viewport — the document's own — with its scrollbar left visible. Sender data tables keep their column relationships and scroll horizontally in a local port when wider than the column, but a wide table never becomes a vertical scrollport: the offline document pins message-table overflow-y so no nested vertical scrollbar or wheel trap exists inside the message. Oracle: `tst_maildetail.qml` `test_newsletterSingleVerticalViewport` (newsletter-lwn/claude at 598): zero nested vertical scrollports on both fixtures, no message table computes a user-scrollable `overflow-y`, the wide Claude table (700/551) still scrolls horizontally (scrollLeft round-trip), document scrollbar not hidden, wheel scrolls and reverses the document.
- **R60 Overflow is actions only and fits the screen** (decided, Darafei
  2026-10-14 msg 15796 «Я нажал многоточие вверху и попал в полностью
  разъебанный юикс», screenshot `img_4974f0d45498.jpg` sha256
  `02b2234fc1fc2ae2d06e03cde529eddaaab17949d4893b2ae2739f37598bcfa4`). The
  overflow menu lists actions only, never mail content, fits the phone viewport at default and 150% text scale, and scrolls within its bounded height when necessary. Labels wrap rather than clipping; touch targets remain at least 48px. Outside tap or Back dismisses the menu with reader/list position kept. Spam is visually distinct from Archive and task Done. Oracle: actual open overflow at 549×1280 and 412×915 at default and 150% scale stays inside the window, contains actions only and returns to the same content position.
- **R61 Preserve the exact original email body** (decided, Darafei
  2026-10-14 msg 15798 «Тут полностью сломано письмо», screenshot
  `img_c63f7a65bd36.jpg` sha256
  `c506409971e1d8bbcd7ffbd64540caece1b996eee99160da1d8a6006a15efd4e`, mail
  «Шаблон B2B-договора TwójStartup»). Establish identity and source bytes from authorized read-only originals before judging rendering; never invent a body or substitute a subject/summary. The approved 2026-10-18 reader plan requires real earlier-message disclosure: the mail-body response's `messages` and `cachedMailBody.messages` carry originals oldest-to-newest as `{id,gmail_message_id,sender_name,sender_address,received_at,html}`, with existing `html`/`source_url` retained. Newest original is expanded; each older original has a compact sender/date header that expands its full HTML inside the SAME document scroll. Sender attribution comes from each message's actual MIME From, not the card sender; legacy unknown senders are truthfully labelled Earlier message. Missing real history keeps the cached original HTML readable, never fake history, generated summary or quoted-text regex reconstruction. Oracle: a real multi-message thread with differing senders/dates retains exact originals/links/code in online and offline reopen, expands every older original, and has no duplicate newest body; include a legacy unknown-sender original and a single-message control at desktop and phone sizes. Source tracing of the reported B2B mail remains required until evidenced. Plain-text-only mail threads render the combined body in the reader: when thread `messages[].html` slots are empty the reader shows the combined `body.html`; otherwise the latest non-empty message html wins and an empty trailing slot never blanks the document. Oracle: `qt/tests/tst_maildetail.qml` `test_scmPlainTextThreadFallsBackToCombinedBody` plus `test_threadPrefersLastNonEmptyMessage`, with `test_originalThreadUsesNativeDisclosureInsideSingleDocumentScroll` as the neighboring control.
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
  single mail uses subject as the primary first text line, then compact sender/time metadata and one smaller preview; sender avatars remain compact without consuming an extra line (R67). Phone actions are distinct open, bundle expand, Archive and overflow roles with 48dp touch targets, never a tiny ambiguous glyph that archives on opening. Long press remains a secondary path, not the only discoverable action; drag reorder starts with long press on touch. One slim inbox app bar and phone navigation drawer preserve space. Bundle heads omit an empty snippet line and show newest member time without altering R30/server model order; nested members align with card padding and omit repeated identity/action clutter. Previews drop leading invisible/unrenderable characters as before. This in-place 2026-10-18 amendment replaces sender-first, hidden-action-only and client-side sort wording to match the approved desktop/mobile plan. Default Compact density target remains >=10 fully visible single mails at 549×1280; R33-V's mixed-card floor remains >=4 at 412×915. At 150% text scale rows may grow to keep content/actions legible rather than forcing the default-scale count. Oracle: actual mixed/default-scale density captures at 549×1280 and 412×915, a 480×915 neighboring view, 150% large-text/touch/Back journey, no action on flick (R56), stripped Polis.ge preview and unchanged durable card order.
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
  хватает»; avatar coverage corrected 2026-10-19; real-photo path 2026-10-20).
  Every displayed sender identity line — single cards, bundle heads, nested
  members and agent cards — shows a compact avatar beside its metadata line
  on desktop and Android without adding row height. Real photos are available
  Google contact photos only: `people.connections.list` with
  `personFields=emailAddresses,photos` (`contacts.readonly`) plus
  `otherContacts.list` with `readMask=emailAddresses,photos` and CONTACT and
  PROFILE read sources (`contacts.other.readonly`); paginated, exact mailbox
  match (no display-name or dot/plus rewriting), non-default photos only
  (`Photo.default=true` rejected), primary preferred. `Gmail.modify` alone
  carries no sender-photo field. Incremental consent is account-bound: `POST
  /v1/gmail/photo-consent` requires the target `{account_id}` — an absent or
  unknown account is refused 400 before any state exists — and returns an
  `authorization_url` (Gmail.modify plus both photo scopes) whose single-use
  state carries that account binding; it performs no grant. The callback
  verifies the approving Google identity against the bound account's address
  (exact case-insensitive mailbox match): a different identity receives a 400
  page naming both the intended and the approving identity and stores nothing
  — no token, no row, no scope. A matching approval stores the token and
  reports "Sender photo access granted for <address>"; a partial approval
  reports the partial state with the still-missing scopes and leaves
  `authorization_required` set; a Gmail-only approval reports "Gmail
  connected". Stored authority is never blindly unioned or downgraded: a
  later Gmail-only reconnect keeps the photo-carrying refresh token and
  `granted_photo_scopes` when its own token lacks photo scopes, a
  photo-carrying token replaces them, and a partial grant stores exactly its
  own scopes. `GET
  /v1/gmail/sender-photos/status` reports per-account `photo_ready` and
  explicit `authorization_required` with `required_scopes` without touching
  Google. The client asks for consent per account row and binds the returned
  URL to that row's identity; returning to the accounts page drops the
  already-consumed URL and refreshes the server-reported status instead of
  inviting a second click. The status report is tenant-scoped through the same request tenant
  transaction the photo route uses (`set_config('litterbox.tenant_id')` inside
  the request transaction): an authenticated device sees exactly its own
  tenant's accounts in every grant state (both `photo_ready` and
  `authorization_required` entries), a neighbor tenant's accounts never
  appear, and an unauthenticated request is denied 401 by the device-token
  auth instead of returning any list. Server-mediated and device-cached: photos are fetched and decoded
  server-side (bounded 512 KiB, `image.DecodeConfig`-verified) into
  tenant/account-scoped `sender_photos`, delivered through authenticated `GET
  /v1/cards/{id}/sender-photo` (card id lookup; no provider URLs, tokens, or
  addresses in URLs/logs), flagged by `sender_has_photo` in cards/snapshot,
  and cached on the device as local `file://` (`sender_photo` role, offline
  capable). Runtime access to `sender_photos` is granted to `litterbox_app`
  with the same `tenant_isolation` row-level-security policy shape as the
  other tenant tables (unset or empty `litterbox.tenant_id` hides every row),
  shipped as the upgrade-safe follow-up migration `023_sender_photos_security`
  (022 created the table and columns but no grants or policy). Tests apply
  the canonical lexical `server/db/*.sql` set (never a hand-maintained
  per-test subset) so new migrations reach every fixture. The
  genuinely-absent fallback is a circle with initials in a stable per-sender
  colour in unchanged geometry, shown on `unavailable` (404) and on
  `authorization_required` (403): a named sender without a Google photo shows
  its initial instead of `?`, a bare address-only sender shows the address's
  first letter, and `?` remains only for a truly empty name and address.
  Explicit non-coverage: a sender absent from both contact datasets stays
  uncovered even when Gmail
  itself renders a photo — there is no supported arbitrary-email lookup
  (`people.get` needs an account id, Gmail messages expose no photo field),
  and no Gravatar, third-party lookup, or undocumented Google endpoint. The
  avatar is omitted only on the identity line that R36 already omits — a
  nested member whose identity repeats the summary's — never on the bundle
  head itself, which always shows its avatar. Remote images inside mail
  bodies stay blocked. Oracle: `tst_inboxlayout` sender-avatar rows at
  1440×1000, 412×915 and 480×915 with a stable fallback colour across
  rebuilds, plus a photo case where decoded provider bytes render in the
  real `SenderAvatar` (image ready, offline reopen keeps them); negative
  control: a Gmail-only grant reports `authorization_required` with no photo
  bytes on any surface; plus the app-role production HTTP oracle
  `TestStatusRouteTenantVisibility` (real device-token auth, real
  pg_virtualenv schema as the granted runtime role): own tenant's two
  accounts visible with both grant states, neighbor tenant invisible,
  unset/bogus credential 401. Account-binding oracle:
  `TestPhotoConsentAccountBinding` (accountless and unknown-account consent
  POSTs are 400; the wrong-account callback names both identities, stores no
  row or token and overwrites nothing; the matching grant stores the
  photo-carrying token and full scope set; a Gmail-only reconnect preserves
  granted photo authority and its token; a partial grant stores exactly its
  scopes) plus the `qt/tests/tst_gmailaccounts` consumer cases (per-row
  consent URL binding with the row's identity in the prompt, consumed-URL
  clearing and status refresh on return, wrong-account error with no stale
  link as the negative control).
  The presentation source model must keep a non-null delegate independently
  of its live photo Connections: a cold empty inbox realizes arriving cards,
  and later snapshots, removals and photo completions update those rows.
  Regression: `BundleExpand::test_liveSnapshotPresentation` covers cold
  insert, same-count replacement and removal; `senderphotopaint` runs only
  through the registered BundleTest runner and preserves no-refresh/offline paint.
- **R68 Scroll stays put after Archive** (decided, Darafei 2026-10-15 msg
  15893 «После архивации сообщений скролл еуда-то улетает постоянно»;
  local-archive reader follow-up 2026-10-19). Archive, Spam, swipe and Undo
  keep the visual position: the next card takes the removed card's place, also
  when a server snapshot arrives afterwards. When a local archive removes the
  currently selected card, the reader presents the next surviving openable
  card after the selected one, excluding the removed scope, while preserving
  the active filter, bundle expansion and list anchor; hidden bundle headers
  and their skipped members are never presented. If no surviving card follows,
  the reader closes to the list with no wrap-around to a previous card. A
  bundle whose open member was removed advances once; otherwise the bundle is
  unchanged. Archive with no reader open keeps the reader closed. Undo
  restores list order and keeps the reader on the card it advanced to.
  On the one-pane phone reader, Undo stays reachable without closing that
  successor, and the native body leaves the actual Undo target uncovered.
  External archive, Spam, Done and Snooze never auto-advance the reader.
  Oracle: archive card N in a 200-card list scrolled to the middle, deliver a
  stale and a new snapshot; the first visible card id is unchanged except for
  the removed one. Repeat with the removed card selected: when a surviving
  openable card follows, the reader advances to that exact successor; when
  none survives, the reader closes to the list without wrapping.
- **R69 No freeze, no black screen on return** (decided, Darafei 2026-10-15
  msg 15894 «14:49 юай простотзалип и завис», msg 15895 «Переключился
  туда-назад - черный экран»). Archive and snapshot handling never block the
  GUI thread for more than 200 ms, and switching away and back on Android
  shows the content, not a black surface. Oracle: archive storm plus snapshot
  fixture with a frame-time log, and an emulator pause/resume journey.
- **R71 Android launcher shows the Litterbox name and icon** (decided, owner 2026-10-21 «на андроиде иконки приложения, названия нету»). The installed Android app appears in the launcher as Litterbox with the app icon, not the build target name and not a default placeholder. Oracle: `aapt dump badging` on the release APK shows `application-label:'Litterbox'` and a resolved non-empty application icon, with an `ic_launcher` mipmap entry for every shipped density.

## Approved desktop and mobile acceptance (2026-10-18)

- **R70 Five-slice actual-client acceptance** (decided, owner: “Implement the plan” and “make sure that mobile is also taken into account”). These are delivery criteria for the real Qt desktop AND Android app, not prototype substitutes or optional follow-ups. Preserve R7/R22/R24/R30/R68 semantics and existing capture/anchor APIs. Use isolated profiles/fixtures only; retain fresh actual-application screenshots and behavioral receipts at 1440×1000, 1280×800, 412×915 and neighboring 480×915, plus large-text phone proof. A successful build or self-report alone is not acceptance.
  1. **Adaptive shell:** adjustable desktop list/reader with the real
   MailDetailPage, stable highlighted selection and visual list anchor;
   one-pane narrow/phone StackView. Open, switch, resize, close, external
   archive and Android Back satisfy R6/R53 without queuing unintended actions.
   Repeat with an expanded bundle and a middle-list position.
  2. **Meaningful rows:** explicit topics survive unchanged; sender bundles show newest original subject, never raw sender keys. Subject-first mixed rows, Comfortable/Compact density and date/source landmarks satisfy R36/R50/R63 with no action overlap; new latest mail updates the subject without regrouping/reordering. Retain continuous bundle frames, shared identity once, pinned/important separation and compact avatars on every displayed sender identity line at 1440×1000 and 412×915, with the avatar omitted only where R36 already omits the identity line and never on the bundle head itself.
  3. **Useful navigation/search/sync:** desktop exposes existing Inbox, Accounts, Agents and Settings destinations plus functional search; phone offers the same destinations/search through a drawer or compact alternative, not a squeezed sidebar. The permanent rail is shown only when the window is wide enough to give both it and its content usable width; at 250px it is hidden and navigation is reachable through the drawer. No Gmail-folder clutter or separate journal destination is added. Search filters actual cards and clearing it restores the same underlying order/position. Healthy Online is quiet (no status-only row); offline pending changes, Retry, reconnect/auth and save failures remain actionable under R55/R51/R52. Inspect a result, empty result, cleared search and phone drawer; verify destination exit and no accidental archive.
  4. **Distinct reversible triage:** open/expand never Archive; mail Archive/Spam, task Done and journal dismissal retain their separate existing semantics. Existing real Undo uses the original operation/callback and truthfully cancels queued mail changes or applies the supported acknowledged Archive inverse, including stale snapshots. Do not advertise an unsupported non-mail or acknowledged Spam inverse. Inspect the actual Undo overlay and click it before/after mail Archive acknowledgement; confirm restored order and no false success on rejection. When a local archive removes the selected card, confirm the reader advances to the exact next surviving openable card — preserving filter, expansion and anchor, skipping hidden headers — or closes to the list with no wrap when none survives, and that Undo restores order and keeps the reader on that successor. Notes and Open in Gmail remain reachable on desktop and phone.
  5. **Readable originals/accessibility:** actual formatted original HTML, links, selectable code and cached offline reopen remain intact; real oldest-to-newest message data powers older-message disclosure (R61), never fake summaries/history. At default/150% phone text scale, inspect older-message expand/collapse, long subject, reader overflow, note dialog and Back; keep a single document scroll, visible focus/tooltips, >=48dp touch targets and no clipping, accidental activation during flick, lost selection or broken return position.
