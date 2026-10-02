// SPDX-License-Identifier: MIT
// Mail message detail: sanitized HTML body plus an "Open in Gmail" link.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: canvas
    Kirigami.Theme.alternateBackgroundColor: surface
    Kirigami.Theme.highlightColor: accent
    Kirigami.Theme.focusColor: accent
    Kirigami.Theme.hoverColor: accent
    Material.theme: Material.Light
    Material.background: surface
    Material.foreground: ink
    Material.accent: accent
    Material.primary: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.text: ink
    palette.button: surface
    palette.buttonText: ink
    palette.highlight: accent
    palette.highlightedText: surface
    padding: Math.max(18, Kirigami.Units.largeSpacing)
    background: Rectangle { color: canvas }

    required property var store
    property string cardId: ""
    // Set to false in tests to avoid spawning an external browser.
    property bool openLinks: true

    property bool loading: false
    property string errorText: ""
    property string html: ""
    property string sourceUrl: ""

    title: root.sourceUrl !== "" ? qsTr("Message") : qsTr("Details")

    function readCache() {
        const body = store.cachedMailBody(cardId)
        html = body.html || ""
        sourceUrl = body.source_url || ""
        errorText = html ? "" : qsTr("This message is not cached on this device yet.")
    }
    function reload() {
        readCache()
        loading = store.online
        if (loading) store.requestMailBody(cardId)
    }
    Connections {
        target: root.store
        function onMailBodyChanged(cardId) {
            if (cardId !== root.cardId) return
            root.loading = false
            root.readCache()
        }
        function onMailBodyFailed(cardId) {
            if (cardId !== root.cardId) return
            root.loading = false
            if (!root.html) root.errorText = qsTr("Message unavailable. Reconnect to cache it; opening it never archives it.")
        }
    }

    function openInGmail() {
        if (sourceUrl && openLinks)
            Qt.openUrlExternally(sourceUrl)
    }

    Component.onCompleted: {
        if (cardId !== "")
            reload()
    }

    ColumnLayout {
        width: Math.min(parent.width, 1200)
        anchors.horizontalCenter: parent.horizontalCenter

        QQC2.BusyIndicator {
            visible: root.loading
            Layout.alignment: Qt.AlignHCenter
        }

        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        Kirigami.LinkButton {
            objectName: "openInGmail"
            text: qsTr("Open in Gmail")
            visible: root.sourceUrl !== ""
            onClicked: root.openInGmail()
        }

        QQC2.Button {
            text: qsTr("Back to inbox")
            implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
            onClicked: root.StackView.view.pop()
        }

        QQC2.Label {
            id: bodyLabel
            objectName: "body"
            text: root.html
            textFormat: Text.RichText
            wrapMode: Text.Wrap
            visible: root.html !== ""
            Layout.fillWidth: true
            // Server-rendered agent bodies embed files as data: links with a
            // name= parameter; open them from the local cached copy only.
            // Other links keep the existing external opener behavior.
            onLinkActivated: function(link) {
                if (link.indexOf("data:") === 0) {
                    var local = "";
                    if (typeof store.openCachedFile === "function")
                        local = store.openCachedFile(root.cardId, link);
                    if (local && openLinks)
                        Qt.openUrlExternally(local);
                } else if (openLinks) {
                    Qt.openUrlExternally(link);
                }
            }
        }
    }
}
