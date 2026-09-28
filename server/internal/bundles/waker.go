package bundles

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// TxBeginner is implemented by pgxpool.Pool and permits a single transaction
// for each periodic due-card wake pass.
type TxBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// StartWaker starts a background worker that wakes due cards every 30 seconds.
// It stops when ctx is cancelled.
func StartWaker(ctx context.Context, db TxBeginner) {
	go runWaker(ctx, db, 30*time.Second)
}

func runWaker(ctx context.Context, db TxBeginner, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tx, err := db.Begin(ctx)
			if err != nil {
				continue
			}
			_, err = WakeDueCards(ctx, tx)
			if err != nil {
				_ = tx.Rollback(ctx)
				continue
			}
			_ = tx.Commit(ctx)
		}
	}
}
