# Calendar invitation candidate

Base: `8b8a70e` (v46), branch `fix/calendar-invite`. Local candidate only;
no push, publication, deployment, service changes or owner-device actions.

Before: standalone calendar leaves produce unsupported/empty-body notices;
calendar attachments retain bytes but have no invitation display.
After: per-message invitation blocks precede original HTML/plain bodies. Files
remain downloadable. METHOD REQUEST/CANCEL/REPLY, event title, local time range,
all-day dates (exclusive DTEND), recurrence presence, organizer, attendee status,
location, description and HTTP(S) conference/event links are retained.

Parser: [emersion/go-ical](https://github.com/emersion/go-ical), pinned at
`439c63cef608` (2025-06-09), MIT, handles RFC 5545 unfolding, text escaping,
properties and parameters. Its DateTime handles IANA TZID/UTC/DATE. Outlook's
arbitrary embedded VTIMEZONE names are resolved using the parsed STANDARD/
DAYLIGHT definitions and the library's existing
[rrule-go](https://github.com/teambition/rrule-go) dependency. There is no separate
homegrown iCalendar text parser. Unresolved times are exposed with their source
value and an explanation rather than guessed.

Owner display zone comes from system timezone state, matching the reader's
existing date display. No geographic zone is hardcoded. The rendered Qt test
also runs with `TZ=America/New_York`, checking local output and all-day stability.
The same QML document is shared by desktop WebEngine and Android WebView;
this lane proves the desktop offscreen consumer at desktop and phone sizes,
not a native Android installation or release.

Repair: content version 1 -> 2. Ordinary account polling reparses retained MIME,
updates existing message parts and advances the existing body identity. The
integration test checks no provider fetch for retained MIME, cleared unsupported
status, returned invitation and downloadable ICS, and unchanged archived/snoozed
state, labels, note and pin. Complete-message and failed-repair cooldown controls
remain in place. No schema migration or production write is needed.

Production SELECT at 2026-11-06 15:01:18 +04: **145 distinct messages** match
`message_parts.media_type='text/calendar' OR lower(message_parts.name) LIKE '%.ics'`.
This includes any tenant/state and counts messages once, rather than MIME leaves.
The reported message's retained MIME was also selected privately and parsed
locally: 7,854 bytes, one invitation, one file, empty decode status and no time
error. No private MIME, subject, addresses or deployment credentials are committed.

Validation and reproducible commands are recorded in the accompanying logs and
scripts. `baseline.log` runs the six fixture regressions against an archived
`8b8a70e` server source tree, not another worktree. All six fail because invitation
metadata is absent. The current fixture/parser tests pass.

Commands (from repository root):

```sh
sh receipts/calendar-invite-20261106/test-baseline.sh # expected six failures
pg_virtualenv sh receipts/calendar-invite-20261106/test-go.sh
pg_virtualenv sh receipts/calendar-invite-20261106/test-go.sh grants
cmake -S qt -B .tmp/build -G Ninja
cmake --build .tmp/build -j4
QT_QUICK_BACKEND=software ctest --test-dir .tmp/build --output-on-failure -E '^(restart_service_survival|scrollresilience)$' -j2
TZ=America/New_York QT_QPA_PLATFORM=offscreen QT_QUICK_BACKEND=software QT_QUICK_CONTROLS_STYLE=org.kde.desktop .tmp/build/tst_maildetail -input qt/tests/tst_maildetail.qml -import qt/tests/qml-imports MailDetailPage::test_calendarInvitation
```

The ordinary suite uses `litterbox_test_calendar_invite` in pg_virtualenv and
refuses port 5435. The three grant tests run separately, with `DATABASE_URL`
unset and the required `PGDATABASE=postgres` administrative connection; their
helper creates/drops its own `litterbox_test_journal_grants_*` databases. An
initial grant invocation with the ordinary suite's DSN was correctly refused by
that helper; the separate invocation above passes. The first ordinary run also
caught a stale revision-1 assertion in the worker test, now parameterized by the
current parser version. No production test database or schema was used.

Final result: full ordinary Go suite PASS; all three separate grant tests PASS;
Qt CTest **32/32 PASS**, excluding only the permitted
`restart_service_survival` and `scrollresilience`. Focused rendered invitation
checks pass at 1280, 412 and 480 px in the normal zone and New York.
MIME fixtures intentionally use RFC 822 CRLF; whitespace checking uses
`git -c core.whitespace=cr-at-eol diff --cached --check`.
