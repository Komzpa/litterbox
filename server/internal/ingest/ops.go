package ingest

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OnDone enqueues the source action transactionally with the card operation.
func OnDone(ctx context.Context, tx pgx.Tx, tenant, cardID uuid.UUID) error {
	_, err := tx.Exec(ctx, `INSERT INTO source_action_callbacks(tenant_id,card_id,source,callback_url,action)
SELECT c.tenant_id,c.id,c.source,st.callback_url,jsonb_build_object('type','done','external_id',c.external_id,'kind',c.source_kind,'actions',c.source_actions)
FROM cards c JOIN source_tokens st ON st.tenant_id=c.tenant_id AND st.source=c.source
WHERE c.tenant_id=$1 AND c.id=$2 AND c.source = 'ha' AND st.callback_url IS NOT NULL AND st.revoked_at IS NULL`, tenant, cardID)
	return err
}
