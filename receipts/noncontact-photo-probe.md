# Non-contact Gmail sender photo probe (step 1)

Date: 2026-10-22. Read-only probes with live tokens, loaded through the
server's own credential loader (`gmail.LoadCredentials` on the production
client JSON + per-account refresh tokens via `litterbox_gmail_sync_accounts()`).
Token values never printed; only HTTP status, counts, photo default flags,
and granted scopes. No contacts created, nothing written.

Probe senders = real open-card senders, each on its owning account:
`fred@fireflies.ai` (5 open cards, maumap `8ce5c0f7`),
`darya.yermashkevich@gmail.com` (1 open card, gmail `e09758a2`),
`pjduplooy.gis@gmail.com` (1 open card, gmail `e09758a2`),
`dmitry@sunbim.io` (1 open card, sunbim `4a08d440`).
Granted scopes on all three tokens:
`contacts.readonly + contacts.other.readonly + gmail.modify`.

## Results

| # | Probe | Request (no secrets) | Doc URL | Status | Real photo? |
|---|-------|----------------------|---------|--------|-------------|
| P0 | warmup `people/me` | `GET people/me?personFields=names,photos` | https://developers.google.com/people/api/rest/v1/people/get | **403** — needs `profile` scope, which we do not request | no |
| P1 | `people:searchContacts` per sender, after warmup | `GET people:searchContacts?query=<email>&readMask=emailAddresses,photos` | https://developers.google.com/people/api/rest/v1/people/searchContacts | **200, results=0** on all 3 accounts for all 4 senders | no |
| P2 | `otherContacts:search` per sender | `GET otherContacts:search?query=<email>&readMask=emailAddresses,names,phoneNumbers` | https://developers.google.com/people/api/rest/v1/otherContacts/search | **200, results=0** everywhere | no (mask cannot include photos by design) |
| P3 | `otherContacts.list` full scan, exact-email match | `GET otherContacts?pageSize=1000&readMask=emailAddresses,photos,metadata&sources=READ_SOURCE_TYPE_CONTACT&sources=READ_SOURCE_TYPE_PROFILE` | https://developers.google.com/people/api/rest/v1/otherContacts/list | **200**; maumap 62 entries (29 real photos), gmail 1467 (327 real), sunbim 3 (1 real). **None of the 4 probe senders present** — not even fred with 5 cards on maumap. Google did not auto-create otherContacts for them. | no |
| P4 | `people.get` on PROFILE source id of the sender | (no sender PROFILE ids exist — see P3) | https://developers.google.com/people/api/rest/v1/people/get | skipped, no ids | no |
| P4b | `people.get` on an arbitrary PROFILE source id (mechanics check: does the call work with our scopes?) | `GET people/<profile-id>?personFields=photos,names,emailAddresses,metadata` | same | **200** on all accounts | **no** — profile returns a photo but `default=true` (silhouette) |
| P4c | gap probe: otherContacts with PROFILE source but no list photo → `people.get` (3 ids on gmail, 3 on maumap, 2 on sunbim) | same as P4b | same | **200**, `photos=1` each | **no** — every one `url present / default=true` |
| P4d | same gap ids with `sources=READ_SOURCE_TYPE_PROFILE` only | `GET people/<id>?personFields=photos&sources=READ_SOURCE_TYPE_PROFILE` | same | **200** | **no** — identical, still default silhouette |
| P4e | unauthenticated `people.get` on a numeric Gaia id (is the profile photo public without auth?) | `GET people/<gaia>?personFields=photos` without `Authorization` | https://developers.google.com/people/v1/profiles | **403** — callers without established identity rejected | no |
| P5 | `connections.list` with `sources=` variants | `sources=READ_SOURCE_TYPE_CONTACT` → **200** (already used in production). `READ_SOURCE_TYPE_PROFILE`, `READ_SOURCE_TYPE_DOMAIN_CONTACT` → **400** `Must request READ_SOURCE_TYPE_CONTACT`. `READ_SOURCE_TYPE_DOMAIN_PROFILE` → **400** invalid value | https://developers.google.com/people/api/rest/v1/people.connections/list | only CONTACT usable | no new surface |
| P6 | Gmail `messages.list q=from:<sender>` + `messages.get format=METADATA` (any Gaia/profile reference in mail data?) | `GET gmail/v1/users/me/messages?q=from:<sender>` then `.../messages/<id>?format=METADATA` | https://developers.google.com/gmail/api/reference/rest/v1/users.messages/get | **200**; message found on owning account. Full header-name dump: `Delivered-To Received ARC-* Return-Path DKIM-Signature X-Gm-* From Date Subject To ...` — **no profile, Gaia, or avatar header** on any of the 4 messages | no |
| P7 | `people:searchDirectoryPeople` | `GET people:searchDirectoryPeople?query=dmitry@sunbim.io&...` | https://developers.google.com/people/api/rest/v1/people/searchDirectoryPeople | **403** `insufficient authentication scopes` (needs `directory.readonly` re-authorize) | not callable |
| P8 | email-keyed `people.get` | `GET people/<email>?personFields=photos` | https://developers.google.com/people/api/rest/v1/people/get | **404** `Requested entity was not found` for all senders | no |
| P9 | Admin SDK `users.get` (Workspace colleague photo angle) | `GET admin/directory/v1/users/dmitry@sunbim.io?projection=BASIC` | https://developers.google.com/admin-sdk/directory/reference/rest/v1/users/get | **403** `insufficientPermissions` (needs admin scope we do not have) | not callable |

## Conclusion

Every candidate fails. The two ideas that looked most promising do not hold:

1. "Sender becomes an otherContact, then read its PROFILE photo": the probe
   senders are **not** otherContacts on any account despite real open cards,
   and where PROFILE sources do exist, `people.get` returns only
   `default=true` silhouettes (P4c/P4d) — zero gain over the list surface
   production already indexes.
2. "Walk the API Gmail walks": Gmail message data carries **no** profile/Gaia
   reference (P6 header dumps), email-keyed `people.get` is 404 (P8), and the
   Gmail web hovercard path has no documented public email-keyed endpoint.

No new source exists under our grants. Per the row instruction, shipping no
fallback: no code change, no 0.1-50 build (nothing to ship), no new tests.
Open-card photo counts are unchanged (see row report); live sources stay
People contacts + otherContacts, BIMI brand logos, and the Workspace
directory after the sunbim re-authorize.
