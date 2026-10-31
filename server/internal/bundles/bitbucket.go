package bundles

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var bitbucketRepoInSubject = regexp.MustCompile(`\(([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)\)\s*$`)
var bitbucketRepoInURL = regexp.MustCompile(`https?://bitbucket\.org/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)/(?:commits|pipelines|addon)`)

type BitbucketRoute struct {
	Key   string
	Title string
}

// RouteBitbucketMail deterministically groups Bitbucket pipeline failures by
// the stable notification address and repository, never by commit or branch.
func RouteBitbucketMail(sender, subject, text string) (BitbucketRoute, bool) {
	address := SenderKey(sender)
	if address != "notifications-noreply@bitbucket.org" {
		return BitbucketRoute{}, false
	}
	if !strings.Contains(strings.ToLower(subject), "pipeline") || !strings.Contains(strings.ToLower(subject), "failed") {
		return BitbucketRoute{}, false
	}
	repo := ""
	if match := bitbucketRepoInSubject.FindStringSubmatch(subject); len(match) == 2 {
		repo = strings.ToLower(match[1])
	} else if match := bitbucketRepoInURL.FindStringSubmatch(text); len(match) == 2 {
		repo = strings.ToLower(match[1])
	}
	if repo == "" {
		return BitbucketRoute{}, false
	}
	return BitbucketRoute{Key: "bitbucket:" + address + ":" + repo, Title: repo + " pipeline failures"}, true
}

func assignBitbucket(ctx context.Context, tx pgx.Tx, tenant uuid.UUID, cards []*mailCard) error {
	groups := map[string][]*mailCard{}
	routes := map[string]BitbucketRoute{}
	var order []string
	for _, c := range cards {
		route, ok := RouteBitbucketMail(c.sender, c.subject, c.text)
		if !ok {
			continue
		}
		if _, exists := groups[route.Key]; !exists {
			order = append(order, route.Key)
		}
		groups[route.Key] = append(groups[route.Key], c)
		routes[route.Key] = route
		c.bundleKey = route.Key
	}
	for _, key := range order {
		route := routes[key]
		var bundleID uuid.UUID
		if err := tx.QueryRow(ctx, upsertTitle, tenant, route.Title, key, "topic").Scan(&bundleID); err != nil {
			return err
		}
		for _, c := range groups[key] {
			var excluded bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bundle_exclusions WHERE tenant_id=$1 AND sender_key=$2 AND bundle_key=$3)`, tenant, BundleSenderKey(c.sender), key).Scan(&excluded); err != nil {
				return err
			}
			if excluded {
				if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=NULL WHERE tenant_id=$1 AND id=$2 AND bundle_id IS NOT NULL`, tenant, c.id); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE cards SET bundle_id=$3 WHERE tenant_id=$1 AND id=$2 AND bundle_id IS DISTINCT FROM $3`, tenant, c.id, bundleID); err != nil {
				return err
			}
		}
	}
	return nil
}
