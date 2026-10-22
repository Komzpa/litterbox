package gmailsync

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/gmail"
	"github.com/google/uuid"
)

func photoPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Production QA: sync keyed any message sender; flag keyed latest-message
// sender. An older-in-thread address must not be selected.
func TestRecentSendersUsesEffectiveSenderNotOlderMessage(t *testing.T) {
	db, tdb := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	card := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token) VALUES($1,$2,'a@example.test',$3)`, tenant, account, []byte("rt")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'t','subj','Old <old@example.test>')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at,sender_name,sender_address) VALUES
		($1,$2,$3,'m-old','{}','','','\x00',now()-interval '1 hour','Old','old@example.test'),
		($1,$4,$3,'m-new','{}','','','\x00',now(),'New','new@example.test')`, tenant, uuid.New(), card, uuid.New()); err != nil {
		t.Fatal(err)
	}
	s := &Syncer{DB: db, Build: "test"}
	_ = tdb
	got, err := recentSenders(ctx, s, Account{TenantID: tenant, ID: account})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, ",")
	if len(got) != 1 || got[0] != "new@example.test" {
		t.Fatalf("recentSenders=%v want only new@example.test (effective sender)", joined)
	}
}

// Production 2026-10-22: otherContacts 503 aborted the whole photo pass.
// Connections still have a photo for the card's effective sender; that
// photo must be stored.
func TestTrySenderPhotosStoresConnectionsWhenOtherContacts503(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	card := uuid.New()
	pngBytes := photoPNG(t)
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token,granted_photo_scopes) VALUES($1,$2,'a@example.test',$3,$4)`,
		tenant, account, []byte("rt"), gmail.FullPhotoScope()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,'t','subj','Alice <alice@example.com>')`, tenant, card, account); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at,sender_name,sender_address) VALUES($1,$2,$3,'m1','{}','','','\x00',now(),'Alice','alice@example.com')`, tenant, uuid.New(), card); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/people/me/connections"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"connections":[{"emailAddresses":[{"value":"alice@example.com"}],"photos":[{"url":"https://` + r.Host + `/a.png","default":false,"metadata":{"primary":true}}]}]}`))
		case strings.HasPrefix(r.URL.Path, "/otherContacts"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":503,"message":"The service is currently unavailable.","status":"UNAVAILABLE"}}`))
		case strings.HasSuffix(r.URL.Path, ".png"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	prevBase, prevHTTP := PeopleBaseURL, PhotoHTTPClient
	PeopleBaseURL = srv.URL
	PhotoHTTPClient = srv.Client()
	defer func() {
		PeopleBaseURL = prevBase
		PhotoHTTPClient = prevHTTP
	}()
	s := &Syncer{DB: db, Build: "test"}
	c := &Client{HTTP: srv.Client(), accessToken: "t", tokenExpiry: time.Now().Add(time.Hour)}
	if err := s.trySenderPhotos(ctx, Account{TenantID: tenant, ID: account}, c); err != nil {
		t.Fatalf("trySenderPhotos: %v", err)
	}
	var n int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM sender_photos WHERE tenant_id=$1 AND account_id=$2 AND sender_address='alice@example.com'`, tenant, account).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("stored photos for alice@example.com = %d, want 1 (connections index kept after otherContacts 503)", n)
	}
}
