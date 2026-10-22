package gmailsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Komzpa/litterbox/server/internal/people"
	"github.com/google/uuid"
)

func photoPNGSize(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gravatarHash(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// Gravatar hit stores; Gravatar 404 falls through to the s2 domain icon;
// s2-globe falls back to DDG; gmail.com gets no icon; globe-identical and
// sub-32px icons are rejected. Old code stored 0 of these (no fallback).
func TestTrySenderPhotosGravatarAndFavicon(t *testing.T) {
	db, _ := testPool(t)
	ctx := context.Background()
	tenant, account := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO tenants(id) VALUES($1)`, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO accounts(tenant_id,id,address,refresh_token,granted_photo_scopes) VALUES($1,$2,'a@example.test',$3,'')`,
		tenant, account, []byte("rt")); err != nil {
		t.Fatal(err)
	}

	senders := []struct{ name, addr string }{
		{"G Hit", "ghit@avatar.example"},
		{"B Ico", "bico@brandico.example"},
		{"D Only", "donly@ddgonly.example"},
		{"Plain", "plain@gmail.com"},
		{"G Def", "gdef@globedef.example"},
		{"Small", "small@smallicon.example"},
	}
	for _, s := range senders {
		card := uuid.New()
		if _, err := db.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,subject,sender) VALUES($1,$2,$3,$4,'subj',$5)`,
			tenant, card, account, "t-"+s.addr, s.name+" <"+s.addr+">"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at,sender_name,sender_address) VALUES($1,$2,$3,$4,'{}','','','\x00',now(),$5,$6)`,
			tenant, uuid.New(), card, "m-"+s.addr, s.name, s.addr); err != nil {
			t.Fatal(err)
		}
	}

	globe := photoPNGSize(t, 64, 64, color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff})
	realIcon := photoPNGSize(t, 64, 64, color.RGBA{R: 0x10, G: 0x80, B: 0xc0, A: 0xff})
	ddgIcon := photoPNGSize(t, 64, 64, color.RGBA{R: 0xc0, G: 0x20, B: 0x10, A: 0xff})
	smallIcon := photoPNGSize(t, 16, 16, color.RGBA{R: 0x10, G: 0x80, B: 0xc0, A: 0xff})
	gravPhoto := photoPNG(t)
	ghitHash := gravatarHash("ghit@avatar.example")

	var hits atomic.Int64
	byURL := map[string]*atomic.Int64{}
	hitFor := func(k string) *atomic.Int64 {
		m, ok := byURL[k]
		if !ok {
			m = &atomic.Int64{}
			byURL[k] = m
		}
		return m
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		hitFor(r.URL.Path + "|" + r.URL.RawQuery).Add(1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/avatar/"):
			if strings.TrimPrefix(r.URL.Path, "/avatar/") == ghitHash {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(gravPhoto)
				return
			}
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/s2"):
			d := r.URL.Query().Get("domain")
			switch d {
			case "brandico.example":
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(realIcon)
			case "smallicon.example":
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(smallIcon)
			default:
				// invalid ref domain, globedef, ddgonly, gmail: the globe.
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(globe)
			}
		case strings.HasPrefix(r.URL.Path, "/ip3/"):
			if strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ip3/"), ".ico") == "ddgonly.example" {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(ddgIcon)
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	prevG, prevSG, prevDD := people.GravatarBaseURL, people.FaviconGoogleBase, people.FaviconDDGBase
	people.GravatarBaseURL = srv.URL + "/avatar"
	people.FaviconGoogleBase = srv.URL + "/s2"
	people.FaviconDDGBase = srv.URL + "/ip3"
	defer func() {
		people.GravatarBaseURL, people.FaviconGoogleBase, people.FaviconDDGBase = prevG, prevSG, prevDD
	}()
	prevLookup := bimiLogoURL
	bimiLogoURL = func(ctx context.Context, addr string) (string, bool) { return "", false }
	defer func() { bimiLogoURL = prevLookup }()
	prevHTTP := PhotoHTTPClient
	PhotoHTTPClient = srv.Client()
	defer func() { PhotoHTTPClient = prevHTTP }()
	people.ClearMisses()
	defer people.ClearMisses()

	s := &Syncer{DB: db, Build: "test"}
	c := &Client{HTTP: srv.Client(), accessToken: "t", tokenExpiry: time.Now().Add(time.Hour)}
	run := func() {
		t.Helper()
		if err := s.trySenderPhotos(ctx, Account{TenantID: tenant, ID: account}, c); err != nil {
			t.Fatalf("trySenderPhotos: %v", err)
		}
	}
	run()

	stored := func(addr string) int {
		var n int
		if err := db.QueryRow(ctx, `SELECT count(*) FROM sender_photos WHERE tenant_id=$1 AND account_id=$2 AND sender_address=$3`, tenant, account, addr).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for addr, want := range map[string]int{
		"ghit@avatar.example": 1, // Gravatar hit
		"bico@brandico.example": 1, // Gravatar 404 -> s2 icon
		"donly@ddgonly.example": 1, // s2 globe -> DDG icon
		"plain@gmail.com":       0, // free-mail: no icon
		"gdef@globedef.example": 0, // default globe rejected
		"small@smallicon.example": 0, // 16px icon rejected
	} {
		if got := stored(addr); got != want {
			t.Fatalf("stored photos for %s = %d, want %d", addr, got, want)
		}
	}

	// Misses must be cached: a second pass refetches nothing for the three
	// definitive negatives (their exact URLs see zero new hits; successes
	// and the per-run globe reference may legitimately refetch).
	missKeys := []string{}
	for k := range byURL {
		if strings.Contains(k, gravatarHash("plain@gmail.com")) ||
			strings.Contains(k, gravatarHash("gdef@globedef.example")) ||
			strings.Contains(k, gravatarHash("small@smallicon.example")) ||
			strings.Contains(k, "globedef.example") ||
			strings.Contains(k, "smallicon.example") {
			missKeys = append(missKeys, k)
		}
	}
	before := map[string]int64{}
	for _, k := range missKeys {
		before[k] = byURL[k].Load()
	}
	run()
	for _, k := range missKeys {
		if got := byURL[k].Load(); got != before[k] {
			t.Fatalf("miss URL refetched on second pass: %s hits %d -> %d (want no refetch)", k, before[k], got)
		}
	}
	t.Logf("external fetches: %d", hits.Load())
}
