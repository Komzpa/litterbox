package ops

import (
 "context"
 "sort"
 "sync"

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

func lockProviderTargets(ctx context.Context, read Reader, tenant uuid.UUID, in request) (func(), error) {
	var keys []string
	err := read(ctx, func(tx pgx.Tx) error {
		ids := []uuid.UUID{in.CardID}
		if in.Type == "bundle_archive" || in.Type == "bundle_spam" {
			var err error
			ids, err = bundleOpenUnpinnedIDs(ctx, tx, tenant, in.Args)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				ids, err = bundleFallbackIDs(ctx, tx, tenant, in.CardID, in.Args)
				if err != nil {
					return err
				}
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
		return nil, err
	}
	sort.Strings(keys)
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
