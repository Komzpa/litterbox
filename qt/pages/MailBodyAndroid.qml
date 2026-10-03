import QtQuick
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

// Android keeps Qt rich text; the page no longer scrolls, so this view does.
Flickable {
    id: root
    property string html: ""
    signal linkActivated(string link)
    clip: true
    contentWidth: width
    contentHeight: body.implicitHeight
    flickableDirection: Flickable.VerticalFlick
    QQC2.ScrollBar.vertical: QQC2.ScrollBar {}
    TextEdit {
        id: body
        objectName: "body"
        width: root.width
        text: "<style>pre { white-space: pre-wrap; }</style>" + root.html
        textFormat: TextEdit.RichText
        wrapMode: TextEdit.Wrap
        readOnly: true
        selectByMouse: true
        color: "#263b3a"
        selectionColor: "#397d73"
        selectedTextColor: "#ffffff"
        font: Kirigami.Theme.defaultFont
        onLinkActivated: function(link) { root.linkActivated(link) }
    }
}
