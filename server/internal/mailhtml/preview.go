package mailhtml

import (
 "strings"
 "unicode/utf8"

 "golang.org/x/net/html"
)

// PreviewText projects visible prose without changing the stored MIME bodies.
// Images (including linked logos), hidden preheaders and active content are not
// prose. Anchor text is retained; its tracking destination is not displayed.
func PreviewText(body string) string {
 root, err := html.Parse(strings.NewReader(body))
 if err != nil { return "" }
 var blocks []string
 var text strings.Builder
 var walk func(*html.Node)
 flush := func() {
  if line := strings.Join(strings.Fields(text.String()), " "); line != "" { blocks = append(blocks, line) }
  text.Reset()
 }
 walk = func(n *html.Node) {
  if n.Type == html.ElementNode {
   switch n.Data { case "head", "script", "style", "img", "svg", "blockquote": return }
   for _, a := range n.Attr {
    style := strings.ToLower(strings.ReplaceAll(a.Val, " ", ""))
    if a.Key == "hidden" || (a.Key == "aria-hidden" && style == "true") || (a.Key == "style" && (strings.Contains(style,"display:none") || strings.Contains(style,"visibility:hidden") || strings.Contains(style,"opacity:0;") || style == "opacity:0")) { return }
   }
   switch n.Data { case "p", "div", "td", "tr", "li", "h1", "h2", "h3", "h4", "br": flush() }
  }
  if n.Type == html.TextNode { text.WriteString(n.Data) }
  for c := n.FirstChild; c != nil; c = c.NextSibling { walk(c) }
  if n.Type == html.ElementNode {
   switch n.Data { case "p", "div", "td", "tr", "li", "h1", "h2", "h3", "h4": flush() }
  }
 }
 walk(root)
 flush()
 // Prefer a prose block over short navigation, promotional labels and image
 // captions. Short messages and genuine link-only mail retain their text.
 for _, line := range blocks { if utf8.RuneCountInString(line) >= 80 { return line } }
 return strings.Join(blocks, "\n")
}
