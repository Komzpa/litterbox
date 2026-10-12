package httpapi

import (
 "net/http/httptest"
 "strings"
 "testing"
)

func TestMessagePageEscapesEmail(t *testing.T) {
 w:=httptest.NewRecorder()
 MessagePage(w,200,"Gmail connected: <script>alert(1)</script>@example.test. You can close this tab and return to Litterbox.")
 if strings.Contains(w.Body.String(),"<script>") || !strings.Contains(w.Body.String(),"&lt;script&gt;") {t.Fatal(w.Body.String())}
 if !strings.Contains(w.Body.String(),pageStyle) {t.Fatal("public shell CSS missing")}
}
