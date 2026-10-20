package gmailsync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/Komzpa/litterbox/server/internal/cards"
	"github.com/Komzpa/litterbox/server/internal/httpapi"
	"github.com/Komzpa/litterbox/server/internal/mailbody"
	"github.com/Komzpa/litterbox/server/internal/mailhtml"
	"github.com/Komzpa/litterbox/server/internal/ops"
	"github.com/Komzpa/litterbox/server/internal/sources/agents"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestArchiveUndoReopensOnceAfterProviderSuccess(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	neighbor := uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,pinned_rank) VALUES($1,$2,'manual','undo-neighbor',1)`, f.tenant, neighbor); err != nil { t.Fatal(err) }
	f.post(t, uuid.New(), f.card, "archive", `{}`, 200)
	f.state(t, f.card, "archived")
	var cursor int64
	if err := f.db.QueryRow(ctx, `SELECT COALESCE(max(seq),0) FROM changes WHERE tenant_id=$1`, f.tenant).Scan(&cursor); err != nil { t.Fatal(err) }
	inverse := uuid.New()
	f.post(t, inverse, f.card, "gmail.label_add", `{"label":"INBOX"}`, 200)
	f.state(t, f.card, "open")
	f.post(t, inverse, f.card, "gmail.label_add", `{"label":"INBOX"}`, 200)
	f.state(t, neighbor, "open")
	var count int
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM changes WHERE tenant_id=$1 AND seq>$2 AND id=$3 AND value->>'state'='open'`, f.tenant, cursor, f.card).Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("open changes=%d, want one normal card change", count) }
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) != 2 { t.Fatalf("calls=%+v, want archive then one inverse", f.calls) }
	call := f.calls[1]
	if call.Thread != "thread-1" || call.Token != "Bearer test-access" || !reflect.DeepEqual(call.Add, []string{"INBOX"}) || len(call.Remove) != 0 { t.Fatalf("wrong inverse account/thread/labels: %+v", call) }
}

func TestArchiveUndoFailureAndIsolation(t *testing.T) {
	f := newSpamFixture(t)
	ctx := context.Background()
	f.post(t, uuid.New(), f.card, "archive", `{}`, 200)
	f.fail = true
	inverse := uuid.New()
	body := f.post(t, inverse, f.card, "gmail.label_add", `{"label":"INBOX"}`, 422)
	if !strings.Contains(body, "fixture Gmail denied") { t.Fatal(body) }
	f.state(t, f.card, "archived")
	var count int
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM ops WHERE tenant_id=$1 AND op_id=$2`, f.tenant, inverse).Scan(&count); err != nil { t.Fatal(err) }
	if count != 0 { t.Fatal("failed inverse persisted a receipt") }
	f.fail = false
	// Adding an unrelated label must not reopen an archived card.
	f.post(t, uuid.New(), f.card, "gmail.label_add", `{"label":"STARRED"}`, 200)
	f.state(t, f.card, "archived")
	foreign, tenant := uuid.New(), uuid.New()
	if _, err := f.db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id) VALUES($1,$2,'manual','foreign')`, tenant, foreign); err != nil { t.Fatal(err) }
	before := len(f.calls)
	f.post(t, uuid.New(), foreign, "gmail.label_add", `{"label":"INBOX"}`, 422)
	if len(f.calls) != before { t.Fatal("foreign card touched Gmail") }
	f.state(t, f.card, "archived")
}

// startNativeConsumer builds the native-consumer HTTP surface — real auth
// middleware, cards handler, ops API, version and the production mailbody
// handler — around the spam fixture's registered loopback Gmail, mirroring the
// cmd/litterbox wiring. The mailbody route is registered exactly as production
// serves it (mailbody.Handler behind the same private auth), including the
// devOnly cards→agents tenant bridge the handler reads; it serves the fixture's
// real card_bodies rows — no fake body is injected.
func startNativeConsumer(t *testing.T, f *spamFixture) *httptest.Server {
	t.Helper()
	for _, name := range []string{"004_card_time_note.sql", "015_card_note_updated.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "db", name))
		if err != nil { t.Fatal(err) }
		if _, err := f.db.Exec(context.Background(), f.tdb.Migration(string(migration))); err != nil { t.Fatal(err) }
	}
	if os.Getenv("PGHOST") == "" || os.Getenv("PGPORT") == "" || os.Getenv("PGPORT") == "5432" || os.Getenv("PGPORT") == "5435" || os.Getenv("PGPORT") == "5436" { t.Fatal("disposable pg_virtualenv cluster required; per-test litterbox_test_ role and schema are created by testdb") }
	db, err := sql.Open("pgx", f.dsn)
	if err != nil { t.Fatal(err) }
	mux := http.NewServeMux()
	cardHandler, err := cards.NewHandler(db, "UTC", "")
	if err != nil { t.Fatal(err) }
	cardHandler.Routes(mux)
	mux.HandleFunc("GET /v1/version", httpapi.Version("frozen-archive-undo-test-host", httpapi.APIVersion))
	mux.Handle("POST /v1/ops", ops.API{DB: f.db, Identity: func(ctx context.Context) (string, string, bool) { d, ok := auth.DeviceFromContext(ctx); return d.TenantID, d.DeviceID, ok }})
	mux.Handle("GET /v1/cards/{id}/body", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := cards.TenantFrom(r.Context())
		if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
		mailbody.Handler{DB: db, Images: mailhtml.NewImageFetcher(nil, nil)}.ServeHTTP(w, r.WithContext(agents.WithTenant(r.Context(), tenant)))
    }))
    mux.HandleFunc("POST /v1/__test/external-archive/{id}", func(w http.ResponseWriter, r *http.Request) {
        tenant, ok := cards.TenantFrom(r.Context())
        if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
        tx, err := db.BeginTx(r.Context(), nil)
        if err != nil { http.Error(w, "internal server error", http.StatusInternalServerError); return }
        defer tx.Rollback()
        if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
            http.Error(w, "internal server error", http.StatusInternalServerError); return
        }
        result, err := tx.ExecContext(r.Context(), `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND id=$2 AND state='open'`, tenant, r.PathValue("id"))
        if err != nil { http.Error(w, "internal server error", http.StatusInternalServerError); return }
        count, err := result.RowsAffected()
        if err != nil { http.Error(w, "internal server error", http.StatusInternalServerError); return }
        if count != 1 { http.NotFound(w, r); return }
        if err := tx.Commit(); err != nil { http.Error(w, "internal server error", http.StatusInternalServerError); return }
        w.WriteHeader(http.StatusNoContent)
    })
    a := auth.NewHandler(auth.NewPostgresStore(f.db), "")
    return httptest.NewServer(a.Middleware(mux))
}

// Opt-in native proof: actual authentication/card/operation handlers and the
// registered Gmail connector, with only the provider replaced by loopback HTTP.
func TestArchiveUndoNativeConsumer(t *testing.T) {
	binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
	if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
	f := newSpamFixture(t)
	server := startNativeConsumer(t, f)
	defer server.Close()
	ctx := context.Background()
	neighbor, device := uuid.New(), uuid.New()
	token := uuid.NewString() + uuid.NewString()
	hash := sha256.Sum256([]byte(token))
	if _, err := f.db.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, f.tenant, device, hash[:]); err != nil { t.Fatal(err) }
	defer f.db.Exec(ctx, `UPDATE devices SET revoked_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, device)
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject,created_at,sort_at) VALUES($1,$2,$3,'thread-2','mail','Neighbor','Successor B',now()-interval '1 hour',now()-interval '1 hour')`, f.tenant, neighbor, f.account); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>B</p>')`, f.tenant, neighbor); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `UPDATE cards SET created_at=now(), sort_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
	if _, err := f.db.Exec(ctx, `UPDATE cards SET subject='Disposable Archive Undo proof' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
    if err := os.MkdirAll(output, 0o700); err != nil { t.Fatal(err) }
    command := exec.Command("python3", filepath.Join("..", "..", "..", "qt", "tests", "archive_undo_consumer.py"), "--binary", binary, "--output", output)
	command.Env = append(os.Environ(), "LB_UNDO_URL="+server.URL, "LB_UNDO_TOKEN="+token, "LB_UNDO_CARD="+f.card.String(), "LB_UNDO_NEIGHBOR="+neighbor.String())
	log, runErr := command.CombinedOutput()
	if err := os.WriteFile(filepath.Join(output, "consumer-driver.log"), log, 0600); err != nil { t.Fatal(err) }
	var states []map[string]any
	rows, err := f.db.Query(ctx, `SELECT id::text,state FROM cards WHERE tenant_id=$1 ORDER BY id`, f.tenant)
	if err != nil { t.Fatal(err) }
	for rows.Next() { var id, state string; if err := rows.Scan(&id, &state); err != nil { t.Fatal(err) }; states = append(states, map[string]any{"id": id, "state": state}) }; rows.Close()
	var durable []json.RawMessage
	rows, err = f.db.Query(ctx, `SELECT payload FROM ops WHERE tenant_id=$1 ORDER BY created_at, op_id`, f.tenant)
	if err != nil { t.Fatal(err) }
	for rows.Next() { var payload []byte; if err := rows.Scan(&payload); err != nil { t.Fatal(err) }; durable = append(durable, json.RawMessage(payload)) }; rows.Close()
	f.mu.Lock()
	receipt := map[string]any{"states": states, "durable_ops": durable, "gmail_calls": f.calls, "actual_handlers": []string{"auth.Middleware", "cards.Handler", "ops.API", "gmailsync.RegisterOps"}, "provider": "loopback fake Gmail only", "cleanup": "deferred httptest servers, own token revocation, testdb schema/role and pg_virtualenv cluster cleanup"}
	f.mu.Unlock()
	data, _ := json.MarshalIndent(receipt, "", "  ")
	if err := os.WriteFile(filepath.Join(output, "server-receipt.json"), data, 0600); err != nil { t.Fatal(err) }
	if runErr != nil { t.Fatalf("native oracle: %v\n%s", runErr, log) }
	f.state(t, f.card, "open")
	f.state(t, neighbor, "open")
	if len(durable) != 2 || len(f.calls) != 2 { t.Fatalf("durable=%s Gmail=%+v, want archive then inverse once", durable, f.calls) }
}
func runArchiveEntry(t *testing.T, entry, output string, mutate func(ctx context.Context, f *spamFixture, neighbor uuid.UUID) (card, finalNeighbor, extra string)) {
    t.Helper()
    binary := os.Getenv("LB_QT_UNDO_BINARY")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    f := newSpamFixture(t)
    server := startNativeConsumer(t, f)
    defer server.Close()
    ctx := context.Background()
    neighbor, device := uuid.New(), uuid.New()
    token := uuid.NewString() + uuid.NewString()
    hash := sha256.Sum256([]byte(token))
    if _, err := f.db.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, f.tenant, device, hash[:]); err != nil { t.Fatal(err) }
    defer f.db.Exec(ctx, `UPDATE devices SET revoked_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, device)
    if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject,created_at,sort_at) VALUES($1,$2,$3,'thread-2','mail','Neighbor','Successor B',now()-interval '1 hour',now()-interval '1 hour')`, f.tenant, neighbor, f.account); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>B</p>')`, f.tenant, neighbor); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `UPDATE cards SET created_at=now(), sort_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `UPDATE cards SET subject='Disposable Archive Undo proof' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
    card, finalNeighbor, extra := f.card.String(), neighbor.String(), ""
    if mutate != nil {
        card, finalNeighbor, extra = mutate(ctx, f, neighbor)
    }
    entryOutput := output + "-" + entry
    if err := os.MkdirAll(entryOutput, 0o700); err != nil { t.Fatal(err) }
    command := exec.Command("python3", filepath.Join("..", "..", "..", "qt", "tests", "archive_undo_consumer.py"), "--binary", binary, "--output", entryOutput, "--entry", entry)
    env := append(os.Environ(), "LB_UNDO_URL="+server.URL, "LB_UNDO_TOKEN="+token, "LB_UNDO_CARD="+card, "LB_UNDO_NEIGHBOR="+finalNeighbor)
    if extra != "" { env = append(env, "LB_UNDO_EXTRA="+extra) }
    command.Env = env
    log, runErr := command.CombinedOutput()
    if err := os.WriteFile(filepath.Join(entryOutput, "consumer-driver.log"), log, 0600); err != nil { t.Fatal(err) }
    var durable []json.RawMessage
    rows, err := f.db.Query(ctx, `SELECT payload FROM ops WHERE tenant_id=$1 ORDER BY created_at, op_id`, f.tenant)
    if err != nil { t.Fatal(err) }
    for rows.Next() { var payload []byte; if err := rows.Scan(&payload); err != nil { t.Fatal(err) }; durable = append(durable, json.RawMessage(payload)) }; rows.Close()
    f.mu.Lock()
    receipt := map[string]any{"entry": entry, "durable_ops": durable, "gmail_calls": f.calls, "actual_handlers": []string{"auth.Middleware", "cards.Handler", "ops.API", "gmailsync.RegisterOps"}, "provider": "loopback fake Gmail only"}
    f.mu.Unlock()
    data, _ := json.MarshalIndent(receipt, "", "  ")
    if err := os.WriteFile(filepath.Join(entryOutput, "server-receipt.json"), data, 0600); err != nil { t.Fatal(err) }
    if runErr != nil { t.Fatalf("%s oracle: %v\n%s", entry, runErr, log) }
}

func TestArchiveRowNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "row", output, func(ctx context.Context, f *spamFixture, neighbor uuid.UUID) (string, string, string) {
        return f.card.String(), neighbor.String(), ""
    })
}

func TestArchiveBundleNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "bundle", output, func(ctx context.Context, f *spamFixture, neighbor uuid.UUID) (string, string, string) {
        bundle := uuid.New()
        if _, err := f.db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Archive bundle','{0}')`, f.tenant, bundle); err != nil { t.Fatal(err) }
        if _, err := f.db.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card, bundle); err != nil { t.Fatal(err) }
        return f.card.String(), neighbor.String(), ""
    })
}

func TestArchiveBundlePinnedNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    f := newSpamFixture(t)
    server := startNativeConsumer(t, f)
    defer server.Close()
    ctx := context.Background()
    neighbor, device := uuid.New(), uuid.New()
    token := uuid.NewString() + uuid.NewString()
    hash := sha256.Sum256([]byte(token))
    if _, err := f.db.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, f.tenant, device, hash[:]); err != nil { t.Fatal(err) }
    defer f.db.Exec(ctx, `UPDATE devices SET revoked_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, device)
    if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject,created_at,sort_at) VALUES($1,$2,$3,'thread-2','mail','Neighbor','Successor B',now()-interval '1 hour',now()-interval '1 hour')`, f.tenant, neighbor, f.account); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>B</p>')`, f.tenant, neighbor); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `UPDATE cards SET created_at=now(), sort_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `UPDATE cards SET subject='Disposable Archive Undo proof' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
    bundle := uuid.New()
    if _, err := f.db.Exec(ctx, `INSERT INTO bundles(tenant_id,id,title,centroid) VALUES($1,$2,'Archive bundle','{0}')`, f.tenant, bundle); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card, bundle); err != nil { t.Fatal(err) }
    second := uuid.New()
    if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject,bundle_id,created_at,sort_at) VALUES($1,$2,$3,'thread-a2','mail','Sender A2','Bundle member A2',$4,now()-interval '30 minutes',now()-interval '30 minutes')`, f.tenant, second, f.account, bundle); err != nil { t.Fatal(err) }
    pinned := uuid.New()
    if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,source,sender_name,subject,bundle_id,pinned_rank,created_at,sort_at) VALUES($1,$2,$3,'thread-p','mail','Pinned','Pinned P',$4,1,now(),now())`, f.tenant, pinned, f.account, bundle); err != nil { t.Fatal(err) }
    if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>P</p>')`, f.tenant, pinned); err != nil { t.Fatal(err) }
    entry := "bundle_pinned"
    entryOutput := output + "-" + entry
    if err := os.MkdirAll(entryOutput, 0o700); err != nil { t.Fatal(err) }
    command := exec.Command("python3", filepath.Join("..", "..", "..", "qt", "tests", "archive_undo_consumer.py"), "--binary", binary, "--output", entryOutput, "--entry", entry)
    command.Env = append(os.Environ(), "LB_UNDO_URL="+server.URL, "LB_UNDO_TOKEN="+token, "LB_UNDO_CARD="+f.card.String(), "LB_UNDO_NEIGHBOR="+neighbor.String(), "LB_UNDO_EXTRA="+second.String(), "LB_UNDO_PINNED="+pinned.String())
    log, runErr := command.CombinedOutput()
    if err := os.WriteFile(filepath.Join(entryOutput, "consumer-driver.log"), log, 0600); err != nil { t.Fatal(err) }
    var durable []json.RawMessage
    rows, err := f.db.Query(ctx, `SELECT payload FROM ops WHERE tenant_id=$1 ORDER BY created_at, op_id`, f.tenant)
    if err != nil { t.Fatal(err) }
    for rows.Next() { var payload []byte; if err := rows.Scan(&payload); err != nil { t.Fatal(err) }; durable = append(durable, json.RawMessage(payload)) }; rows.Close()
    f.mu.Lock()
    receipt := map[string]any{"entry": entry, "durable_ops": durable, "gmail_calls": f.calls, "actual_handlers": []string{"auth.Middleware", "cards.Handler", "ops.API", "gmailsync.RegisterOps"}, "provider": "loopback fake Gmail only"}
    f.mu.Unlock()
    data, _ := json.MarshalIndent(receipt, "", "  ")
    if err := os.WriteFile(filepath.Join(entryOutput, "server-receipt.json"), data, 0600); err != nil { t.Fatal(err) }
    if runErr != nil { t.Fatalf("%s oracle: %v\n%s", entry, runErr, log) }
}
func TestArchiveEndNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "end", output, nil)
}

func TestArchiveExternalNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "external", output, nil)
}

func TestArchiveSpamNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "spam", output, nil)
}

func TestArchiveDoneNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "done", output, func(ctx context.Context, f *spamFixture, neighbor uuid.UUID) (string, string, string) {
        manual := uuid.New()
        if _, err := f.db.Exec(ctx, `UPDATE cards SET state='archived' WHERE tenant_id=$1 AND id=$2`, f.tenant, f.card); err != nil { t.Fatal(err) }
        if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,subject,created_at,sort_at) VALUES($1,$2,'manual','done-proof','Disposable Done proof',now(),now())`, f.tenant, manual); err != nil { t.Fatal(err) }
        if _, err := f.db.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,'<p>M</p>')`, f.tenant, manual); err != nil { t.Fatal(err) }
        return manual.String(), neighbor.String(), ""
    })
}

func TestArchiveSnoozeNativeConsumer(t *testing.T) {
    binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
    if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
    runArchiveEntry(t, "snooze", output, nil)
}

// Consumer-level fail=true Undo-rejection leg: the provider rejects the archive
// and the actual Qt reader must show the failure truthfully (no false success —
// not a lingering "Archived", and the card is not left archived). Separate from
// TestArchiveUndoNativeConsumer so its preserved ACK scope (exactly 2 durable /
// 2 Gmail calls) is untouched; this leg owns its own fixture (f.fail) and
// asserts the rejected flow persisted no receipt.
func TestArchiveUndoNativeConsumerReject(t *testing.T) {
	binary, output := os.Getenv("LB_QT_UNDO_BINARY"), os.Getenv("LB_QT_UNDO_OUTPUT")
	if binary == "" || output == "" { t.Skip("LB_QT_UNDO_BINARY and LB_QT_UNDO_OUTPUT required") }
	f := newSpamFixture(t)
	server := startNativeConsumer(t, f)
	defer server.Close()
	ctx := context.Background()
	neighbor, device := uuid.New(), uuid.New()
	token := uuid.NewString() + uuid.NewString()
	hash := sha256.Sum256([]byte(token))
	if _, err := f.db.Exec(ctx, `INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, f.tenant, device, hash[:]); err != nil { t.Fatal(err) }
	defer f.db.Exec(ctx, `UPDATE devices SET revoked_at=now() WHERE tenant_id=$1 AND id=$2`, f.tenant, device)
	if _, err := f.db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,pinned_rank) VALUES($1,$2,'manual','reject-neighbor',1)`, f.tenant, neighbor); err != nil { t.Fatal(err) }
	f.fail = true // provider rejects the archive op: the reader must surface the failure truthfully
	rejectOutput := output + "-reject"
	if err := os.MkdirAll(rejectOutput, 0o700); err != nil { t.Fatal(err) }
	command := exec.Command("python3", filepath.Join("..", "..", "..", "qt", "tests", "archive_undo_consumer.py"), "--binary", binary, "--output", rejectOutput)
	command.Env = append(os.Environ(), "LB_UNDO_URL="+server.URL, "LB_UNDO_TOKEN="+token, "LB_UNDO_CARD="+f.card.String(), "LB_UNDO_NEIGHBOR="+neighbor.String(), "LB_UNDO_REJECT=1")
	log, runErr := command.CombinedOutput()
	if err := os.WriteFile(filepath.Join(rejectOutput, "consumer-driver.log"), log, 0600); err != nil { t.Fatal(err) }
	var durable []json.RawMessage
	rows, err := f.db.Query(ctx, `SELECT payload FROM ops WHERE tenant_id=$1 ORDER BY created_at, op_id`, f.tenant)
	if err != nil { t.Fatal(err) }
	for rows.Next() { var payload []byte; if err := rows.Scan(&payload); err != nil { t.Fatal(err) }; durable = append(durable, json.RawMessage(payload)) }; rows.Close()
	f.mu.Lock()
	callCount := len(f.calls)
	receipt := map[string]any{"durable_ops": durable, "gmail_calls": f.calls, "fail": f.fail, "leg": "consumer fail=true undo rejection", "provider": "loopback fake Gmail only"}
	f.mu.Unlock()
	data, _ := json.MarshalIndent(receipt, "", "  ")
	if err := os.WriteFile(filepath.Join(rejectOutput, "server-receipt.json"), data, 0600); err != nil { t.Fatal(err) }
	if runErr != nil { t.Fatalf("reject oracle: %v\n%s", runErr, log) }
	if len(durable) != 0 { t.Fatalf("durable=%s, rejected archive must persist no receipt", durable) }
	if callCount != 1 { t.Fatalf("Gmail calls=%d, want one rejected archive modify", callCount) }
	f.state(t, f.card, "open") // truthful: the rejected archive left the card unarchived
}
