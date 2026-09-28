package agents

import (
 "context"
 "database/sql"
)

// Dismiss closes an agent or todo card only in Litterbox; no source file is changed.
func Dismiss(ctx context.Context, db *sql.DB, tenantID, externalID string) error {
 tx, err := db.BeginTx(ctx, nil)
 if err != nil { return err }
 defer tx.Rollback()
 if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenantID); err != nil { return err }
 if _, err = tx.ExecContext(ctx, `UPDATE cards SET state='done' WHERE tenant_id=$1 AND source IN ('agent','todo') AND external_id=$2`, tenantID, externalID); err != nil { return err }
 return tx.Commit()
}
