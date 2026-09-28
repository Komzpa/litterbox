package cards

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Events distributes PostgreSQL card notifications to connected tenant streams.
type Events struct {
	dbURL       string
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
	ready       chan struct{}
	readyOnce   sync.Once
}

func NewEvents(dbURL string) *Events {
	return &Events{dbURL: dbURL, subscribers: make(map[string]map[chan struct{}]struct{}), ready: make(chan struct{})}
}

func (e *Events) Run(ctx context.Context) {
	for ctx.Err() == nil {
		conn, err := pgx.Connect(ctx, e.dbURL)
		if err == nil {
			_, err = conn.Exec(ctx, "LISTEN cards")
		}
		if err == nil {
			e.readyOnce.Do(func() { close(e.ready) })
			for ctx.Err() == nil {
				notification, waitErr := conn.WaitForNotification(ctx)
				if waitErr != nil {
					err = waitErr
					break
				}
				e.publish(notification.Payload)
			}
		}
		if conn != nil {
			_ = conn.Close(context.Background())
		}
		if ctx.Err() != nil {
			return
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

func (e *Events) publish(tenant string) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
		delete(e.subscribers[tenant], ch)
		if len(e.subscribers[tenant]) == 0 {
			delete(e.subscribers, tenant)
		}
		close(ch)
		e.mu.Unlock()
	}
}

func (e *Events) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tenant, ok := (&Handler{}).tenant(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	select {
	case <-e.ready:
	case <-r.Context().Done():
		return
	}
	ch, unsubscribe := e.subscribe(tenant)
	defer unsubscribe()
	write := func(s string) bool {
		if _, err := fmt.Fprint(w, s); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !write("event: cards\ndata: {}\n\n") {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case _, open := <-ch:
			if !open || !write("event: cards\ndata: {}\n\n") {
				return
			}
		case <-ticker.C:
			if !write(": ping\n\n") {
				return
			}
		}
	}
}

// StartEvents attaches a dedicated pgx LISTEN connection independent of the SQL pool.
func StartEvents(ctx context.Context, dbURL string) *Events {
	e := NewEvents(dbURL)
	go e.Run(ctx)
	return e
}
