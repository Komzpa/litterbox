package agents

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func Upsert(ctx context.Context, db *sql.DB, tenantID string, cards []Card) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id', $1, true)`, tenantID); err != nil {
		return err
	}
	for _, c := range cards {
		if c.SortAt.IsZero() {
			c.SortAt = time.Now().UTC()
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cards (tenant_id,id,source,external_id,title,summary,sort_at,state) VALUES ($1,gen_random_uuid(),'agent',$2,$3,$4,$5,$6) ON CONFLICT (tenant_id,source,external_id) DO UPDATE SET title=EXCLUDED.title,summary=EXCLUDED.summary,sort_at=EXCLUDED.sort_at`, tenantID, c.ExternalID, c.Title, c.Summary, c.SortAt, c.State)
		if err != nil {
			return fmt.Errorf("upsert agent session %q: %w", c.ExternalID, err)
		}
	}
	return tx.Commit()
}

// Dismiss closes an agent card only in Litterbox; no session file is changed.
func Dismiss(ctx context.Context, db *sql.DB, tenantID, externalID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenantID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE cards SET state='done' WHERE tenant_id=$1 AND source='agent' AND external_id=$2`, tenantID, externalID); err != nil {
		return err
	}
	return tx.Commit()
}
