package gmailsync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTriggerCoalescesAndSerializesAccount(t *testing.T) {
	var g accountGate
	id := accountKey{uuid.New(), uuid.New()}
	started, release := make(chan struct{}), make(chan struct{})
	var active, maxActive, runs atomic.Int32
	work := func() error {
		n := active.Add(1)
		if n > maxActive.Load() {
			maxActive.Store(n)
		}
		runs.Add(1)
		close(started)
		<-release
		active.Add(-1)
		return nil
	}
	if !g.trigger(context.Background(), id, work) {
		t.Fatal("first trigger suppressed")
	}
	<-started
	if g.trigger(context.Background(), id, func() error { t.Error("duplicate ran"); return nil }) {
		t.Fatal("concurrent trigger accepted")
	}
	done := make(chan error, 1)
	go func() {
		done <- g.run(context.Background(), id, func() error {
			if active.Add(1) != 1 {
				t.Error("overlap")
			}
			runs.Add(1)
			active.Add(-1)
			return nil
		})
	}()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if g.trigger(context.Background(), id, func() error { t.Error("repeated trigger ran"); return nil }) {
		t.Fatal("repeat accepted")
	}
	if runs.Load() != 2 || maxActive.Load() != 1 {
		t.Fatalf("runs=%d max=%d", runs.Load(), maxActive.Load())
	}
	g.mu.Lock()
	g.state(id).lastTrigger = time.Now().Add(-16 * time.Second)
	g.mu.Unlock()
	complete := make(chan struct{})
	if !g.trigger(context.Background(), id, func() error { close(complete); return nil }) {
		t.Fatal("later trigger suppressed")
	}
	<-complete
}

func TestTriggerScopesAccountIDToTenant(t *testing.T) {
	var g accountGate
	account := uuid.New()
	first := accountKey{uuid.New(), account}
	second := accountKey{uuid.New(), account}
	started := make(chan struct{})
	release := make(chan struct{})
	if !g.trigger(context.Background(), first, func() error { close(started); <-release; return nil }) {
		t.Fatal("first trigger suppressed")
	}
	<-started
	done := make(chan struct{})
	if !g.trigger(context.Background(), second, func() error { close(done); return nil }) {
		t.Fatal("different tenant was suppressed")
	}
	<-done
	close(release)
}

func TestTriggerDuringBackgroundSyncIsCoalesced(t *testing.T) {
	var gate accountGate
	id := accountKey{uuid.New(), uuid.New()}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- gate.run(context.Background(), id, func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	if gate.trigger(context.Background(), id, func() error { t.Error("trigger overlapped background sync"); return nil }) {
		t.Fatal("trigger accepted during background sync")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if gate.trigger(context.Background(), id, func() error { t.Error("coalesced trigger ran after background sync"); return nil }) {
		t.Fatal("repeat accepted within coalescing window")
	}
}
