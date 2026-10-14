package testdb

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGuardedDSNFreezesDatabase(t *testing.T) {
	for _, base := range []string{"", "host=localhost", "postgresql://localhost/", "postgresql://localhost/?dbname=litterbox_test_explicit"} {
		t.Run(base, func(t *testing.T) {
			t.Setenv("PGDATABASE", "litterbox_test_environment")
			t.Setenv("DATABASE_URL", base)
			dsn := guardedDSN(t)
			t.Setenv("PGDATABASE", "kom")
			cfg, err := pgx.ParseConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}
			want := "litterbox_test_environment"
			if base == "postgresql://localhost/?dbname=litterbox_test_explicit" {
				want = "litterbox_test_explicit"
			}
			if cfg.Database != want {
				t.Fatalf("database changed after guard: got %q, want %q", cfg.Database, want)
			}
		})
	}
}
