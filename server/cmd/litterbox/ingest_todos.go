package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
    "time"

	"github.com/Komzpa/litterbox/server/internal/sources/todos"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunIngestTodos imports open daily-note tasks. main.go wires this command with:
//
//	if len(os.Args) > 1 && os.Args[1] == "ingest-todos" {
//		os.Exit(RunIngestTodos(os.Args[2:], os.Stdout))
//	}
//
// before normal flag parsing. Notes remain read-only; dismissing a card changes
// only its Litterbox state.
func RunIngestTodos(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("ingest-todos", flag.ContinueOnError)
	fs.SetOutput(stdout)
	notesDir := fs.String("notes-dir", os.Getenv("LITTERBOX_DAILY_NOTES_DIR"), "daily notes directory (or LITTERBOX_DAILY_NOTES_DIR)")
	tenant := fs.String("tenant", os.Getenv("LITTERBOX_TENANT_ID"), "tenant UUID (or LITTERBOX_TENANT_ID)")
	dsn := fs.String("database-url", os.Getenv("DATABASE_URL"), "database URL (or DATABASE_URL)")
	timezone := fs.String("tz", time.Local.String(), "timezone interpreting daily-note slots (default system local)")
	dryRun := fs.Bool("dry-run", false, "read notes and print count and up to three titles without writing")
	if fs.Parse(args) != nil {
		return 2
	}
	if *notesDir == "" {
		fmt.Fprintln(stdout, "ingest-todos: set -notes-dir or LITTERBOX_DAILY_NOTES_DIR")
		return 2
	}
	loc, err := time.LoadLocation(*timezone)
	if err != nil { fmt.Fprintln(stdout, "ingest-todos:", err); return 2 }
	cards, err := todos.ReadDir(*notesDir, time.Now().In(loc))
	if err != nil {
		fmt.Fprintln(stdout, "ingest-todos:", err)
		return 1
	}
	if !*dryRun {
		if *tenant == "" || *dsn == "" {
			fmt.Fprintln(stdout, "ingest-todos: tenant and database URL required (or use -dry-run)")
			return 2
		}
		db, err := sql.Open("pgx", *dsn)
		if err != nil {
			fmt.Fprintln(stdout, "ingest-todos:", err)
			return 1
		}
		defer db.Close()
		if err = todos.Upsert(context.Background(), db, *tenant, cards); err != nil {
			fmt.Fprintln(stdout, "ingest-todos:", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "count=%d\n", len(cards))
	for i := 0; i < len(cards) && i < 3; i++ {
		fmt.Fprintln(stdout, cards[i].Title)
	}
	return 0
}
