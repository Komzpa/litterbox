// Package ops is a local integration stub. OfflineCardStore replaces it with
// the production registry implementation when branches are integrated.
package ops

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Handler func(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cardID uuid.UUID, args json.RawMessage) error

var handlers = map[string]Handler{}

func Register(kind string, h Handler) { handlers[kind] = h }

// CallIfRegistered invokes an optional operation hook, such as a Gmail effect.
// Missing hooks are intentionally a no-op so non-mail cards remain supported.
func CallIfRegistered(ctx context.Context, tx pgx.Tx, tenant, cardID uuid.UUID, kind string, args json.RawMessage) error {
	if h, ok := handlers[kind]; ok {
		return h(ctx, tx, tenant, cardID, args)
	}
	return nil
}
