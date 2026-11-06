package mailhtml

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Uses only the pre-existing Parse API, so this regression also runs on v46.
func TestCalendarMIMEFixtures(t *testing.T) {
	for _, name := range []string{"google", "outlook", "all-day", "cancel", "attachment", "alternative"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/calendar/" + name + ".eml")
			if err != nil {
				t.Fatal(err)
			}
			m, err := Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(m)
			var projection struct {
				Invitations []struct {
					Summary, Start, End, Method, Location, Description string
					AllDay                                             bool `json:"all_day"`
					Cancelled, Recurring                               bool
					TimeError                                          string `json:"time_error"`
					Organizer                                          struct{ Name, Address string }
					Attendees                                          []struct{ Name, Address, Status string }
					Links                                              []string
				}
			}
			if err := json.Unmarshal(encoded, &projection); err != nil {
				t.Fatal(err)
			}
			if len(projection.Invitations) != 1 {
				t.Fatalf("want invitation, got %s; status=%s", encoded, m.DecodeStatus)
			}
			e := projection.Invitations[0]
			if e.Summary != "Meet with Example" || e.Location != "Room 2, upstairs" || !strings.Contains(e.Description, "long description") || !strings.Contains(e.Description, "\nJoin https://") || e.TimeError != "" || !e.Recurring || e.Organizer.Address != "host@example.test" || len(e.Attendees) != 2 || e.Attendees[0].Status != "ACCEPTED" || len(e.Links) != 2 {
				t.Fatalf("incomplete event: %+v", e)
			}
			if m.DecodeStatus != "" || !bytes.Equal(m.Raw, raw) || len(m.Attachments) != 1 || !strings.HasSuffix(m.Attachments[0].Filename, ".ics") {
				t.Fatalf("source lost or unsupported: %+v", m)
			}
			switch name {
			case "outlook":
				if e.Start != "2026-07-06T10:00:00+02:00" {
					t.Fatal(e.Start)
				}
			case "all-day":
				if !e.AllDay || e.Start != "2026-11-06" || e.End != "2026-11-08" {
					t.Fatal(e)
				}
			case "cancel":
				if !e.Cancelled || e.Method != "CANCEL" {
					t.Fatal(e)
				}
			default:
				if e.Start != "2026-11-06T10:00:00Z" || e.End != "2026-11-06T11:00:00Z" {
					t.Fatal(e)
				}
			}
			if name == "alternative" && !strings.Contains(m.Text, "Original text alternative") {
				t.Fatal("text lost")
			}
			if (name == "alternative" || name == "attachment") && !strings.Contains(m.HTML, "Original HTML alternative") {
				t.Fatal("HTML lost")
			}
		})
	}
}

func TestCalendarReplyFloatingAndMalformed(t *testing.T) {
	raw, _ := os.ReadFile("testdata/calendar/cancel.eml")
	raw = bytes.ReplaceAll(raw, []byte("METHOD:CANCEL"), []byte("METHOD:REPLY"))
	raw = bytes.ReplaceAll(raw, []byte("20261106T100000Z"), []byte("20261106T100000"))
	m, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Invitations) != 1 || m.Invitations[0].Method != "REPLY" || m.Invitations[0].Cancelled || m.Invitations[0].Start != "2026-11-06T10:00:00" {
		t.Fatal(m)
	}
	bad := []byte("Content-Type: text/calendar\r\n\r\nbroken")
	m, err = Parse(bad)
	if err != nil || m.DecodeStatus == "" || len(m.Attachments) != 1 || !bytes.Equal(m.Attachments[0].Data, []byte("broken")) {
		t.Fatal(m, err)
	}
}

func TestCalendarTimeZonesAndDefaults(t *testing.T) {
	raw, _ := os.ReadFile("testdata/calendar/outlook.eml")
	for _, tc := range []struct{ name, old, new, want string }{
		{"winter", "20260706", "20261106", "2026-11-06T10:00:00+01:00"},
		{"iana", "TZID=\"Customized Time Zone\"", "TZID=Europe/Berlin", "2026-07-06T10:00:00+02:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse(bytes.ReplaceAll(raw, []byte(tc.old), []byte(tc.new)))
			if err != nil {
				t.Fatal(err)
			}
			if m.Invitations[0].Start != tc.want || m.Invitations[0].TimeError != "" {
				t.Fatal(m.Invitations)
			}
		})
	}
	raw, _ = os.ReadFile("testdata/calendar/all-day.eml")
	raw = bytes.ReplaceAll(raw, []byte("DTEND;VALUE=DATE:20261108\r\n"), nil)
	m, err := Parse(raw)
	if err != nil || m.Invitations[0].End != "2026-11-07" {
		t.Fatal(m, err)
	}
}
