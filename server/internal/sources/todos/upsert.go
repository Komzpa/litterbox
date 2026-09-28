package todos

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Upsert writes parsed todo cards while preserving user-owned state and notes.
func Upsert(ctx context.Context, db *sql.DB, tenant string, cards []Card) error {
	if _, err := uuid.Parse(tenant); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		return err
	}
	for _, card := range cards {
		state := card.State
		if state == "" {
			state = "open"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,title,state,note_order,at,timed)
VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(tenant_id,source,external_id) DO UPDATE SET title=EXCLUDED.title,note_order=EXCLUDED.note_order,at=EXCLUDED.at,timed=EXCLUDED.timed`, tenant, card.Source, card.ExternalID, card.Title, state, card.Order, card.At, card.Timed)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
