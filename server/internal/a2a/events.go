package a2a

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Events listens on a dedicated connection. NOTIFY carries only a tenant ID;
// SSE queries the tenant-scoped high-water seq, never forwarding message text.
// Clients refetch after their own cursor; notifications may be coalesced.
type Events struct {
	dbURL       string
	store       *Store
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
	ready       chan struct{}
	readyOnce   sync.Once
	pingEvery   time.Duration
}

func NewEvents(dbURL string, store *Store) *Events {
	return &Events{dbURL: dbURL, store: store, subscribers: make(map[string]map[chan struct{}]struct{}), ready: make(chan struct{}), pingEvery: 15 * time.Second}
}

func StartEvents(ctx context.Context, dbURL string, store *Store) *Events {
	e := NewEvents(dbURL, store)
	go e.Run(ctx)
	return e
}

func (e *Events) Run(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := pgx.Connect(ctx, e.dbURL)
		if err == nil {
			_, err = conn.Exec(ctx, "LISTEN a2a")
		}
		if err == nil {
			e.readyOnce.Do(func() { close(e.ready) })
			// Reconnects may have missed a notification. Wake every subscribed
			// tenant to refetch its cursor before resuming notifications.
			e.mu.Lock()
			for tenant := range e.subscribers {
				e.publishLocked(tenant)
			}
			e.mu.Unlock()
			for ctx.Err() == nil {
				n, waitErr := conn.WaitForNotification(ctx)
				if waitErr != nil {
					break
				}
				e.mu.Lock()
				e.publishLocked(n.Payload)
				e.mu.Unlock()
			}
		}
		if conn != nil {
			_ = conn.Close(context.Background())
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (e *Events) publishLocked(tenant string) {
	for ch := range e.subscribers[tenant] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (e *Events) subscribe(tenant string) (chan struct{}, func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ch := make(chan struct{}, 1)
	if e.subscribers[tenant] == nil {
		e.subscribers[tenant] = make(map[chan struct{}]struct{})
	}
	e.subscribers[tenant][ch] = struct{}{}
	return ch, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		delete(e.subscribers[tenant], ch)
		if len(e.subscribers[tenant]) == 0 {
			delete(e.subscribers, tenant)
		}
	}
}

func (e *Events) seq(ctx context.Context, tenant string) (int64, error) {
	var seq int64
	err := e.store.inTenant(ctx, tenant, false, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "SELECT COALESCE(max(last_seq),0) FROM a2a_threads").Scan(&seq)
	})
	return seq, err
}

func (e *Events) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, ok := deviceTenant(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	select {
	case <-e.ready:
	case <-r.Context().Done():
		return
	}
	ch, unsubscribe := e.subscribe(tenant)
	defer unsubscribe()
	seq, err := e.seq(r.Context(), tenant)
	if err != nil {
		http.Error(w, "could not load cursor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	write := func(frame string) bool {
		if _, err := fmt.Fprint(w, frame); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !write(fmt.Sprintf("event: a2a\ndata: {\"seq\":%d}\n\n", seq)) {
		return
	}
	ticker := time.NewTicker(e.pingEvery)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			seq, err = e.seq(r.Context(), tenant)
			if err != nil || !write(fmt.Sprintf("event: a2a\ndata: {\"seq\":%d}\n\n", seq)) {
				return
			}
		case <-ticker.C:
			if !write(": ping\n\n") {
				return
			}
		}
	}
}
