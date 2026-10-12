package httpapi

import (
	"html"
	"io"
	"net/http"
)

// RegisterPublicPages serves the OAuth app's homepage and privacy policy.
func RegisterPublicPages(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", publicPage(homePage))
	mux.HandleFunc("GET /privacy", publicPage(privacyPage))
}

func publicPage(page string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, page)
	}
}

// MessagePage uses the public homepage shell and escapes all dynamic text.
func MessagePage(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, pageStart+"<title>Litterbox</title>\n"+pageStyle+"<h1>Litterbox</h1>\n<p>"+html.EscapeString(message)+"</p>\n</main>\n</body>\n</html>\n")
}

const pageStart = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
`

const pageStyle = `<style>
body { max-width: 42rem; margin: 4rem auto; padding: 0 1.5rem; font: 1.1rem/1.6 system-ui, sans-serif; color: #202632; background: #fafafa; }
h1, h2 { line-height: 1.2; }
a { color: #164ca0; }
nav { margin-top: 2rem; }
</style>
</head>
<body>
<main>
`

const homePage = pageStart + `<title>Litterbox</title>
` + pageStyle + `<h1>Litterbox</h1>
<p>A personal inbox app by Darafei Praliaskouski.</p>
<p>Litterbox helps its owner view and sync their own mail on their own server.</p>
<p>Contact: <a href="mailto:komzpa@gmail.com">komzpa@gmail.com</a>.</p>
<nav><a href="/privacy">Privacy policy</a></nav>
</main>
</body>
</html>
`

const privacyPage = pageStart + `<title>Litterbox Privacy Policy</title>
` + pageStyle + `<h1>Litterbox Privacy Policy</h1>
<p>Litterbox is a personal inbox app by Darafei Praliaskouski.</p>
<h2>Google API data and Limited Use</h2>
<p>Gmail data is read only to show and sync the owner's own mail inside Litterbox. It is stored on the owner's own server, is not shared with third parties, and is not used for ads or model training.</p>
<p>Litterbox's use and transfer of information received from Google APIs adheres to the <a href="https://developers.google.com/terms/api-services-user-data-policy">Google API Services User Data Policy</a>, including the Limited Use requirements.</p>
<p>For privacy questions, contact <a href="mailto:komzpa@gmail.com">komzpa@gmail.com</a>.</p>
<nav><a href="/">Litterbox home</a></nav>
</main>
</body>
</html>
`