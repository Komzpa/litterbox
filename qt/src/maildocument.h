#pragma once
#include <QRegularExpression>
#include <QString>

// Both Chromium renderers receive exactly the same offline document and CSS.
// The reader canvas is a forced-light surface, and a dark-preferring host must
// not wake newsletter dark-palette blocks over it (pale links on white, dark
// quote slabs, invisible titles). Chromium does not clamp prefers-color-scheme
// to the document's color-scheme, so the newsletter's stylesheet conditions are
// pinned to their light-host truth value instead: 'prefers-color-scheme: dark'
// is false and 'light'/'no-preference' are true there. Substituting constant
// atoms preserves every surrounding not/and/or/comma semantics, keeps all
// declarations (no blanket style strip), and touches only <style> blocks, so
// visible HTML, images, quotes and data-table horizontal scrolling stay exact.
inline QString mailDocumentPinLightScheme(const QString &html)
{
    static const QRegularExpression styleBlock(
        QStringLiteral("(<style[^>]*>)(.*?)(</style>)"),
        QRegularExpression::CaseInsensitiveOption | QRegularExpression::DotMatchesEverythingOption);
    static const QRegularExpression darkQuery(
        QStringLiteral("\\(\\s*prefers-color-scheme\\s*:\\s*dark\\s*\\)"),
        QRegularExpression::CaseInsensitiveOption);
    static const QRegularExpression lightQuery(
        QStringLiteral("\\(\\s*prefers-color-scheme\\s*:\\s*(?:light|no-preference)\\s*\\)"),
        QRegularExpression::CaseInsensitiveOption);
    QString out;
    out.reserve(html.size());
    qsizetype pos = 0;
    QRegularExpressionMatchIterator blocks = styleBlock.globalMatch(html);
    while (blocks.hasNext()) {
        const QRegularExpressionMatch block = blocks.next();
        out += html.mid(pos, block.capturedStart(0) - pos);
        QString css = block.captured(2);
        css.replace(darkQuery, QStringLiteral("(min-width: 999999px)"));
        css.replace(lightQuery, QStringLiteral("(min-width: 0px)"));
        out += block.captured(1) + css + block.captured(3);
        pos = block.capturedEnd(0);
    }
    out += html.mid(pos);
    return out;
}

inline QString mailDocument(const QString &html)
{
    return QStringLiteral(
        "<!doctype html><html><head><meta charset='utf-8'>"
        "<meta name='viewport' content='width=device-width,initial-scale=1'>"
        "<meta http-equiv='Content-Security-Policy' content=\"default-src 'none'; "
        "img-src data:; style-src 'unsafe-inline'; script-src 'none'; "
        "frame-src 'none'; object-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'\">"
        // Belt and braces with the stylesheet pin below: the document declares
        // itself light-only, so no UA may auto-darken it either.
        "<meta name='color-scheme' content='only light'>"
        "</head><body>") + mailDocumentPinLightScheme(html) + QStringLiteral(
        "<style>:root{color-scheme:only light!important}"
        "html{color-scheme:only light;background:white}"
        // Links the author left unstyled stay readable over any band they sit
        // on (dark footer bands included); :where() is specificity 0, so every
        // designed link color keeps winning the cascade.
        ":where(a:link,a:visited,a:hover,a:active){color:#4577c4}"
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
        // Actual data tables keep their column relationships and scroll
        // horizontally only. overflow-x:auto alone computes overflow-y to
        // auto, which made every wide table a second vertical scroller inside
        // the message; pinning overflow-y to hidden keeps the horizontal port
        // and leaves the document as the only vertical viewport.
        "table:not([role=presentation]){display:block;overflow-x:auto;overflow-y:hidden}"
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
