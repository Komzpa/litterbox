package gmailsync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/Komzpa/litterbox/server/internal/bundles"
	"github.com/Komzpa/litterbox/server/internal/mailbody"
	"github.com/Komzpa/litterbox/server/internal/mailhtml"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Account struct {
	TenantID, ID uuid.UUID
	RefreshToken []byte
	HistoryID    string
}
type Syncer struct {
	DB           *pgxpool.Pool
	Client       func(Account) *Client
	Images       *mailhtml.ImageFetcher
	PollInterval time.Duration
}

func (s *Syncer) SyncAccount(ctx context.Context, a Account) error {
	c := s.Client(a)
	var cursor string
	tx, e := s.beginTenant(ctx, a.TenantID)
	if e != nil {
		return e
	}
	e = tx.QueryRow(ctx, "SELECT COALESCE(history_id,'') FROM accounts WHERE tenant_id=$1 AND id=$2", a.TenantID, a.ID).Scan(&cursor)
	if e != nil {
		tx.Rollback(ctx)
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	if cursor == "" {
		return s.initial(ctx, a, c)
	}
	return s.poll(ctx, a, c, cursor)
}
func (s *Syncer) initial(ctx context.Context, a Account, c *Client) error {
	var profile struct {
		HistoryID string `json:"historyId"`
	}
	if err := c.request(ctx, "GET", "/profile", nil, &profile); err != nil {
		return err
	}
	seen := map[string]bool{}
	page := ""
	for {
		x, e := c.ListInbox(ctx, page)
		if e != nil {
			return e
		}
		for _, t := range x.Threads {
			seen[t.ID] = true
			if e = s.saveThread(ctx, a, c, t.ID, true); e != nil {
				return e
			}
		}
		if x.NextPageToken == "" {
			break
		}
		page = x.NextPageToken
	}
	tx, e := s.beginTenant(ctx, a.TenantID)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	rows, e := tx.Query(ctx, "SELECT gmail_thread_id FROM cards WHERE tenant_id=$1 AND account_id=$2 AND state <> 'done'", a.TenantID, a.ID)
	if e != nil {
		return e
	}
	var old []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		old = append(old, id)
	}
	rows.Close()
	for _, id := range old {
		if !seen[id] {
			_, e = tx.Exec(ctx, "UPDATE cards SET state='archived',version=version+1 WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id=$3 AND state NOT IN ('archived','done')", a.TenantID, a.ID, id)
			if e != nil {
				return e
			}
		}
	}
	if profile.HistoryID != "" {
		_, e = tx.Exec(ctx, "UPDATE accounts SET history_id=$1,gmail_synced_at=now() WHERE tenant_id=$2 AND id=$3", profile.HistoryID, a.TenantID, a.ID)
		if e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	// Replay changes that arrived while the initial INBOX snapshot was loading.
	if profile.HistoryID != "" {
		return s.poll(ctx, a, c, profile.HistoryID)
	}
	return nil
}
func (s *Syncer) poll(ctx context.Context, a Account, c *Client, cursor string) error {
	page := ""
	ids := map[string]bool{}
	newest := cursor
	for {
		x, e := c.History(ctx, cursor, page)
		if e != nil {
			return e
		}
		if x.HistoryID != "" {
			newest = x.HistoryID
		}
		for _, h := range x.History {
			if h.ID != "" {
				newest = h.ID
			}
			for _, m := range h.MessagesAdded {
				ids[m.Message.ThreadID] = true
			}
			for _, m := range h.LabelsAdded {
				ids[m.Message.ThreadID] = true
			}
			for _, m := range h.LabelsRemoved {
				ids[m.Message.ThreadID] = true
			}
		}
		if x.NextPageToken == "" {
			break
		}
		page = x.NextPageToken
	}
	for id := range ids {
		t, e := c.GetThread(ctx, id)
		if e != nil {
			return e
		}
		inbox := false
		for _, m := range t.Messages {
			for _, l := range m.LabelIDs {
				if l == "INBOX" {
					inbox = true
				}
			}
		}
		if inbox {
			if e = s.saveThread(ctx, a, c, id, true); e != nil {
				return e
			}
		} else {
			tx, e := s.beginTenant(ctx, a.TenantID)
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, "UPDATE cards SET state='archived',version=version+1 WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id=$3 AND state NOT IN ('archived','done')", a.TenantID, a.ID, id)
			if e != nil {
				tx.Rollback(ctx)
				return e
			}
			if e = tx.Commit(ctx); e != nil {
				return e
			}
		}
	}
	if newest != cursor {
		tx, e := s.beginTenant(ctx, a.TenantID)
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, "UPDATE accounts SET history_id=$1,gmail_synced_at=now() WHERE tenant_id=$2 AND id=$3", newest, a.TenantID, a.ID)
		if e != nil {
			tx.Rollback(ctx)
			return e
		}
		return tx.Commit(ctx)
	}
	return nil
}
func (s *Syncer) saveThread(ctx context.Context, a Account, c *Client, threadID string, inbox bool) error {
	t, e := c.GetThread(ctx, threadID)
	if e != nil {
		return e
	}
	prepared := make([]mailhtml.Message, len(t.Messages))
	fetcher := s.Images
	if fetcher == nil {
		fetcher = mailhtml.NewImageFetcher(nil, nil)
	}
	for i, m := range t.Messages {
		raw, err := c.GetMessageRaw(ctx, m.ID)
		if err != nil {
			return err
		}
		parsed, err := mailhtml.Parse(raw)
		if err != nil {
			return err
		}
		if parsed.HTML != "" {
			for cid, part := range parsed.Inline {
				if strings.HasPrefix(part.ContentType, "image/") {
					parsed.HTML = strings.ReplaceAll(parsed.HTML, "cid:"+cid, "data:"+part.ContentType+";base64,"+base64.StdEncoding.EncodeToString(part.Data))
				}
			}
			parsed.HTML, err = mailbody.Sanitize(ctx, parsed.HTML, fetcher)
			if err != nil {
				return err
			}
		}
		prepared[i] = *parsed
	}
	subject, sender := "", ""
	var newest time.Time
	for _, m := range t.Messages {
		millis, _ := strconv.ParseInt(m.InternalDate, 10, 64)
		date := time.UnixMilli(millis).UTC()
		if date.After(newest) {
			newest = date
		}
		for _, h := range m.Payload.Headers {
			switch strings.ToLower(h.Name) {
			case "subject":
				subject = h.Value
			case "from":
				sender = h.Value
			}
		}
	}
	if newest.IsZero() {
		newest = time.Now().UTC()
	}
	state := "open"
	if !inbox {
		state = "archived"
	}
	tx, e := s.beginTenant(ctx, a.TenantID)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, `INSERT INTO cards(tenant_id,id,account_id,gmail_thread_id,state,subject,sender,sort_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,account_id,gmail_thread_id) DO UPDATE SET state=CASE WHEN EXCLUDED.state='open' THEN 'open' ELSE cards.state END,subject=EXCLUDED.subject,sender=EXCLUDED.sender,sort_at=EXCLUDED.sort_at,version=cards.version+1`, a.TenantID, uuid.New(), a.ID, threadID, state, subject, sender, newest)
	if e != nil {
		return e
	}
	var cardID uuid.UUID
	e = tx.QueryRow(ctx, "SELECT id FROM cards WHERE tenant_id=$1 AND account_id=$2 AND gmail_thread_id=$3", a.TenantID, a.ID, threadID).Scan(&cardID)
	if e != nil {
		return e
	}
	var body strings.Builder
	for i, m := range t.Messages {
		labels := m.LabelIDs
		hash := sha256.Sum256([]byte(strings.Join(labels, "\x00")))
		millis, _ := strconv.ParseInt(m.InternalDate, 10, 64)
		received := time.UnixMilli(millis).UTC()
		message := prepared[i]
		_, e = tx.Exec(ctx, `INSERT INTO messages(tenant_id,id,card_id,gmail_message_id,labels,html,text,body_hash,received_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(tenant_id,card_id,gmail_message_id) DO UPDATE SET labels=EXCLUDED.labels,html=EXCLUDED.html,text=EXCLUDED.text,body_hash=EXCLUDED.body_hash`, a.TenantID, uuid.New(), cardID, m.ID, labels, message.HTML, message.Text, hash[:], received)
		if e != nil {
			return e
		}
		if message.HTML != "" {
			body.WriteString(message.HTML)
		} else {
			body.WriteString("<pre>")
			body.WriteString(html.EscapeString(message.Text))
			body.WriteString("</pre>")
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) VALUES($1,$2,$3) ON CONFLICT(tenant_id,card_id) DO UPDATE SET html=EXCLUDED.html,updated_at=now()`, a.TenantID, cardID, body.String())
	if e != nil {
		return e
	}
	if e = bundles.AfterIngest(ctx, tx, a.TenantID, nil); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *Syncer) Run(ctx context.Context, accounts func(context.Context) ([]Account, error)) error {
	d := s.PollInterval
	if d <= 0 || d > 5*time.Minute {
		d = 5 * time.Minute
	}
	tick := time.NewTicker(d)
	defer tick.Stop()
	for {
		as, e := accounts(ctx)
		if e != nil {
			return e
		}
		var failed []error
		for _, a := range as {
			if e = s.SyncAccount(ctx, a); e != nil {
				// One account's failure must not starve the others.
				failed = append(failed, fmt.Errorf("sync account %s: %w", a.ID, e))
			}
		}
		if len(failed) > 0 {
			return errors.Join(failed...)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// LoadAccounts returns every active connected account; refresh tokens stay
// tenant-scoped and are read only by the sync worker.
func LoadAccounts(ctx context.Context, db *pgxpool.Pool) ([]Account, error) {
	rows, err := db.Query(ctx, `SELECT tenant_id,id,refresh_token,history_id FROM litterbox_gmail_sync_accounts()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		var a Account
		if err = rows.Scan(&a.TenantID, &a.ID, &a.RefreshToken, &a.HistoryID); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// ClientFactory builds a Gmail API client using the account's own durable
// refresh token. The token exchange is lazy and cached only for that client.
func ClientFactory(clientID, clientSecret, apiBase, tokenURL string, httpClient *http.Client) func(Account) *Client {
	return func(a Account) *Client {
		return &Client{HTTP: httpClient, APIBase: apiBase, TokenURL: tokenURL, ClientID: clientID, ClientSecret: clientSecret, RefreshToken: string(a.RefreshToken)}
	}
}

// ClientForCard binds a write action to the account that owns the card's Gmail
// thread, rather than to whichever account was connected most recently.
func ClientForCard(clientID, clientSecret, apiBase, tokenURL string, httpClient *http.Client) func(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) (*Client, string, error) {
	return func(ctx context.Context, tx pgx.Tx, tenant, cardID uuid.UUID) (*Client, string, error) {
		var a Account
		var thread string
		err := tx.QueryRow(ctx, `SELECT a.id,a.refresh_token,c.gmail_thread_id FROM cards c JOIN accounts a ON a.tenant_id=c.tenant_id AND a.id=c.account_id WHERE c.tenant_id=$1 AND c.id=$2 AND c.source='mail'`, tenant, cardID).Scan(&a.ID, &a.RefreshToken, &thread)
		if err != nil {
			return nil, "", err
		}
		return ClientFactory(clientID, clientSecret, apiBase, tokenURL, httpClient)(a), thread, nil
	}
}

func (s *Syncer) beginTenant(ctx context.Context, tenant uuid.UUID) (pgx.Tx, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT set_config('litterbox.tenant_id',$1,true)", tenant.String()); err != nil {
		tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}
