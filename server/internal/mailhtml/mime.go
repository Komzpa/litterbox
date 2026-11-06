// Package mailhtml turns raw RFC 822 messages into renderable bodies and
// their associated parts.
package mailhtml

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/ianaindex"
)

// Part is a transfer-decoded MIME resource or attachment.
type Part struct {
	ContentType string // media type, lowercased (e.g. "image/png")
	Filename    string // decoded filename, empty when absent
	Data        []byte // body with Content-Transfer-Encoding removed
}

// Message is the parsed content of one RFC 822 message.
type Message struct {
	Raw          []byte          // exact RFC 822 source, including every MIME leaf
	DecodeStatus string          // visible explanation when decoding or rendering is incomplete
	HTML         string          // text/html body decoded to UTF-8, empty when absent
	Text         string          // text/plain body decoded to UTF-8, empty when absent
	Inline       map[string]Part // inline parts keyed by Content-ID without angle brackets
	Attachments  []Part          // explicit files and unsupported non-CID leaves
}

// Parse reads a raw RFC 822 message and extracts its bodies and parts.
// Multipart alternative/related/mixed containers are resolved recursively;
// alternatives select representations while mixed parts remain in source order.
// Raw source is retained even when a charset cannot be decoded.
func Parse(raw []byte) (*Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("mailhtml: reading message: %w", err)
	}
	m := &Message{Raw: bytes.Clone(raw), Inline: map[string]Part{}}
	if err := m.walk(textproto.MIMEHeader(msg.Header), msg.Body, false); err != nil {
		return nil, err
	}
	if strings.TrimSpace(m.HTML) == "" && strings.TrimSpace(m.Text) == "" {
		m.note("No readable message body. Open in Gmail to view the original.")
	}
	if m.DecodeStatus != "" {
		if m.HTML == "" && m.Text != "" {
			m.HTML = "<pre>" + html.EscapeString(m.Text) + "</pre>"
		}
		m.HTML += "<p>" + html.EscapeString(m.DecodeStatus) + "</p>"
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
	// Resolve each subtree before combining it: alternatives replace one
	// representation; mixed children are successive sections of the message.
	var sections []string
	hasHTML := false
	for _, part := range parts {
		child := &Message{Inline: m.Inline}
		if err := child.walk(part.header, bytes.NewReader(part.body), true); err != nil {
			return err
		}
		m.Attachments = append(m.Attachments, child.Attachments...)
		m.note(child.DecodeStatus)
		if mediaType == "multipart/alternative" {
			if strings.TrimSpace(child.HTML) != "" {
				m.HTML = child.HTML
			}
			if strings.TrimSpace(child.Text) != "" {
				m.Text = child.Text
			}
		} else {
			if child.Text != "" {
				if m.Text != "" {
					m.Text += "\n"
				}
				m.Text += child.Text
			}
			if child.HTML != "" {
				hasHTML = true
				sections = append(sections, child.HTML)
			} else if child.Text != "" {
				sections = append(sections, "<pre>"+html.EscapeString(child.Text)+"</pre>")
			}
		}
	}
	if mediaType != "multipart/alternative" {
		// A lone plain body retains the plain-only API contract.
		if len(sections) > 1 || hasHTML {
			m.HTML = strings.Join(sections, "\n")
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
	part := Part{ContentType: mediaType, Filename: filename, Data: raw}
	cid := contentID(h)
	if cid != "" {
		if _, exists := m.Inline[cid]; !exists {
			m.Inline[cid] = part
		}
	}
	if disposition == "attachment" || filename != "" && cid == "" {
		m.Attachments = append(m.Attachments, part)
		return nil
	}
	switch mediaType {
	case "text/html", "text/plain":
		decoded, status := decodeText(raw, params["charset"])
		m.note(status)
		if mediaType == "text/html" {
			m.HTML = decoded
		} else {
			m.Text = decoded
		}
	default:
		if cid == "" {
			m.Attachments = append(m.Attachments, part)
			m.note("Unsupported message part (" + mediaType + "). Open in Gmail to view the original.")
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

func (m *Message) note(status string) {
	if status == "" {
		return
	}
	if m.DecodeStatus != "" {
		m.DecodeStatus += " "
	}
	m.DecodeStatus += status
}

func decodeCharset(raw []byte, charset string) string {
	decoded, _ := decodeText(raw, charset)
	return decoded
}

func decodeText(raw []byte, charset string) (string, string) {
	label := strings.ToLower(strings.Trim(strings.TrimSpace(charset), `"`))
	note := "Message text could not be decoded completely (charset " + label + "). Original bytes retained; open in Gmail."
	if label == "" || label == "utf-8" || label == "utf8" || label == "us-ascii" {
		if utf8.Valid(raw) && (label != "us-ascii" || isASCII(raw)) {
			return string(raw), ""
		}
		return strings.ToValidUTF8(string(raw), "�"), note
	}
	enc := charsetEncoding(label)
	if enc == nil {
		return strings.ToValidUTF8(string(raw), "�"), note
	}
	decoded, err := enc.NewDecoder().Bytes(raw)
	if err != nil || !utf8.Valid(decoded) || bytes.Contains(decoded, []byte("�")) {
		return strings.ToValidUTF8(string(decoded), "�"), note
	}
	return string(decoded), ""
}

func isASCII(raw []byte) bool {
	for _, b := range raw {
		if b > 127 {
			return false
		}
	}
	return true
}

func charsetEncoding(charset string) encoding.Encoding {
	label := strings.ToLower(strings.Trim(strings.TrimSpace(charset), `"`))
	enc, err := ianaindex.MIME.Encoding(label)
	if err == nil && enc != nil {
		return enc
	}
	enc, _ = htmlindex.Get(label)
	return enc
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
