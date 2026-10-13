package a2a

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/google/uuid"
)

// Handler serves the device API. Device middleware supplies the authenticated
// tenant; the store applies that tenant to every transaction under RLS.
// Messages are ordered by seq, including delivery-state updates. next_after is
// the last returned seq (or the requested cursor for an empty page).
// POST returns the stored row with 202; retries retain its original state/text.
type Handler struct {
	Store  *Store
	Events *Events
}

func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/a2a/threads", h.threads)
	mux.HandleFunc("GET /v1/a2a/threads/{id}/messages", h.messages)
	mux.HandleFunc("POST /v1/a2a/threads/{id}/messages", h.send)
	mux.Handle("GET /v1/a2a/events", h.Events)
}

func deviceTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	d, ok := auth.DeviceFromContext(r.Context())
	if !ok || d.TenantID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return d.TenantID, true
}

func a2aJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h *Handler) threads(w http.ResponseWriter, r *http.Request) {
	tenant, ok := deviceTenant(w, r)
	if !ok {
		return
	}
	threads, err := h.Store.ListThreads(r.Context(), tenant)
	if err != nil {
		http.Error(w, "could not load threads", http.StatusInternalServerError)
		return
	}
	a2aJSON(w, http.StatusOK, map[string]any{"threads": threads})
}

func (h *Handler) thread(w http.ResponseWriter, r *http.Request, tenant string) (Thread, bool) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		http.Error(w, "invalid thread id", http.StatusBadRequest)
		return Thread{}, false
	}
	var thread Thread
	err := h.Store.inTenant(r.Context(), tenant, false, func(tx *sql.Tx) error {
		var err error
		thread, err = scanThread(tx.QueryRowContext(r.Context(), "SELECT "+threadColumns+" FROM a2a_threads WHERE id = $1", id))
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "thread not found", http.StatusNotFound)
		return Thread{}, false
	}
	if err != nil {
		http.Error(w, "could not load thread", http.StatusInternalServerError)
		return Thread{}, false
	}
	return thread, true
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	tenant, ok := deviceTenant(w, r)
	if !ok {
		return
	}
	after := int64(0)
	limit := 200
	var err error
	if raw := r.URL.Query().Get("after"); raw != "" {
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			http.Error(w, "limit must be between 1 and 200", http.StatusBadRequest)
			return
		}
	}
	thread, ok := h.thread(w, r, tenant)
	if !ok {
		return
	}
	messages, err := h.Store.ListMessages(r.Context(), tenant, thread.ID, after, limit)
	if err != nil {
		http.Error(w, "could not load messages", http.StatusInternalServerError)
		return
	}
	if len(messages) > 0 {
		after = messages[len(messages)-1].Seq
	}
	a2aJSON(w, http.StatusOK, map[string]any{"messages": messages, "next_after": after})
}

func (h *Handler) send(w http.ResponseWriter, r *http.Request) {
	tenant, ok := deviceTenant(w, r)
	if !ok {
		return
	}
	var body struct {
		ClientMsgID string `json:"client_msg_id"`
		Text        string `json:"text"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "invalid message", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "invalid message", http.StatusBadRequest)
		return
	}
	clientID, err := uuid.Parse(body.ClientMsgID)
	if err != nil || clientID == uuid.Nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "client_msg_id and text are required", http.StatusBadRequest)
		return
	}
	thread, ok := h.thread(w, r, tenant)
	if !ok {
		return
	}
	message, err := h.Store.CreateOutbound(r.Context(), tenant, thread.ID, "agent:darafei", thread.PeerID, body.Text, clientID.String())
	if err != nil {
		http.Error(w, "could not send message", http.StatusInternalServerError)
		return
	}
	a2aJSON(w, http.StatusAccepted, message)
}
