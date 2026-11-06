BEGIN READ ONLY;
SELECT now() AS as_of;
WITH candidates AS (
 SELECT c.*,
 EXISTS(SELECT 1 FROM messages m WHERE m.tenant_id=c.tenant_id AND m.card_id=c.id AND 'INBOX'=ANY(m.labels)) AS stored_inbox,
 EXISTS(SELECT 1 FROM ops o WHERE o.tenant_id=c.tenant_id AND o.payload->>'card_id'=c.id::text AND o.payload->>'type' IN ('archive','spam')) AS local_receipt
 FROM cards c WHERE c.source='mail' AND c.state='archived'
)
SELECT count(*) AS all_archived_checks,
 count(*) FILTER(WHERE stored_inbox) AS q8,
 count(*) FILTER(WHERE stored_inbox AND local_receipt) AS q8_local_receipt,
 count(*) FILTER(WHERE stored_inbox AND NOT local_receipt) AS q8_no_receipt,
 count(*) FILTER(WHERE NOT stored_inbox) AS no_stored_inbox
FROM candidates;
SELECT count(*) AS nonempty_notes FROM cards WHERE note<>'';
SELECT count(*) AS changed_journals FROM journal_entries WHERE updated_at>created_at;
COMMIT;
