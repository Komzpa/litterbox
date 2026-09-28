package gmailsync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type actionArgs struct {
	Label string `json:"label"`
}

// RegisterOps binds mail actions to account/thread ownership stored on the card.
func RegisterOps(clientFor func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error)) {
	register := func(name string, add, remove []string) {
		ops.Register("gmail."+name, func(ctx context.Context, tx pgx.Tx, tenant, cardID uuid.UUID, args json.RawMessage) error {
			c, thread, e := clientFor(ctx, tx, tenant, cardID)
			if e != nil {
				return e
			}
			adds := append([]string(nil), add...)
			removes := append([]string(nil), remove...)
			var a actionArgs
			if len(args) > 0 {
				if e = json.Unmarshal(args, &a); e != nil {
					return e
				}
			}
			if name == "snooze" || name == "unsnooze" {
				a.Label = "Litterbox/Snoozed"
			}
			if name == "label_add" || name == "label_remove" || name == "snooze" || name == "unsnooze" {
				if a.Label == "" {
					return fmt.Errorf("gmail %s: label required", name)
				}
				label, e := c.labelID(ctx, a.Label)
				if e != nil {
					return e
				}
				if label == "" {
					label, e = c.CreateLabel(ctx, a.Label)
					if e != nil {
						return e
					}
				}
				if name == "label_add" || name == "snooze" {
					adds = []string{label}
					removes = nil
				} else {
					removes = []string{label}
					adds = nil
				}
			}
			return c.Modify(ctx, thread, adds, removes)
		})
	}
	register("archive", nil, []string{"INBOX"})
	register("star", []string{"STARRED"}, nil)
	register("unstar", nil, []string{"STARRED"})
	register("label_add", nil, nil)
	register("label_remove", nil, nil)
	register("snooze", nil, nil)
	register("unsnooze", nil, nil)
}
func (c *Client) labelID(ctx context.Context, name string) (string, error) {
	var x struct {
		Labels []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"labels"`
	}
	if e := c.request(ctx, "GET", "/labels", nil, &x); e != nil {
		return "", e
	}
	for _, l := range x.Labels {
		if l.Name == name {
			return l.ID, nil
		}
	}
	return "", nil
}
