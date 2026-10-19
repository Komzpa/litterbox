# Native menu sizing repair — git publication receipt

Date: 2026-10-19
Scope: Retro (required). Canonical base ee307c350f925bc3cf059d96ea8f3597141944fc.

## Pre-mutation state

- Local branch: `ui-desktop-mobile-redesign-20261018`
- HEAD: `ee307c350f925bc3cf059d96ea8f3597141944fc`
- Remote: `git@github.com:Komzpa/litterbox.git`
- Published ref (fresh `git ls-remote` readback): `refs/heads/ui-desktop-mobile-redesign-20261018` → `ee307c350f925bc3cf059d96ea8f3597141944fc`
- Staged before mutation: none. Unstaged: 3 files (the three below). Untracked: 16 files (receipts, PNGs, pycache — left untouched).
- Shared-metadata isolation: `core.gitdir` unset, `--git-common-dir` = `.git`, no alternates, no `GIT_DIR` env. Safe for metadata ops.

## Hash verification (before stage)

| File | Expected SHA-256 | Observed | Match |
|---|---|---|---|
| `qt/src/qml/CardActions.qml` | `91d4297a090331fe75eedf59a1e36022c544ab3c0a8296ba8cff4f459e2c2a97` | `91d4297a090331fe75eedf59a1e36022c544ab3c0a8296ba8cff4f459e2c2a97` | yes |
| `REQUIREMENTS.md` | `42bc6118c7b3ed47ba07116877e30e4918b91d3b42ae2e79f9087102ee78683e` | `42bc6118c7b3ed47ba07116877e30e4918b91d3b42ae2e79f9087102ee78683e` | yes |
| `qt/debian/changelog` | `2e97044d767600593f5ebf14f487cd0f03b72fee3d432fc441201884edc02d11` | `2e97044d767600593f5ebf14f487cd0f03b72fee3d432fc441201884edc02d11` | yes |
| `source-manifest.json` | `6374c2c68d42a40912d2f7b8913f997e3cbde597847243e5c53a45bc2902a7b1` | `6374c2c68d42a40912d2f7b8913f997e3cbde597847243e5c53a45bc2902a7b1` | yes |

## Operation run

- Staged exactly: `qt/src/qml/CardActions.qml`, `REQUIREMENTS.md`, `qt/debian/changelog`. No other paths staged; no untracked files staged.
- Commit: `7bbfab265327d918ac68a88d630316503299f6e7` — message quotes R20 clause verbatim from the actual diff:
  > "A visible/open accessibility node is not render proof: the menu must paint with nonzero width inside the viewport, and a physical preset/OK tap must persist the displayed instant in the payload."
- Push command:
  ```
  git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:ee307c350f925bc3cf059d96ea8f3597141944fc origin refs/heads/ui-desktop-mobile-redesign-20261018
  ```
- Push evidence: published ref readback matches `7bbfab265327d918ac68a88d630316503299f6e7`; original push exit was not retained. The later dry-run exit 0 does not prove the original push exit.

## Post-mutation readback

- Published ref: `refs/heads/ui-desktop-mobile-redesign-20261018` → `7bbfab265327d918ac68a88d630316503299f6e7`
- Local branch: `ui-desktop-mobile-redesign-20261018`
- Local HEAD: `7bbfab265327d918ac68a88d630316503299f6e7`
- Tuple match: remote URL `git@github.com:Komzpa/litterbox.git`, ref `refs/heads/ui-desktop-mobile-redesign-20261018`, old OID `ee307c350f925bc3cf059d96ea8f3597141944fc` → new OID `7bbfab265327d918ac68a88d630316503299f6e7`. Local branch OID equals published OID.

## Rollback hint

```
git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:7bbfab265327d918ac68a88d630316503299f6e7 git@github.com:Komzpa/litterbox.git ee307c350f925bc3cf059d96ea8f3597141944fc:refs/heads/ui-desktop-mobile-redesign-20261018
```

Restores `ee307c3` to the same destination ref under a lease expecting the published OID `7bbfab2`. No branch delete/recreate.

## Unrelated state preserved

- 16 untracked files (receipts, PNGs, `__pycache__`) untouched.
- No rebase, reset, merge, fetch, stash, cleanup, test, build, or emulator run.
- SSH `GIT_SSH_COMMAND`/owner unknown — preserved, not killed.

## Product conflicts

None. No merge/rebase conflicts; no product-decision conflicts left for the user.