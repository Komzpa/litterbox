package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Komzpa/litterbox/server/internal/journal"
	"github.com/Komzpa/litterbox/server/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStaleOwnerEditsKeepVisibleConflictAndJournalProjection(t *testing.T) {
	if os.Getenv("CARD_TEST_POSTGRES") != "1" {
		t.Skip("requires pg_virtualenv")
	}
	f := testdb.Setup(t)
	ctx := context.Background()
	db, err := pgxpool.New(ctx, f.DSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	files, err := filepath.Glob("../../db/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, f.Migration(string(b))); err != nil {
			t.Fatal(err)
		}
	}
	tenant, card := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO cards(tenant_id,id,source,external_id,title) VALUES($1,$2,'manual',$2::uuid::text,'Original')`, tenant, card); err != nil {
		t.Fatal(err)
	}
	t.Run("notes", func(t *testing.T) {
		api := API{DB: db, Identity: func(context.Context) (string, string, bool) { return tenant.String(), "", true }}
		for _, edit := range []struct {
			text string
			base int
		}{{"newer text", 0}, {"stale offline text", 0}} {
			body := fmt.Sprintf(`{"op_id":%q,"card_id":%q,"type":"note","args":{"note":%q,"base_version":%d}}`, uuid.New(), card, edit.text, edit.base)
			for i := 0; i < 2; i++ {
				w := httptest.NewRecorder()
				api.ServeHTTP(w, httptest.NewRequest("POST", "/v1/ops", strings.NewReader(body)))
				if w.Code != 200 {
					t.Fatalf("note: %d %s", w.Code, w.Body.String())
				}
			}
		}
		var note string
		var copies int
		if err = db.QueryRow(ctx, `SELECT note FROM cards WHERE tenant_id=$1 AND id=$2`, tenant, card).Scan(&note); err != nil {
			t.Fatal(err)
		}
		if err = db.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND title='Conflicting card note' AND state='open' AND summary='stale offline text'`, tenant).Scan(&copies); err != nil {
			t.Fatal(err)
		}
		if note != "newer text" || copies != 1 {
			t.Fatalf("note=%q visible conflicts=%d", note, copies)
		}
	})
	t.Run("journal", func(t *testing.T) {
		var note string
		var copies int
		store := journal.Store{DB: db}
		entry, err := store.Create(ctx, tenant.String(), "initial")
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		store.RegisterPrivate(mux)
		for _, text := range []string{"new journal text", "stale journal text"} {
			b, _ := json.Marshal(map[string]any{"body": text, "base_version": 0})
			r := httptest.NewRequest("PUT", "/v1/journal/"+entry.ID, strings.NewReader(string(b)))
			r = r.WithContext(journal.WithTenant(r.Context(), tenant.String()))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("journal %d %s", w.Code, w.Body.String())
			}
		}
		if err = db.QueryRow(ctx, `SELECT body FROM journal_entries WHERE tenant_id=$1 AND id=$2`, tenant, entry.ID).Scan(&note); err != nil {
			t.Fatal(err)
		}
		if note != "new journal text" {
			t.Fatalf("journal overwritten: %q", note)
		}
		if err = db.QueryRow(ctx, `SELECT count(*) FROM cards WHERE tenant_id=$1 AND source='journal' AND summary IN ('new journal text','stale journal text') AND state='open'`, tenant).Scan(&copies); err != nil || copies != 2 {
			t.Fatalf("journal projections=%d err=%v", copies, err)
		}
	})
}
