import QtQuick

// Desktop regression runs exercise the shared browser document. Android loads
// its native WebView, never QTextDocument or a scaled desktop mail canvas.
Loader {
    id: root
    property string html: ""
    signal linkActivated(string link)
    source: Qt.platform.os === "android" ? "MailBodyNative.qml" : "MailBodyDesktop.qml"
    onLoaded: item.html = Qt.binding(function() { return root.html })
    Connections {
        target: root.item
        function onLinkActivated(link) { root.linkActivated(link) }
    }
}
