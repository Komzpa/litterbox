package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/Komzpa/litterbox/server/internal/httpapi"
	"github.com/Komzpa/litterbox/server/internal/journal"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var buildSHA = "unknown"
var minClientAPI = httpapi.APIVersion

func main() {
	if len(os.Args) > 1 && os.Args[1] == "ingest-todos" {
		os.Exit(RunIngestTodos(os.Args[2:], os.Stdout))
	}
	if len(os.Args) > 1 && os.Args[1] == "ingest-agents" {
		os.Exit(RunIngestAgents(os.Args[2:], os.Stdout))
	}
	if len(os.Args) > 1 && os.Args[1] == "connect" {
		os.Exit(RunConnect(os.Args[2:], os.Stdout))
	}

	listenAddr := envOrDefault("LISTEN_ADDR", ":8080")
	databaseURL := os.Getenv("DATABASE_URL")
	devTenantID := os.Getenv("LITTERBOX_DEV_TENANT_ID")
	timezone := time.Local.String()
	noteSink := os.Getenv("LITTERBOX_NOTE_SINK")
	flag.StringVar(&listenAddr, "listen", listenAddr, "HTTP listen address (or LISTEN_ADDR)")
	flag.StringVar(&databaseURL, "database-url", databaseURL, "PostgreSQL connection URL (or DATABASE_URL)")
	flag.StringVar(&devTenantID, "dev-tenant-id", devTenantID, "development-only tenant UUID for cards API (or LITTERBOX_DEV_TENANT_ID)")
	flag.StringVar(&timezone, "tz", timezone, "timezone for card display (default system local)")
	flag.StringVar(&noteSink, "note-sink", noteSink, "Markdown daily-note path receiving generator feedback")
	flag.IntVar(&minClientAPI, "min-client-api", minClientAPI, "minimum supported client API version")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", httpapi.Healthz)
	mux.HandleFunc("GET /v1/version", httpapi.Version(buildSHA, minClientAPI))
	if databaseURL != "" {
		db, err := sql.Open("pgx", databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		if err := db.Ping(); err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		journalPool, err := pgxpool.New(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer journalPool.Close()
		journalStore := journal.Store{DB: journalPool, Cards: journal.SQLCardIngest{DB: journalPool}}
		journalStore.Register(mux)
		journalMux := http.NewServeMux()
		journalStore.RegisterPrivate(journalMux)
		mux.Handle("/v1/journal", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if devTenantID != "" {
				ctx = journal.WithTenant(ctx, devTenantID)
			}
			journalMux.ServeHTTP(w, r.WithContext(ctx))
		}))
		mux.Handle("/v1/journal/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if devTenantID != "" {
				ctx = journal.WithTenant(ctx, devTenantID)
			}
			journalMux.ServeHTTP(w, r.WithContext(ctx))
		}))
		mux.Handle("/v1/mcp-tokens", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if devTenantID != "" {
				ctx = journal.WithTenant(ctx, devTenantID)
			}
			journalMux.ServeHTTP(w, r.WithContext(ctx))
		}))
		mux.Handle("/v1/mcp-tokens/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if devTenantID != "" {
				ctx = journal.WithTenant(ctx, devTenantID)
			}
			journalMux.ServeHTTP(w, r.WithContext(ctx))
		}))
		if devTenantID != "" {
			devOnly := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ctx := agents.WithTenant(r.Context(), devTenantID)
					ctx = cards.WithTenant(ctx, devTenantID)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			}
			cardHandler, err := cards.NewHandler(db, timezone, noteSink)
			if err != nil {
				log.Fatal(err)
			}
			eventsCtx, stopEvents := context.WithCancel(context.Background())
			defer stopEvents()
			cardHandler.Events = cards.StartEvents(eventsCtx, databaseURL)
			cardMux := http.NewServeMux()
			cardHandler.Routes(cardMux)
			mux.Handle("/v1/cards", devOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cardMux.ServeHTTP(w, r) })))
			mux.Handle("/v1/cards/", devOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { cardMux.ServeHTTP(w, r) })))
		}
	}
	server := &http.Server{Addr: listenAddr, Handler: httpapi.BuildHeader(buildSHA, httpapi.APIVersion, mux)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", listenAddr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			if closeErr := server.Close(); closeErr != nil {
				log.Printf("server close failed: %v", closeErr)
			}
		}
		if err := <-serveErr; err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server failed: %v", err)
		}
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
