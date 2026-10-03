// SPDX-License-Identifier: MIT
// Desktop uses a network-isolated browser; Android keeps its native rich-text view.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

// A plain Page: the message body scrolls itself inside the remaining height,
// so a browser view never has to report its document height back to QML.
Kirigami.Page {
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
    property string cardTitle: ""
    property string accountName: ""
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
        id: messageColumn
        width: Math.min(parent.width, 1200)
        height: parent.height
        anchors.horizontalCenter: parent.horizontalCenter

        spacing: Kirigami.Units.largeSpacing

        RowLayout {
            Layout.fillWidth: true
            MailActionButton {
                objectName: "backToInbox"
                text: qsTr("Back to inbox")
                icon.name: "go-previous"
                icon.color: root.ink
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                onClicked: root.QQC2.StackView.view.pop()
            }
            Item { Layout.fillWidth: true }
            MailActionButton {
                objectName: "openInGmail"
                text: qsTr("Open in Gmail")
                icon.name: "document-open"
                icon.color: root.ink
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                visible: root.sourceUrl !== ""
                onClicked: root.openInGmail()
            }
        }

        Kirigami.Heading {
            text: root.cardTitle || root.title
            textFormat: Text.PlainText
            color: root.ink
            level: 2
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.Label {
            text: root.accountName
            textFormat: Text.PlainText
            color: root.mutedInk
            visible: text !== ""
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.BusyIndicator {
            visible: root.loading
            running: root.loading
            Layout.alignment: Qt.AlignHCenter
        }

        QQC2.Label {
            visible: root.errorText !== "" && !root.loading
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        QQC2.Frame {
            objectName: "messageDocument"
            visible: root.html !== ""
            padding: Math.max(18, Kirigami.Units.largeSpacing)
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.minimumWidth: 0
            Layout.preferredWidth: 0
            background: Rectangle { color: root.surface; radius: Kirigami.Units.cornerRadius; border.color: "#dce5e3" }
            contentItem: Loader {
                id: bodyLoader
                source: Qt.platform.os === "android" ? "MailBodyAndroid.qml" : "MailBodyDesktop.qml"
                onLoaded: item.html = Qt.binding(function() { return root.html })
                Connections {
                    target: bodyLoader.item
                    function onLinkActivated(link) {
                        if (link.indexOf("data:") === 0) {
                            var local = ""
                            if (typeof store.openCachedFile === "function")
                                local = store.openCachedFile(root.cardId, link)
                            if (local && openLinks)
                                Qt.openUrlExternally(local)
                        } else if (openLinks && /^(https?:|mailto:)/i.test(link)) {
                            Qt.openUrlExternally(link)
                        }
                    }
                }
            }
        }
    }
}
