// Package bodyversion owns the database-derived identity of a reader body.
package bodyversion

// SQL is used with cards c and accounts a in scope. Message row identities and
// PostgreSQL row versions cover insertion, deletion, edits and label changes;
// the body row version covers its HTML. Source and URL inputs are included too.
// Ordering by id makes the aggregate deterministic. The value is opaque to clients.
const SQL = `md5(json_build_array(
 c.source, c.account_id::text, c.gmail_thread_id, a.address,
 (SELECT b.xmin::text FROM card_bodies b WHERE b.tenant_id=c.tenant_id AND b.card_id=c.id),
 (SELECT string_agg(m.id::text || ':' || m.xmin::text, ',' ORDER BY m.id)
    FROM messages m WHERE m.tenant_id=c.tenant_id AND m.card_id=c.id)
)::text)`
