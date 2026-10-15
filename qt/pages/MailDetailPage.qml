// SPDX-License-Identifier: MIT
// Offline browser renderers preserve real HTML and responsive email layouts.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami
import litterbox 1.0 as Litterbox

// A plain Page: one browser document holds subject, account and body and is the
// page's only scroll, so a browser view never reports its height back to QML.
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
    background: Rectangle { color: canvas }

    required property var store
    property string cardId: ""
    property string cardTitle: ""
    property string accountName: ""
    property var card: ({ source: "mail" })
    property var requestNote: null
    property var requestArchiveUndo: null
    // Set to false in tests to avoid spawning an external browser.
    property bool openLinks: true
    // Pinned in tests so the today / this-year / older arrival branches are exact.
    property double arrivalNow: Date.now()

    property bool loading: false
    property string errorText: ""
    property string html: ""
    property string sourceUrl: ""
    property string bodySource: Qt.platform.os === "android" ? "MailBodyAndroid.qml" : "MailBodyDesktop.qml"

    title: root.sourceUrl !== "" ? qsTr("Message") : qsTr("Details")

    function readCache() {
        const body = store.cachedMailBody(cardId)
        html = body.html || ""
        if (!html && (root.card || {}).source !== "mail") html = fallbackHtml()
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

    // R59: one slim app bar, then a single page scroll. Subject, account and
    // body live in one document, so metadata scrolls away with the message and
    // there is no inner bordered scroll box under half-screen chrome.
    component BarButton: QQC2.ToolButton {
        id: barButton
        display: QQC2.AbstractButton.IconOnly
        icon.width: 24
        icon.height: 24
        icon.color: root.ink
        implicitWidth: 48
        implicitHeight: 48
        Accessible.name: text
        QQC2.ToolTip.text: text
        QQC2.ToolTip.visible: hovered
        // Kirigami resolves the bundled/theme icon on every style; a bare
        // ToolButton icon renders nothing under some desktop styles.
        contentItem: Kirigami.Icon {
            source: barButton.icon.name
            isMask: true
            color: root.ink
            implicitWidth: 24
            implicitHeight: 24
        }
        background: Rectangle {
            radius: 24
            color: barButton.down ? "#e8eeed" : barButton.hovered ? "#eef3f2" : root.surface
            border.width: barButton.visualFocus ? 2 : 0
            border.color: root.ink
        }
    }

    function escapeHtml(text) {
        return String(text || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;")
    }
    // R58: agent and other non-mail cards without a cached body still open in
    // full: their complete summary becomes the document, links stay links.
    function fallbackHtml() {
        const c = root.card || {}
        const text = String(c.summary || c.snippet || "")
        if (!text) return ""
        const linked = escapeHtml(text).replace(/(https?:\/\/[^\s<]+)/g, "<a href='$1'>$1</a>")
        return linked.split(/\n{2,}/).map(function(part) { return "<p>" + part.replace(/\n/g, "<br>") + "</p>" }).join("")
    }
    readonly property string headerHtml: "<div style='padding:4px 0 12px'><h2 style='font-size:20px;line-height:1.3;margin:0 0 4px'>"
        + escapeHtml(root.cardTitle || root.title) + "</h2>"
        + (root.accountName ? "<div style='color:#586d70;font-size:14px'>" + escapeHtml(root.accountName) + "</div>" : "")
        + "</div>"
    readonly property string documentHtml: root.html === "" ? "" : "<div style='padding:0 16px'>" + headerHtml + "</div>"
        + "<div style='padding:0 16px 24px'>" + root.html + "</div>"

    padding: 0
    ColumnLayout {
        id: messageColumn
        anchors.fill: parent
        spacing: 0

        Rectangle {
            objectName: "mailAppBar"
            Layout.fillWidth: true
            implicitHeight: 56
            color: root.surface
            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: "#dce5e3" }
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
            RowLayout {
                objectName: "mailActionRow"
                anchors.fill: parent
                anchors.leftMargin: 4
                anchors.rightMargin: 4
                spacing: 0
                BarButton {
                    objectName: "backToInbox"
                    text: qsTr("Back to inbox")
                    icon.name: "go-previous"
                    onClicked: root.QQC2.StackView.view.pop()
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.leftMargin: 4
                    spacing: 0
                    QQC2.Label {
                        objectName: "mailBarTitle"
                        text: root.cardTitle || root.title
                        textFormat: Text.PlainText
                        color: root.ink
                        font.weight: Font.DemiBold
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        Layout.fillWidth: true
                    }
                    QQC2.Label {
                        objectName: "mailArrival"
                        text: root.arrivalLine()
                        textFormat: Text.PlainText
                        color: root.mutedInk
                        font.pointSize: Kirigami.Theme.defaultFont.pointSize - 2
                        visible: text !== ""
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        Layout.fillWidth: true
                    }
                }
                BarButton {
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
                BarButton {
                    objectName: "openInGmail"
                    text: qsTr("Open in Gmail")
                    icon.name: "document-open"
                    visible: root.sourceUrl !== ""
                    onClicked: root.openInGmail()
                }
                Litterbox.CardActions {
                    id: mailActions
                    objectName: "mailActions"
                    implicitWidth: 48
                    implicitHeight: 48
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
                    onCardHandled: function(operation, action) {
                        root.QQC2.StackView.view.pop()
                        if (operation && root.requestArchiveUndo) root.requestArchiveUndo(operation, action)
                    }
                }
            }
        }

        QQC2.Label {
            visible: root.errorText !== "" && !root.loading
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            padding: 16
            Layout.fillWidth: true
        }

        Item {
            objectName: "messageDocument"
            visible: root.html !== "" || root.loading
            Layout.fillWidth: true
            Layout.fillHeight: true
            QQC2.BusyIndicator {
                // Refreshing a cached message never pushes the document down.
                z: 1
                visible: root.loading
                running: root.loading
                width: 32
                height: 32
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.margins: 8
            }
            Loader {
                id: bodyLoader
                anchors.fill: parent
                source: root.bodySource
                onLoaded: item.html = Qt.binding(function() { return root.documentHtml })
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
        Item { Layout.fillHeight: true; visible: root.html === "" && !root.loading }
    }
}
