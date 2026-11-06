package mailbody

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"

	"github.com/Komzpa/litterbox/server/internal/sources/agents"
)

type Recipient struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}
type Attachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}
type ContentMetadata struct {
	To           []Recipient  `json:"to"`
	Cc           []Recipient  `json:"cc"`
	Attachments  []Attachment `json:"attachments"`
	DecodeStatus string       `json:"decode_status"`
}

// MessageContent reads within the body's repeatable-read tenant transaction.
func MessageContent(ctx context.Context, tx *sql.Tx, tenant, card, message string) (ContentMetadata, error) {
	result := ContentMetadata{To: []Recipient{}, Cc: []Recipient{}, Attachments: []Attachment{}}
	var to, cc []byte
	var rawSize int64
	err := tx.QueryRowContext(ctx, `SELECT to_recipients,cc_recipients,decode_status,octet_length(raw_mime) FROM messages WHERE tenant_id=$1 AND card_id=$2 AND id=$3`, tenant, card, message).Scan(&to, &cc, &result.DecodeStatus, &rawSize)
	if err != nil {
		return result, err
	}
	if err = decodeRecipients(to, &result.To); err != nil {
		return result, err
	}
	if err = decodeRecipients(cc, &result.Cc); err != nil {
		return result, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id::text,name,media_type,octet_length(data) FROM message_parts WHERE tenant_id=$1 AND message_id=$2 ORDER BY part_index`, tenant, message)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Attachment
		if err = rows.Scan(&a.ID, &a.Name, &a.Type, &a.Size); err != nil {
			return result, err
		}
		a.DownloadURL = "/v1/cards/" + card + "/mail-files/" + a.ID
		result.Attachments = append(result.Attachments, a)
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	if rawSize > 0 {
		result.Attachments = append(result.Attachments, Attachment{ID: message, Name: "original.eml", Type: "message/rfc822", Size: rawSize, DownloadURL: "/v1/cards/" + card + "/mail-files/" + message})
	}
	return result, nil
}

// DownloadFile never accepts a filesystem path or a provider URL.
func (h Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	tenant, ok := agents.TenantFrom(r.Context())
	if !ok || tenant == "" {
		http.Error(w, "unauthorized", 401)
		return
	}
	tx, err := h.DB.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `SELECT set_config('litterbox.tenant_id',$1,true)`, tenant); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	var name, kind string
	var data []byte
	err = tx.QueryRowContext(r.Context(), `SELECT p.name,p.media_type,p.data FROM message_parts p JOIN messages m ON m.tenant_id=p.tenant_id AND m.id=p.message_id WHERE p.tenant_id=$1 AND m.card_id=$2 AND p.id::text=$3 UNION ALL SELECT 'original.eml','message/rfc822',m.raw_mime FROM messages m WHERE m.tenant_id=$1 AND m.card_id=$2 AND m.id::text=$3 AND octet_length(m.raw_mime)>0`, tenant, r.PathValue("id"), r.PathValue("fileID")).Scan(&name, &kind, &data)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "internal server error", 500)
		return
	}
	// Always download bytes: an HTML attachment must not execute in the API origin.
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func decodeRecipients(data []byte, target *[]Recipient) error { return json.Unmarshal(data, target) }
