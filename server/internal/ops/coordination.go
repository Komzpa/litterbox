package ops

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The server has one ops consumer process. Coordinate provider work through the
// local commit, not just through the HTTP response: Gmail may execute batch
// members in any order. Unrelated threads retain their batching concurrency.
var operationLocks = struct {
 sync.Mutex
 entries map[string]*operationLock
}{entries: make(map[string]*operationLock)}

type operationLock struct {
	ready chan struct{}
	refs  int
}

func lockOperation(ctx context.Context, key string) (func(), error) {
	operationLocks.Lock()
	entry := operationLocks.entries[key]
	if entry == nil {
		entry = &operationLock{ready: make(chan struct{}, 1)}
		entry.ready <- struct{}{}
		operationLocks.entries[key] = entry
	}
	entry.refs++
	operationLocks.Unlock()
	releaseRef := func() {
		operationLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(operationLocks.entries, key)
		}
		operationLocks.Unlock()
	}
	select {
	case <-ctx.Done():
		releaseRef()
		return nil, ctx.Err()
	case <-entry.ready:
		if err := ctx.Err(); err != nil {
			entry.ready <- struct{}{}
			releaseRef()
			return nil, err
		}
		return func() {
			entry.ready <- struct{}{}
			releaseRef()
		}, nil
	}
}

// targetKey is the context key under which the op's resolved target set travels.
type targetKey struct{}

func withTargets(ctx context.Context, ids []uuid.UUID) context.Context {
	return context.WithValue(ctx, targetKey{}, ids)
}

// Targets returns the card set ServeHTTP resolved once for this operation. The
// provider phase and the commit both use exactly this set, so Gmail is never
// asked to archive a thread the commit leaves open, and a member that appears
// after resolution cannot reach a synchronous provider call under the tenant
// sequence lock. ok is false when the handler runs without ServeHTTP.
func Targets(ctx context.Context) (ids []uuid.UUID, ok bool) {
	ids, ok = ctx.Value(targetKey{}).([]uuid.UUID)
	return ids, ok
}

// resolveBundleTargets lists the cards a bundle op acts on: the open unpinned
// members, else the client's named cards.
func resolveBundleTargets(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, args json.RawMessage) ([]uuid.UUID, error) {
	ids, err := bundleOpenUnpinnedIDs(ctx, tx, tenant, args)
	if err == nil && len(ids) == 0 {
		ids, err = bundleFallbackIDs(ctx, tx, tenant, card, args)
	}
	return ids, err
}

// resolveTargets reads the op's target set and its thread guard keys in one
// short read transaction. Malformed args are a semantic error; database
// failures arrive as *StorageError.
func resolveTargets(ctx context.Context, read Reader, tenant uuid.UUID, in request) ([]uuid.UUID, []string, error) {
	bundle := in.Type == "bundle_archive" || in.Type == "bundle_spam"
	if bundle {
		var args bundles.ArchiveArgs
		if err := json.Unmarshal(in.Args, &args); err != nil {
			return nil, nil, err
		}
	}
	var ids []uuid.UUID
	var keys []string
	err := read(ctx, func(tx pgx.Tx) error {
		ids = []uuid.UUID{in.CardID}
		if bundle {
			var err error
			if ids, err = resolveBundleTargets(ctx, tx, tenant, in.CardID, in.Args); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, `SELECT DISTINCT account_id::text,gmail_thread_id FROM cards WHERE tenant_id=$1 AND id=ANY($2) AND source='mail' AND account_id IS NOT NULL AND gmail_thread_id IS NOT NULL`, tenant, ids)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var account, thread string
			if err := rows.Scan(&account, &thread); err != nil {
				return err
			}
			keys = append(keys, tenant.String()+"/thread/"+account+"/"+thread)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(keys)
	return ids, keys, nil
}

// lockThreads takes the per-thread guards in sorted order.
func lockThreads(ctx context.Context, keys []string) (func(), error) {
	var releases []func()
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	for _, key := range keys {
		unlock, err := lockOperation(ctx, key)
		if err != nil {
			release()
			return nil, err
		}
		releases = append(releases, unlock)
	}
	return release, nil
}
