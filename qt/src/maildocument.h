#pragma once
#include <QString>

// Both Chromium renderers receive exactly the same offline document and CSS.
inline QString mailDocument(const QString &html)
{
    return QStringLiteral(
        "<!doctype html><html><head><meta charset='utf-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        "<meta http-equiv='Content-Security-Policy' content=\"default-src 'none'; "
        "img-src data:; style-src 'unsafe-inline'; script-src 'none'; "
        "frame-src 'none'; object-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'\">"
        "</head><body>") + html + QStringLiteral(
        "<style>html{color-scheme:light;background:white}"
        "body{margin:0!important;width:auto!important;max-width:100%!important;"
        "font-family:sans-serif;font-size:16px;line-height:1.45;color:#263b3a;background-color:white}"
        // Break only otherwise unbreakable URLs; ordinary words stay intact.
        "*{min-width:0!important;box-sizing:border-box;word-break:normal!important;overflow-wrap:break-word}"
        // Email preheaders marked hidden for Outlook must not create text boxes
        // when responsive wrapping expands their otherwise zero-size container.
        "[style*='mso-hide: all']{display:none!important}"
        "img{max-width:100%!important;height:auto}"
        "table[align=center] img{margin-inline:auto}"
        "table{max-width:100%!important}"
        "table[role=presentation]{width:100%!important}"
        // Actual data tables keep their column relationships and scroll locally.
        "table:not([role=presentation]){display:block;overflow-x:auto}"
        "pre{white-space:pre-wrap!important;overflow-wrap:break-word}"
        // Fixed email canvases and layout columns must reflow, not shrink text.
        "@media(max-width:600px){"
        "table[role=presentation],table[role=presentation]>tbody,"
        "table[role=presentation]>tbody>tr,table[role=presentation]>tbody>tr>td,"
        "table[role=presentation]>tbody>tr>th{display:block!important;width:100%!important;max-width:100%!important}"
        "table[role=presentation]>tbody>tr>td,table[role=presentation]>tbody>tr>th{height:auto!important}"
        "div{max-width:100%!important}"
        "}"
        "</style></body></html>");
}
