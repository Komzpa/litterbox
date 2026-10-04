package cards

import (
	"net/mail"
	"sort"
	"strings"
	"time"
)

type Card struct {
	ID         string     `json:"id"`
	BundleID   *string    `json:"bundle_id"`
	PinnedRank *int64     `json:"pinned_rank"`
	Source     string     `json:"source"`
	SourceKind string     `json:"source_kind,omitempty"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary"`
	At         *time.Time `json:"at"`
	Timed      bool       `json:"timed"`
	State      string     `json:"state"`
	Note       string     `json:"note"`
	// Display-origin metadata. AccountName is the tenant-scoped account
	// address; BundleTitle is the tenant-scoped bundle title. SourceURL is
	// only populated for an established source destination (none yet), so it
	// stays empty rather than fabricating a Gmail browser index.
	AccountName string `json:"account_name,omitempty"`
	BundleTitle string `json:"bundle_title,omitempty"`
 SourceURL   string `json:"source_url,omitempty"`
 SenderName string `json:"sender_name"`
 Snippet string `json:"snippet,omitempty"`
	// ReceivedAt is the card's latest message arrival and SenderAddress the
	// addr-spec of its stored From header; both are omitted for non-mail cards.
	ReceivedAt    *time.Time `json:"received_at,omitempty"`
	SenderAddress string     `json:"sender_address,omitempty"`
	// Important marks a thread that carries Gmail's IMPORTANT label; the
	// client renders it standalone, never concealed inside a bundle.
	Important bool `json:"important,omitempty"`
	// HasBody reports that card_bodies holds a rendered document for this
	// card (a Gmail mail body or a server-built file document); the client
	// prefetches it through GET /v1/cards/{id}/body for offline reads.
	HasBody bool `json:"has_body"`
	// internal ordering metadata, excluded from the API
	createdAt time.Time
	order     int
}

type Sections struct {
	Now    []Card `json:"now"`
	Later  []Card `json:"later"`
	Missed []Card `json:"missed"`
}

// senderAddress returns the addr-spec of a raw From header, falling back to
// the trimmed header when it does not parse.
func senderAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed, err := mail.ParseAddress(raw); err == nil {
		return parsed.Address
	}
	return raw
}

// Section classifies open timed tasks into slots and leaves untimed and agent
// results in now. at values are absolute instants; location controls date/time.
func Section(input []Card, now time.Time) Sections {
	out := Sections{Now: []Card{}, Later: []Card{}, Missed: []Card{}}
	timed := make([]Card, 0, len(input))
	sort.SliceStable(input, func(i, j int) bool {
		ownerI := input[i].Source == "manual" || input[i].Source == "journal"
		ownerJ := input[j].Source == "manual" || input[j].Source == "journal"
		if ownerI && ownerJ {
			if input[i].order != input[j].order {
				return input[i].order < input[j].order
			}
			return input[i].createdAt.After(input[j].createdAt)
		}
		if ownerI {
			return true
		}
		if ownerJ {
			return false
		}
		if input[i].Source == "todo" && input[j].Source == "todo" {
			return input[i].order < input[j].order
		}
		if input[i].Source == "todo" {
			return true
		}
		if input[j].Source == "todo" {
			return false
		}
		return input[i].createdAt.After(input[j].createdAt)
	})
	for _, c := range input {
		if c.State != "open" {
			continue
		}
		if c.Timed && c.At != nil {
			timed = append(timed, c)
		} else if c.Source == "todo" {
			out.Now = append(out.Now, c)
		} else if c.Source == "agent" {
			out.Now = append(out.Now, c)
		} else {
			out.Now = append(out.Now, c)
		}
	}
	sort.SliceStable(timed, func(i, j int) bool { return timed[i].At.Before(*timed[j].At) })
	current := -1
	for i := range timed {
		if !timed[i].At.After(now) {
			current = i
		} else {
			break
		}
	}
	currentTimed := make([]Card, 0)
	for _, c := range timed {
		switch {
		case c.At.After(now):
			out.Later = append(out.Later, c)
		case current >= 0 && c.At.Equal(*timed[current].At):
			currentTimed = append(currentTimed, c)
		default:
			out.Missed = append(out.Missed, c)
		}
	}
	out.Now = append(currentTimed, out.Now...)
	// Pins lead each R30 section by rank; stable sorting preserves its existing
	// ordering for unpinned cards and for tied ranks.
	sort.SliceStable(out.Now, func(i, j int) bool {
		a, b := out.Now[i].PinnedRank, out.Now[j].PinnedRank
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a < *b
	})
	// Pinned future timeline ascending; past missed descending, with pins
	// promoted ahead of unpinned cards in either timeline.
	sort.SliceStable(out.Later, func(i, j int) bool {
		a, b := out.Later[i], out.Later[j]
		if a.PinnedRank != nil {
			if b.PinnedRank == nil {
				return true
			}
			return *a.PinnedRank < *b.PinnedRank
		}
		if b.PinnedRank != nil {
			return false
		}
		return a.At.Before(*b.At)
	})
	sort.SliceStable(out.Missed, func(i, j int) bool {
		a, b := out.Missed[i], out.Missed[j]
		if a.PinnedRank != nil {
			if b.PinnedRank == nil {
				return true
			}
			return *a.PinnedRank < *b.PinnedRank
		}
		if b.PinnedRank != nil {
			return false
		}
		return a.At.After(*b.At)
	})
	return out
}
