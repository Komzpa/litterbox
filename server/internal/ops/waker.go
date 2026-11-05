package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WakeTenant processes one tenant under RLS. Provider failures leave the card
// due so the next pass retries it. Thread guards cover the provider and commit.
func (a API) WakeTenant(ctx context.Context, tenant uuid.UUID) error {
	read := a.reader(tenant.String())
	var ids []uuid.UUID
	err := read(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id FROM cards WHERE tenant_id=$1 AND state='snoozed' AND snooze_until<=now() ORDER BY snooze_until`, tenant)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		return err
	}
	var first error
	for _, id := range ids {
		if err := a.wakeCard(ctx, tenant, id); err != nil {
			log.Printf("wake card %s: %v", id, err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (a API) wakeCard(ctx context.Context, tenant, card uuid.UUID) error {
	read := a.reader(tenant.String())
	_, keys, err := resolveTargets(ctx, read, tenant, request{CardID: card})
	if err != nil {
		return err
	}
	unlock, err := lockThreads(ctx, keys)
	if err != nil {
		return err
	}
	defer unlock()
	var source string
	var until time.Time
	err = read(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT source,snooze_until FROM cards WHERE tenant_id=$1 AND id=$2 AND state='snoozed' AND snooze_until<=now()`, tenant, card).Scan(&source, &until)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if source == "mail" {
		pre, ok := lookupPreCommit("gmail.unsnooze")
		if !ok {
			return fmt.Errorf("Gmail wake operation unavailable")
		}
		if _, err = pre(ctx, read, tenant, card, json.RawMessage(`{}`)); err != nil {
			return err
		}
	}
	tx, err := a.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant.String()); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE cards SET state='open',snooze_until=NULL WHERE tenant_id=$1 AND id=$2 AND state='snoozed' AND snooze_until=$3`, tenant, card, until)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// StartWaker discovers tenants and retries due cards every 30 seconds.
func StartWaker(ctx context.Context, db *pgxpool.Pool) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			if err := wakeAll(ctx, db); err != nil && ctx.Err() == nil {
				log.Printf("snooze waker: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func wakeAll(ctx context.Context, db *pgxpool.Pool) error {
	rows, err := db.Query(ctx, `SELECT id FROM litterbox_worker_tenants()`)
	if err != nil {
		return err
	}
	var tenants []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		tenants = append(tenants, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	var first error
	for _, id := range tenants {
		if err := (API{DB: db}).WakeTenant(ctx, id); err != nil && first == nil {
			first = err
		}
	}
	return first
}
