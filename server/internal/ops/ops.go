// Package ops applies durable, idempotent offline actions from app outboxes.
// Handlers run inside the caller's transaction and share the cards table and
// change-notify trigger already owned by internal/cards; this package adds no
// second push path.
package ops

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/Komzpa/litterbox/server/internal/ingest"
	"github.com/Komzpa/litterbox/server/internal/journal"
	"github.com/google/uuid"
)

// Handler applies one operation type to a card inside tx. tenant and card are
// canonical string UUIDs, matching internal/cards' representation.
type Handler func(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cardID uuid.UUID, args json.RawMessage) error

var registry = struct {
	sync.RWMutex
	handlers map[string]Handler
}{handlers: make(map[string]Handler)}

// Register adds a named operation handler. It panics on an empty type, a nil
// handler, or a duplicate registration, since either indicates a programming
// error discoverable at startup.
func Register(typ string, h Handler) {
	if typ == "" || h == nil {
		panic("ops: type and handler are required")
	}
	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.handlers[typ]; exists {
		panic("ops: duplicate handler " + typ)
	}
	registry.handlers[typ] = h
}

func init() {
	Register("done", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		if err := applyCardUpdate(ctx, tx, tenant, card, true, ""); err != nil {
			return err
		}
		return ingest.OnDone(ctx, tx, tenant, card)
	})
	Register("archive", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) error {
		var source string
		if err := tx.QueryRow(ctx, `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND id=$2 RETURNING source`, tenant, card).Scan(&source); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("card not found")
			}
			return err
		}
		if source == "mail" {
			if err := CallIfRegistered(ctx, tx, tenant, card, "gmail.archive", args); err != nil {
				return err
			}
		}
		return ingest.OnDone(ctx, tx, tenant, card)
	})
	Register("note", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) error {
		var v struct {
			Note string `json:"note"`
		}
		if err := json.Unmarshal(args, &v); err != nil {
			return err
		}
		return applyCardUpdate(ctx, tx, tenant, card, false, v.Note)
	})
	Register("snooze", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) error {
		return bundles.Snooze(ctx, tx, tenant, card, args)
	})
	Register("pin", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		return bundles.Pin(ctx, tx, tenant, card)
	})
	Register("unpin", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, _ json.RawMessage) error {
		return bundles.Unpin(ctx, tx, tenant, card)
	})
	Register("reorder_pins", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		return bundles.Reorder(ctx, tx, tenant, args)
	})
	Register("reorder_cards", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		var in struct {
			Cards []uuid.UUID `json:"cards"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return err
		}
		if len(in.Cards) == 0 {
			return fmt.Errorf("cards required")
		}
		seen := make(map[uuid.UUID]bool, len(in.Cards))
		for i, id := range in.Cards {
			if id == uuid.Nil || seen[id] {
				return fmt.Errorf("invalid card order")
			}
			seen[id] = true
			tag, err := tx.Exec(ctx, `UPDATE cards SET note_order=$3 WHERE tenant_id=$1 AND id=$2 AND state='open'`, tenant, id, i)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("card not found")
			}
		}
		var openCount int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND state='open'`, tenant).Scan(&openCount); err != nil {
			return err
		}
		if openCount != len(in.Cards) {
			return fmt.Errorf("card order must include every open card")
		}
		return nil
	})
	Register("create_card", func(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) error {
		var in struct {
			Title   string `json:"title"`
			Summary string `json:"summary"`
			Kind    string `json:"kind"`
			Body    string `json:"body"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return err
		}
		in.Title = strings.TrimSpace(in.Title)
		if in.Title == "" {
			return fmt.Errorf("title required")
		}
		source := "manual"
		switch in.Kind {
		case "", "manual":
		case "journal":
			if strings.TrimSpace(in.Body) == "" {
				return fmt.Errorf("journal body required")
			}
			source = "journal"
			if _, err := journal.CreateInTx(ctx, tx, tenant.String(), card.String(), in.Body); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown card kind")
		}
		_, err := tx.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,external_id,title,summary,state) VALUES($1,$2,NULL,NULL,$3,$4,$5,$6,'open')`, tenant, card, source, card.String(), in.Title, in.Summary)
		return err
	})
	Register("bundle_archive", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		ids, err := bundleOpenUnpinnedIDs(ctx, tx, tenant, args)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return fmt.Errorf("bundle has no open unpinned cards")
		}
		// Reroute through the per-card archive op so every member leaves
		// Gmail's INBOX exactly like a single-card archive.
		for _, id := range ids {
			if err := CallIfRegistered(ctx, tx, tenant, id, "archive", args); err != nil {
				return err
			}
		}
		return nil
	})
	Register("bundle_done", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		if err := enqueueBundleDone(ctx, tx, tenant, args); err != nil {
			return err
		}
		return bundles.CompleteOperation(ctx, tx, tenant, args)
	})
	Register("take_out", func(ctx context.Context, tx pgx.Tx, tenant, _ uuid.UUID, args json.RawMessage) error {
		return bundles.TakeOutOperation(ctx, tx, tenant, args)
	})
	bundles.SetOperationHook(CallIfRegistered)
}

// bundleOpenUnpinnedIDs lists the open, unpinned members of the target bundle.
func bundleOpenUnpinnedIDs(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, raw json.RawMessage) ([]uuid.UUID, error) {
	var args bundles.ArchiveArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM cards WHERE tenant_id=$1 AND bundle_id=$2 AND state='open' AND pinned_rank IS NULL`, tenant, args.Bundle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func enqueueBundleDone(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, raw json.RawMessage) error {
	ids, err := bundleOpenUnpinnedIDs(ctx, tx, tenant, raw)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := ingest.OnDone(ctx, tx, tenant, id); err != nil {
			return err
		}
	}
	return nil
}

func applyCardUpdate(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, done bool, note string) error {
	var tag pgconn.CommandTag
	var err error
	if done {
		tag, err = tx.Exec(ctx, `UPDATE cards SET state='done' WHERE tenant_id=$1 AND id=$2`, tenant, card)
	} else {
		tag, err = tx.Exec(ctx, `UPDATE cards SET note=$1 WHERE tenant_id=$2 AND id=$3`, note, tenant, card)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("card not found")
	}
	return nil
}

func mustUUID(value string) uuid.UUID { id, _ := uuid.Parse(value); return id }

// Identity resolves the authenticated tenant and device for the request. No
// request payload or header may override the identity middleware attaches.
type Identity func(ctx context.Context) (tenant, device string, ok bool)

// API serves POST /v1/ops. DB is the shared *sql.DB also used by internal/cards.
type API struct {
	DB       *pgxpool.Pool
	Identity Identity
}

type request struct {
	OpID   uuid.UUID       `json:"op_id"`
	CardID uuid.UUID       `json:"card_id"`
	Type   string          `json:"type"`
	Args   json.RawMessage `json:"args"`
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a.DB == nil || a.Identity == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	tenant, device, ok := a.Identity(r.Context())
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var in request
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&in); err != nil || in.OpID == uuid.Nil || in.CardID == uuid.Nil || in.Type == "" || len(in.Args) == 0 {
		http.Error(w, "invalid operation", http.StatusBadRequest)
		return
	}
	registry.RLock()
	handler, known := registry.handlers[in.Type]
	registry.RUnlock()
	if !known {
		http.Error(w, "unknown operation", http.StatusBadRequest)
		return
	}
	payload, _ := json.Marshal(in)
	hash := sha256.Sum256(payload)

	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	var prior []byte
	err = tx.QueryRow(r.Context(), `SELECT payload_hash FROM ops WHERE tenant_id=$1 AND op_id=$2`, tenant, in.OpID).Scan(&prior)
	switch {
	case err == nil:
		if string(prior) != string(hash[:]) {
			http.Error(w, "op_id reused with different payload", http.StatusConflict)
			return
		}
		// Same op_id and payload: already applied. Fall through to commit so
		// the response is identical to the original attempt without redoing
		// the handler's side effects.
	case errors.Is(err, pgx.ErrNoRows):
		var deviceArg any
		if device != "" {
			deviceArg = mustUUID(device)
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO ops(tenant_id,op_id,device_id,payload_hash,payload) VALUES($1,$2,$3,$4,$5)`, tenant, in.OpID, deviceArg, hash[:], payload); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		if err = handler(r.Context(), tx, mustUUID(tenant), in.CardID, in.Args); err != nil {
			log.Printf("operation failed: type=%s card_id=%s error=%v", in.Type, in.CardID, err)
			http.Error(w, "operation failed", http.StatusUnprocessableEntity)
			return
		}
	default:
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// CallIfRegistered invokes an optional handler when one exists.
func CallIfRegistered(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, name string, args json.RawMessage) error {
	registry.RLock()
	h, ok := registry.handlers[name]
	registry.RUnlock()
	if !ok {
		return nil
	}
	return h(ctx, tx, tenant, card, args)
}
