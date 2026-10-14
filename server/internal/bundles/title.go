package bundles

import (
 "net/mail"
 "sort"
 "strings"
)

type TitleMember struct {
 Subject string `json:"subject"`
 Sender string `json:"sender"`
}

// HumanTitle preserves independent and unknown-origin topics. Only a recorded
// generated title (or an absent title) may use sender-domain presentation.
// Domains never change clustering or membership.
func HumanTitle(stored string, origin string, members []TitleMember) string {
 if stored != "" && origin != "seed" { return stored }
 domains := map[string]bool{}
 for _, m := range members {
  address := strings.TrimSpace(m.Sender)
  if a, err := mail.ParseAddress(address); err == nil { address = a.Address }
  if i := strings.LastIndexByte(address, '@'); i >= 0 && i+1 < len(address) { domains[strings.ToLower(address[i+1:])] = true }
 }
 names := make([]string, 0, len(domains))
 for domain := range domains { names = append(names, domain) }
 sort.Strings(names)
 if len(names) == 0 { return "Related mail" }
 return strings.Join(names, ", ")
}

// Producers may refresh their own generated titles, never an independent or
// unknown-origin topic. All title writers use this same preservation rule.
const upsertTitle = `INSERT INTO bundles (tenant_id,id,title,centroid,bundle_key,title_origin) VALUES ($1,gen_random_uuid(),$2,'{}',$3,$4) ON CONFLICT (tenant_id,bundle_key) WHERE bundle_key IS NOT NULL DO UPDATE SET title=CASE WHEN bundles.title_origin IN ('seed','topic') THEN EXCLUDED.title ELSE bundles.title END, title_origin=CASE WHEN bundles.title_origin IN ('seed','topic') THEN EXCLUDED.title_origin ELSE bundles.title_origin END RETURNING id`
