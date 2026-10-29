package cards

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// acceptsGzip respects an explicit gzip exclusion even when a wildcard is present.
func acceptsGzip(header string) bool {
	wildcard := false
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		name := strings.TrimSpace(parts[0])
		if !strings.EqualFold(name, "gzip") && name != "*" { continue }
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(strings.TrimSpace(key), "q") {
				q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || q < 0 || q > 1 { quality = 0 } else { quality = q }
			}
		}
		if strings.EqualFold(name, "gzip") { return quality > 0 }
		wildcard = quality > 0
	}
	return wildcard
}

func jsonResponseWriter(w http.ResponseWriter, r *http.Request) (io.Writer, func()) {
	w.Header().Add("Vary", "Accept-Encoding")
	if !acceptsGzip(strings.Join(r.Header.Values("Accept-Encoding"), ",")) { return w, func() {} }
	w.Header().Set("Content-Encoding", "gzip")
	writer := gzip.NewWriter(w)
	return writer, func() { _ = writer.Close() }
}
