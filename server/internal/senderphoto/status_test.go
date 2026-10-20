package senderphoto

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/auth"
	"github.com/Komzpa/litterbox/server/internal/gmail"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestStatusRouteTenantVisibility drives GET /v1/gmail/sender-photos/status
// through the production HTTP path — canonical device-token auth middleware
// (no dev bypass) around the registered StatusHandler — against a real
// pg_virtualenv schema queried as the granted runtime role (SET ROLE, so the
// 023 tenant_isolation RLS policy is active exactly as in production). The
// own tenant's accounts must be visible in every grant state, a neighbor
// tenant's accounts must never appear, and an unauthenticated request must be
// denied 401 by the canonical HTTP auth instead of returning any list.
func TestStatusRouteTenantVisibility(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with CARD_TEST_POSTGRES=1")
	}
	if os.Getenv("PGHOST") == "" || os.Getenv("PGPORT") == "" || os.Getenv("PGPORT") == "5432" || os.Getenv("PGPORT") == "5435" || os.Getenv("PGPORT") == "5436" {
		t.Fatal("disposable pg_virtualenv cluster required; per-test litterbox_test_ role and schema are created by testdb")
	}
	tdb := testdb.Setup(t)
	db, err := sql.Open("pgx", tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	// Canonical lexical migration set (R67): never a per-test subset.
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no migrations discovered")
	}
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(tdb.Migration(string(body))); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}

	const (
		tenantA = "11111111-1111-4111-8111-111111111111"
		tenantB = "22222222-2222-4222-8222-222222222222"
		acctA1  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" // both photo scopes + stored photo
		acctA2  = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" // Gmail.modify only
		acctB1  = "cccccccc-cccc-4ccc-8ccc-cccccccccccc" // neighbor tenant
		granted = gmail.ScopeGmailModify + " " + gmail.ScopeContactsReadonly + " " + gmail.ScopeOtherContactsReadonly
	)
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES($1),($2)`, tenantA, tenantB); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO accounts(tenant_id,id,address,refresh_token,granted_photo_scopes) VALUES
		($1,$3,'granted@example.test',decode('00','hex'),$6),
		($1,$4,'missing@example.test',decode('00','hex'),$7),
		($2,$5,'neighbor@example.test',decode('00','hex'),$6)`,
		tenantA, tenantB, acctA1, acctA2, acctB1, granted, gmail.ScopeGmailModify); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO sender_photos(tenant_id,account_id,sender_address,photo,content_type) VALUES
		($1,$2,'photo.sender@example.test',decode('89504e47','hex'),'image/png'),
		($3,$4,'photo.sender@example.test',decode('89504e47','hex'),'image/png')`,
		tenantA, acctA1, tenantB, acctB1); err != nil {
		t.Fatal(err)
	}
	tokenA, tokenB := "device-token-tenant-a", "device-token-tenant-b"
	for _, d := range []struct{ tenant, device, token string }{
		{tenantA, "00000000-0000-4000-8000-000000000001", tokenA},
		{tenantB, "00000000-0000-4000-8000-000000000002", tokenB},
	} {
		hash := sha256.Sum256([]byte(d.token))
		if _, err = db.Exec(`INSERT INTO devices(tenant_id,id,token_hash) VALUES($1,$2,$3)`, d.tenant, d.device, hash[:]); err != nil {
			t.Fatal(err)
		}
	}

	// From here on the application role runs with the RLS policies active,
	// exactly like the production runtime role.
	if _, err = db.Exec(`SET ROLE ` + tdb.Role); err != nil {
		t.Fatal(err)
	}
	// Negative control for the context itself: with litterbox.tenant_id
	// unset the app role must see zero account rows. Visibility therefore
	// comes only from the request tenant transaction, and a handler that
	// forgets it can only ever return an empty list.
	var visible int
	if err = db.QueryRow(`SELECT count(*) FROM accounts`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("unset tenant context leaked %d account rows to the app role", visible)
	}

	authConn, err := pgx.Connect(t.Context(), tdb.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer authConn.Close(t.Context())
	if _, err = authConn.Exec(t.Context(), `SET ROLE `+tdb.Role); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /v1/gmail/sender-photos/status", StatusHandler{DB: db})
	server := httptest.NewServer(auth.NewHandler(auth.NewPostgresStore(authConn), "").Middleware(mux))
	defer server.Close()

	get := func(token string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+"/v1/gmail/sender-photos/status", nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, string(body)
	}
	type entry struct {
		AccountID             string   `json:"account_id"`
		Address               string   `json:"address"`
		PhotoReady            bool     `json:"photo_ready"`
		AuthorizationRequired bool     `json:"authorization_required"`
		RequiredScopes        []string `json:"required_scopes"`
	}
	decode := func(body string) []entry {
		t.Helper()
		var payload struct {
			Accounts []entry `json:"accounts"`
		}
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("status body %q: %v", body, err)
		}
		return payload.Accounts
	}

	// Unset/bogus credentials are denied by the canonical HTTP auth before
	// the handler runs — never an empty-but-200 list.
	for name, token := range map[string]string{"unset": "", "bogus": "not-a-device-token"} {
		code, body := get(token)
		if code != http.StatusUnauthorized {
			t.Fatalf("%s credential: status=%d body=%q, want 401", name, code, body)
		}
	}

	// Own tenant: exactly its two accounts, both grant states visible.
	code, body := get(tokenA)
	if code != http.StatusOK {
		t.Fatalf("own tenant: status=%d body=%q, want 200", code, body)
	}
	accounts := decode(body)
	if len(accounts) != 2 {
		t.Fatalf("own tenant accounts=%+v, want exactly 2", accounts)
	}
	byID := map[string]entry{}
	for _, e := range accounts {
		byID[e.AccountID] = e
	}
	a1, ok := byID[acctA1]
	if !ok {
		t.Fatalf("granted account missing from %+v", accounts)
	}
	if !a1.PhotoReady || a1.AuthorizationRequired || len(a1.RequiredScopes) != 0 {
		t.Fatalf("granted+stored account entry = %+v, want photo_ready and no authorization_required", a1)
	}
	a2, ok := byID[acctA2]
	if !ok {
		t.Fatalf("Gmail-only account missing from %+v", accounts)
	}
	if a2.PhotoReady || !a2.AuthorizationRequired {
		t.Fatalf("Gmail-only account entry = %+v, want authorization_required without photo", a2)
	}
	if len(a2.RequiredScopes) != 2 || a2.RequiredScopes[0] != gmail.ScopeContactsReadonly || a2.RequiredScopes[1] != gmail.ScopeOtherContactsReadonly {
		t.Fatalf("Gmail-only account required_scopes = %v, want %v", a2.RequiredScopes, gmail.PhotoScopes())
	}

	// Neighbor tenant's accounts never appear in the own-tenant report.
	if !json.Valid([]byte(body)) {
		t.Fatalf("status body not JSON: %q", body)
	}
	for _, leak := range []string{acctB1, "neighbor@example.test"} {
		if strings.Contains(body, leak) {
			t.Fatalf("own-tenant status leaked neighbor %q: %s", leak, body)
		}
	}

	// The neighbor's own device still sees its account — the rows exist and
	// visibility is scoped, not emptied.
	code, body = get(tokenB)
	if code != http.StatusOK {
		t.Fatalf("neighbor tenant: status=%d body=%q, want 200", code, body)
	}
	accounts = decode(body)
	if len(accounts) != 1 || accounts[0].AccountID != acctB1 || !accounts[0].PhotoReady {
		t.Fatalf("neighbor tenant accounts=%+v, want its one photo_ready account", accounts)
	}
}
