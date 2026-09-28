// Package bundles provides sender-based bundle assignment and bundle actions.
package bundles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SenderKey creates a stable grouping key from an address. The full address
// keeps distinct senders separate; domain and List-ID can be passed as keys by
// the classifier when constructing a broader bundle.
func SenderKey(sender string) string {
	a, err := mail.ParseAddress(strings.TrimSpace(sender))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(sender))
	}
	return strings.ToLower(a.Address)
}

// Assign clusters new mail by sender, domain and list-id, unless the sender was
// explicitly corrected out of that bundle. Importance is deliberately separate.
func Assign(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, sender, domain, listID, importance string) error {
	senderKey := SenderKey(sender)
	key := "sender:" + senderKey
	if listID != "" {
		key = "list:" + strings.ToLower(strings.Trim(listID, "<> "))
	} else if domain != "" {
		key = "domain:" + strings.ToLower(domain)
	}
	var excluded bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND sender_key=$2 AND bundle_key=$3)`, tenant, senderKey, key).Scan(&excluded); err != nil {
		return err
	}
	var bundleID any
	if !excluded {
		if err := tx.QueryRow(ctx, `INSERT INTO bundles (tenant_id,id,title,centroid,bundle_key) VALUES ($1,gen_random_uuid(),$2,'{}',$3) ON CONFLICT (tenant_id,bundle_key) WHERE bundle_key IS NOT NULL DO UPDATE SET title=EXCLUDED.title RETURNING id`, tenant, key, key).Scan(&bundleID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$3, importance=COALESCE(NULLIF($4,''),'normal') WHERE tenant_id=$1 AND id=$2`, tenant, card, bundleID, importance)
	return err
}

// TakeOut permanently records the sender's correction against its current bundle.
func TakeOut(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID) error {
	var sender, key string
	if err := tx.QueryRow(ctx, `SELECT c.sender,COALESCE(b.bundle_key,'') FROM cards c LEFT JOIN bundles b ON b.tenant_id=c.tenant_id AND b.id=c.bundle_id WHERE c.tenant_id=$1 AND c.id=$2 FOR UPDATE OF c`, tenant, card).Scan(&sender, &key); err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO bundle_exclusions (tenant_id,sender_key,bundle_key) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, tenant, SenderKey(sender), key)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2`, tenant, card)
	return err
}

// SnoozeArgs is the wire payload for a snooze operation.
type SnoozeArgs struct {
	Until string `json:"until"`
}

// Snooze moves a card out of the open list until its requested time.
func Snooze(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, raw json.RawMessage) error {
	var args SnoozeArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE cards SET state='snoozed',snooze_until=$3::timestamptz WHERE tenant_id=$1 AND id=$2 AND $3::timestamptz > now()`, tenant, card, args.Until)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("card missing or snooze time is not in the future")
	}
	var source string
	if err = tx.QueryRow(ctx, `SELECT source FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&source); err != nil {
		return err
	}
	if source == "mail" {
		return ops.CallIfRegistered(ctx, tx, tenant, card, "gmail.snooze_label", raw)
	}
	return nil
}

// WakeDueCards wakes expired snoozes; caller schedules this at least once a minute.
func WakeDueCards(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, `UPDATE cards SET state='open',snooze_until=NULL WHERE state='snoozed' AND snooze_until<=now()`)
	return tag.RowsAffected(), err
}

// WakeThread wakes a snoozed card when a new thread message is ingested.
func WakeThread(ctx context.Context, tx pgx.Tx, tenant, account uuid.UUID, threadID string) error {
	_, err := tx.Exec(ctx, `UPDATE cards SET state='open',snooze_until=NULL WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id=$3 AND state='snoozed'`, tenant, account, threadID)
	return err
}

// ArchiveBundle archives all open bundle cards except pinned cards atomically.
func ArchiveBundle(ctx context.Context, tx pgx.Tx, tenant, bundle uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND bundle_id=$2 AND state='open' AND pinned_rank IS NULL`, tenant, bundle)
	return err
}

// ReorderPins applies a complete ordered list of pinned card UUIDs.
func ReorderPins(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	for i, id := range ids {
		tag, err := tx.Exec(ctx, `UPDATE cards SET pinned_rank=$3 WHERE tenant_id=$1 AND id=$2 AND pinned_rank IS NOT NULL`, tenant, id, i+1)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("card %s is not pinned", id)
		}
	}
	return nil
}

// Pin marks a card pinned, assigning its rank after existing pins.
func Pin(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID) error {
	tag, err := tx.Exec(ctx, `UPDATE cards SET pinned_rank=COALESCE((SELECT max(pinned_rank)+1 FROM cards WHERE tenant_id=$1),1) WHERE tenant_id=$1 AND id=$2 AND state='open'`, tenant, card)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	var source string
	if err = tx.QueryRow(ctx, `SELECT source FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&source); err != nil {
		return err
	}
	if source == "mail" {
		return ops.CallIfRegistered(ctx, tx, tenant, card, "gmail.star", json.RawMessage(`{"starred":true}`))
	}
	return nil
}

// Unpin removes a card from the pinned order.
func Unpin(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID) error {
	tag, err := tx.Exec(ctx, `UPDATE cards SET pinned_rank=NULL WHERE tenant_id=$1 AND id=$2`, tenant, card)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	var source string
	if err = tx.QueryRow(ctx, `SELECT source FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&source); err != nil {
		return err
	}
	if source == "mail" {
		return ops.CallIfRegistered(ctx, tx, tenant, card, "gmail.star", json.RawMessage(`{"starred":false}`))
	}
	return nil
}

type ReorderArgs struct {
	Cards []uuid.UUID `json:"cards"`
}

func Reorder(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, raw json.RawMessage) error {
	var args ReorderArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	return ReorderPins(ctx, tx, tenant, args.Cards)
}

type TakeOutArgs struct {
	Card uuid.UUID `json:"card"`
}

func TakeOutOperation(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, raw json.RawMessage) error {
	var args TakeOutArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	return TakeOut(ctx, tx, tenant, args.Card)
}

type ArchiveArgs struct {
	Bundle uuid.UUID `json:"bundle_id"`
}

func ArchiveOperation(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, raw json.RawMessage) error {
	var args ArchiveArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	return ArchiveBundle(ctx, tx, tenant, args.Bundle)
}

func init() {
	ops.Register("snooze", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) error {
		return Snooze(ctx, tx, tenant, card, args)
	})
	ops.Register("pin", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		return Pin(ctx, tx, tenant, card)
	})
	ops.Register("unpin", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		return Unpin(ctx, tx, tenant, card)
	})
	ops.Register("reorder_pins", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		return Reorder(ctx, tx, tenant, args)
	})
	ops.Register("bundle_archive", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		return ArchiveOperation(ctx, tx, tenant, args)
	})
	ops.Register("take_out", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		return TakeOutOperation(ctx, tx, tenant, args)
	})
}
