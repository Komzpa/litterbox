// Package ops provides the handler registry shared by offline and online actions.
package ops

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sync"
)

type Handler func(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cardID uuid.UUID, args json.RawMessage) error

var mu sync.RWMutex
var handlers = map[string]Handler{}

func Register(name string, h Handler) {
	mu.Lock()
	defer mu.Unlock()
	if h == nil {
		panic("ops: nil handler")
	}
	if _, ok := handlers[name]; ok {
		panic("ops: duplicate handler " + name)
	}
	handlers[name] = h
}
func Lookup(name string) (Handler, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := handlers[name]
	return h, ok
}
