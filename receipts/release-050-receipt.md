# Litterbox server 0.1-50 release receipt (2026-10-22)

Scope: server only. Desktop stays 0.1-46 (Qt not changed).

User choice: «Gravatar + иконка домена». Gmail photos of non-contacts are not
available to this app (see `receipts/avatar-internet-research.md`); when
People/contacts, directory, and BIMI all miss, the sync now tries Gravatar,
then the sender-domain icon.

## Code

- Commit `f28135c9e6ae2b112d4d4e9a87cd24313c81cde5`
  ("gmailsync: Gravatar + domain-icon sender photo fallback"), on branch
  `ui-desktop-mobile-redesign-20261018`.
- Push readback: `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018`
  = `f28135c9e6ae2b112d4d4e9a87cd24313c81cde5`.
- `server/internal/people/avatar.go` (new): `GravatarURL` (sha256 of the
  lowercased trimmed email, `?d=404&s=128`), `IsFreeMailDomain` (gmail,
  googlemail, outlook, hotmail, live, msn, yahoo, ymail, rocketmail, icloud,
  me, mac, proton.me, protonmail, pm.me, mail.ru, inbox.ru, list.ru, bk.ru,
  yandex.ru, yandex.com, ya.ru, gmx.de/com/net, aol, zoho, tutanota, tuta —
  favicon pass skips these), `FetchGravatar` (404 → `ErrNoAvatar`), 
...[truncated 4057 chars]