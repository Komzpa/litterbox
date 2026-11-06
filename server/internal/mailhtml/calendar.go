package mailhtml

import (
	"fmt"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
)

// Invitation is a read projection of retained MIME, never a calendar mutation.
// Dates remain dates; floating times remain local wall times for the client.
type Invitation struct {
	Summary     string           `json:"summary"`
	Start       string           `json:"start"`
	End         string           `json:"end"`
	AllDay      bool             `json:"all_day"`
	Method      string           `json:"method"`
	Cancelled   bool             `json:"cancelled"`
	Recurring   bool             `json:"recurring"`
	Location    string           `json:"location"`
	Organizer   CalendarPerson   `json:"organizer"`
	Attendees   []CalendarPerson `json:"attendees"`
	Description string           `json:"description"`
	Links       []string         `json:"links"`
	TimeError   string           `json:"time_error,omitempty"`
}
type CalendarPerson struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Status  string `json:"status,omitempty"`
}

func calendarText(props ical.Props, name string) string {
	s, _ := props.Text(name)
	return s
}
func calendarPerson(p *ical.Prop) CalendarPerson {
	if p == nil {
		return CalendarPerson{}
	}
	return CalendarPerson{Name: p.Params.Get("CN"), Address: strings.TrimPrefix(strings.TrimPrefix(p.Value, "mailto:"), "MAILTO:"), Status: p.Params.Get("PARTSTAT")}
}
func parseCalendar(source, method string) ([]Invitation, error) {
	cal, err := ical.NewDecoder(strings.NewReader(source)).Decode()
	if err != nil {
		return nil, err
	}
	if value := cal.Props.Get("METHOD"); value != nil {
		method = value.Value
	}
	result := []Invitation{}
	for _, event := range cal.Events() {
		e := Invitation{Summary: calendarText(event.Props, "SUMMARY"), Method: strings.ToUpper(method), Location: calendarText(event.Props, "LOCATION"), Description: calendarText(event.Props, "DESCRIPTION"), Organizer: calendarPerson(event.Props.Get("ORGANIZER")), Attendees: []CalendarPerson{}, Links: []string{}, Recurring: event.Props.Get("RRULE") != nil}
		e.Cancelled = e.Method == "CANCEL" || strings.EqualFold(calendarText(event.Props, "STATUS"), "CANCELLED")
		start := event.Props.Get("DTSTART")
		e.AllDay = start != nil && start.ValueType() == ical.ValueDate
		e.Start, err = calendarTime(cal, start)
		if err != nil {
			e.TimeError = err.Error()
		}
		e.End, err = calendarTime(cal, event.Props.Get("DTEND"))
		if err != nil {
			e.TimeError = err.Error()
		}
		if e.End == "" && e.Start != "" {
			// RFC 5545: absent end means one day for DATE, zero duration otherwise.
			if e.AllDay {
				if d, err := time.Parse("2006-01-02", e.Start); err == nil {
					e.End = d.AddDate(0, 0, 1).Format("2006-01-02")
				}
			} else if duration := event.Props.Get("DURATION"); duration != nil {
				if d, err := duration.Duration(); err == nil {
					if t, err := time.Parse(time.RFC3339, e.Start); err == nil {
						e.End = t.Add(d).Format(time.RFC3339)
					}
				}
			}
		}
		for _, p := range event.Props["ATTENDEE"] {
			e.Attendees = append(e.Attendees, calendarPerson(&p))
		}
		for _, name := range []string{"URL", "CONFERENCE", "X-GOOGLE-CONFERENCE"} {
			for _, p := range event.Props[name] {
				e.Links = append(e.Links, p.Value)
			}
		}
		result = append(result, e)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("calendar contains no events")
	}
	return result, nil
}
func calendarTime(cal *ical.Calendar, p *ical.Prop) (string, error) {
	if p == nil {
		return "", nil
	}
	if p.ValueType() == ical.ValueDate {
		t, err := p.DateTime(nil)
		if err != nil {
			return p.Value, err
		}
		return t.Format("2006-01-02"), nil
	}
	t, err := p.DateTime(nil)
	if err == nil {
		if p.Params.Get("TZID") == "" && !strings.HasSuffix(p.Value, "Z") {
			return t.Format("2006-01-02T15:04:05"), nil
		}
		return t.Format(time.RFC3339), nil
	}
	// Outlook can use arbitrary TZIDs defined only by its embedded VTIMEZONE.
	wall, parseErr := time.Parse("20060102T150405", p.Value)
	if parseErr != nil {
		return p.Value, err
	}
	for _, zone := range cal.Children {
		if zone.Name != "VTIMEZONE" || calendarText(zone.Props, "TZID") != p.Params.Get("TZID") {
			continue
		}
		var latest time.Time
		offset := ""
		for _, rule := range zone.Children {
			if rule.Name != "STANDARD" && rule.Name != "DAYLIGHT" {
				continue
			}
			onset, e := rule.Props.DateTime("DTSTART", time.UTC)
			if e != nil {
				continue
			}
			if rp := rule.Props.Get("RRULE"); rp != nil {
				opt, e := rrule.StrToROption(rp.Value)
				if e != nil {
					continue
				}
				opt.Dtstart = onset
				r, e := rrule.NewRRule(*opt)
				if e != nil {
					continue
				}
				onset = r.Before(wall, true)
			}
			candidates := []time.Time{onset}
			for _, rp := range rule.Props["RDATE"] {
				for _, value := range strings.Split(rp.Value, ",") {
					if d, e := time.Parse("20060102T150405", value); e == nil {
						candidates = append(candidates, d)
					}
				}
			}
			for _, d := range candidates {
				if !d.IsZero() && !d.After(wall) && (latest.IsZero() || d.After(latest)) {
					latest = d
					if p := rule.Props.Get("TZOFFSETTO"); p != nil {
						offset = p.Value
					}
				}
			}
		}
		if offset != "" {
			resolved, e := time.Parse("20060102T150405-0700", p.Value+offset)
			if e == nil {
				return resolved.Format(time.RFC3339), nil
			}
		}
	}
	return p.Value, fmt.Errorf("unresolved calendar time %s (%s)", p.Value, p.Params.Get("TZID"))
}
