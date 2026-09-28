// Package agents reads OMP/Codex session JSONL into tenant-scoped cards.
//
// Operator setup: apply server/db/003_agent_cards.sql after migrations 001/002.
// Set session roots explicitly with -omp-root/OMP_AGENT_SESSIONS and
// -codex-root/CODEX_SESSIONS; no machine-specific paths are built into the reader.
// Run ingest-agents with -tenant/LITTERBOX_TENANT_ID and
// -database-url/DATABASE_URL, or use -dry-run to print count and three titles.
//
// main.go wiring (owned by the command integrator): before flag.Parse, dispatch
// os.Exit(RunIngestAgents(os.Args[2:], os.Stdout)) when os.Args[1] is
// "ingest-agents". After opening the database, register
// mux.Handle("GET /v1/cards", authenticated(agents.CardsHandler(db))). The
// authentication middleware must validate the device token/revocation using
// litterbox_device_by_token and call agents.WithTenant with its tenant UUID;
// never attach tenant identity from request JSON or URL parameters.
//
// OMP's stop/endTurn assistant message and Codex's final response are results
// awaiting the user. Tool-use, interrupted, and newer user turns are excluded.
// External IDs are namespaced by reader, so equal OMP/Codex IDs cannot collide.
// Summaries contain only assistant text (not thinking/tool arguments), bounded
// to 500 Unicode code points plus an ellipsis. OMP titles use session metadata or the first result line;
// Codex titles use the first result line. sort_at is the result timestamp.
//
// Upsert preserves the card ID and existing lifecycle state, including done,
// even if the session's result text changes. Dismiss sets an agent card to done
// within the authenticated tenant; it never writes back to a session file.
// CardsHandler returns {"cards":[...]} including lifecycle state and card UUID.
//
// Scoped proof: go test ./internal/sources/agents; PostgreSQL isolation and
// dismissal proof: pg_virtualenv env AGENT_TEST_POSTGRES=1 go test
// ./internal/sources/agents -run TestPostgresCardsIntegration -v.
package agents
