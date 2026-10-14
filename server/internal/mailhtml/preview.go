package mailhtml

import (
 "strings"

 "golang.org/x/net/html"
)

type previewBlock struct {
	text      string
	cell      *html.Node
	heading   bool
	paragraph bool
	band      bool
}

// PreviewText projects visible prose without changing the stored MIME bodies.
// Images (including linked logos), hidden preheaders and active content are not
// prose. Anchor text is retained; its tracking destination is not displayed.
// A heading-led article after a presentation-table label band supplies body
// prose, not the surrounding template. Unsectioned correspondence keeps order.
func PreviewText(body string) string {
 root, err := html.Parse(strings.NewReader(body))
 if err != nil { return "" }
	var blocks []previewBlock
	var text strings.Builder
	var block previewBlock
	var hasText bool
	var walk func(*html.Node, *html.Node, bool, bool, bool)
	flush := func() {
		if line := strings.Join(strings.Fields(text.String()), " "); line != "" {
			block.text = line
			blocks = append(blocks, block)
		}
		text.Reset()
		hasText = false
 }
	walk = func(n, cell *html.Node, band, heading, paragraph bool) {
  if n.Type == html.ElementNode {
   switch n.Data { case "head", "script", "style", "img", "svg", "blockquote": return }
   for _, a := range n.Attr {
    style := strings.ToLower(strings.ReplaceAll(a.Val, " ", ""))
    if a.Key == "hidden" || (a.Key == "aria-hidden" && style == "true") || (a.Key == "style" && (strings.Contains(style,"display:none") || strings.Contains(style,"visibility:hidden") || strings.Contains(style,"opacity:0;") || style == "opacity:0")) { return }
   }
   switch n.Data { case "p", "div", "td", "tr", "li", "h1", "h2", "h3", "h4", "br": flush() }
			// A nested data table owns its content; an outer layout cannot
			// classify it as decoration. Presentation wrappers retain the band.
			if n.Data == "table" && !presentationTable(n) {
				band = false
			}
			if n.Data == "td" || n.Data == "th" {
				cell = n
				band = band || presentationBand(n)
			}
			heading = heading || n.Data == "h1"
			paragraph = paragraph || n.Data == "p"
  }
		if n.Type == html.TextNode {
			if !hasText && strings.TrimSpace(n.Data) != "" {
				block = previewBlock{cell: cell, band: band, heading: heading, paragraph: paragraph}
				hasText = true
			}
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, cell, band, heading, paragraph)
		}
  if n.Type == html.ElementNode {
   switch n.Data { case "p", "div", "td", "tr", "li", "h1", "h2", "h3", "h4": flush() }
  }
 }
	walk(root, nil, false, false, false)
	flush()
	// Only a separate multi-column presentation band may precede the article.
	// A normal introductory sentence or data table prevents this selection;
	// neither short text nor everything before a heading is template chrome.
	start := 0
	for i, b := range blocks {
		if i > 0 && b.heading && b.cell != nil && i+1 < len(blocks) &&
			containsCell(b.cell, blocks[i+1].cell) && blocks[i+1].paragraph {
			separate := true
			for _, prefix := range blocks[:i] {
				if containsCell(b.cell, prefix.cell) {
					separate = false
					break
				}
			}
			if separate {
				start = i + 1
			}
			break
		}
		if !b.band {
			break
		}
	}
	text.Reset()
	for i, b := range blocks[start:] {
		if i > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(b.text)
	}
	return text.String()
}

// presentationBand recognizes side-by-side layout cells, not data-table cells.
// It uses document structure and the author's semantic role, never CSS classes,
// viewport rules, text length or promotional vocabulary.
func presentationBand(cell *html.Node) bool {
	row := cell.Parent
	if row == nil || row.Data != "tr" {
		return false
	}
	columns := 0
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			columns++
		}
	}
	if columns < 2 {
		return false
	}
	for n := row.Parent; n != nil; n = n.Parent {
		if n.Data == "table" {
			return presentationTable(n)
		}
	}
	return false
}

func containsCell(outer, inner *html.Node) bool {
	for n := inner; n != nil; n = n.Parent {
		if n == outer {
			return true
		}
	}
	return false
}

func presentationTable(table *html.Node) bool {
	for _, a := range table.Attr {
		if a.Key == "role" && strings.EqualFold(a.Val, "presentation") {
			return true
		}
	}
	return false
}
