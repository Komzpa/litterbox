// Package trackers recognizes known email tracking URLs and tiny or hidden images.
package trackers

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed data/trackers.json
var trackerData []byte

type trackerSnapshot struct {
	Vendors map[string][]string `json:"vendors"`
}

type trackerRule struct {
	name    string
	pattern *regexp.Regexp
}

var rules = loadRules()

func loadRules() []trackerRule {
	var snapshot trackerSnapshot
	if err := json.Unmarshal(trackerData, &snapshot); err != nil {
		panic("trackers: invalid embedded rule data: " + err.Error())
	}
	names := make([]string, 0, len(snapshot.Vendors))
	count := 0
	for name, patterns := range snapshot.Vendors {
		names = append(names, name)
		count += len(patterns)
	}
	sort.Strings(names)
	out := make([]trackerRule, 0, count)
	for _, name := range names {
		patterns := snapshot.Vendors[name]
		for _, pattern := range patterns {
			out = append(out, trackerRule{name: name, pattern: regexp.MustCompile("(?i)" + pattern)})
		}
	}
	return out
}

// IsTracker reports whether url matches a vendored MailTrackerBlocker rule and
// returns the matching vendor name. A non-match returns false and an empty name.
func IsTracker(url string) (bool, string) {
	for _, rule := range rules {
		if rule.pattern.MatchString(url) {
			return true, rule.name
		}
	}
	return false, ""
}

// IsTinyOrHidden recognizes explicit dimensions no larger than 2 CSS pixels
// and inline styles that suppress rendering or make an image transparent.
func IsTinyOrHidden(attrs map[string]string) bool {
	for name, value := range attrs {
		switch strings.ToLower(name) {
		case "width", "height":
			if dimensionAtMostTwo(value) {
				return true
			}
		case "style":
			if hiddenStyle(value) {
				return true
			}
		}
	}
	return false
}

func dimensionAtMostTwo(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "px")
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && n >= 0 && n <= 2
}

func hiddenStyle(style string) bool {
	for _, declaration := range strings.Split(style, ";") {
		property, value, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		property = strings.ToLower(strings.TrimSpace(property))
		value = strings.ToLower(strings.TrimSpace(value))
		if base, priority, ok := strings.Cut(value, "!"); ok && strings.TrimSpace(priority) == "important" {
			value = strings.TrimSpace(base)
		}
		switch property {
		case "width", "height":
			if dimensionAtMostTwo(value) {
				return true
			}
		case "display":
			if value == "none" {
				return true
			}
		case "visibility":
			if value == "hidden" {
				return true
			}
		case "opacity":
			if n, err := strconv.ParseFloat(value, 64); err == nil && n == 0 {
				return true
			}
		}
	}
	return false
}
