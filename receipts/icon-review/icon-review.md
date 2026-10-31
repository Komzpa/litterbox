# Litterbox icon review — 2026-10-31

**Proposal only; no commit/push.** Branch `icon-review-20261031`, starting revision `2b15b40`. Owners review `icon-review.png` before approval. QML screenshot evidence is from the maintained Qt Quick test runner with `-import qt/tests/qml-imports`; the 549×1280 inbox was rendered before and after the proposed changes.

## Decision count

**15 flagged QML icon-declaration callsites of 53 named/conditional icon-declaration callsites audited.** The 53 count is named or conditional icon assignments; generic icon-renderer bindings are inventoried below but not double-counted as icon choices. The review sheet groups those 15 findings into 10 proposals (archive has two action locations; Accounts, Agents and Settings each appear in both desktop navigation and menu/mobile navigation; Clear search appears in both the toolbar and empty/search action). The other 38 declarations use action-appropriate glyphs. The audit also inventories every SVG file in the bundled icon theme: 82 SVGs across actions, places and status; new bundled resources are the consistent same-viewBox, monochrome glyphs shown on the sheet.

## QML icon-use inventory

File:line is the QML assignment/render source. Repeated contexts are grouped only where they use the same source and intent. “Pass” means the glyph is semantically appropriate and available at runtime; “Flagged” entries are changed in this proposal.

| File:line | Current glyph/source → intended action | Verdict |
| --- | --- | --- |
| `qt/qml/InboxView.qml:152,1211` | `mail-receive` → Inbox destination | Pass — inbox/mail tray convention. |
| `qt/qml/InboxView.qml:153,952,1212` | Old desktop `mail-receive` / mobile `user-identity` → Accounts destination | **Flagged** — not distinctive and inconsistent; replaced with `user-group`. |
| `qt/qml/InboxView.qml:154,953` | Old `user-identity` → Agents destination | **Flagged** — single-person symbol mismatched the conversations/agents page; replaced with `dialog-messages`. |
| `qt/qml/InboxView.qml:1216` | `dialog-messages` → Agents destination | Pass — conversation glyph. |
| `qt/qml/InboxView.qml:155,954` | Old `application-menu` → Settings destination | **Flagged** — hamburger is a navigation-menu glyph, not settings; replaced with `configure`. |
| `qt/qml/InboxView.qml:1225` | `configure` → Settings destination | Pass — settings gear. |
| `qt/qml/InboxView.qml:855` | `application-menu` → open navigation | Pass — standard menu/hamburger. |
| `qt/qml/InboxView.qml:870,1234` | `edit-find` → search inbox | Pass — magnifying glass. |
| `qt/qml/InboxView.qml:883,2324` | `list-add` → create/add card | Pass — plus/add. |
| `qt/qml/InboxView.qml:907,1928` | `overflow-menu` → inbox/bundle overflow actions | Pass — three-dot overflow. |
| `qt/qml/InboxView.qml:952` | `user-group` → Accounts (proposed) | Pass after replacement — distinguishes account listing from Inbox. |
| `qt/qml/InboxView.qml:955` | `user-identity` → enroll device | Pass — single-person/device identity. |
| `qt/qml/InboxView.qml:956` | `view-refresh` → sync mail | Pass — refresh/sync arrows. |
| `qt/qml/InboxView.qml:957` | `system-software-update` → check updates | Pass — update/status symbol. |
| `qt/qml/InboxView.qml:308` | `dialog-ok` → checked density-menu choice | Pass — checkmark denotes selected option. |
| `qt/qml/InboxView.qml:984,2324` | `edit-clear` → clear search | **Flagged** — source existed in host Breeze but was omitted from bundled theme; added to theme to avoid device-dependent/missing glyph. |
| `qt/qml/InboxView.qml:1783` | `arrow-down` / `arrow-right` → expand/collapse bundle | Pass — standard disclosure chevrons. |
| `qt/qml/InboxView.qml:1903,2217` | Old `mail-mark-read-symbolic` → archive email/bundle | **Flagged** — open-envelope/read state suggests mark read/unread. Replaced with `mail-archive` (archive box/folder). |
| `qt/qml/InboxView.qml:1958` | `mail-mark-junk` → mark as spam | Pass — junk-mail glyph. |
| `qt/qml/InboxView.qml:2028` | `transform-move` (hidden in compact/mail rows) → reorder/drag affordance | Pass — movement/reorder glyph. |
| `qt/qml/InboxView.qml:2217` | `dialog-ok` (non-mail branch) → complete/done | Pass — checkmark means completion. |
| `qt/qml/InboxView.qml:2297` | Old `mail-mark-read` → empty inbox / “You're all caught up” | **Flagged** — envelope mark-read glyph does not communicate completion; replaced with a checkmark (`dialog-ok`). |
| `qt/qml/InboxView.qml:2515` | `mail-message` → reader “Select a card” placeholder | **Flagged** — icon was not in bundled Android theme; added same-named SVG. |
| `qt/src/qml/CardActions.qml:150` | `overflow-menu` → more card actions | Pass — three-dot overflow. |
| `qt/src/qml/CardActions.qml:252` | `dialog-close` → close action sheet | **Flagged** — standard close glyph was not in bundled theme; added. |
| `qt/src/qml/CardActions.qml:309` | `mail-archive` / `dialog-ok` → archive or complete primary action | Pass — archive/check by action. |
| `qt/src/qml/CardActions.qml:315` | `edit-delete` → delete journal note | **Flagged** — standard delete glyph was not bundled; added. |
| `qt/src/qml/CardActions.qml:322` | `mail-mark-junk` → mark mail as spam | Pass — junk-mail glyph. |
| `qt/src/qml/CardActions.qml:334` | `document-open` → open source mail/source | Pass — open/external document. |
| `qt/src/qml/CardActions.qml:341` | `document-preview` → read cached message | Pass — preview/read cached copy. |
| `qt/src/qml/CardActions.qml:348,504` | `document-edit` → edit note / custom snooze date | Pass — edit action. |
| `qt/src/qml/CardActions.qml:354,456,468,480,492` | `appointment-new` → choose snooze / preset snooze time | Pass — appointment/calendar action. |
| `qt/src/qml/CardActions.qml:360` | `pin` → pin/unpin | Pass — pin action. |
| `qt/src/qml/CardActions.qml:367` | `list-remove` → take card out of bundle | Pass — remove from list/group. |
| `qt/pages/MailDetailPage.qml:203` | `barButton.icon.name` → detail-toolbar action (bound to action button) | Pass — reuses the corresponding action icon; not a separate glyph. |
| `qt/pages/MailDetailPage.qml:300` | `go-previous` → back to inbox | Pass — back navigation arrow. |
| `qt/pages/MailDetailPage.qml:340` | `mail-archive` / `dialog-ok` → archive/done in reader | Pass — action-specific archive/check. |
| `qt/pages/MailDetailPage.qml:353` | `document-open` → open source | Pass — open/external document. |
| `qt/pages/AgentChatPage.qml:138` | `go-previous` → back from agent chat | Pass — back navigation. |
| `qt/pages/BackToInboxBar.qml:15` | `go-previous` → back to inbox | Pass — back navigation. |
| `qt/pages/GmailAccountsPage.qml:243,264` | `mail-receive` → connect account | Pass — receive/connect mail; the action is not the Accounts navigation item. |
| `qt/pages/GmailAccountsPage.qml:440` | `document-open` → open photo-consent information | Pass — open external document/link. |
| `qt/pages/MailActionButton.qml:18` | `root.icon.name` → shared caller-provided action | Pass — icon mirrors the action supplied by caller; no independent icon choice. |

## Bundled Qt icon-resource inventory

All SVG paths below are `qt/icons/litterbox/<category>/<size>/<name>.svg`; source SVGs have their viewBox/geometry at line 1 and the theme chooses nearest size. Theme contains **83 SVG files** total: 71 action, 10 place, and 2 status resources. Existing Breeze-derived vectors are monochrome/currentColor where expected; new files follow that 24×24 vector convention and share the same glyph at 22/24/32 where those sizes are declared.

| Named glyph | Resource sizes | Intended use / verdict |
| --- | --- | --- |
| `application-menu` | actions 16,22,24,32 | Hamburger/navigation; Pass. |
| `appointment-new` | actions 16,22,24,32 | Snooze/calendar; Pass. |
| `arrow-down`, `arrow-right` | actions 22 | Disclosure arrows; Pass. |
| `configure` | actions 24 | Settings; Pass. |
| `dialog-close` | actions 22,24,32 (added) | Close dialog/action sheet; Pass. |
| `dialog-messages` | actions 22,24 | Agent conversations; Pass. |
| `dialog-ok` | actions 16,22,24,32 | Done/checked menu; Pass. |
| `document-edit` | actions 16,22,24,32 | Edit notes/date; Pass. |
| `document-open` | actions 16,22,24,32 | Open message/source; Pass. |
| `document-preview` | actions 16,22,24,32 | Cached-message preview; Pass. |
| `edit-clear` | actions 24 (added) | Clear search; Pass. |
| `edit-delete` | actions 22,24,32 (added) | Delete note; Pass. |
| `edit-find` | actions 24 | Search; Pass. |
| `go-previous` | actions 24 | Back navigation; Pass. |
| `list-add` | actions 16,22,24 | Compose/add; Pass. |
| `list-remove` | actions 16,22,24 | Remove from bundle; Pass. |
| `mail-archive` | actions 24 | Archive; Material-style archive tray with a down arrow (not an open envelope/read or folder-plus glyph). |
| `mail-mark-junk` | actions 22,24 | Mark spam/junk; Pass. |
| `mail-mark-read-symbolic` | actions 16,22,24,32 | Mark read/unread meaning; must not be used as archive. |
| `mail-message` | actions 22,24,32 (added) | Reader placeholder; Pass. |
| `mail-receive` | actions 16,22,24 | Inbox / connect-account; Pass. |
| `overflow-menu` | actions 16,22,24,32 | More actions; Pass. |
| `pin` | actions 16,22,24 | Pin; Pass. |
| `transform-move` | actions 24 | Reorder/drag; Pass. |
| `user-group` | actions 22,24,32 (added) | Accounts; distinguishes it from Inbox and person-only identity. |
| `view-refresh` | actions 16,22,24,32 | Refresh/sync; Pass. |
| `folder` | places 16,22,24,32,48,64 | Folder/place; Pass. |
| `user-identity` | places 16,22,24 | Individual identity/enroll; Pass. |
| `system-software-update` | status 22,24 | App update; Pass. |

`mail-message`, `dialog-close`, `edit-delete`, and `edit-clear` were already referenced by QML but absent from the subset. That omission is material on Android: the app selects the bundled `litterbox` theme without a system Breeze installation. The missing-resource rows therefore count as flagged even though the host's system theme can mask the gap in desktop screenshots.

## 549px phone evidence

- Before: `receipts/icon-review/before-549x1280.png` (legacy archive mark-read icon still shown on each mail action).
- After: `receipts/icon-review/after-549x1280.png` (archive icon shown on each mail action).
- Before/after row-detail captures: `receipts/icon-review/drawer-before/navigation-drawer-popup-412x915.png`, `receipts/icon-review/drawer/navigation-drawer-popup-412x915.png`.
- Composite sheet: `receipts/icon-review/icon-review.png`.

Rendered by `qt/tests/tst_androidinboxpolish.qml::test_capture`, which captures the inbox at 549×1280. Full visible renders are retained as evidence. The sheet crops and doubles the right card-action column for phone-scale visual comparison and shows the drawer before/after so Accounts/Agents/Settings distinctions are reviewable.

## Changed paths and test receipt

Implementation: `qt/qml/InboxView.qml`; `qt/icons/litterbox/README`; modified `qt/icons/litterbox/actions/24/mail-archive.svg`; added SVGs `actions/{22,24,32}/{dialog-close,edit-delete,mail-message,user-group}.svg` and `actions/24/edit-clear.svg`. The QML replaces two archive glyphs, three Accounts glyphs, two Agents glyphs, two Settings glyphs, and the empty-state glyph.

Review receipts: `receipts/icon-review/icon-review.png`, `icon-review.md`, full before/after 549×1280, 480×1280, 412×915, and 1440×900 inbox screenshots, plus before/after drawer captures under `drawer-before/` and `drawer/`.

Maintained invocation (from `ROOT`):

```sh
QT_QPA_PLATFORM=offscreen QT_QUICK_CONTROLS_STYLE=org.kde.desktop \
  build/tst_maildetail -input qt/tests/tst_inboxlayout.qml -import qt/tests/qml-imports
QT_QPA_PLATFORM=offscreen QT_QUICK_CONTROLS_STYLE=org.kde.desktop \
  build/tst_maildetail -input qt/tests/tst_androidinboxpolish.qml -import qt/tests/qml-imports
```

Results on the final icon state: `tst_inboxlayout` 58 passed, 0 failed, exit **0**; `tst_androidinboxpolish` 11 passed, 0 failed, exit **0**. Phone renders were captured using the same runner and import, selecting `AndroidInboxPolish::test_capture` (all four sizes pass).
The `tst_inboxlayout` run also logged Settings/QSettings initialization warnings and a `GmailAccountsPage.qml:112` `get` TypeError warning while navigating its account-page test fixture; the named test and full suite still passed. The changed account navigation glyph and test warning are reported separately; no icon assertion failed.

**Approval requested:** confirm/reject the proposals in the PNG before these icon changes are committed.
