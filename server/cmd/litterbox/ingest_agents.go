package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/Komzpa/litterbox/server/internal/ingest"
)

func RunIngestAgents(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("ingest-agents", flag.ContinueOnError)
	fs.SetOutput(stdout)
	omp := fs.String("omp-root", os.Getenv("OMP_AGENT_SESSIONS"), "OMP session root (or OMP_AGENT_SESSIONS)")
	codex := fs.String("codex-root", os.Getenv("CODEX_SESSIONS"), "Codex session root (or CODEX_SESSIONS)")
	endpoint := fs.String("ingest-url", os.Getenv("LITTERBOX_INGEST_URL"), "Litterbox server URL (or LITTERBOX_INGEST_URL)")
	token := fs.String("source-token", os.Getenv("LITTERBOX_SOURCE_TOKEN"), "agent-scoped source token (or LITTERBOX_SOURCE_TOKEN)")
	dryRun := fs.Bool("dry-run", false, "read sessions and print result titles without writing")
	if fs.Parse(args) != nil {
		return 2
	}
	if *omp == "" && *codex == "" {
		fmt.Fprintln(stdout, "ingest-agents: set -omp-root/OMP_AGENT_SESSIONS or -codex-root/CODEX_SESSIONS")
		return 2
	}
	cards, err := agents.ReadRoots(*omp, *codex)
	if err != nil {
		fmt.Fprintln(stdout, "ingest-agents:", err)
		return 1
	}
	if !*dryRun {
		if *endpoint == "" || *token == "" {
			fmt.Fprintln(stdout, "ingest-agents: ingest URL and source token required (or use -dry-run)")
			return 2
		}
		for _, card := range cards {
			err = ingest.Post(context.Background(), *endpoint, *token, ingest.Request{Card: ingest.Card{ExternalID: card.ExternalID, Kind: "agent_result", Title: card.Title, Summary: card.Summary, At: &card.SortAt}})
			if err != nil { fmt.Fprintln(stdout, "ingest-agents:", err); return 1 }
		}
	}
	fmt.Fprintf(stdout, "count=%d\n", len(cards))
	for i := 0; i < len(cards) && i < 3; i++ {
		fmt.Fprintf(stdout, "%s\n", cards[i].Title)
	}
	return 0
}
