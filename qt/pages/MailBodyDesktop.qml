import QtQuick
import QtWebEngine
import Litterbox.Mail 1.0

WebEngineView {
    id: root
    objectName: "body"
    property string html: ""
    signal linkActivated(string link)
    backgroundColor: "white"
    profile: MailDocumentProfile { id: mailProfile }
    property string renderedHtml: ""
    property int htmlRevision: 0
    url: mailProfile.documentUrl(renderedHtml)
    onHtmlChanged: {
        const revision = ++htmlRevision
        const nextHtml = html
        if (loading || loadProgress !== 100 || renderedHtml === "") {
            renderedHtml = nextHtml
            return
        }
        // ApplicationWorld is isolated from mail; document JavaScript stays disabled.
        runJavaScript(`Array.from(document.querySelectorAll("main.lb-reader > details.lb-previous[data-lb-key]"))
            .map(d => [d.getAttribute("data-lb-key"), d.open])`, WebEngineScript.ApplicationWorld, function(states) {
            if (revision !== root.htmlRevision) return
            let result = nextHtml
            for (const state of (states || [])) {
                const marker = "<details class='lb-previous' data-lb-key='" + state[0] + "'"
                result = result.replace(marker + " open", marker)
                if (state[1]) result = result.replace(marker, marker + " open")
            }
            root.renderedHtml = result
        })
    }
    settings.javascriptEnabled: false
    settings.javascriptCanOpenWindows: false
    settings.localContentCanAccessRemoteUrls: false
    settings.localContentCanAccessFileUrls: false
    settings.pluginsEnabled: false
    settings.webGLEnabled: false
    settings.errorPageEnabled: false
    onNavigationRequested: function(request) {
        if (request.url.toString().indexOf("litterbox-mail://message/body?") === 0) return
        request.reject()
        if (request.isMainFrame && request.navigationType === WebEngineView.LinkClickedNavigation)
            root.linkActivated(request.url.toString())
    }
    onNewWindowRequested: function(request) {
        if (request.userInitiated) root.linkActivated(request.requestedUrl.toString())
    }
    onContextMenuRequested: function(request) { request.accepted = true }
}
