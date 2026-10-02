// Package mailhtml turns raw RFC 822 messages into renderable bodies and
// their associated parts.
package mailhtml

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// Part is one non-body MIME part: an inline resource or an attachment.
type Part struct {
	ContentType string // media type, lowercased (e.g. "image/png")
	Filename    string // decoded filename, empty when absent
	Data        []byte // body with Content-Transfer-Encoding removed
}

// Message is the parsed content of one RFC 822 message.
type Message struct {
	HTML        string          // text/html body decoded to UTF-8, empty when absent
	Text        string          // text/plain body decoded to UTF-8, empty when absent
	Inline      map[string]Part // inline parts keyed by Content-ID without angle brackets
	Attachments []Part          // parts with Content-Disposition: attachment
}

// Parse reads a raw RFC 822 message and extracts its bodies and parts.
// Multipart alternative/related/mixed containers are resolved recursively;
// the first text/html and text/plain alternatives win. Unknown charsets and
// transfer encodings pass through undecoded rather than failing the message.
func Parse(raw []byte) (*Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("mailhtml: reading message: %w", err)
	}
	m := &Message{Inline: map[string]Part{}}
	if err := m.walk(textproto.MIMEHeader(msg.Header), msg.Body, false); err != nil {
		return nil, err
	}
	return m, nil
}

// walk descends one MIME entity. multipartPart reports whether body comes
// from a multipart.Reader, which already decodes quoted-printable children.
func (m *Message) walk(h textproto.MIMEHeader, body io.Reader, multipartPart bool) error {
	mediaType, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		// Missing or malformed Content-Type defaults to text/plain (RFC 2046 §5.1).
		mediaType, params = "text/plain", nil
	}
	mediaType = strings.ToLower(mediaType)
	if !strings.HasPrefix(mediaType, "multipart/") {
		return m.leaf(mediaType, params, h, body, multipartPart)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return fmt.Errorf("mailhtml: %s part without boundary", mediaType)
	}
	mr := multipart.NewReader(body, boundary)
	var parts []rawPart
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("mailhtml: reading %s: %w", mediaType, err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("mailhtml: reading %s part: %w", mediaType, err)
		}
		parts = append(parts, rawPart{header: part.Header, body: data})
	}
	if mediaType == "multipart/related" {
		parts = startFirst(parts, params["start"])
	}
	for _, part := range parts {
		if err := m.walk(part.header, bytes.NewReader(part.body), true); err != nil {
			return err
		}
	}
	return nil
}

type rawPart struct {
	header textproto.MIMEHeader
	body   []byte
}

// startFirst orders the multipart/related root document (the "start"
// parameter, defaulting to the first part) ahead of the inline resources.
func startFirst(parts []rawPart, start string) []rawPart {
	start = strings.Trim(strings.TrimSpace(start), "<>")
	if start == "" || len(parts) < 2 {
		return parts
	}
	for i, part := range parts {
		if contentID(part.header) == start && i > 0 {
			return append([]rawPart{part}, append(parts[:i], parts[i+1:]...)...)
		}
	}
	return parts
}

// leaf handles one non-multipart entity.
func (m *Message) leaf(mediaType string, params map[string]string, h textproto.MIMEHeader, r io.Reader, multipartPart bool) error {
	disposition, dispParams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	disposition = strings.ToLower(disposition)
	raw, err := io.ReadAll(transferReader(h, r, multipartPart))
	if err != nil {
		return fmt.Errorf("mailhtml: reading %s part: %w", mediaType, err)
	}
	filename := decodeHeader(firstOf(dispParams["filename"], params["name"]))
	if disposition == "attachment" {
		m.Attachments = append(m.Attachments, Part{ContentType: mediaType, Filename: filename, Data: raw})
		return nil
	}
	switch mediaType {
	case "text/html":
		if m.HTML == "" {
			m.HTML = decodeCharset(raw, params["charset"])
			return nil
		}
	case "text/plain":
		if m.Text == "" {
			m.Text = decodeCharset(raw, params["charset"])
			return nil
		}
	}
	cid := contentID(h)
	if cid != "" {
		if _, exists := m.Inline[cid]; !exists {
			m.Inline[cid] = Part{ContentType: mediaType, Filename: filename, Data: raw}
		}
	}
	return nil
}

// transferReader removes Content-Transfer-Encoding. A multipart.Reader
// already decodes quoted-printable transparently, so only base64 is handled
// for multipart children; top-level bodies get both.
func transferReader(h textproto.MIMEHeader, r io.Reader, multipartPart bool) io.Reader {
	switch strings.ToLower(strings.TrimSpace(h.Get("Content-Transfer-Encoding"))) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		if multipartPart {
			return r
		}
		return quotedprintable.NewReader(r)
	default:
		return r
	}
}

// decodeCharset converts a text body to UTF-8. Charsets without a decoder
// keep their bytes but are forced to valid UTF-8, so an exotic label never
// loses the message and never poisons a text column with invalid UTF-8.
func decodeCharset(raw []byte, charset string) string {
	enc := charsetEncoding(charset)
	if enc == nil {
		return strings.ToValidUTF8(string(raw), "�")
	}
	decoded, err := enc.NewDecoder().Bytes(raw)
	if err != nil {
		return strings.ToValidUTF8(string(raw), "�")
	}
	return string(decoded)
}

// charsetEncoding maps the charset labels Litterbox must read (utf-8,
// windows-1251, windows-1252, koi8-r, iso-8859-1, gb2312, big5) onto
// decoders. utf-8 and us-ascii need none; unknown labels return nil for
// pass-through.
func charsetEncoding(charset string) encoding.Encoding {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(charset), `"`)) {
	case "windows-1251":
		return charmap.Windows1251
	case "windows-1252", "cp1252", "x-cp1252":
		return charmap.Windows1252
	case "koi8-r", "koi8r":
		return charmap.KOI8R
	case "iso-8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1
	case "gb2312", "gb-2312", "gbk", "gb18030":
		return simplifiedchinese.GB18030
	case "big5", "big-5", "cp950":
		return traditionalchinese.Big5
	default:
		return nil
	}
}

// contentID normalizes a Content-ID header to the bare addr-spec used by
// cid: URLs in HTML bodies.
func contentID(h textproto.MIMEHeader) string {
	return strings.Trim(strings.TrimSpace(h.Get("Content-Id")), "<>")
}

// decodeHeader decodes RFC 2047 encoded words (e.g. in filenames), keeping
// the original text when decoding fails.
func decodeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	decoder := mime.WordDecoder{CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		if enc := charsetEncoding(charset); enc != nil {
			return enc.NewDecoder().Reader(input), nil
		}
		return input, nil
	}}
	decoded, err := decoder.DecodeHeader(s)
	if err != nil {
		return strings.ToValidUTF8(s, "�")
	}
	return strings.ToValidUTF8(decoded, "�")
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
