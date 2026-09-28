package agents

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresCardsIntegration(t *testing.T) {
	if os.Getenv("AGENT_TEST_POSTGRES") != "1" {
		t.Skip("run under pg_virtualenv with AGENT_TEST_POSTGRES=1")
	}
	db, err := sql.Open("pgx", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`DROP SCHEMA public CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP ROLE IF EXISTS litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`GRANT USAGE ON SCHEMA public TO PUBLIC`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../../db/001_mail.sql", "../../../db/002_security.sql", "../../../db/003_agent_cards.sql"} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
	}
	a := "11111111-1111-4111-8111-111111111111"
	b := "22222222-2222-4222-8222-222222222222"
	if _, err = db.Exec(`INSERT INTO tenants(id) VALUES ($1),($2)`, a, b); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	c := Card{Source: "agent", ExternalID: "omp:fixture", Title: "Fixture result", Summary: "Synthetic result.", SortAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC), State: "open"}
	for _, tenant := range []string{a, b} {
		if _, err := db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, tenant); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO cards(tenant_id,id,source,external_id,title,summary,sort_at,state) VALUES($1,gen_random_uuid(),$2,$3,$4,$5,$6,$7)`, tenant, c.Source, c.ExternalID, c.Title, c.Summary, c.SortAt, c.State); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	list := func(tenant string) []Card {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/cards", nil)
		if tenant != "" {
			req = req.WithContext(WithTenant(req.Context(), tenant))
		}
		w := httptest.NewRecorder()
		CardsHandler(db)(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET status=%d body=%s", w.Code, w.Body.String())
		}
		var response struct {
			Cards []Card `json:"cards"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Cards
	}
	ac := list(a)
	bc := list(b)
	if len(ac) != 1 || len(bc) != 1 || ac[0].ID == bc[0].ID {
		t.Fatalf("bad tenant-scoped identity: %#v %#v", ac, bc)
	}
	idBefore := ac[0].ID
	mux := http.NewServeMux()
	mux.Handle("POST /v1/cards/{id}/dismiss", DismissHandler(db))
	dismiss := func(tenant, id string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/cards/"+id+"/dismiss", nil)
		req = req.WithContext(WithTenant(req.Context(), tenant))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w.Code
	}
	if status := dismiss(a, bc[0].ID); status != http.StatusNotFound {
		t.Fatalf("foreign dismissal status=%d", status)
	}
	if status := dismiss(a, ac[0].ID); status != http.StatusNoContent {
		t.Fatalf("dismissal status=%d", status)
	}

	c.Summary = "Updated synthetic result."
	if _, err := db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE cards SET summary=$3 WHERE tenant_id=$1 AND source='agent' AND external_id=$2`, a, c.ExternalID, c.Summary); err != nil {
		t.Fatal(err)
	}
	ac = list(a)
	if ac[0].State != "done" || ac[0].Summary != c.Summary || ac[0].ID != idBefore {
		t.Fatalf("dismiss/update failed: %#v", ac)
	}
	if list(b)[0].State != "open" {
		t.Fatal("dismiss crossed tenants")
	}
	if _, err = db.Exec(`SET ROLE litterbox_app`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`SELECT set_config('litterbox.tenant_id',$1,false)`, a); err != nil {
		t.Fatal(err)
	}
	var foreign int
	if err = db.QueryRow(`SELECT count(*) FROM cards WHERE tenant_id=$1`, b).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if foreign != 0 {
		t.Fatal("RLS leaked foreign cards")
	}
	if _, err := db.Exec(`RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	CardsHandler(db)(w, httptest.NewRequest("GET", "/v1/cards", nil))
	if w.Code != 401 || strings.Contains(w.Body.String(), c.Summary) {
		t.Fatalf("unauthenticated response: %d %s", w.Code, w.Body.String())
	}
}

func TestCardsHandlerRejectsUnauthenticatedBeforeDB(t *testing.T) {
	w := httptest.NewRecorder()
	CardsHandler(nil)(w, httptest.NewRequest("GET", "/v1/cards", nil))
	if w.Code != 401 {
		t.Fatalf("got %d", w.Code)
	}
}
