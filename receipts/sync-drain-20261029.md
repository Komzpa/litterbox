# Sync drain + ghost-row fix — receipt 2026-10-29

Branch: `ui-desktop-mobile-redesign-20261018` (contains v36 sync work `83a1cdd`).
Checkout: `/home/kom/litterbox`. No APK publication, no production DB writes.

## 1. Measured pacing (ops/min before)

Local fake transport, 100 `saveNote` ops, pre-fix code (`tst_cardstore outboxDrainsInBoundedRounds`
before the fix, `QT_QPA_PLATFORM=offscreen`):

- `drain 100 ops: 100 rounds, max 1 in flight, 4020 ms` → test failed as designed
  (`rounds <= 28` false, `maxOutstanding >= 3` false).
- Local event-loop cost ≈ 40 ms/op, i.e. each op pays one full serial round trip
  (GUI → transport → ACK → storage-thread commit → next send).

On the owner's phone (device `41b1c087`, Android 34, v36) the round trip is the
cellular RTT (~1 s, hence "one per second"), and two amplifiers sit on top:

- any transient failure re-armed a **global** 1–60 s backoff that parked even
  unrelated cards;
- an in-flight op with no completion (half-open socket on a dead link) froze
  the pump entirely — nothing else could send while `m_inFlight` was set.

Net observed: **8 ops in 90 min ≈ 0.09 ops/min** against 81–86 pending.

## 2. File:line causes

Defect 1 — serial pump, `qt/src/CardStore.cpp` (pre-fix numbering):

- `1197-1199`: `flush()` only ran `flushNext()` when `m_inFlight.isEmpty()`
  **and** no retry backoff was active.
- `1202`: `flushNext()` returned immediately when `!m_inFlight.isEmpty()` —
  exactly one op on the wire per network RTT plus two storage-thread hops.
- `1389-1396` (old): every transport/5xx failure started a global retry timer
  that stalled the whole queue instead of just the failed card.

Defect 2 — server-deleted card keeps rendering, `qt/src/CardStore.cpp`:

- old `1295`: `if (missing) { retireRejectedOperation(opId); return; }` —
  a 404 / 422-`card not found` retired the op but never touched the local row.
  With zero pending writes left, the row survived until the next snapshot that
  omitted it; on an offline phone that snapshot never arrives, so e.g.
  DrawerNote01..15 render forever although the server has 0 such rows.
  (The snapshot path itself was already correct: snapshot-absent rows without
  pending writes are dropped by `applyRemoteCards` → `replaceCards`/`writeCards`.)

## 3. Fix diffs (post-fix line numbers)

`qt/src/CardStore.h`:

- `162-165`: `dropServerDeletedCard()` declaration.
- `172-176`: `kMaxInFlightOps = 4`, `flushNext()` now returns `bool`.
- `243-246`: `m_inFlight: QString` → `QSet<QString>` (bounded pipeline),
  new `m_rateCooldown` (429-only global park).

`qt/src/CardStore.cpp`:

- `1197-1205`: `flush()` fills the pipeline to 4; per-op deferral
  (`m_deferredWrites`) parks one card, never the queue; `flushFinished`
  when outbox and wire are both empty.
- `1206-1254`: `flushNext()` picks the head-eligible op: skips in-flight /
  deferred ops (their card blocks only itself → per-card FIFO preserved),
  reorder_* ops fly alone against a coherent open set, 429 cooldown parks all,
  non-durable ops are never sent (durability rule kept).
- `1255-1274`: `dropServerDeletedCard()` — removes the model row and persists
  the deletion unless a remaining op still writes that card (reorder_* and
  agent_message carry nominal ids and never block a drop).
- `1275-1320`: `retireRejectedOperation()` takes `retiredCardId`; completion
  (after the atomic outbox retire) calls the drop — journal bytes stay
  retained, 422-retention and retirement semantics unchanged.
- `1341-1353`: the `missing` (404 / 422-`card not found`) path passes the
  subject card id for single-subject op types only.
- `1448-1463`: transient failures defer just the failed op and `flush()` the
  rest; 429 sets the 60 s shared-window cooldown. Storage-error paths also
  defer + re-flush instead of stranding the op.

`qt/tests/tst_cardstore.cpp`:

- `114-134`: `LatencyTransport` — each POST completes after a fixed 50 ms
  wire delay; high-water mark proves pipelining.
- `2479-2516`: `outboxDrainsInBoundedRounds` — 100 ops, send order preserved,
  `maxOutstanding >= 3`, wall `< 5000 ms` (serial floor ≈ 8.5 s).
- `2517-2551`: `serverDeletedCardLeavesNoGhostRow` — 404-retired card
  disappears (model + cold-reopen DB) while a 422-retained owner edit on a
  second card survives (negative control: `saveFailed`, no `operationFailed`).
- `2289-2305`: `partialBundleRejectionsThenUndoPreservesFullOrder` re-scripted
  for the pipeline (bundle of 3 posts at once; undo queues the INBOX re-add
  only for the already-sent member).
- `2472-2474`: concurrent `gmail.label_add` arrival order made set-based.

## 4. Test commands + exit statuses

```
cd qt/obj-x86_64-linux-gnu && cmake --build . --target tst_cardstore -j8   # exit 0
QT_QPA_PLATFORM=offscreen ./tests/tst_cardstore                            # exit 0
Totals: 129 passed, 0 failed, 0 skipped, 0 blacklisted
```

Before/after on the local fake transport (100 ops × 50 ms RTT):

- before: 100 rounds, max 1 in flight, ≈4020 ms (0 ms RTT variant); serial
  floor with RTT ≈ 8.5 s.
- after: max 4 in flight, 4457 ms total (= 25 flights × 50 ms + one durable
  ack commit per op; fsync-per-ack stays serial by design).

## 5. What the owner will see for DrawerNote01..15 after the next sync

Rows with **no remaining pending local write** vanish from the inbox (and from
SQLite on the next commit) at the first snapshot that omits them — and as of
this fix, immediately when the server answers their op 404 / 422-`card not
found`, without waiting for that snapshot. Rows that still carry a pending
owner write (unacked creates/edits) stay until those drain; once drained the
server holds the rows, so they remain visible as real synced notes.

## 6. Notes (not in the diff)

- Per-op durable commits (fsync'd journal + one storage transaction per ACK)
  are intentionally untouched; they bound local drain speed, not wire usage.
- No in-flight timeout was added: duplicate sends after a lost ACK stay
  impossible by construction (completion frees the slot), and the server
  already dedups by op_id.

## 7. Addendum — 150 ms RTT head-to-head (Main's check)

Same 100-op `LatencyTransport` drain, delay raised to 150 ms per round trip
(temporary worktree edit, reverted; no committed churn). `QT_QPA_PLATFORM=offscreen`:

| build | max in flight | drain wall | composition |
|---|---|---|---|
| pre-fix (serial) | 1 | **20052 ms** | 100 × 150 ms RTT + ~5 s durable ack commits |
| post-fix (pipeline) | 4 | **7236 ms** | 25 flights × 150 ms + ~3.5 s durable ack commits |

The pipelining beats the serial baseline by **2.8× at 150 ms RTT**, exactly
the RTT/4 scaling the fix targets. (At 0 ms simulated RTT the local numbers
look flat — 4020 ms before vs ~3700 ms after — because per-op fsync'd ack
commits serialize on both builds; that cost is intentional durability, and on
the phone it is dwarfed by the wire RTT.)

Which mechanism caused the owner's 8-ops-per-90-minutes? **The stall, not the
serialisation.** One-op-per-RTT serialisation caps a working link at ~1 op/s
(the owner's own "one per second" observation) — over 90 minutes that is
thousands of ops, not 8. ~11 min/op means the pump was wedged, by two lines
in the pre-fix `qt/src/CardStore.cpp`:

- `1202: if (!m_online || !m_inFlight.isEmpty()) return;` — a single
  in-flight slot gates the entire queue, and
- the `staleOrder` early return (old `1290-1294`, after the three 08:54
  reorder rejections) exits `reportPostResult` **without freeing that slot**;
  the slot frees only when a fresh snapshot triggers retirement. Between
  snapshots — and behind the global retry-timer gate (old `1198-1199`) after
  any transient failure — nothing else could send.

The 5-op burst at 13:07 is the wedge clearing for a moment, then re-wedging.
This fix removes both: the pipeline keeps draining around a stuck slot, and
backoff parks one card instead of the whole queue.

## 8. Addendum — group-committed ACKs (gigabit drain)

Measured (temporary timers, removed): at ~1 ms RTT one ACK cost ~14 ms draft-journal
fsync (GUI thread) + ~21 ms SQLite commit ≈ 35 ms/op ⇒ ~29 ops/s local ceiling vs
~1000 ops/s the 1 ms wire allows per slot. Local commit dominates ~35:1.

Fix: ACKs queue and are committed as one batch (`CardStore::commitAcks`): one
journal append + one fsync, one SQLite transaction (same statements/order, outbox
row deleted only inside the committed transaction, ops hold their in-flight slot
until commit, so per-card order and no-ACK-for-retired-ops are unchanged); the
draft directory is fsynced once, not per append. `m_maxInFlight` 4 → 32 so batches
are large (cost: ≤32 outbox payloads on the wire; reorder ops still fly alone).

100/200 ops (outbox pre-durable, `drainBench`), wall ms (max in flight):

| case | before (cap 4) | after (cap 32) |
|---|---|---|
| 100 ops, 1 ms | 3347 (4) | 202-302 (32) |
| 200 ops, 1 ms | 6792 (4) | 452-573 (32) |
| 100 ops, 150 ms | 5797 (4) | 806-856 (32) |

Caveat: server limits 60 req/min per device (`auth.go` IPLimiter); a burst >60
POSTs gets 429 and parks the pump 60 s. A batch endpoint is needed for that.
