package a2a

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

const humanPeer = "agent:darafei"

// JournalEvent mirrors the canonical hermes_a2a_journal events_after row.
// Identity is namespaced by request_id internally so delivery/reply events can
// locate their sent row without duplicating the canonical text journal.
type JournalEvent struct {
	Seq int64 `json:"seq"`
	EventKey string `json:"event_key"`
	Kind string `json:"kind"`
	Sender string `json:"sender"`
	Recipient string `json:"recipient"`
	ConversationID string `json:"conversation_id"`
	RequestID string `json:"request_id"`
	Text *string `json:"text"`
	Redacted int `json:"redacted"`
	Status string `json:"status"`
	ReasonCode *string `json:"reason_code"`
	RecordedAt string `json:"recorded_at"`
}

type BridgeRecipient struct {
	PeerID string `json:"peer_id"`
	DisplayName string `json:"display_name"`
	Kind string `json:"kind"`
	Availability string `json:"availability"`
}

type BridgeHandler struct { Store *Store }

// NewBridgeHandler authenticates every bridge request with a source=a2a token.
// The pool belongs to the server; the SQL adapter does not own its connections.
func NewBridgeHandler(pool *pgxpool.Pool) http.Handler {
	return &BridgeHandler{Store: &Store{DB: stdlib.OpenDBFromPool(pool)}}
}

func (h *BridgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))))
	var tenant, source string
	if err := h.Store.DB.QueryRowContext(r.Context(), `SELECT tenant_id::text,source FROM litterbox_source_by_token($1)`, hash[:]).Scan(&tenant, &source); err != nil || source != "a2a" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case r.URL.Path == "/v1/a2a/ingest" && r.Method == http.MethodPost:
		h.ingest(w, r, tenant)
	case r.URL.Path == "/v1/a2a/outbox" && r.Method == http.MethodGet:
		h.outbox(w, r, tenant)
	case strings.HasPrefix(r.URL.Path, "/v1/a2a/outbox/") && strings.HasSuffix(r.URL.Path, "/result") && r.Method == http.MethodPost:
		h.result(w, r, tenant)
	default:
		http.NotFound(w, r)
	}
}

func decodeBridge(w http.ResponseWriter, r *http.Request, dst any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func bridgeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

func deliveryState(status string) (string, bool) {
	switch status {
	case "", "pending", "sent", "submitted": return "sent", true
	case "delivered": return "delivered", true
	case "failed": return "not_delivered", true
	case "completed": return "replied", true
	default: return "", false
	}
}

func (h *BridgeHandler) ingest(w http.ResponseWriter, r *http.Request, tenant string) {
	var req struct {
		Events []JournalEvent `json:"events"`
		Recipients []BridgeRecipient `json:"recipients"`
	}
	if !decodeBridge(w, r, &req) { return }
	if len(req.Events) > 200 || len(req.Recipients) > 200 { http.Error(w, "batch too large", 400); return }
	for _, p := range req.Recipients {
		if p.PeerID == "" || p.DisplayName == "" || (p.Kind != "token_agent" && p.Kind != "omp_session") || (p.Availability != "online" && p.Availability != "offline" && p.Availability != "busy" && p.Availability != "unknown") {
			http.Error(w, "invalid recipient", 400); return
		}
	}
	for _, e := range req.Events {
		_, validStatus := deliveryState(e.Status)
		if e.EventKey == "" || e.RequestID == "" || e.Sender == "" || e.Recipient == "" || (e.Kind != "sent" && e.Kind != "delivery" && e.Kind != "reply") || !validStatus {
			http.Error(w, "invalid event", 400); return
		}
	}
	err := h.Store.inTenant(r.Context(), tenant, true, func(tx *sql.Tx) error {
		for _, p := range req.Recipients {
			if _, err := bridgeThread(r.Context(), tx, tenant, p); err != nil { return err }
		}
		for _, e := range req.Events {
			if err := projectEvent(r.Context(), tx, tenant, e); err != nil { return err }
		}
		return nil
	})
	if err != nil { http.Error(w, "ingest failed", 500); return }
	w.WriteHeader(http.StatusNoContent)
}

func bridgeThread(ctx context.Context, tx *sql.Tx, tenant string, p BridgeRecipient) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `INSERT INTO a2a_threads(tenant_id,peer_id,display_name,kind,availability)
VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,peer_id) DO UPDATE
SET display_name=EXCLUDED.display_name,kind=EXCLUDED.kind,availability=EXCLUDED.availability
WHERE (a2a_threads.display_name,a2a_threads.kind,a2a_threads.availability) IS DISTINCT FROM (EXCLUDED.display_name,EXCLUDED.kind,EXCLUDED.availability)
RETURNING id::text`, tenant, p.PeerID, p.DisplayName, p.Kind, p.Availability).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) { err = tx.QueryRowContext(ctx, `SELECT id::text FROM a2a_threads WHERE peer_id=$1`, p.PeerID).Scan(&id) }
	return id, err
}

func eventThread(ctx context.Context, tx *sql.Tx, tenant, peer string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id::text FROM a2a_threads WHERE peer_id=$1`, peer).Scan(&id)
	if !errors.Is(err, sql.ErrNoRows) { return id, err }
	kind := "omp_session"
	if strings.HasPrefix(peer, "agent:") { kind = "token_agent" }
	return bridgeThread(ctx, tx, tenant, BridgeRecipient{peer, peer, kind, "unknown"})
}

func projectEvent(ctx context.Context, tx *sql.Tx, tenant string, e JournalEvent) error {
	peers := []string{e.Sender, e.Recipient}
	if e.Sender == humanPeer { peers = []string{e.Recipient} } else if e.Recipient == humanPeer { peers = []string{e.Sender} }
	if len(peers) == 2 && peers[0] == peers[1] { peers = peers[:1] }
	state, _ := deliveryState(e.Status)
	for _, peer := range peers {
		thread, err := eventThread(ctx, tx, tenant, peer)
		if err != nil { return err }
		sender, recipient := e.Sender, e.Recipient
		if e.Kind == "reply" { sender, recipient = recipient, sender }
		prefix := e.RequestID + ":"
		client := ""
		if strings.HasPrefix(e.RequestID, "litterbox-") {
			if id, err := uuid.Parse(strings.TrimPrefix(e.RequestID, "litterbox-")); err == nil { client = id.String() }
		}
		// A composer message already exists. Journal sent/delivery events update
		// it instead of creating a second outgoing bubble. Terminal states never
		// regress when an older journal event is replayed after a result receipt.
		var updated int64
		if client != "" || e.Kind != "sent" {
			res, err := tx.ExecContext(ctx, `UPDATE a2a_messages SET delivery_state=$1,reason_code=$2
WHERE thread_id=$3 AND sender=$4 AND recipient=$5 AND direction IN ('out','observed')
AND ((client_msg_id IS NOT NULL AND client_msg_id::text=$6) OR left(event_key,length($7))=$7)
AND delivery_state NOT IN ('replied','not_delivered')`, state, e.ReasonCode, thread, sender, recipient, client, prefix)
			if err != nil { return err }
			updated, _ = res.RowsAffected()
			if e.Kind != "reply" && (updated > 0 || e.Kind == "delivery" || client != "") { continue }
		}
		if e.Kind == "delivery" { continue }
		direction := "observed"
		if e.Sender == humanPeer { direction = "out" } else if e.Recipient == humanPeer { direction = "in" }
		text := ""
		if e.Text != nil { text = *e.Text }
		if _, err := tx.ExecContext(ctx, `INSERT INTO a2a_messages(tenant_id,thread_id,event_key,direction,sender,recipient,text,delivery_state,reason_code)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(tenant_id,thread_id,event_key) DO NOTHING`, tenant, thread, prefix+e.EventKey, direction, e.Sender, e.Recipient, text, state, e.ReasonCode); err != nil { return err }
	}
	return nil
}

func (h *BridgeHandler) outbox(w http.ResponseWriter, r *http.Request, tenant string) {
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 { http.Error(w, "invalid limit", 400); return }
	}
	messages := []Message{}
	err := h.Store.inTenant(r.Context(), tenant, false, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(r.Context(), `SELECT `+messageColumns+` FROM a2a_messages WHERE direction='out' AND client_msg_id IS NOT NULL AND delivery_state='pending' ORDER BY seq LIMIT $1`, limit)
		if err != nil { return err }
		defer rows.Close()
		for rows.Next() { m, err := scanMessage(rows); if err != nil { return err }; messages = append(messages, m) }
		return rows.Err()
	})
	if err != nil { http.Error(w, "outbox failed", 500); return }
	bridgeJSON(w, map[string]any{"messages": messages})
}

func (h *BridgeHandler) result(w http.ResponseWriter, r *http.Request, tenant string) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/a2a/outbox/"), "/result")
	if _, err := uuid.Parse(id); err != nil { http.Error(w, "invalid message id", 400); return }
	var req struct { Status string `json:"status"`; ReasonCode *string `json:"reason_code"` }
	if !decodeBridge(w, r, &req) { return }
	state, ok := deliveryState(req.Status)
	if !ok || req.Status == "" { http.Error(w, "invalid status", 400); return }
	var m Message
	err := h.Store.inTenant(r.Context(), tenant, true, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `UPDATE a2a_messages SET delivery_state=$1,reason_code=$2 WHERE id=$3 AND direction='out' AND client_msg_id IS NOT NULL AND delivery_state NOT IN ('replied','not_delivered')`, state, req.ReasonCode, id)
		if err != nil { return err }
		m, err = scanMessage(tx.QueryRowContext(r.Context(), `SELECT `+messageColumns+` FROM a2a_messages WHERE id=$1 AND direction='out' AND client_msg_id IS NOT NULL`, id))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) { http.Error(w, "message not found", 404); return }
	if err != nil { http.Error(w, "result failed", 500); return }
	bridgeJSON(w, m)
}
