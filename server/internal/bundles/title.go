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

// HumanTitle preserves an independently named topic, but never presents a
// member's dated subject as the topic of the whole bundle. Sender domains are
// presentation fallbacks only; they do not affect clustering or membership.
func HumanTitle(stored string, members []TitleMember) string {
 seeded := stored == "" || strings.HasPrefix(stored, "sender:")
 for _, m := range members { if stored == m.Subject { seeded = true } }
 if !seeded { return stored }
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
