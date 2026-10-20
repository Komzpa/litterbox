// Package senderphoto owns tenant/account-scoped Google sender photos.
//
// Source: the documented People API contact surfaces only
// (people.connections.list personFields=emailAddresses,photos plus
// otherContacts.list readMask=emailAddresses,photos with CONTACT and
// PROFILE read sources). Gmail.modify alone carries no sender-photo field,
// and arbitrary senders absent from both datasets stay explicitly
// uncovered. No Gravatar, third-party lookup, or undocumented endpoint.
//
// Transport: photos are fetched and decoded server-side (bounded,
// image.DecodeConfig-verified), stored as bytes in sender_photos, and
// delivered through the authenticated card-scoped route
// GET /v1/cards/{id}/sender-photo. The route never exposes the provider
// URL, OAuth tokens, or raw email addresses in URLs, logs, or errors:
// lookup is by card id, and failures are generic (unavailable vs
// authorization_required).
package senderphoto

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/Komzpa/litterbox/server/internal/people"
)

// MaxPhotoBytes re-exports the provider bound so sync and delivery agree.
const MaxPhotoBytes = people.MaxPhotoBytes

// NormalizeAddress lowercases one addr-spec for the account-scoped join.
// Display names are never matched; dots/plus-tags are never rewritten.
func NormalizeAddress(addr string) string {
	return people.NormalizeEmail(addr)
}

// Upsert stores one decoded provider image for an account-scoped sender.
// Photo bytes MUST already be bounded and DecodeConfig-verified (see
// people.FetchPhoto or DecodeAndBound below); this guard is defense in
// depth so a programming error cannot persist an unbounded blob.
// Execer is the pgx-shaped Exec surface; pgx.Tx and pgxpool both satisfy it,
// so sync can upsert inside its existing transaction without new plumbing.
type Execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// ErrInvalidPhoto reports bytes that must never persist: empty, oversized,
// or not DecodeConfig-verified.
var ErrInvalidPhoto = errors.New("senderphoto: invalid photo")

// Upsert stores one decoded provider image for an account-scoped sender.
// Photo bytes MUST already be bounded and DecodeConfig-verified (see
// people.FetchPhoto or DecodeAndBound below); this guard is defense in
// depth so a programming error cannot persist an unbounded blob.
func Upsert(ctx context.Context, db Execer, tenant, account, address string, photo []byte, contentType string) error {
	key := NormalizeAddress(address)
	if key == "" || len(photo) == 0 || len(photo) > MaxPhotoBytes {
		return ErrInvalidPhoto
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(photo)); err != nil {
		return ErrInvalidPhoto
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		ct = "image/jpeg"
	}
	_, err := db.Exec(ctx, `INSERT INTO sender_photos(tenant_id,account_id,sender_address,photo,content_type,updated_at)
VALUES($1,$2,$3,$4,$5,now())
ON CONFLICT(tenant_id,account_id,sender_address)
DO UPDATE SET photo=EXCLUDED.photo,content_type=EXCLUDED.content_type,updated_at=now()`,
		tenant, account, key, photo, ct)
	return err
}

// DecodeAndBound validates candidate bytes before persistence: non-empty,
// within MaxPhotoBytes, and image.DecodeConfig-verified.
func DecodeAndBound(photo []byte) bool {
	if len(photo) == 0 || len(photo) > MaxPhotoBytes {
		return false
	}
	_, _, err := image.DecodeConfig(bytes.NewReader(photo))
	return err == nil
}
