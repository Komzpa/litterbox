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
