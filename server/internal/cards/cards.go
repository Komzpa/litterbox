package cards

import (
	"sort"
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

// Section classifies open timed tasks into slots and leaves untimed and agent
// results in now. at values are absolute instants; location controls date/time.
func Section(input []Card, now time.Time) Sections {
	out := Sections{Now: []Card{}, Later: []Card{}, Missed: []Card{}}
	timed := make([]Card, 0, len(input))
	sort.SliceStable(input, func(i, j int) bool {
		if input[i].Source == "manual" && input[j].Source == "manual" {
			return input[i].order < input[j].order
		}
		if input[i].Source == "manual" {
			return true
		}
		if input[j].Source == "manual" {
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
