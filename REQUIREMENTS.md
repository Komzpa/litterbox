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
  bundles is not enough. Litterbox forms bundles from structured thread identity when the source provides one (GitHub: owner/repo from the subject; mailing lists: List-Id), stripping provider boilerplate such as unsubscribe footers; remaining mail is clustered across all accounts by embeddings with a local model on the home server, and the model can create new topics. If the local model is unavailable, Litterbox falls back to structured keys (repository, list), never to the sender address alone, and reports the missing model; unrelated threads are never merged into one bundle just because they share a sender. Gmail categories are not bundles. Every
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
- **R10 Offline actions** (decided, owner 2026-09-27). Archive, bundle
  archive, snooze, pin, done and writing journal notes work with no network.
  They apply locally at once and sync when the connection returns. Gmail
  changes for archive and snooze (R7, R20), and star changes for pin (R21),
  run then; other source actions run on sync through R12.
  A rejected action never holds back later queued actions; it is shown as failed on its card.
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
  entry point limits the request rate per client address; limit 60 requests
  per minute (owner accepted 2026-09-28).
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

## Open questions

None.

## Live inbox

- **R30 Inbox timing, ordering, and text** (decided, owner 2026-09-28; replaces any conflicting inbox-feed ordering, time-display, or card-summary wording). `GET /v1/cards?now=<RFC3339 optional>` returns `now`, `later`, and `missed` arrays. `now` contains untimed open todos, current timed slots, and real agent results, ordered current timed slot first, untimed todos in note order, then agent results newest first. `later` contains future timed cards in ascending `at` order; `missed` contains superseded past timed cards in descending order. A timed card's displayed position is anchored to its actual `at`; untimed todos have `at=null` and are never assigned a fabricated time. The display uses the system locale and timezone. Every summary is plain text: it contains no raw JSON or XML, Markdown tables, or citation blocks. Oracle: automated server contract tests assert section membership and ordering at fixed `now`, null times for untimed todos, and rendered clock/date/timezone and summary exclusions for synthetic fixtures.
- **R31 Agent results are substantive** (decided, owner 2026-09-28). An agent card is shown only when it contains a real result; probe or acknowledgement sessions, including `CHANNEL OK`, never produce cards. A card must not repeat its title as its summary. Oracle: automated server contract tests feed synthetic probe/ack and real-result sessions, asserting only substantive results appear and no returned card duplicates title and summary.
- **R32 Card note reaches task generator** (decided, owner 2026-09-28). Every card exposes a per-card note that the owner can write and update; the stored note is delivered to the task generator as instruction/context for that card. Oracle: automated integration test writes and updates a synthetic card note through the card API, then verifies the task-generator input for that card contains the latest note and no other card's note.

- **R38 Manual card creation and order** (decided, owner 2026-09-29; grab-and-drag restated 2026-09-30; named move icon replaces the tiny `=` on 2026-10-03 to match R36 and the owner's restart-layout correction). The Qt desktop/Android inbox exposes an obvious Add card control that creates an open manual card with title and optional details; creation is saved locally and sent through the authenticated idempotent operation API. Each card exposes a visible named move-icon drag handle: the owner presses it and drags the card to the position where it must land, then releases. The drop moves the card to that position; a press with no movement changes nothing. The drag never overrides an order the server decides elsewhere: a pinned card moves only within the pinned block (R21), and a card whose position is anchored to its time (R30) does not move. Order changes persist offline and sync through the same operation API across clients. The server accepts reorder only for the complete current open-card set and updates `note_order`; manually created cards use a tenant-scoped manual identity and appear in the inbox. Oracle: Qt offscreen app capture opens the visible add-card dialog and records a frame of it, then drags the rendered move handle with real pointer events to a target row (both up and down), verifies the rendered new card and the card's changed position, verifies a pinned drag stays inside the pinned block and a time-anchored drag is refused, verifies a press on the handle with no movement leaves both the order and the queue unchanged, verifies the rendered Note and Done controls beside the drag surface still respond to real pointer clicks, and independently confirms local cache/outbox operations; server operation test verifies tenant-scoped create/reorder and rejects unknown/closed IDs.
  Create card and Card note dialogs retain the same explicit light palette under forced Material Dark, with English footer labels. Oracle: run `tst_uirequirements.qml` under forced Material Dark, then run `capture.sh <commit>` and inspect `material-dark-action-sheet.png` and `create-card-material-dark.png` for white dialog chrome, `#263b3a` ink, `#397d73` actions, and explicit English button labels. The R38 pointer/no-motion, pinned-clamp, timed-refusal, and cache/outbox checks remain required; use the capture scenario below for those behavioral proofs.
- **R39 Android self-update** (decided, owner 2026-09-30). Android users can explicitly check for updates from the app. The client uses the configured API origin and enrolled device Bearer token for the manifest and its fixed same-origin APK route; it rejects a package ID that differs from the running app, treats `version` as a decimal Android `versionCode`, and offers only strictly newer releases. It streams the complete APK to a temporary file and verifies SHA-256 against the authenticated manifest before showing Install/Later; invalid metadata, transport failure, or checksum mismatch must not launch an installer or replace the installed app. Install is always an explicit user action followed by Android's installer confirmation; if Android requires permission for this source, the user must grant it in system settings and explicitly retry. The app never grants installer permission or installs silently. Cancelling or denying either platform prompt leaves the old app and its data intact. Oracle: `tst_androidupdater` exercises the real updater with a loopback HTTP server and verifies authenticated fixed-route requests, valid newer-release readiness only after complete-byte digest match, no APK fetch for equal/older versions, and rejection of package/path/checksum mismatches. Device acceptance additionally requires a same-package, signature-compatible upgrade initiated from the in-app UI, observed Android installer/consent UI with no automated approval, then installed package/version readback and app-data preservation; an `adb install` result or toolbar screenshot alone does not satisfy this oracle.
## Developer tooling

- **R33 App-native Qt behavior capture** (decided, owner 2026-09-28). The Qt desktop app has an opt-in capture scenario that captures PNGs from its own rendered window and exercises the real QML snooze and pin-order actions against an isolated, pre-seeded fixture database. Capture mode must not access the normal application database or server. Oracle: run `litterbox-qt --capture-scenario <fixture-copy.sqlite> <empty-output-directory>` on the R20/R21 fixture; it exits successfully only after writing baseline, post-snooze, and post-pin-order window captures and independently reading the SQLite cache to verify the snoozed card disappeared, pinned order changed, and exactly the snooze and reorder operations were durably queued. Running with a non-fixture database must reject the scenario without changing it. A screenshot alone is not evidence that either action occurred.
- **R34 Debian package with desktop menu launcher** (decided, owner 2026-09-29). The Qt desktop app builds into a normal `.deb` package that installs via `apt install ./litterbox-qt_*.deb` or `dpkg -i`. After install, the app appears in the system application menu as "Litterbox" under Office/Utility, launches from the menu entry, and its binary is at `/usr/bin/litterbox-qt`. Oracle: build the package with `dpkg-buildpackage -us -uc -b` from the `qt/` directory, install the resulting `.deb`, then verify all of: `dpkg -l litterbox-qt` shows `ii`, `which litterbox-qt` returns `/usr/bin/litterbox-qt`, `desktop-file-validate /usr/share/applications/litterbox-qt.desktop` passes, `test -f /usr/share/icons/hicolor/scalable/apps/litterbox-qt.svg` succeeds, and `gtk-launch litterbox-qt.desktop` starts the app process within 5 seconds. Running `dpkg --verify litterbox-qt` reports no missing or altered files.
- **R35 Desktop inbox scrolling** (decided, owner 2026-09-29; source: "hi precision scroll ... скроллит его в час по чайной ложке; скролл с клавы не работает (home/end/pgup/pgdn/up/dn)"). The desktop inbox list takes focus and supports Home/End, PageUp/PageDown, and Up/Down with useful, bounded movement. High-precision wheel/trackpad pixel deltas move a useful distance rather than crawling; conventional wheel angle deltas also scroll. Oracle: run the Qt app-native capture scenario on an isolated scroll fixture and inject key and pixel/angle wheel events into the real rendered QML inbox; assert the scroll position changes in both directions, End renders the last card at the viewport's bottom edge and Home renders the first card at the top (a bare numeric contentY bound is not sufficient, because a jump on a list with variable-height cards can land on a blank, unrendered region), page/line keys move, and pixel-delta wheel movement is not negligible; inspect the resulting window capture. Inbox Home/End MUST move through the list's own positioner (`positionViewAtBeginning()`/`positionViewAtEnd()`), NOT by assigning `contentY` directly: `contentHeight` is only an estimate until every variable-height delegate has been created once, so a direct assignment can land past the rendered content and leave the viewport blank.
- **R46 Desktop scroll resilience** (decided, owner 2026-10-04; source: "ой оно теперь не скроллится даже"). The inbox, including an expanded bundle with hundreds of members, and the open-mail page always scroll by wheel/touchpad. Expanding or collapsing a bundle never freezes the window: a 300+ member expansion or collapse must finish in <100 ms in the offscreen oracle. Presentation ordering must not use per-item delegate moves or trigger its own regrouping, and must preserve R36's contiguous expanded members and zero hidden-member height. Oracle: the CMake-registered `scrollresilience` test (`tst_scrollresilience.qml`, real CardStore, 308 interleaved members in 1100 cards) checks expansion/collapse timing, source-order preservation, no self-triggered presentation resets, hidden-member height, and wheel scrolling on inbox and open-mail pages.
- **R36 Consistent light UI and touch targets** (decided, owner 2026-10-02; replaces the 2026-09-29 fullscreen-only width rule). The inbox, every page, and every dialog use one centered content column no wider than 1200px with gutters at least 18px. Fonts use Qt/Kirigami theme font roles, never literal point sizes. Actions are visible at rest and are either named icons or labelled buttons—never bare Unicode glyphs—with hit targets at least 48×48px. Every page and dialog pins one explicit light palette (white surfaces, ink `#263b3a`, teal `#397d73`) independently of the host, including under forced Material Dark; dialog buttons have explicit English labels. Oracle: run `tst_uirequirements.qml` via Qt 6 `qmltestrunner` with the real `litterbox` QML import path and forced Material Dark; it checks the real InboxView/CardActions action labels/icons and sizes and the action-sheet content palette. Run `capture.sh <commit>` and inspect `inbox-3840.png`, `inbox-1280.png`, `inbox-560.png`, `accounts-page.png`, `material-dark-action-sheet.png`, and `create-card-material-dark.png` in the capture output for centered gutters, theme typography, visible actions, and light surfaces.
  Bundle-summary correction (owner, 2026-10-03; replaces the sender/count row contract): a bundle is one distinct, focusable summary row with a human topic title or sender-domain fallback (never a raw `sender:` key), an open-member count chip, sender/account context, a collapsed latest-subject preview, and a visible labelled **Archive bundle** action that archives at once, with no confirmation (owner, 2026-10-03: "Сразу + Undo"). Clicking either its chevron or title area, or pressing Enter/Space on the focused summary, expands every ordinary member contiguously inside one enclosing bundle frame at its anchor; clicking again collapses them with zero hidden-row height or leftover spacing. Header title, account context, important/archive disclosure and member subjects share one text column; one continuous left rail spans every expanded member, with thin separators rather than individually bordered member cards. Expanded members appear once, with per-message archive and options (including Take out of bundle), and omit repeated account context unless it differs. Pinned and important cards remain standalone; whole-bundle archive still includes every unpinned card under R22, with important-related scope disclosed. Archive bundle removes those cards at once, queues one archive per email in its originating Gmail account (R7, R10), and shows an inline bar `Archived N emails · Undo` for about 8 s; Undo puts every card back where it was, cancels the archives still queued, and queues the existing Gmail INBOX re-add for any archive already sent. Per-message archive is unchanged. Presentation grouping never reorders durable cards; status and section headings retain distinct breathing room. Tab focuses the opener, Right expands, and Left/Escape collapse and return focus to the summary. At window widths below 640 px, the bundle opener spans the card's content width, the count badge and archive/options actions sit below the title without overlapping it, and title wrapping occurs only at word boundaries. At a 412×915 px window, the title is at least 60% of the card width in both collapsed and expanded states. The Archive bundle action is icon-only with an accessible name and tooltip `Archive bundle`, retains a 48×48 px hit target, and bundle title and account text are not elided at 520 px or 598 px. At widths of 640 px and above, the existing single-row title/count/actions layout and labelled Archive bundle action are unchanged. Oracle: run `tst_bundleexpand`, including its 412 px title-width and badge non-overlap check, `tst_inboxlayout` and the `tst_cardstore` archive-Undo test (cards gone and ops queued; Undo restores order and cancels queued ops; an already-sent archive queues the INBOX re-add), then `qt/tests/bundlecapture_driver.py` with the real CardStore runner, a read-only copied LinkedIn cache (only its disposable temporary copy is writable), an owned Xvfb, and the installed org.kde.desktop style; each viewport uses a separate process. Inspect fresh collapsed and after-real-xdotool-click expanded captures at 520×900, 598×1200, and 1440×1000; also inspect mixed captures at 1440×1000 and 598×1200, the Undo bar after a real click on Archive bundle and the restored cards after a real click on Undo at 520×900 and 1440×1000, plus recorded xdotool Enter/Space activation. For the narrow-title regression, compare base/candidate X11 root captures at 412×915 and 1440×900, collapsed and expanded.
  Bundle-frame regression checks (2026-10-04): expanded two- and three-email ordinary bundles paint continuous left and right edges from the summary header to the final ordinary member, with only the first top edge and the last closing edge. No expanded ordinary member paints a standalone rectangle border, including when its controls have focus; control-local focus indicators remain available. An important email presented last paints its own rounded standalone border, separated from the ordinary frame, and must not steal its closing edge. While the archive Undo bar is visible, the first delegate starts immediately after the header plus the normal ListView top margin (18 px), with no additional status-dependent band. The focused `tst_bundleexpand.qml` frame and archive-header geometry cases cover 520×900 and 598×1200; they inspect all painted edge items and standalone border widths rather than only the cached last-member ID or the bottommost closing edge.

- **R40 Isolated Qt test profile** (decided, owner 2026-09-30). The Qt client accepts an explicit `--test-profile <existing-directory>` startup option whose `profile.json` supplies `server_url` and a non-empty bearer `token`; its SQLite cache and QSettings are confined to that profile directory and never overlap the normal app-data database or normal settings file. Missing/malformed profile input exits before loading QML or opening the normal cache. The profile uses the same application code and UI as normal startup; this is not a fixture or mock mode. Oracle: launch the actual Qt UI with an isolated profile and a disposable local service, verify the process uses that profile's configured URL/token and creates `cards.sqlite`/`settings.ini` only there, verify normal settings and normal cache remain unchanged, then remove only the disposable profile after stopping the process. This startup option does not itself grant Android launch-extra, loopback routing, install, profile, or permission authority.

- **R41 Server Debian package with systemd redeploy** (decided, owner 2026-10-01). The home application server ships as a real `litterbox-server` `.deb` (packaging in `deploy/server/debian`, built from the pinned release binary) that owns `/opt/litterbox/server/litterbox` and the `litterbox-server.service` unit; database credentials and environment in `/etc/litterbox` are never bundled. Production cutover is `dpkg -i` of that package followed by an explicit unit restart, with the previous binary and unit preserved for rollback. Oracle: `dpkg -s litterbox-server` shows the release version, the installed executable's SHA-256 matches the pinned candidate, `systemctl is-active litterbox-server.service` is `active`, `GET /healthz` returns `200`, and unauthenticated `/v1/cards` returns `401`.

- **R44 Desktop self-update on package upgrade** (decided, owner 2026-10-04). When the `litterbox-qt` Debian package replaces `/usr/bin/litterbox-qt` while the app is running, the running app moves itself onto the newly installed binary with no restart by the user: it detects the replacement, waits until the new binary is stable and the package manager holds no dpkg lock, then starts the new binary in its own transient systemd user service with the same arguments and required desktop environment, and quits only after systemd confirms the replacement service started. If systemd is unavailable or cannot start the replacement, the old app stays running. The normal quit drains queued offline actions through `~CardStore` before the replacement opens the same SQLite database, so no queued action is lost; window placement is restored from settings and no open state is lost beyond the current page. The behavior works including when the app runs as a systemd user service (desktop launcher). Watching is armed only for an installed prefix and never for capture or build-tree runs. Oracle: `tst_updatewatcher` covers restart into an atomically replaced binary, the dpkg-lock wait, debounce of flapping replacements, pre-start replacement, a single restart per replacement, and installed-prefix arming; the CMake-registered `restart_service_survival` test launches a desktop-service test copy, atomically replaces its installed-style binary, and asserts that the generation+1 replacement is alive 10 seconds after the original service exits.

## Feed quality

- **R37 Ingest rejects noise and duplicate summaries** (decided, owner 2026-09-29, source: "перебери задачи которые туда попадают - там ща хуйня какая-то"). Ingest rejects summaries that duplicate the title after trimming surrounding whitespace and ignoring case, and summaries classified as acknowledgement/probe or process-status noise by R31's shared agent-result matcher (including `CHANNEL OK`, `still waiting`, `stopped, owned by`, `waiting for`, `still running`, `in progress`, and `working on it`). Empty and whitespace-only summaries remain allowed. Rejected cards are not stored and therefore cannot be returned by the cards endpoint. Oracle: `TestIngestRejectsNoiseCards` verifies HTTP 400 and no stored row for duplicate-title and representative shared-matcher noise, and HTTP 204 plus persistence for a useful summary and empty/whitespace-only summaries; `TestAgentResultNoiseFilters` continues to verify the R31 probe/ack contract.

- **R43 Production feed excludes proof/test cards** (decided, owner 2026-10-02). No proof, test, fixture, or smoke run may create a card in a production feed. Producer tick proofs must use a disposable Litterbox endpoint, tenant, and source token; the proof harness refuses production ingest targets and never loads production source credentials. Oracle: run the native producer tick against a disposable receiver and confirm it creates the expected card only there, then use the production device-token feed readback to verify zero proof/test/fixture/smoke cards while genuine mail and reminder cards remain visible.

- **R45 Open-mail page shows sender and arrival** (decided, owner 2026-10-04, source: the open-mail page showed only the subject and the owner's own address). The open-mail page shows, directly under the subject, the sender as name and address (`Cerebras Systems <welcome@cerebras.net>`) and when the mail arrived in the local zone: time only (`· 11:43`) for mail received today, day and month plus time (`· 4 Oct 11:43`) for mail from this year, and the full date (`· 4 Oct 2026 11:43`) otherwise. A card without an arrival time shows no separator and no time. The card API carries `received_at` (the card's latest `messages.received_at`, RFC3339 UTC, omitted for non-mail cards) and `sender_address`; the Qt card model and SQLite cache carry both. Oracle: `tst_maildetail.qml` loads the open-mail page with a fixed `received_at` and asserts the rendered line for all three date branches plus a no-stray-separator negative control, and `TestCardArrivalAndSenderAddress` asserts both API fields and their omission for non-mail cards.
