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
  journal notes in Litterbox. Notes are never sent anywhere; the owner's AI
  assistant may read them. How the assistant gets access: open.
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
- **R6 Native mobile app first** (decided, owner 2026-09-27). The main client
  is a native mobile app. Reminders set on the phone show up on every client.
  Platform and desktop surface: open.

## Mail

- **R7 Archive goes to Gmail** (decided, owner 2026-09-27). Archiving a mail
  card or a whole bundle archives those messages in the Gmail account they
  came from. A user can connect several Gmail accounts.
- **R8 No reply in Litterbox** (decided, owner 2026-09-27). Litterbox has no
  composer. A mail card opens the thread in the regular Gmail app.

## Offline

- **R9 Full local copy** (decided, owner 2026-09-27). The phone keeps every
  open card with its content, not only titles. Reading needs no network.
- **R10 Offline actions** (decided, owner 2026-09-27). Archive, bundle
  archive and writing journal notes work with no network. They apply locally
  at once and sync when the connection returns; the Gmail archive of R7 runs
  then.
- **R11 Network loss is not an error** (decided, owner 2026-09-27: "must not
  break in the elevator"). Losing the network mid-use never blocks the
  interface or shows an error screen.

## Sources

- **R12 Own Gmail connector, one ingest API for the rest** (decided, owner
  2026-09-27). Litterbox talks to Gmail itself, both ways. Every other source
  (phone notifications, messengers, calendar, alerts, AI research) pushes
  cards through one ingest API. Collectors for private sources live outside
  this repository.

## Server

- **R13 Self-hosted PostgreSQL** (decided, owner 2026-09-27). The server runs
  on the owner's home server with PostgreSQL; the database is part of that
  host's backups.
- **R14 Multi-tenant** (decided, owner 2026-09-27). Every stored row belongs
  to one tenant, and no request can read or change another tenant's rows.
  Load sizing: 1, then 2, then about 10 users (family, colleagues).
- **R15 Remote access resists scanners** (open, owner 2026-09-27). The phone
  reaches the home server from mobile networks, possibly through the owner's
  VPS, and the entry point withstands internet-wide scanner bots. Mechanism:
  open.

## Project

- **R16 Personal first** (decided, owner 2026-09-27). Built for one user
  first; cleaned up for others only if it proves useful. The architecture
  still follows R14.
- **R17 Public repository without private data** (decided, owner
  2026-09-27). The code is public. Credentials, personal data and deployment
  details of the owner's machines never enter the repository.

## Open questions

- phone-to-server access path (R15)
- how the database backups of R13 are made and where they go
- card lifecycle: done, snooze, pin, bundles, cards that close themselves
- what "done" does for non-mail cards (chats, alerts, meetings)
- client platform and desktop surface
- AI research cards and how the assistant reads the journal
- first slice and its acceptance scenario
