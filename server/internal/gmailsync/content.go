package gmailsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"golang.org/x/net/html"
	"log"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Komzpa/litterbox/server/internal/mailbody"
	"github.com/Komzpa/litterbox/server/internal/mailhtml"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const mailContentVersion = 2
const providerDeletedContent = "Original unavailable: provider deleted message (Gmail 404)"
const repairMessageTimeout = 3 * time.Minute
const repairPendingSQL = `(content_version<$3 OR html LIKE '%data-fetch-state="unavailable"%') AND NOT content_provider_deleted AND (content_repair_attempted_at IS NULL OR content_repair_attempted_at < now()-interval '1 hour')`

func (s *Syncer) prepareContent(ctx context.Context, raw []byte, cached ...string) (*mailhtml.Message, error) {
	parsed, err := mailhtml.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.HTML != "" {
		parsed.HTML = resolveCIDImages(parsed.HTML, parsed.Inline)
		if len(cached) > 0 {
			parsed.HTML = retainFetchedImages(parsed.HTML, cached[0])
		}
		fetcher := s.Images
		if fetcher == nil {
			fetcher = mailhtml.NewImageFetcher(nil, nil)
		}
		parsed.HTML, err = mailbody.Sanitize(ctx, parsed.HTML, fetcher)
	}
	return parsed, err
}

func recipients(raw []byte, field string) []mailbody.Recipient {
	result := []mailbody.Recipient{}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return result
	}
	addresses, err := msg.Header.AddressList(field)
	if err != nil {
		return result
	}
	for _, a := range addresses {
		result = append(result, mailbody.Recipient{Name: a.Name, Address: a.Address})
	}
	return result
}

// persistMessageContent uses the stable existing message row identity. The raw
// original remains available even for unsupported bodies or undecodable leaves.
func persistMessageContent(ctx context.Context, tx pgx.Tx, tenant, card uuid.UUID, gmailID string, message mailhtml.Message) error {
	to, _ := json.Marshal(recipients(message.Raw, "To"))
	cc, _ := json.Marshal(recipients(message.Raw, "Cc"))
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM messages WHERE tenant_id=$1 AND card_id=$2 AND gmail_message_id=$3`, tenant, card, gmailID).Scan(&id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE messages SET content_provider_deleted=false,raw_mime=$1,decode_status=$2,to_recipients=$3,cc_recipients=$4,content_version=$5 WHERE tenant_id=$6 AND id=$7 AND (content_provider_deleted OR (raw_mime,decode_status,to_recipients,cc_recipients,content_version) IS DISTINCT FROM ($1::bytea,$2::text,$3::jsonb,$4::jsonb,$5::integer))`, message.Raw, message.DecodeStatus, to, cc, mailContentVersion, tenant, id)
	if err != nil {
		return err
	}
	parts := append([]mailhtml.Part{}, message.Attachments...)
	cids := make([]string, len(parts))
	keys := make([]string, 0, len(message.Inline))
	for key := range message.Inline {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, cid := range keys {
		part := message.Inline[cid]
		found := -1
		for i, p := range parts {
			if p.Filename == part.Filename && p.ContentType == part.ContentType && bytes.Equal(p.Data, part.Data) {
				found = i
				break
			}
		}
		if found >= 0 {
			cids[found] = cid
			continue
		}
		parts = append(parts, part)
		cids = append(cids, cid)
	}
	for i, p := range parts {
		name := p.Filename
		if name == "" {
			name = fmt.Sprintf("part-%d", i+1)
		}
		data := p.Data
		if data == nil {
			data = []byte{}
		}
		partID := uuid.NewSHA1(id, []byte(fmt.Sprint(i)))
		_, err = tx.Exec(ctx, `INSERT INTO message_parts(tenant_id,message_id,id,part_index,name,media_type,content_id,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(tenant_id,message_id,part_index) DO UPDATE SET name=EXCLUDED.name,media_type=EXCLUDED.media_type,content_id=EXCLUDED.content_id,data=EXCLUDED.data WHERE (message_parts.name,message_parts.media_type,message_parts.content_id,message_parts.data) IS DISTINCT FROM (EXCLUDED.name,EXCLUDED.media_type,EXCLUDED.content_id,EXCLUDED.data)`, tenant, id, partID, i, name, p.ContentType, cids[i], data)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM message_parts WHERE tenant_id=$1 AND message_id=$2 AND part_index >= $3`, tenant, id, len(parts))
	return err
}

// backfillContent repairs bounded message batches through ordinary account polls.
// Each card gets one message with its own deadline. Claims are durable before
// WAN access; a failed message waits an hour without blocking its siblings.
// Provider labels and card lifecycle stay with ordinary thread synchronization.
func (s *Syncer) backfillContent(ctx context.Context, a Account, c *Client) error {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT c.id FROM cards c WHERE c.tenant_id=$1 AND c.account_id=$2 AND c.source='mail' AND EXISTS(SELECT 1 FROM messages m WHERE m.tenant_id=c.tenant_id AND m.card_id=c.id AND `+repairPendingSQL+`) ORDER BY c.mail_content_checked_at NULLS FIRST,c.sort_at DESC,c.id LIMIT 10`, a.TenantID, a.ID, mailContentVersion)
	if err != nil {
		return err
	}
	var cards []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		cards = append(cards, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range cards {
		if _, err = tx.Exec(ctx, `UPDATE cards SET mail_content_checked_at=now() WHERE tenant_id=$1 AND id=$2`, a.TenantID, id); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	for _, card := range cards {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = s.repairCardContent(ctx, a, c, card); err != nil {
			log.Printf("mail content repair: build=%s card=%s: %v", s.Build, card, err)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}

func (s *Syncer) repairCardContent(ctx context.Context, a Account, c *Client, card uuid.UUID) error {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT gmail_message_id,raw_mime,html FROM messages WHERE tenant_id=$1 AND card_id=$2 AND `+repairPendingSQL+` ORDER BY content_repair_attempted_at NULLS FIRST,content_version,received_at,gmail_message_id LIMIT 1`, a.TenantID, card, mailContentVersion)
	if err != nil {
		return err
	}
	type cachedMessage struct {
		id   string
		raw  []byte
		html string
	}
	var ids []cachedMessage
	for rows.Next() {
		var id cachedMessage
		if err = rows.Scan(&id.id, &id.raw, &id.html); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.Exec(ctx, `UPDATE messages SET content_repair_attempted_at=now() WHERE tenant_id=$1 AND card_id=$2 AND gmail_message_id=$3`, a.TenantID, card, id.id); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	for _, id := range ids {
		ctx, cancel := context.WithTimeout(ctx, repairMessageTimeout)
		defer cancel()
		raw := id.raw
		if len(raw) == 0 {
			raw, err = c.GetMessageRaw(ctx, id.id)
			if err != nil {
				if isNotFound(err) {
					return s.markProviderDeleted(ctx, a, card, id.id)
				}
				return err
			}
		}
		message, err := s.prepareContent(ctx, raw, id.html)
		if err != nil {
			return err
		}
		tx, err := s.beginTenant(ctx, a.TenantID)
		if err != nil {
			return err
		}
		// If normal sync deleted the row meanwhile, never recreate a removed message.
		_, err = tx.Exec(ctx, `UPDATE messages SET html=$1,text=$2 WHERE tenant_id=$3 AND card_id=$4 AND gmail_message_id=$5 AND (html,text) IS DISTINCT FROM ($1::text,$2::text)`, message.HTML, message.Text, a.TenantID, card, id.id)
		if err == nil {
			err = persistMessageContent(ctx, tx, a.TenantID, card, id.id, *message)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO card_bodies(tenant_id,card_id,html) SELECT $1,$2,coalesce(string_agg(CASE WHEN html<>'' THEN html ELSE '<pre>'||replace(replace(replace(text,'&','&amp;'),'<','&lt;'),'>','&gt;')||'</pre>' END,'' ORDER BY received_at,gmail_message_id),'') FROM messages WHERE tenant_id=$1 AND card_id=$2 AND NOT (labels && ARRAY['DRAFT','TRASH','SPAM']) ON CONFLICT(tenant_id,card_id) DO UPDATE SET html=EXCLUDED.html,updated_at=now() WHERE card_bodies.html IS DISTINCT FROM EXCLUDED.html`, a.TenantID, card)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE cards SET version=version+1 WHERE tenant_id=$1 AND id=$2`, a.TenantID, card)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Preserve readable bodies and attachments. Only normal thread sync owns
// deletion after comparing Gmail's complete message list.
func (s *Syncer) markProviderDeleted(ctx context.Context, a Account, card uuid.UUID, id string) error {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE messages SET content_provider_deleted=true,decode_status=$4 WHERE tenant_id=$1 AND card_id=$2 AND gmail_message_id=$3`, a.TenantID, card, id, providerDeletedContent)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE cards SET version=version+1 WHERE tenant_id=$1 AND id=$2`, a.TenantID, card)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// retainFetchedImages reuses already downloaded bytes when the same MIME source
// is processed again. A temporary remote failure must not erase an offline image.
func retainFetchedImages(source, cached string) string {
	successful := map[string]string{}
	walk := func(root *html.Node, visit func(*html.Node)) {
		var visitTree func(*html.Node)
		visitTree = func(n *html.Node) {
			visit(n)
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				visitTree(c)
			}
		}
		visitTree(root)
	}
	old, err := html.Parse(strings.NewReader(cached))
	if err != nil {
		return source
	}
	walk(old, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "img" {
			return
		}
		original, data := "", ""
		for _, a := range n.Attr {
			if a.Key == "data-original-src" {
				original = a.Val
			}
			if a.Key == "src" {
				data = a.Val
			}
		}
		if original != "" && strings.HasPrefix(data, "data:image/") {
			successful[original] = data
		}
	})
	if len(successful) == 0 {
		return source
	}
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "img" {
			return
		}
		for i, a := range n.Attr {
			if a.Key == "src" {
				if data, ok := successful[a.Val]; ok {
					n.Attr[i].Val = data
					n.Attr = append(n.Attr, html.Attribute{Key: "data-original-src", Val: a.Val}, html.Attribute{Key: "data-fetch-state", Val: "available"})
				}
				break
			}
		}
	})
	var out strings.Builder
	if html.Render(&out, root) != nil {
		return source
	}
	return out.String()
}

func (s *Syncer) cachedThreadContent(ctx context.Context, a Account, thread string) (map[string]string, error) {
	tx, err := s.beginTenant(ctx, a.TenantID)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT m.gmail_message_id,m.html FROM messages m JOIN cards c ON c.tenant_id=m.tenant_id AND c.id=m.card_id WHERE c.tenant_id=$1 AND c.account_id=$2 AND c.gmail_thread_id=$3`, a.TenantID, a.ID, thread)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for rows.Next() {
		var id, body string
		if err = rows.Scan(&id, &body); err != nil {
			rows.Close()
			return nil, err
		}
		result[id] = body
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	err = tx.Commit(ctx)
	return result, err
}

// Resolve complete CID attributes rather than replacing identifier prefixes in
// arbitrary HTML text (photo and photo-large may both be real Content-IDs).
func resolveCIDImages(source string, resources map[string]mailhtml.Part) string {
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return source
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			for i, a := range n.Attr {
				if a.Key != "src" || !strings.HasPrefix(strings.ToLower(a.Val), "cid:") {
					continue
				}
				cid, err := url.PathUnescape(a.Val[4:])
				if err != nil {
					continue
				}
				if part, ok := resources[cid]; ok && strings.HasPrefix(part.ContentType, "image/") {
					n.Attr[i].Val = "data:" + part.ContentType + ";base64," + base64.StdEncoding.EncodeToString(part.Data)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	var out strings.Builder
	if html.Render(&out, root) != nil {
		return source
	}
	return out.String()
}
