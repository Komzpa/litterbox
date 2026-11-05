package ops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Reader runs fn inside a short read-only transaction scoped to the tenant.
// It holds a connection only for the duration of fn, so a PreCommit must not
// make network calls inside it.
type Reader func(ctx context.Context, fn func(pgx.Tx) error) error

// PreCommit performs an operation's idempotent external (provider) mutation
// before the operation's transaction opens. Provider calls take wall-clock
// time on the WAN; running them inside the transaction would hold the tenant
// sequence lock (cards trigger: tenants.next_seq) for their whole duration and
// serialize every op of the tenant. It returns the cards whose provider
// mutation succeeded; the handler then skips those provider calls via Applied.
// Returning (nil, nil) leaves everything to the handler, which re-validates.
type PreCommit func(ctx context.Context, read Reader, tenant, card uuid.UUID, args json.RawMessage) ([]uuid.UUID, error)

var preCommits = struct {
	sync.RWMutex
	hooks map[string]PreCommit
}{hooks: make(map[string]PreCommit)}

// RegisterPreCommit adds the provider phase for an operation type. It panics
// on a duplicate, like Register.
func RegisterPreCommit(typ string, p PreCommit) {
	if typ == "" || p == nil {
		panic("ops: type and pre-commit hook are required")
	}
	preCommits.Lock()
	defer preCommits.Unlock()
	if _, exists := preCommits.hooks[typ]; exists {
		panic("ops: duplicate pre-commit hook " + typ)
	}
	preCommits.hooks[typ] = p
}

func lookupPreCommit(typ string) (PreCommit, bool) {
	preCommits.RLock()
	defer preCommits.RUnlock()
	p, ok := preCommits.hooks[typ]
	return p, ok
}

type appliedKey struct{}

func withApplied(ctx context.Context, cards []uuid.UUID) context.Context {
	if len(cards) == 0 {
		return ctx
	}
	set := make(map[uuid.UUID]struct{}, len(cards))
	for _, id := range cards {
		set[id] = struct{}{}
	}
	return context.WithValue(ctx, appliedKey{}, set)
}

// Applied reports whether this operation's PreCommit already performed the
// provider mutation for card, so the in-transaction handler must not repeat it.
func Applied(ctx context.Context, card uuid.UUID) bool {
	set, _ := ctx.Value(appliedKey{}).(map[uuid.UUID]struct{})
	_, ok := set[card]
	return ok
}

func (a API) reader(tenant string) Reader {
	return func(ctx context.Context, fn func(pgx.Tx) error) error {
		tx, err := a.DB.Begin(ctx)
		if err != nil {
			return &StorageError{Err: err}
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
			return &StorageError{Err: err}
		}
		if err = fn(tx); isStorageFailure(err) {
			return &StorageError{Err: err}
		}
		return err
	}
}

// StorageError marks a database or infrastructure failure, as opposed to a
// semantic rejection of the operation. The API answers it with 503, which the
// client retries; 422 would make the client drop the queued operation.
type StorageError struct{ Err error }

func (e *StorageError) Error() string { return "database unavailable: " + e.Err.Error() }
func (e *StorageError) Unwrap() error { return e.Err }

// TransientProviderError marks an exhausted or interrupted provider request.
// Like storage failures, it must retain the client's durable queued operation.
type TransientProviderError struct{ Err error }

func (e *TransientProviderError) Error() string { return "provider unavailable: " + e.Err.Error() }
func (e *TransientProviderError) Unwrap() error { return e.Err }

// isStorageFailure reports errors raised by the database or its connection.
func isStorageFailure(err error) bool {
	if err == nil || errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	var pg *pgconn.PgError
	var conn *pgconn.ConnectError
	var netErr net.Error
	return errors.As(err, &pg) || errors.As(err, &conn) || errors.As(err, &netErr) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || pgconn.Timeout(err)
}

func (a API) opRecorded(ctx context.Context, read Reader, tenant string, op uuid.UUID) (bool, error) {
	recorded := false
	err := read(ctx, func(tx pgx.Tx) error {
		var hash []byte
		err := tx.QueryRow(ctx, `SELECT payload_hash FROM ops WHERE tenant_id=$1 AND op_id=$2`, tenant, op).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		recorded = err == nil
		return err
	})
	return recorded, err
}

func init() {
	// Take the same thread guard as wake. Snooze validates and applies Gmail
	// under the card row lock before its local update.
	RegisterPreCommit("snooze", func(context.Context, Reader, uuid.UUID, uuid.UUID, json.RawMessage) ([]uuid.UUID, error) {
		return nil, nil
	})

	for _, kind := range []string{"archive", "spam"} {
		RegisterPreCommit(kind, func(ctx context.Context, read Reader, tenant, card uuid.UUID, args json.RawMessage) ([]uuid.UUID, error) {
			return mailProviderPhase(ctx, read, tenant, card, kind, args)
		})
		RegisterPreCommit("bundle_"+kind, func(ctx context.Context, read Reader, tenant, card uuid.UUID, args json.RawMessage) ([]uuid.UUID, error) {
			ids, resolved := Targets(ctx)
			if !resolved {
				err := read(ctx, func(tx pgx.Tx) (err error) {
					ids, err = resolveBundleTargets(ctx, tx, tenant, card, args)
					return err
				})
				if err != nil {
					return nil, err
				}
			}
			if len(ids) == 0 {
				return nil, nil // the handler reports empty bundles
			}
			// The first member runs alone: it warms the account's token and label
			// caches and surfaces a revoked grant once, instead of every member
			// repeating the failing OAuth exchange. The rest coalesce into batches.
			applied, err := mailProviderPhase(ctx, read, tenant, ids[0], kind, args)
			if err != nil {
				return nil, err
			}
			type result struct {
				applied []uuid.UUID
				err     error
			}
			results := make([]result, len(ids)-1)
			var wg sync.WaitGroup
			for i, id := range ids[1:] {
				wg.Add(1)
				go func() {
					defer wg.Done()
					results[i].applied, results[i].err = mailProviderPhase(ctx, read, tenant, id, kind, args)
				}()
			}
			wg.Wait()
			for _, r := range results {
				if r.err != nil {
					return nil, r.err
				}
				applied = append(applied, r.applied...)
			}
			return applied, nil
		})
	}
}

// mailProviderPhase runs the provider mutation for one archive/spam target.
// Non-mail and unknown cards are left to the handler's own validation.
func mailProviderPhase(ctx context.Context, read Reader, tenant, card uuid.UUID, kind string, args json.RawMessage) ([]uuid.UUID, error) {
	var source string
	err := read(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT source FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&source)
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && source != "mail") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	provider, ok := lookupPreCommit("gmail." + kind)
	if !ok {
		return nil, nil
	}
	return provider(ctx, read, tenant, card, args)
}

// reject answers 503 for temporary failures and 422 for terminal rejections.
// tx is the open operation transaction, or nil during the provider phase.
func (a API) reject(w http.ResponseWriter, r *http.Request, tenant string, in request, err error, tx pgx.Tx) {
	var storage *StorageError
	if errors.As(err, &storage) {
		log.Printf("operation unavailable: type=%s card_id=%s error=%v", in.Type, in.CardID, err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	var transient *TransientProviderError
	if errors.As(err, &transient) {
		log.Printf("operation unavailable: type=%s card_id=%s error=%v", in.Type, in.CardID, err)
		http.Error(w, "provider unavailable", http.StatusServiceUnavailable)
		return
	}
	log.Printf("operation rejected: type=%s card_id=%s error=%v", in.Type, in.CardID, err)
	response := map[string]string{"error": err.Error()}
	var reconnect *GmailReconnectError
	if errors.As(err, &reconnect) {
		if tx != nil {
			// Restore the complete operation before persisting account health.
			if rollbackErr := tx.Rollback(r.Context()); rollbackErr != nil {
				http.Error(w, "database unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		health, healthErr := a.DB.Begin(r.Context())
		if healthErr == nil {
			defer health.Rollback(r.Context())
			_, healthErr = health.Exec(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant)
			if healthErr == nil {
				_, healthErr = health.Exec(r.Context(), `UPDATE accounts SET status='reauth_required' WHERE tenant_id=$1 AND id=$2`, tenant, reconnect.AccountID)
			}
			if healthErr == nil {
				healthErr = health.Commit(r.Context())
			}
		}
		if healthErr != nil {
			log.Printf("Gmail account health update: %v", healthErr)
		}
		response["code"] = "gmail_reconnect_required"
		response["account_address"] = reconnect.Address
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(response)
}
