package gmailsync

import (
	"context"
	"github.com/Komzpa/litterbox/server/internal/ops"
)

// Reconcile each legacy archived row once from provider metadata, never from
// stored message labels. Failures retain the unchecked row for the next poll.
func (s *Syncer) reconcileLabelState(ctx context.Context, a Account, c *Client) error {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT gmail_thread_id FROM cards WHERE tenant_id=$1 AND account_id=$2 AND source='mail' AND state='archived' AND NOT gmail_state_checked ORDER BY id LIMIT 50`, a.TenantID, a.ID)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			tx.Rollback(ctx)
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	tx.Rollback(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.confirmLabelState(ctx, a, c, id, "legacy_provider_reconciliation"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Syncer) confirmLabelState(ctx context.Context, a Account, c *Client, id, reason string) error {
	unlock, err := ops.LockThread(ctx, a.TenantID, a.ID, id)
	if err != nil {
		return err
	}
	defer unlock()
	t, err := c.GetThreadMetadata(ctx, id)
	if err != nil && !isNotFound(err) {
		return err
	}
	inbox := threadInInbox(t)
	if isNotFound(err) {
		reason += "_not_found"
	} else if inbox {
		reason += "_inbox"
	} else {
		reason += "_no_inbox"
	}
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE cards SET state=CASE WHEN state='done' THEN state WHEN gmail_pending_inbox=false AND $4 THEN state WHEN $4 THEN 'open' WHEN state='snoozed' THEN state ELSE 'archived' END,gmail_pending_inbox=CASE WHEN gmail_pending_inbox=$4 THEN NULL ELSE gmail_pending_inbox END,gmail_provider_inbox=$4,gmail_state_checked=true,gmail_state_reason=$5,version=version+1 WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id=$3`, a.TenantID, a.ID, id, inbox, reason)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func threadInInbox(t Thread) bool {
	for _, m := range t.Messages {
		for _, label := range m.LabelIDs {
			if label == "INBOX" {
				return true
			}
		}
	}
	return false
}
