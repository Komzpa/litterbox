package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Komzpa/litterbox/server/internal/sources/todos"
	"github.com/Komzpa/litterbox/server/internal/ingest"
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
	timezone := fs.String("tz", time.Local.String(), "timezone interpreting daily-note slots (default system local)")
	endpoint := fs.String("ingest-url", os.Getenv("LITTERBOX_INGEST_URL"), "Litterbox server URL (or LITTERBOX_INGEST_URL)")
	token := fs.String("source-token", os.Getenv("LITTERBOX_SOURCE_TOKEN"), "todo-scoped source token (or LITTERBOX_SOURCE_TOKEN)")
	dryRun := fs.Bool("dry-run", false, "read notes and print count and up to three titles without writing")
	if fs.Parse(args) != nil {
		return 2
	}
	if *notesDir == "" {
		fmt.Fprintln(stdout, "ingest-todos: set -notes-dir or LITTERBOX_DAILY_NOTES_DIR")
		return 2
	}
	loc, err := time.LoadLocation(*timezone)
	if err != nil {
		fmt.Fprintln(stdout, "ingest-todos:", err)
		return 2
	}
	cards, err := todos.ReadDir(*notesDir, time.Now().In(loc))
	if err != nil {
		fmt.Fprintln(stdout, "ingest-todos:", err)
		return 1
	}
	if !*dryRun {
		if *endpoint == "" || *token == "" {
			fmt.Fprintln(stdout, "ingest-todos: ingest URL and source token required (or use -dry-run)")
			return 2
		}
		for _, card := range cards {
			err = ingest.Post(context.Background(), *endpoint, *token, ingest.Request{Card: ingest.Card{ExternalID: card.ExternalID, Kind: "todo", Title: card.Title, At: card.At, Timed: card.Timed, Order: card.Order}})
			if err != nil { fmt.Fprintln(stdout, "ingest-todos:", err); return 1 }
		}
	}
	fmt.Fprintf(stdout, "count=%d\n", len(cards))
	for i := 0; i < len(cards) && i < 3; i++ {
		fmt.Fprintln(stdout, cards[i].Title)
	}
	return 0
}
