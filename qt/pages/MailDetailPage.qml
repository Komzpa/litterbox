// SPDX-License-Identifier: MIT
// Desktop uses a network-isolated browser; Android keeps its native rich-text view.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami
import litterbox 1.0 as Litterbox

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
    property var card: ({ source: "mail" })
    property var requestNote: null
    // Set to false in tests to avoid spawning an external browser.
    property bool openLinks: true
    // Pinned in tests so the today / this-year / older arrival branches are exact.
    property double arrivalNow: Date.now()

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

    // Esc closes the mail view and returns to the inbox, matching the
    // toolbar back button. Only the page root consumes Esc; inner controls
    // keep their own Esc handling (e.g. dialogs) because unaccepted key
    // events bubble up to this handler.
    Keys.onEscapePressed: function(event) {
        const view = root.QQC2.StackView.view
        if (view && view.depth > 1) {
            view.pop()
            event.accepted = true
        }
    }

    // Sender and arrival line: "Name <address> · 11:43" for mail received
    // today, day + month + time this year, full date otherwise; no separator
    // without an arrival time.
    function arrivalLine() {
        const c = root.card || {}
        const name = String(c.sender_name || "").trim()
        const addr = String(c.sender_address || "").trim()
        let sender = name
        if (addr) sender = name ? name + " <" + addr + ">" : "<" + addr + ">"
        const arrived = c.received_at ? new Date(c.received_at) : null
        if (!arrived || isNaN(arrived.getTime())) return sender
        const now = new Date(root.arrivalNow)
        let stamp = Qt.formatDateTime(arrived, "HH:mm")
        if (arrived.getFullYear() !== now.getFullYear())
            stamp = Qt.formatDateTime(arrived, "d MMM yyyy HH:mm")
        else if (arrived.getDate() !== now.getDate() || arrived.getMonth() !== now.getMonth())
            stamp = Qt.formatDateTime(arrived, "d MMM HH:mm")
        return sender ? sender + " · " + stamp : stamp
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
            Connections {
                target: root.store
                // Keep pin/bundle state current even if the inbox delegate moves away.
                ignoreUnknownSignals: true
                function onDataChanged() {
                    const row = root.store.cardIds().indexOf(root.cardId)
                    if (row >= 0)
                        root.card = root.store.data(root.store.index(row, 0), Litterbox.CardStore.CardRole)
                }
            }
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
            MailActionButton {
                id: archiveButton
                objectName: "archiveMail"
                text: mailActions.source === "mail" || mailActions.source === "journal" ? qsTr("Archive") : qsTr("Done")
                icon.name: mailActions.source === "mail" ? "mail-mark-read-symbolic" : "dialog-ok"
                Accessible.name: mailActions.primaryName
                onClicked: mailActions.primaryAction()
                Shortcut {
                    sequence: "E"
                    enabled: root.QQC2.StackView.status === QQC2.StackView.Active && (!root.QQC2.Overlay.overlay || !root.QQC2.Overlay.overlay.visible)
                    onActivated: mailActions.primaryAction()
                }
            }
            Litterbox.CardActions {
                id: mailActions
                objectName: "mailActions"
                store: root.store
                cardKey: root.cardId
                source: root.card.source || ""
                hasBody: !!root.card.has_body
                bundleId: root.card.bundle_id || ""
                pinnedRank: root.card.pinned_rank
                cardTitle: root.cardTitle
                accountName: root.accountName
                canOpenSource: root.sourceUrl !== ""
                onOpenRequested: root.openInGmail()
                onReadCachedRequested: root.reload()
                onNoteRequested: { if (root.requestNote) root.requestNote(root.cardId, root.card.note || "") }
                onCardHandled: root.QQC2.StackView.view.pop()
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
            objectName: "mailArrival"
            text: root.arrivalLine()
            textFormat: Text.PlainText
            color: root.mutedInk
            visible: text !== ""
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
