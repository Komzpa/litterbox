package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Komzpa/litterbox/server/internal/sources/agents"
)

func RunIngestAgents(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("ingest-agents", flag.ContinueOnError)
	fs.SetOutput(stdout)
	omp := fs.String("omp-root", os.Getenv("OMP_AGENT_SESSIONS"), "OMP session root (or OMP_AGENT_SESSIONS)")
	codex := fs.String("codex-root", os.Getenv("CODEX_SESSIONS"), "Codex session root (or CODEX_SESSIONS)")
	tenant := fs.String("tenant", os.Getenv("LITTERBOX_TENANT_ID"), "tenant UUID (or LITTERBOX_TENANT_ID)")
	dsn := fs.String("database-url", os.Getenv("DATABASE_URL"), "database URL (or DATABASE_URL)")
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
		if *tenant == "" || *dsn == "" {
			fmt.Fprintln(stdout, "ingest-agents: tenant and database URL required (or use -dry-run)")
			return 2
		}
		db, err := sql.Open("pgx", *dsn)
		if err != nil {
			fmt.Fprintln(stdout, "ingest-agents:", err)
			return 1
		}
		defer db.Close()
		if err = agents.Upsert(context.Background(), db, *tenant, cards); err != nil {
			fmt.Fprintln(stdout, "ingest-agents:", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "count=%d\n", len(cards))
	for i := 0; i < len(cards) && i < 3; i++ {
		fmt.Fprintf(stdout, "%s\n", cards[i].Title)
	}
	return 0
}
