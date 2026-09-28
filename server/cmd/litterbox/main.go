package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Komzpa/litterbox/server/internal/httpapi"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "connect" {
		os.Exit(RunConnect(os.Args[2:], os.Stdout))
	}

	listenAddr := envOrDefault("LISTEN_ADDR", ":8080")
	databaseURL := os.Getenv("DATABASE_URL")
	flag.StringVar(&listenAddr, "listen", listenAddr, "HTTP listen address (or LISTEN_ADDR)")
	flag.StringVar(&databaseURL, "database-url", databaseURL, "PostgreSQL connection URL (or DATABASE_URL)")
	flag.Parse()
	_ = databaseURL // Connection setup is owned by the database layer.

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", httpapi.Healthz)
	server := &http.Server{Addr: listenAddr, Handler: mux}

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
