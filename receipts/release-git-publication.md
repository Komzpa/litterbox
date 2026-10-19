# Release Git Publication Receipt — 0.1-36

Status: prepublication (written before commit; post-push readback appended after push)

## Pre-mutation state

- Repository: `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`
- HEAD: `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367` (production `a6cb`, on branch `ui-desktop-mobile-redesign-20261018`)
- Remote: `git@github.com:Komzpa/litterbox.git`
- Destination ref: `refs/heads/ui-desktop-mobile-redesign-20261018`
- Remote ref observed state: **ABSENT** — `git ls-remote origin refs/heads/ui-desktop-mobile-redesign-20261018` returned empty; the branch does not exist on the remote. Push will create it.
- Local branch ahead of `origin/fix/mobile-ux-20261015` by 6 commits (unrelated, preserved, not touched)
- Working tree: `docs/DEPLOYMENT.md` modified (35 insertions, 4 deletions); `receipts/` untracked; `qt/*.png` untracked (prior accepted artifacts, deliberately not staged)

## Publication tuple (derived from one fresh forge-head readback)

| Field | Value |
|---|---|
| Remote URL | `git@github.com:Komzpa/litterbox.git` |
| Full destination ref | `refs/heads/ui-desktop-mobile-redesign-20261018` |
| Observed old OID | ABSENT (ref does not exist on remote) |
| Lease bound | `--lease=refs/heads/ui-desktop-mobile-redesign-20261018:<oldOID>` where `<oldOID>` is empty (absence) |

Push semantics: since the remote ref is absent, the push creates it. The lease is bound to the full ref with an empty expected OID (absence lease). If the ref unexpectedly exists on the remote, the push will fail with a specific collision rather than overwrite.

## Paths to be committed

### Modified (tracked)
- `docs/DEPLOYMENT.md` — operator-owned release documentation; reflects current truth of installed server 0.1-36 (hash `6c23d73e`), desktop `ead409d5`, Android v23b186bf5e, and the failed-CLI-apply/receipt-status-failed lesson.

### New receipts (untracked, being added)
- `receipts/release-install-preflight.md`
- `receipts/release-handoff-prep.md`
- `receipts/frozen-release-cohort.md`
- `receipts/release-plan-readiness.md`
- `receipts/release-git-preparation.md`
- `receipts/mobile-final-consumer-proof.md`
- `receipts/named-b2b-native-proof.md`
- `receipts/repaired-consumer-proof.md`
- `receipts/original-message-desktop-proof.md`
- `receipts/offline-live-history-proof.md`
- `receipts/bundle-frame-consumer-proof.md`
- `receipts/remaining-consumer-proof.md`
- `receipts/queued-retry-consumer-proof.md`
- `receipts/unified-release-execution.md` (SHA `6bb7a30f6522ae25c9acf91cbf5999924dacb899a32b842f1142e1d2490a601a`)
- `receipts/ui-release-proof.md` (SHA `08cdf095cb7e3d5f6628d1a35f5a1c1015f839635196eb12b846c119bcd2a3f2`)
- `receipts/release-git-publication.md` (this file)

### New artifacts (untracked, being added)
- `receipts/update-offer-proof/00-emulator-https-origin-reachable.png`
- `receipts/update-offer-proof/01-enrolled-online-v21.png`
- `receipts/update-offer-proof/02-update23-offer.png`
- `receipts/update-offer-proof/03-update23-banner-later.png`

### Deliberately NOT staged
- `qt/mobileux-inbox-bundle-expanded.png`, `qt/mobileux-inbox-offline.png`, `qt/mobileux-inbox-online.png`, `qt/mobileux-inbox-reader-long-mail.png` — prior accepted artifacts, not needed for this release commit.

## Commit

- Message: `docs: record unified desktop and Android release 0.1-36`
- No Claude attribution.
- docs commit separate from artifact/source commits (existing history preserved).

## Installed release evidence (ground truth, not a CLI claim)

- Installed server: desktop 0.1-36, hash `6c23d73e`
- Installed desktop: hash `ead409d5`
- Server `a6cb`: all 3 origins served
- Android: v23b186bf5e, auth served and owned; AVD installed
- Normal UI: older v21 LAN online → Check updates → downloaded verified dialog; no installer prompt; approved
- Explicit old v21 HTTPS enroll gap: no physical owner phone install claim; owner desktop zero apps; never restarted
- Failed CLI apply: receipt status `failed` (transient endpoint readback); later independent all-origins PASS/Main retains working decision. This is recorded as a reusable lesson, not a success claim.

## Rollback

If the push must be undone:
```
git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<publishedOID> git@github.com:Komzpa/litterbox.git <priorOID>:refs/heads/ui-desktop-mobile-redesign-20261018
```
Where `<publishedOID>` is the OID observed on the remote after push, and `<priorOID>` is the OID that was on the remote before push. Since the remote ref was absent before push, there is no prior OID to restore; on abort the published ref is preserved (never deleted/recreated).

## Post-push readback

 appended after push —
