package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunRecluster re-clusters open mail cards for a tenant using the current
// Ollama embedder. It is an administrative subcommand for backfilling bundles
// after an embedding model or endpoint change.
//
// main.go wires this command with:
//
//	if len(os.Args) > 1 && os.Args[1] == "recluster" {
//		os.Exit(RunRecluster(os.Args[2:], os.Stdout))
//	}
func RunRecluster(args []string, stdout io.Writer) int {
	fs := flag.NewFlagSet("recluster", flag.ContinueOnError)
	fs.SetOutput(stdout)
	databaseURL := fs.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL (or DATABASE_URL)")
	tenantID := fs.String("tenant-id", os.Getenv("LITTERBOX_DEV_TENANT_ID"), "tenant UUID to recluster (or LITTERBOX_DEV_TENANT_ID)")
	dryRun := fs.Bool("dry-run", false, "print what would be clustered without writing")
	if fs.Parse(args) != nil {
		return 2
	}
	if *databaseURL == "" {
		fmt.Fprintln(stdout, "recluster: -database-url or DATABASE_URL is required")
		return 2
	}
	if *tenantID == "" {
		fmt.Fprintln(stdout, "recluster: -tenant-id or LITTERBOX_DEV_TENANT_ID is required")
		return 2
	}
	tenant, err := uuid.Parse(*tenantID)
	if err != nil {
		fmt.Fprintf(stdout, "recluster: invalid tenant-id: %v\n", err)
		return 2
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *databaseURL)
	if err != nil {
		fmt.Fprintf(stdout, "recluster: open db: %v\n", err)
		return 1
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fmt.Fprintf(stdout, "recluster: ping db: %v\n", err)
		return 1
	}
	embedder := bundles.NewOllamaEmbedder()
	if *dryRun {
		fmt.Fprintf(stdout, "recluster: dry-run; would cluster tenant %s with model %s at %s\n", tenant, embedder.Model, embedder.Host)
		return 0
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		fmt.Fprintf(stdout, "recluster: begin tx: %v\n", err)
		return 1
	}
	defer tx.Rollback(ctx)
	// RLS policies on cards/bundles read current_setting('litterbox.tenant_id');
	// without this the Cluster query sees zero rows and silently succeeds.
	if _, err := tx.Exec(ctx, "SELECT set_config('litterbox.tenant_id',$1,true)", tenant.String()); err != nil {
		fmt.Fprintf(stdout, "recluster: set tenant config: %v\n", err)
		return 1
	}
	if err := bundles.AfterIngest(ctx, tx, tenant, embedder); err != nil {
		fmt.Fprintf(stdout, "recluster: %v\n", err)
		return 1
	}
	if err := tx.Commit(ctx); err != nil {
		fmt.Fprintf(stdout, "recluster: commit: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "recluster: done for tenant %s\n", tenant)
	return 0
}