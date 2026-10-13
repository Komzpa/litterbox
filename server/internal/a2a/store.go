// Package a2a stores the tenant-scoped projection of the canonical a2a journal:
// one thread per peer and one message row per journal event_key.
package a2a

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Thread is the device-facing thread summary.
type Thread struct {
	ID           string     `json:"id"`
	PeerID       string     `json:"peer_id"`
	DisplayName  string     `json:"display_name"`
	Kind         string     `json:"kind"`
	Availability string     `json:"availability"`
	LastText     *string    `json:"last_text"`
	LastAt       *time.Time `json:"last_at"`
	LastSeq      int64      `json:"last_seq"`
}

// Message is the device-facing message row. Seq advances on insert and on
// every delivery-state change, so clients page with after=<seq>.
type Message struct {
	ID            string    `json:"id"`
	ThreadID      string    `json:"thread_id"`
	Seq           int64     `json:"seq"`
	Direction     string    `json:"direction"`
	Sender        string    `json:"sender"`
	Recipient     string    `json:"recipient"`
	Text          string    `json:"text"`
	DeliveryState string    `json:"delivery_state"`
	ReasonCode    *string   `json:"reason_code"`
	ClientMsgID   *string   `json:"client_msg_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// ThreadInput describes a peer to upsert.
type ThreadInput struct {
	PeerID       string
	DisplayName  string
	Kind         string
	Availability string
}

// MessageInput is one journal event projected into a thread.
type MessageInput struct {
	ThreadID      string
	EventKey      string
	Direction     string
	Sender        string
	Recipient     string
	Text          string
	DeliveryState string
	ReasonCode    *string
	ClientMsgID   *string
}

// Store reads and writes a2a rows under row-level security.
type Store struct {
	DB *sql.DB
}

func (s *Store) inTenant(ctx context.Context, tenant string, write bool, fn func(*sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: !write})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

const threadColumns = `id, peer_id, display_name, kind, availability, last_text, last_at, last_seq`

type scanner interface{ Scan(...any) error }

func scanThread(r scanner) (Thread, error) {
	var t Thread
	err := r.Scan(&t.ID, &t.PeerID, &t.DisplayName, &t.Kind, &t.Availability, &t.LastText, &t.LastAt, &t.LastSeq)
	return t, err
}

const messageColumns = `id, thread_id, seq, direction, sender, recipient, text, delivery_state, reason_code, client_msg_id::text, created_at`

func scanMessage(r scanner) (Message, error) {
	var m Message
	err := r.Scan(&m.ID, &m.ThreadID, &m.Seq, &m.Direction, &m.Sender, &m.Recipient, &m.Text, &m.DeliveryState, &m.ReasonCode, &m.ClientMsgID, &m.CreatedAt)
	return m, err
}

// UpsertThread creates the peer's single thread or refreshes its name, kind
// and availability.
func (s *Store) UpsertThread(ctx context.Context, tenant string, in ThreadInput) (Thread, error) {
	var t Thread
	err := s.inTenant(ctx, tenant, true, func(tx *sql.Tx) error {
		var err error
		t, err = scanThread(tx.QueryRowContext(ctx, `
INSERT INTO a2a_threads (tenant_id, peer_id, display_name, kind, availability)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, peer_id) DO UPDATE
SET display_name = EXCLUDED.display_name,
    kind = EXCLUDED.kind,
    availability = EXCLUDED.availability,
    updated_at = CASE WHEN (a2a_threads.display_name, a2a_threads.kind, a2a_threads.availability)
        IS DISTINCT FROM (EXCLUDED.display_name, EXCLUDED.kind, EXCLUDED.availability)
        THEN now() ELSE a2a_threads.updated_at END
RETURNING `+threadColumns, tenant, in.PeerID, in.DisplayName, in.Kind, in.Availability))
		return err
	})
	return t, err
}

// IngestMessage upserts a journal event by (thread, event_key). Re-ingesting
// the same event creates no duplicate and leaves seq alone unless the delivery
// state or reason changed.
func (s *Store) IngestMessage(ctx context.Context, tenant string, in MessageInput) (Message, error) {
	var m Message
	err := s.inTenant(ctx, tenant, true, func(tx *sql.Tx) error {
		var err error
		m, err = scanMessage(tx.QueryRowContext(ctx, `
INSERT INTO a2a_messages (tenant_id, thread_id, event_key, direction, sender, recipient, text, delivery_state, reason_code, client_msg_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::uuid)
ON CONFLICT (tenant_id, thread_id, event_key) DO UPDATE
SET delivery_state = EXCLUDED.delivery_state,
    reason_code = EXCLUDED.reason_code
RETURNING `+messageColumns, tenant, in.ThreadID, in.EventKey, in.Direction, in.Sender, in.Recipient, in.Text, in.DeliveryState, in.ReasonCode, in.ClientMsgID))
		return err
	})
	return m, err
}

// CreateOutbound stores a device-composed message as pending. The same
// client_msg_id in the same thread returns the existing row unchanged.
func (s *Store) CreateOutbound(ctx context.Context, tenant, threadID, sender, recipient, text, clientMsgID string) (Message, error) {
	var m Message
	err := s.inTenant(ctx, tenant, true, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
INSERT INTO a2a_messages (tenant_id, thread_id, event_key, direction, sender, recipient, text, delivery_state, client_msg_id)
VALUES ($1, $2, 'litterbox:' || $3::text, 'out', $4, $5, $6, 'pending', $3::uuid)
ON CONFLICT (tenant_id, thread_id, client_msg_id) WHERE client_msg_id IS NOT NULL DO NOTHING`,
			tenant, threadID, clientMsgID, sender, recipient, text)
		if err != nil {
			return err
		}
		m, err = scanMessage(tx.QueryRowContext(ctx, `SELECT `+messageColumns+` FROM a2a_messages
WHERE thread_id = $1 AND client_msg_id = $2::uuid`, threadID, clientMsgID))
		return err
	})
	return m, err
}

// ListThreads returns every thread of the tenant, newest activity first.
func (s *Store) ListThreads(ctx context.Context, tenant string) ([]Thread, error) {
	out := []Thread{}
	err := s.inTenant(ctx, tenant, false, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT `+threadColumns+` FROM a2a_threads
ORDER BY last_at DESC NULLS LAST, display_name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			t, err := scanThread(rows)
			if err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

// ListMessages returns up to limit rows of a thread with seq > after, in seq
// order.
func (s *Store) ListMessages(ctx context.Context, tenant, threadID string, after int64, limit int) ([]Message, error) {
	if limit <= 0 {
		return nil, errors.New("a2a: limit must be positive")
	}
	out := []Message{}
	err := s.inTenant(ctx, tenant, false, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT `+messageColumns+` FROM a2a_messages
WHERE thread_id = $1 AND seq > $2 ORDER BY seq LIMIT $3`, threadID, after, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			m, err := scanMessage(rows)
			if err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	return out, err
}
