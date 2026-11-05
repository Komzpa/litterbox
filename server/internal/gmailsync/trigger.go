package gmailsync

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// accountGate serializes all syncs for an account and suppresses repeated
// foreground requests. The background loop remains an independent fallback.
type accountGate struct {
	mu     sync.Mutex
	states map[accountKey]*accountState
}
type accountKey struct{ tenant, account uuid.UUID }
type accountState struct {
	running     bool
	done        chan struct{}
	lastTrigger time.Time
}

func (g *accountGate) state(id accountKey) *accountState {
	if g.states == nil {
		g.states = make(map[accountKey]*accountState)
	}
	if g.states[id] == nil {
		g.states[id] = &accountState{}
	}
	return g.states[id]
}
func (g *accountGate) run(ctx context.Context, id accountKey, fn func() error) error {
	for {
		g.mu.Lock()
		s := g.state(id)
		if !s.running {
			s.running = true
			s.done = make(chan struct{})
			g.mu.Unlock()
			defer func() { g.mu.Lock(); s.running = false; close(s.done); g.mu.Unlock() }()
			return fn()
		}
		done := s.done
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
		}
	}
}
func (g *accountGate) trigger(ctx context.Context, id accountKey, fn func() error) bool {
	g.mu.Lock()
	s := g.state(id)
	now := time.Now()
	if now.Sub(s.lastTrigger) < 15*time.Second {
		g.mu.Unlock()
		return false
	}
	if s.running {
		s.lastTrigger = now
		g.mu.Unlock()
		return false
	}
	s.lastTrigger = now
	s.running = true
	s.done = make(chan struct{})
	g.mu.Unlock()
	go func() {
		defer func() { g.mu.Lock(); s.running = false; close(s.done); g.mu.Unlock() }()
		if err := fn(); err != nil && ctx.Err() == nil {
			log.Printf("Gmail triggered synchronization: account=%s %v", id.account, err)
		}
	}()
	return true
}

// Trigger starts an immediate sync unless one is already running or recently
// requested. The context must outlive the HTTP request.
func (s *Syncer) Trigger(ctx context.Context, a Account) bool {
	return s.gate.trigger(ctx, accountKey{a.TenantID, a.ID}, func() error { return s.syncAccount(ctx, a) })
}
