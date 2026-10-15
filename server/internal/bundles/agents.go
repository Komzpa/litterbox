package bundles

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AgentDigestKey and AgentDigestTitle name the one collapsible section that
// holds open agent cards without a source mail (R66), instead of interleaving
// them with mail by time.
const (
	AgentDigestKey   = "agent:digest"
	AgentDigestTitle = "Agent digest"
)

// assignAgentDigest bundles every open, unbundled agent card into the agent
// digest when at least two exist; a lone agent card stays standalone and a
// recorded take-out of the digest is honoured. Cards already placed in another
// bundle (a future source group) are never moved.
func assignAgentDigest(ctx context.Context, tx pgx.Tx, tenant uuid.UUID) error {
	var digest *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM bundles WHERE tenant_id=$1 AND bundle_key=$2`, tenant, AgentDigestKey).Scan(&digest); err != nil && err != pgx.ErrNoRows {
		return err
	}
	var excluded bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND bundle_key=$2)`, tenant, AgentDigestKey).Scan(&excluded); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND source='agent' AND state='open' AND (bundle_id IS NULL OR bundle_id IS NOT DISTINCT FROM $2)`, tenant, digest).Scan(&count); err != nil {
		return err
	}
	if excluded || count < 2 {
		if digest != nil {
			_, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND bundle_id=$2`, tenant, *digest)
			return err
		}
		return nil
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx, upsertTitle, tenant, AgentDigestTitle, AgentDigestKey, "topic").Scan(&id); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$2 WHERE tenant_id=$1 AND source='agent' AND state='open' AND bundle_id IS NULL`, tenant, id)
	return err
}
