# a2a bridge API (2026-10-13)

The canonical text log is `hermes_a2a_journal.py`. Litterbox is its tenant-scoped projection, not a second journal. Bridge routes accept only a non-revoked source token whose source is `a2a`; device tokens and other source tokens receive 401. Bearers belong in Authorization headers, never payloads, argv or logs.

## Ingest

`POST /v1/a2a/ingest` accepts `{ "events": [...], "recipients": [...] }` and returns 204. Each optional array is limited to 200 items. An event is exactly a canonical `events_after` row: `seq`, `event_key`, `kind` (`sent`, `delivery`, `reply`), `sender`, `recipient`, `conversation_id`, `request_id`, nullable `text`, `redacted`, `status`, nullable `reason_code`, `recorded_at`. Reply addressing is reversed from the original request: the answering agent is sender. Events must arrive in journal order, including sent before delivery/reply. Recipients are `{peer_id,display_name,kind,availability}` with kind `token_agent` or `omp_session`, and availability `online`, `offline`, `busy` or `unknown`.

Canonical journal statuses `submitted`, `queued`, and `indeterminate` map to projection delivery state `sent`; `started` maps to `delivered`. Native journal events remain unchanged; unknown statuses are rejected.

A batch commits atomically. One peer maps to one thread. `agent:darafei` conversations project into the other participant's thread; agent-to-agent conversations appear as observed in both participants' threads. Recipient metadata refreshes existing threads; unspecified metadata is preserved. Internally the event identity is namespaced by request_id to correlate subsequent delivery/reply rows. Replaying the same sent/reply event does not duplicate messages or regress delivery state. A completed reply marks the outgoing request replied and adds an incoming row in the same transaction/thread. Delivery events update the original sent row, rather than creating empty bubbles.

## Outbox

`GET /v1/a2a/outbox?limit=200` returns `{ "messages": [...] }`, using the frozen device Message shape. Only pending device-composed outbound messages with a `client_msg_id` are returned, in seq order. Limit must be 1–200. Reads do not claim messages: the bridge persists its own state and must use the deterministic a2a request ID `litterbox-<client_msg_id>` for sends/retries. Ingest recognizes that request ID and updates the existing composer row rather than duplicating it.

`POST /v1/a2a/outbox/{message-id}/result` accepts `{ "status": "failed", "reason_code": "recipient_busy" }` and returns the Message row. Status mapping: pending/sent → sent, delivered → delivered, failed → not_delivered, completed → replied. A busy failure therefore displays not_delivered/recipient_busy. Repeated terminal receipts are idempotent. Terminal rows cannot be regressed by an older receipt or journal sent event. The bridge must persist pending work and poll a2a receipts until completed or failed; a sent row no longer appears in the pending outbox. Completed text comes from the subsequent canonical reply event, not from the receipt body. Missing or cross-tenant message IDs return 404.

## Scoped acceptance command

Run only against disposable infrastructure:

```sh
cd server
pg_virtualenv bash -c 'unset DATABASE_URL; createdb litterbox_test_bridge && PGDATABASE=litterbox_test_bridge CARD_TEST_POSTGRES=1 go test ./internal/a2a -run "^TestBridge" -v -count=1'
```

The neighboring auth suite checks that existing card/device/producer route behavior remains intact:

```sh
go test ./internal/auth -count=1
```
