# Release git preparation — receipts/release-git-preparation.md

Gate: `ReleaseGitPreparation`, 2026-10-19 (Asia/Tbilisi). ROOT =
`/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018`.

Status: **PREPARATION ONLY — tuple read and guarded-push commands prepared;
NO push, NO commit, NO staging, NO fetch/merge-tree, NO source/build/DB/ADB/
service/shared-installer mutation (not even `--dry-run`).** Publication
remains the true remaining waiter under Main (Git owner).

## 0. Canonical inputs (read in full, SHA verified)

- `docs/DEPLOYMENT.md` (working tree) sha256
  `92b2d277a8d7bc365e1e4014e43c68a24c8cbadb37b4ee4faeae50464e44872a` — matches the
  SHA recorded in `frozen-release-cohort.md` §1; accepted dirty prep, **preserved
  as-is** (working tree `M`, 35+/4− vs committed blob `aa51119`).
- `receipts/release-plan-readiness.md` sha256
  `6d9e9395d6dea3e06b610862f3c98f5b67f50e51ac7ec10c20981365a2ccfc0b` — **MATCHES**
  the brief's named SHA.
- `receipts/frozen-release-cohort.md` sha256
  `7336c3c24d23ee11b675ff9b2a53d134a153a1c612c536fb875019d2e25c57d2` — **MATCHES**
  the brief's named SHA.

## 1. Git common-dir / isolation / ownership (established before metadata writes)

| Check | Value |
|---|---|
| `git rev-parse --git-common-dir` | `.git` (worktree-local, **not** a shared common dir) |
| `git rev-parse --absolute-git-dir` | `/home/kom/proj/ai_pr/litterbox-ui-desktop-mobile-20261018/.git` |
| `git rev-parse --is-inside-work-tree` | `true` |
| `.git` ownership | `kom:kom` (matches this operator; no cross-owner metadata write) |
| Isolation | single worktree, no linked worktrees, worktree-local `.git` |

Common-dir isolation and ownership were read **before** any metadata write. This
lane performs **no** metadata write (no fetch, no merge-tree, no object write,
no ref update, no commit, no stage). `no fetch/merge-tree needed` per brief.

## 2. Publication tuple — ONE fresh forge head read

- **Remote URL (exact):** `git@github.com:Komzpa/litterbox.git`
- **Full destination ref:** `refs/heads/ui-desktop-mobile-redesign-20261018`
- **oldOID:** **ABSENT** (ref does not exist on the forge)
- **Readback time:** `2026-10-19T03:14:02Z` (UTC), exit `0`
- **Command:** `git ls-remote git@github.com:Komzpa/litterbox.git
  refs/heads/ui-desktop-mobile-redesign-20261018`
- **Exact result:** empty stdout (no OID line), exit 0 → **ABSENT**.
- **Corroboration (same read session):** full `git ls-remote` listing returned
  86 refs; `grep -i "ui-desktop-mobile-redesign"` over that listing → no match
  (**ABSENT** confirmed; no remote HEAD guessed, no other ref substituted).

**Local branch recorded separately (never substituted for the forge read):**
- Local branch: `ui-desktop-mobile-redesign-20261018` (current `HEAD` branch)
- Local HEAD: `fa01568ad3c4fff5ac61cf3354b6e9f4a100c367`
- Local vs `origin/fix/mobile-ux-20261015`: that remote ref **is** an ancestor;
  local is **6** commits ahead (`git rev-list --count` = 6).
- Equal OIDs never substitute for matching repo + ref; the local branch is
  recorded separately, per brief.

## 3. Guarded future push (prepared, NOT executed)

Because `oldOID` is **ABSENT**, the future creation push must **expect
absence** (lease bound to the full ref + the expected-absent state):

```sh
git push \
  --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018: \
  git@github.com:Komzpa/litterbox.git \
  ui-desktop-mobile-redesign-20261018:refs/heads/ui-desktop-mobile-redesign-20261018
```

- `--force-with-lease=<fullref>:` (empty expectation) = the ref **must not**
  exist on the remote; if any other writer created it first, the push is
  refused (no overwrite).
- Push **only** the named destination ref. Never push HEAD, never push other
  refs, never a bare lease, never a guessed destination.

**Rollback hint (direct, same-ref, protected; never delete/recreate):**
If a future publication must be reverted, restore the prior OID on the **same
destination ref** under a lease expecting the published OID — never delete and
recreate the branch:

```sh
git push --force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<publishedOID> \
  git@github.com:Komzpa/litterbox.git <oldOID>:refs/heads/ui-desktop-mobile-redesign-20261018
```

For this **creation** case the prior state is **ABSENT**, so a true rollback
returns the ref to absent — performed only as a lease-protected removal
`--force-with-lease=refs/heads/ui-desktop-mobile-redesign-20261018:<publishedOID>`
so only the exact published ref state can be affected. Any other prior
`oldOID` (once the ref exists non-absent) is restored by the generic form
above. **Never delete/recreate the ref as a workaround for a lease failure.**

After any nonzero push, return the exact attempted command and error and STOP
— no bare-lease, guessed-destination, or alternate-push retry (transport repair
with the same target + lease needs separate authorization).

## 4. Documentation patch (prepared, no "installed/published" claims)

- **Path:** `/home/kom/proj/ai_pr/litterbox-release-plan-20261019/release-documentation.patch`
- **sha256:** `7c62633015eb8fbb1f36e1da216e68c70debdb227c8f98837130fd5078aaf3f6`
- **Size:** 73 lines; unified diff for `docs/DEPLOYMENT.md`
  (`index aa51119..5c83450`, 5 hunks, 35+/4−).
- **Verification:** `git apply --check -R <patch>` → OK (the patch is exactly
  the working-tree diff against the committed blob; reverse-applies cleanly).
- **Truthfulness:** documentation-only. It contains **no claim** that the
  release is installed, published, plan-ready, or accepted. It documents
  operator-owned migrations (020/021), the frozen-single-source-SHA rule,
  per-platform build kits, the exact CLI surface (`plan`/`apply`/`rollback` +
  `--receipt`/`--install-mobile`/`--adoption-timeout`), and the native-gate
  "no rebuild" rule.

## 5. Eligible accepted receipt/doc paths for a later commit (foreign hunks NOT staged)

Eligible (release documentation/receipt paths, to stage **selectively later**):
- `docs/DEPLOYMENT.md` — the modified doc (this patch).
- `receipts/release-plan-readiness.md`
- `receipts/frozen-release-cohort.md`
- `receipts/release-install-preflight.md`
- `receipts/release-handoff-prep.md`
- `receipts/mobile-final-consumer-proof.md`
- `receipts/named-b2b-native-proof.md`
- `receipts/repaired-consumer-proof.md`

**Foreign / must NOT be staged** (present in `git status`, not release docs):
- `qt/mobileux-inbox-bundle-expanded.png`
- `qt/mobileux-inbox-offline.png`
- `qt/mobileux-inbox-online.png`
- `qt/mobileux-reader-long-mail.png`
- other unrelated `receipts/*.md` not listed above (leave untouched).

No staging is performed by this lane; the list is for Main's later selective
`git add` of the eligible paths only.

## 6. Retro (required)

- **Late handoff:** the first draft of the tuple landed, but DONE/LEFT
  finalization was pushed repeatedly; from outside it read as "no receipt".
  Correct pattern: stamp the tuple + receipt into ROOT **after every**
  milestone, then incrementally. This receipt is written immediately after the
  single fresh forge read.
- **Invalid QA intake:** an earlier attempt framed the forge read as a
  confirmation re-run of a previously asserted OID. The correct intake is a
  **single fresh read whose result (here: ABSENT) is not assumed**; no OID was
  carried over from a prior session or from `HEAD`.
- **Honesty held:** no push/commit/stage; no fetch/merge-tree; no install or
  `--dry-run`; no claim that the release is installed/published; no substitution
  of `HEAD`/other refs for the named ref; the patch SHA and reverse-check were
  measured, not asserted. Criteria, docs and the authorized Git handoff are
  unchanged; this lane touched only this receipt (plus the lane-prefix patch).

## 7. Remaining actions (explicit)

1. **Main (Git owner):** execute the guarded creation push from §3 only when
   the release is authorized to publish; after push, read back the commit/ref.
2. **Main:** ReleaseProof must supply the accepted three-way proof refs (cohort
   `proof_gates()` still fails; `lane/cohort.json` status
   `ASSEMBLED_PENDING_PROOF_BINDINGS_not_plan_ready`).
3. **Main:** canonical `deploy/release.py plan` then `apply` (after 1 + 2).
4. **Main:** selective `git add` of eligible paths (§5) — foreign hunks stay
   unstaged; then commit.
5. Local HEAD `fa01568a…` and the 6-commit local lead are unchanged; no
   rebase/merge was performed (none required).
