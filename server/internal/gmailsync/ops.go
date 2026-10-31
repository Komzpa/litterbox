package gmailsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type actionArgs struct {
	Label string `json:"label"`
}

// actionLabel returns the label a label-carrying action targets ("" for
// archive, spam, star and unstar) and rejects a missing one.
func actionLabel(name string, args json.RawMessage) (string, error) {
	var a actionArgs
	if len(args) > 0 {
		if e := json.Unmarshal(args, &a); e != nil {
			return "", e
		}
	}
	switch name {
	case "snooze", "unsnooze":
		a.Label = "Litterbox/Snoozed"
	case "label_add", "label_remove":
		if a.Label == "" {
			return "", fmt.Errorf("gmail %s: label required", name)
		}
	default:
		a.Label = ""
	}
	return a.Label, nil
}

// planModify returns the label additions and removals for an action, resolving
// (and if needed creating) the target label through the client's label cache.
func (c *Client) planModify(ctx context.Context, name, label string, add, remove []string) (adds, removes []string, err error) {
	if label == "" {
		return append([]string(nil), add...), append([]string(nil), remove...), nil
	}
	id, err := c.resolveLabel(ctx, label)
	if err != nil {
		return nil, nil, err
	}
	if name == "label_add" || name == "snooze" {
		return []string{id}, nil, nil
	}
	return nil, []string{id}, nil
}

// providerOperationError distinguishes temporary provider outages from terminal
// rejections, and attaches the account to revoked/expired refresh grants.
func providerOperationError(err error, lookup func() (uuid.UUID, string, error)) error {
	if !errors.Is(err, ErrInvalidGrant) {
		var api *APIError
		var transport net.Error
		if (errors.As(err, &api) && retryableStatus(api.StatusCode, []byte(api.Body))) ||
			errors.As(err, &transport) || errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return &ops.TransientProviderError{Err: err}
		}
		return err
	}
	if account, address, e := lookup(); e == nil {
		return &ops.GmailReconnectError{AccountID: account, Address: address}
	}
	return err
}

const accountOfCard = `SELECT a.id,a.address FROM accounts a JOIN cards c ON c.tenant_id=a.tenant_id AND c.account_id=a.id WHERE c.tenant_id=$1 AND c.id=$2`

// RegisterOps binds mail actions to account/thread ownership stored on the card.
//
// Each action has two entry points. The PreCommit runs from ops.API before the
// operation's transaction opens and performs the provider mutation through the
// account's shared, batching client, so no WAN call holds the tenant sequence
// lock. The handler (inside the transaction) then only applies the local
// effects; called directly without a PreCommit (ops.Applied false) it performs
// the provider call itself.
func RegisterOps(clientFor func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error)) {
	register := func(name string, add, remove []string) {
		ops.RegisterPreCommit("gmail."+name, func(ctx context.Context, read ops.Reader, tenant, cardID uuid.UUID, args json.RawMessage) (applied []uuid.UUID, err error) {
			defer func() {
				err = providerOperationError(err, func() (account uuid.UUID, address string, e error) {
					e = read(ctx, func(tx pgx.Tx) error {
						return tx.QueryRow(ctx, accountOfCard, tenant, cardID).Scan(&account, &address)
					})
					return
				})
			}()
			label, err := actionLabel(name, args)
			if err != nil {
				return nil, err
			}
			var c *Client
			var thread string
			if err = read(ctx, func(tx pgx.Tx) (e error) {
				c, thread, e = clientFor(ctx, tx, tenant, cardID)
				return e
			}); err != nil {
				return nil, err
			}
			adds, removes, err := c.planModify(ctx, name, label, add, remove)
			if err != nil {
				return nil, err
			}
			if err = c.ModifyBatched(ctx, thread, adds, removes); err != nil {
				return nil, err
			}
			return []uuid.UUID{cardID}, nil
		})
		ops.Register("gmail."+name, func(ctx context.Context, tx pgx.Tx, tenant, cardID uuid.UUID, args json.RawMessage) (err error) {
			defer func() {
				err = providerOperationError(err, func() (account uuid.UUID, address string, e error) {
					e = tx.QueryRow(ctx, accountOfCard, tenant, cardID).Scan(&account, &address)
					return
				})
			}()
			label, e := actionLabel(name, args)
			if e != nil {
				return e
			}
			if !ops.Applied(ctx, cardID) {
				c, thread, e := clientFor(ctx, tx, tenant, cardID)
				if e != nil {
					return e
				}
				adds, removes, e := c.planModify(ctx, name, label, add, remove)
				if e != nil {
					return e
				}
				if e = c.Modify(ctx, thread, adds, removes); e != nil {
					return e
				}
			}
			if name == "label_add" && label == "INBOX" {
				// Archive Undo commits the provider inverse and normal card change
				// together; a provider failure leaves the card archived.
				_, err = tx.Exec(ctx, `UPDATE cards SET state='open' WHERE tenant_id=$1 AND id=$2 AND source='mail'`, tenant, cardID)
				return err
			}
			return nil
		})
	}
	register("archive", nil, []string{"INBOX"})
	register("spam", []string{"SPAM"}, []string{"INBOX"})
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
