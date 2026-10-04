// Package testdb isolates PostgreSQL-backed tests from each other so the whole
// package tree can run concurrently against one pg_virtualenv database.
//
// Every call to Setup gets its own schema and its own application role, and the
// DSN it returns makes unqualified object references resolve in that schema.
// That replaces the old package-wide "DROP SCHEMA public CASCADE" reset, which
// only worked when packages ran serially.
package testdb

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DB names the database objects owned by a single test.
type DB struct {
	// Schema holds the tables, functions and indexes created by the test.
	Schema string
	// Role is the application role the test's migrations create grants for.
	Role string
	// DSN connects with search_path set to Schema.
	DSN string
}

// Setup creates the test's schema and registers its removal. Call it after any
// t.Skip and before opening connections with DSN.
func Setup(t *testing.T) *DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	// One identifier per test keeps concurrent packages from creating the same
	// role or replacing each other's objects.
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	db := &DB{
		Schema: "test_" + suffix,
		Role:   "litterbox_test_" + suffix,
	}
	db.DSN = withSearchPath(base, db.Schema)

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	// The migrations grant table privileges to Role, but PostgreSQL also
	// requires schema USAGE; grant it broadly the way the shared public schema
	// did before, since Role does not exist until the migrations run.
	if _, err := admin.Exec(ctx, `CREATE SCHEMA `+db.Schema+` ; GRANT USAGE ON SCHEMA `+db.Schema+` TO PUBLIC`); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanup, err := pgx.Connect(ctx, base)
		if err != nil {
			t.Errorf("testdb: connect for cleanup: %v", err)
			return
		}
		defer cleanup.Close(ctx)
		if _, err := cleanup.Exec(ctx, `DROP SCHEMA `+db.Schema+` CASCADE`); err != nil {
			t.Errorf("testdb: drop schema %s: %v", db.Schema, err)
		}
		if _, err := cleanup.Exec(ctx, `DROP ROLE IF EXISTS `+db.Role); err != nil {
			t.Errorf("testdb: drop role %s: %v", db.Role, err)
		}
	})
	return db
}

// Migration rewrites the shared identifiers in migration SQL so the objects and
// grants it creates stay private to this test's schema and role.
func (d *DB) Migration(sql string) string {
	// Rewrite schema-qualified references and search_path entries. Bare PUBLIC
	// in GRANT/REVOKE statements remains the SQL keyword.
	sql = strings.ReplaceAll(sql, "public.", d.Schema+".")
	sql = strings.ReplaceAll(sql, "= public ", "= "+d.Schema+" ")
	sql = strings.ReplaceAll(sql, "= public,", "= "+d.Schema+",")
	sql = strings.ReplaceAll(sql, ", public ", ", "+d.Schema+" ")
	sql = strings.ReplaceAll(sql, ", public,", ", "+d.Schema+",")
	return strings.ReplaceAll(sql, "litterbox_app", d.Role)
}

// withSearchPath returns base with search_path added, keeping whatever
// connection settings the environment or DATABASE_URL already supplied.
func withSearchPath(base, schema string) string {
	value := "search_path=" + schema
	switch {
	case base == "":
		return value
	case strings.HasPrefix(base, "postgres://"), strings.HasPrefix(base, "postgresql://"):
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		return base + separator + "search_path=" + url.QueryEscape(schema)
	default:
		return base + " " + value
	}
}
