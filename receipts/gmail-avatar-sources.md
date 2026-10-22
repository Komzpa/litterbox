# Gmail sender avatar sources (step 1)

Date: 2026-10-22. Probes: read-only with live refresh tokens for
`darafei@maumap.com` (scopes: `gmail.modify` + `contacts.readonly` +
`contacts.other.readonly`). Probe senders from open cards:
`darya.yermashkevich@gmail.com`, `dmitry@sunbim.io`, `fred@fireflies.ai`.
Primary docs only.

Before counts (open cards with `sender_has_photo` join):  
`8ce5c0f7` maumap 406/0 · `4a08d440` sunbim 593/1 · `e09758a2` gmail 489/0.

## Source table

| # | What Gmail shows | Official public API / endpoint | Doc URL | OAuth scope we can use | Works for non-contact sender? | Probe (this host, 2026-10-22) |
|---|------------------|--------------------------------|---------|------------------------|-------------------------------|-------------------------------|
| 1 | Saved **contact** photo | People API `people.connections.list` `personFields=emailAddresses,photos` | https://developers.google.com/people/api/rest/v1/people.connections/list | `https://www.googleapis.com/auth/contacts.readonly` | **No** — only My Contacts | HTTP **200**. Full index used in production sync. None of the 3 probe senders are contacts with non-default photos. |
| 2 | **Other contacts** / auto-saved people (incl. linked profile photo when Google merges PROFILE) | People API `otherContacts.list` `readMask=emailAddresses,photos` + repeated `sources=READ_SOURCE_TYPE_CONTACT` and `sources=READ_SOURCE_TYPE_PROFILE` | https://developers.google.com/people/api/rest/v1/otherContacts/list · https://developers.google.com/people/v1/other-contacts | `https://www.googleapis.com/auth/contacts.other.readonly` | **Only if** the address is already an “Other contact” (prior interaction), not arbitrary internet mail | HTTP **200**. Same 3 probe senders: **0** matches. `otherContacts.search` is **200** but documented `readMask` **cannot include `photos`** (names/email/phone only) — not a photo pipeline. |
| 3 | Contact search (same contact graph) | People API `people.searchContacts` `readMask` including `photos` | https://developers.google.com/people/api/rest/v1/people/searchContacts | `contacts.readonly` | **No** — grouped contacts only | For all 3 probe addresses: HTTP **200**, `results=0`. Redundant with #1 for exact mailbox coverage. |
| 4 | **Workspace domain directory** profile/contact photo | People API `people.searchDirectoryPeople` / `people.listDirectoryPeople` with `DIRECTORY_SOURCE_TYPE_DOMAIN_*` | https://developers.google.com/people/api/rest/v1/people/searchDirectoryPeople · https://developers.google.com/people/v1/directory | `https://www.googleapis.com/auth/directory.readonly` (**not** on our tokens) | Domain colleagues only; not consumer Gmail strangers | With current scopes: HTTP **403** `PERMISSION_DENIED` / insufficient scopes (all 3 probes + listDirectoryPeople). **Cannot call** until incremental `directory.readonly` consent. |
| 5 | **Google Account profile photo** of a non-contact person | People API `people.get` / `people.batchGet` on `people/{account_id}` (numeric Gaia id). Docs: “To get information about a google account, specify `people/{account_id}`.” Public data may need no auth; private needs profile/contacts scopes. **No email→photo lookup is documented.** | https://developers.google.com/people/api/rest/v1/people/get · https://developers.google.com/people/v1/profiles | Profile/contacts scopes for private fields; public still needs **account id**, not email | Gmail UI does this internally; **no public email-keyed API** | `GET .../people/{email}` and `batchGet resourceNames=people/{email}` → **404** NOT_FOUND for all 3 probe addresses. Gmail API `users.messages` resource has **no avatar/photo field** (https://developers.google.com/workspace/gmail/api/reference/rest/v1/users.messages). |
| 6 | **BIMI brand logo** (authenticated brand mail) | DNS TXT `default._bimi.<domain>` (and selector variants); Assertion Record `v=BIMI1; l=<https logo URL>; a=<optional evidence>`. Spec: Domain Owners publish Indicators via DNS; MUAs display for authenticated mail. | https://datatracker.ietf.org/doc/html/draft-brand-indicators-for-message-identification-12 | **None** (public DNS + HTTPS GET of `l=`) | **Yes** for domains that publish BIMI (brand senders), not personal Gmail users | `host -t TXT default._bimi.*`: **paypal.com**, **linkedin.com**, **airbnb.com**, **duolingo.com**, **cloudflare.com**, **apple.com**, **amazon.com** return `v=BIMI1; l=https://...svg`. **fireflies.ai**, **hetzner.com**, **gmail.com**, **google.com**, **sunbim.io** → NXDOMAIN / no BIMI. HTTPS GET of LinkedIn + Duolingo + Cloudflare `l=` URLs → **200** `image/svg+xml` (1056–44212 B). Open-card domains today include linkedin.com (5), airbnb.com (1). |
| 7 | Closed MBP brand packs / favicons / Gravatar | Not an open Gmail-documented client API | — | — | — | **Out of scope** per “do not invent a source Gmail does not use” and prior R67 ban on Gravatar/third-party. |

## What we can implement with our scopes

| Implement? | Source | Reason |
|------------|--------|--------|
| **Keep** | #1 + #2 People index | Already shipped; only path for real person photos under current grants. |
| **Add** | #6 BIMI `l=` logo | Proven public DNS + HTTPS; Gmail/MBPs use BIMI for brand Indicators; no new OAuth. SVG logos rasterized server-side to bounded PNG so existing `sender_photos` + Qt `Image` path stay valid. |
| **Not now** | #4 Directory | Needs new scope `directory.readonly` + Workspace; probes are 403. |
| **Impossible via public API** | #5 arbitrary Google profile by email | people.get needs account id; email resource names 404; Gmail message has no photo field. Gmail’s own non-contact person avatars use **non-public** internals. |
| **Skip** | #3 searchContacts | Same graph as #1; 0 hits on probes; list already paginates. |

## Probe log (redacted)

- Token account: `8ce5c0f7…` / `darafei@maumap.com`.
- `connections.list` pageSize=5 → 200; `otherContacts.list` sources CONTACT+PROFILE → 200; `people/me` photos → 0 entries on this account profile.
- Per sender: searchContacts 200/0; otherContacts.search 200/0; searchDirectoryPeople 403; people.get(email) 404; batchGet(people/email) 404 (fred batchGet body status code 5 / NOT_FOUND).
- Open cards total 1496; with stored photo match **1** before this change.
