// Package offline serves the initial card snapshot and incremental change
// feed the app store uses to stay current without a live connection.
package offline

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

type API struct {
	DB       *sql.DB
	Identity func(r *http.Request) (string, bool)
}

func (a API) Snapshot(w http.ResponseWriter, r *http.Request) {
	tenant, ok := a.Identity(r)
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `
		SELECT c.id, c.subject, c.sender, c.state, c.sort_at, c.note, c.account_id, c.gmail_thread_id,
		       COALESCE((
		           SELECT jsonb_agg(jsonb_build_object(
		               'id', m.id, 'text', m.text, 'html', m.html,
		               'received_at', m.received_at, 'gmail_message_id', m.gmail_message_id,
		               'account_id', c.account_id, 'gmail_thread_id', c.gmail_thread_id
		           ) ORDER BY m.received_at)
		           FROM messages m WHERE m.tenant_id = c.tenant_id AND m.card_id = c.id
		       ), '[]'::jsonb)
		FROM cards c
		WHERE c.tenant_id = $1 AND c.state = 'open'
		ORDER BY c.sort_at DESC`, tenant)
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()

	cards := make([]json.RawMessage, 0)
	for rows.Next() {
		var id, subject, sender, state, note, accountID, threadID string
		var sortAt any
		var messages []byte
		if err = rows.Scan(&id, &subject, &sender, &state, &sortAt, &note, &accountID, &threadID, &messages); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		b, _ := json.Marshal(map[string]any{
			"id": id, "subject": subject, "sender": sender, "state": state, "account_id": accountID, "gmail_thread_id": threadID,
			"sort_at": sortAt, "note": note, "messages": json.RawMessage(messages),
		})
		cards = append(cards, b)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	var cursor int64
	if err = tx.QueryRowContext(r.Context(), `SELECT next_seq FROM tenants WHERE id=$1`, tenant).Scan(&cursor); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"cursor": cursor, "cards": cards})
}

func (a API) Changes(w http.ResponseWriter, r *http.Request) {
	tenant, ok := a.Identity(r)
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var since int64
	if _, err := fmt.Sscan(r.URL.Query().Get("since"), &since); err != nil || since < 0 {
		http.Error(w, "invalid cursor", http.StatusBadRequest)
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	rows, err := tx.QueryContext(r.Context(), `SELECT seq, entity, id, value FROM changes WHERE tenant_id=$1 AND seq>$2 ORDER BY seq`, tenant, since)
	if err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()

	changes := make([]map[string]any, 0)
	for rows.Next() {
		var seq int64
		var entity, id string
		var value []byte
		if err = rows.Scan(&seq, &entity, &id, &value); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		changes = append(changes, map[string]any{"cursor": seq, "entity": entity, "id": id, "value": json.RawMessage(value)})
	}
	if err = rows.Err(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"changes": changes})
}
