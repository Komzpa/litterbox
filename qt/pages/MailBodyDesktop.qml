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
    url: mailProfile.documentUrl(html)
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
