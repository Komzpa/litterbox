// SPDX-License-Identifier: MIT
// Offline browser renderers preserve real HTML and responsive email layouts.
import QtQuick
import QtQuick.Dialogs
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
    // Inbox callback: (cardId) => opener | null. Called before a local Archive
    // mutates the list, so the next presented card is captured pre-removal.
    property var requestArchiveContinuation: null
    property bool embeddedMode: false
    property var requestClose: null
    // Real popup state owns the Android native WebView visibility. The window
    // owner (InboxView) binds ownerPopupVisible to its named dialogs/menu —
    // the source of truth; standalone pages leave it false and count only
    // this page's own snooze/action-sheet popups. Never a generic
    // Overlay.visible: the overlay item exists and stays visible with no
    // popup open (it hid the native body on Android).
    property bool ownerPopupVisible: false
    readonly property bool localPopupsVisible: {
        // One retained reader instance only, so loading its two dialogs here
        // is cheap — unlike the per-card rows where lazy creation keeps
        // scrolling fast (R63).
        const sheet = mailActions.actionSheetItem()
        const snooze = mailActions.snoozeDialogItem()
        return !!((sheet && sheet.visible) || (snooze && snooze.visible))
    }
    readonly property bool actualPopupsVisible: ownerPopupVisible || localPopupsVisible || saveFileDialog.visible
    // 48 dp tap floor in item px. Android AX reports a box one device px short of its painted size, so Android adds one px above ceil(48 dp).
    readonly property real tapTarget: (Math.ceil(48 * Screen.devicePixelRatio) + (Qt.platform.os === "android" ? 1 : 0)) / Screen.devicePixelRatio
    readonly property bool compactActions: width < Math.max(720, Kirigami.Units.gridUnit * 42)
    property bool closing: false
    property bool ready: false
    // Set to false in tests to avoid spawning an external browser.
    property bool openLinks: true
    // Pinned in tests so the today / this-year / older arrival branches are exact.
    property double arrivalNow: Date.now()

    property bool loading: false
    property string errorText: ""
    property string html: ""
    property string sourceUrl: ""
    property var messages: []
    property string attachmentError: ""
    property string saveSource: ""
    property var initialUnread: ({})
    property bool legacyIncomplete: false
    readonly property bool hasDocument: html !== "" || messages.length > 0
    property string bodySource: Qt.platform.os === "android" ? "MailBodyAndroid.qml" : "MailBodyDesktop.qml"

    title: root.sourceUrl !== "" ? qsTr("Message") : qsTr("Details")

    function readCache() {
        const body = store.cachedMailBody(cardId)
        const incoming = body.messages || []
        let flags = Object.assign({}, initialUnread)
        let changed = false
        let stable = []
        for (let i = 0; i < incoming.length; ++i) {
            const key = incoming[i].gmail_message_id || incoming[i].id || ("index:" + i)
            if (!(key in flags)) { flags[key] = !!incoming[i].unread || i === incoming.length - 1; changed = true }
            let message = Object.assign({}, incoming[i])
            message.unread = flags[key]
            stable.push(message)
        }
        if (changed) initialUnread = flags
        if (JSON.stringify(messages) !== JSON.stringify(stable)) messages = stable
        legacyIncomplete = Number(body.shape_version || 0) < 1 && stable.some(m => !m.html)
        html = body.html || ""
        if ((root.card || {}).source && root.card.source !== "mail") html = fallbackHtml() + html || "<div></div>"
        sourceUrl = body.source_url || ((root.card || {}).source_url || "")
        errorText = hasDocument ? "" : qsTr("This message is not cached on this device yet.")
    }
    function reload() {
        readCache()
        loading = !hasDocument && store.online
        if (store.online && ((root.card || {}).source === "mail" || (root.card || {}).has_body)) store.requestMailBody(cardId)
    }
    function recipientSummary(message) {
        const rows = (message && message.to) || []
        if (!rows.length) return qsTr("To") + ": —"
        const first = rows[0].name || rows[0].address || ""
        const count = rows.length - 1
        return qsTr("To") + ": " + first + (count > 0 ? qsTr(" +%1").arg(count) : "")
    }
    function recipientDetails(message, field) {
        return ((message && message[field]) || []).map(function(row) {
            const name = String(row.name || "").trim()
            const address = String(row.address || "").trim()
            return name && address ? name + " <" + address + ">" : (name || address)
        }).filter(function(value) { return !!value }).join(", ")
    }
    // System locale/timezone is the same owner state used by arrivalLine.
    function invitationRange(event) {
        function format(value, dateOnly) {
            if (!value) return ""
            const d = new Date(dateOnly ? value + "T00:00:00" : value)
            if (isNaN(d.getTime())) return value
            return dateOnly ? d.toLocaleDateString(Qt.locale(), Locale.LongFormat)
                            : d.toLocaleString(Qt.locale(), Locale.ShortFormat)
        }
        if (event.all_day) {
            // DTEND is exclusive for all-day events. Never convert dates via UTC.
            let end = new Date(event.end + "T00:00:00")
            end.setDate(end.getDate() - 1)
            const start = format(event.start, true)
            const last = isNaN(end.getTime()) ? "" : end.toLocaleDateString(Qt.locale(), Locale.LongFormat)
            return start + (last && last !== start ? " – " + last : "") + " · " + qsTr("All day")
        }
        return format(event.start, false) + (event.end ? " – " + format(event.end, false) : "")
    }
    function invitationHtml(message) {
        function person(p) {
            p = p || {}
            return (p.name ? p.name + (p.address ? " <" + p.address + ">" : "") : p.address || "")
        }
        function linked(value) {
            return escapeHtml(value).replace(/(https?:\/\/[^\s<>]+)/g, '<a href="$1">$1</a>').replace(/\n/g, "<br>")
        }
        let result = ""
        for (const event of (message.invitations || [])) {
            result += "<section class='lb-invitation' style='border-left:4px solid #397d73;padding:12px;margin:12px 0;overflow-wrap:anywhere'>"
                + "<strong>" + escapeHtml(event.cancelled ? qsTr("Cancelled") : event.method === "REPLY" ? qsTr("Invitation reply") : qsTr("Calendar invitation")) + "</strong>"
                + "<h2>" + escapeHtml(event.summary || qsTr("Untitled event")) + "</h2>"
                + "<p class='lb-invitation-time'>" + escapeHtml(invitationRange(event)) + "</p>"
            if (event.time_error) result += "<p>" + escapeHtml(event.time_error) + "</p>"
            if (event.recurring) result += "<p>" + escapeHtml(qsTr("Repeating event")) + "</p>"
            if (event.location) result += "<p>" + linked(event.location) + "</p>"
            for (const link of (event.links || [])) {
                if (/^https?:\/\//i.test(link)) result += "<p><a href='" + escapeHtml(link).replace(/'/g, "&#39;") + "'>" + escapeHtml(link) + "</a></p>"
            }
            if (person(event.organizer)) result += "<p>" + escapeHtml(qsTr("Organizer: %1").arg(person(event.organizer))) + "</p>"
            for (const attendee of (event.attendees || []))
                result += "<p>" + escapeHtml(person(attendee) + (attendee.status ? " · " + attendee.status : "")) + "</p>"
            if (event.description) result += "<p>" + linked(event.description) + "</p>"
            result += "</section>"
        }
        return result
    }
    function messageMetadata(message) {
        let result = "<details class='lb-recipients'><summary>" + escapeHtml(recipientSummary(message)) + "</summary><p>"
            + escapeHtml(qsTr("To: %1").arg(recipientDetails(message, "to") || "—")) + "<br>"
            + escapeHtml(qsTr("Cc: %1").arg(recipientDetails(message, "cc") || "—")) + "</p></details>"
        for (const file of (message.attachments || [])) {
            const id = encodeURIComponent(String(file.id)).replace(/'/g, "%27")
            result += "<p class='lb-file'>" + escapeHtml(file.name || qsTr("Attachment")) + " · " + Number(file.size || 0) + " B "
                + "<a href='litterbox-file:open/" + id + "'>" + escapeHtml(qsTr("Open")) + "</a> · "
                + "<a href='litterbox-file:save/" + id + "'>" + escapeHtml(qsTr("Save")) + "</a></p>"
        }
        return result
    }
    FileDialog {
        id: saveFileDialog
        fileMode: FileDialog.SaveFile
        onAccepted: {
            if (!root.store.saveMailFile(root.saveSource, selectedFile)) root.attachmentError = qsTr("Could not save attachment")
        }
    }
    Connections {
        target: root.store
        ignoreUnknownSignals: true
        function onMailFileReady(card, local, save) {
            if (card !== root.cardId) return
            root.attachmentError = ""
            if (save) { root.saveSource = local; saveFileDialog.selectedFile = local; saveFileDialog.open() }
            else if (root.openLinks && !Qt.openUrlExternally(local)) root.attachmentError = qsTr("Could not open attachment")
        }
        function onMailFileFailed(card) { if (card === root.cardId) root.attachmentError = qsTr("Attachment unavailable. Connect and try again.") }
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
            if (!root.hasDocument) root.errorText = qsTr("Message unavailable. Reconnect to cache it; opening it never archives it.")
        }
    }

    function openInGmail() {
        if (sourceUrl && openLinks)
            Qt.openUrlExternally(sourceUrl)
    }

    function closeReader() {
        if (closing) return
        closing = true
        if (requestClose) {
            requestClose()
        } else if (!embeddedMode) {
            const view = root.QQC2.StackView.view
            if (view && view.depth > 1) view.pop()
        }
    }
    Keys.onEscapePressed: function(event) {
        root.closeReader()
        event.accepted = true
    }
    Keys.onBackPressed: function(event) {
        root.closeReader()
        event.accepted = true
    }
    onVisibleChanged: { if (visible) closing = false }
    onCardIdChanged: {
        closing = false
        initialUnread = ({})
        if (ready && cardId !== "") reload()
    }

    // Sender and arrival line: "Name <address> · 11:43" for mail received
    // today, day + month + time this year, full date otherwise; no separator
    // without an arrival time.
    function arrivalLine(message) {
        const c = message || root.card || {}
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
        ready = true
        if (cardId !== "")
            reload()
    }

    // R59: metadata and all original messages share the browser's only scroll.
    // Desktop labels collapse to named icons on a narrow/touch-sized app bar.
    component BarButton: QQC2.ToolButton {
        id: barButton
        property bool showLabel: !root.compactActions
        display: showLabel ? QQC2.AbstractButton.TextBesideIcon : QQC2.AbstractButton.IconOnly
        icon.width: 24
        icon.height: 24
        icon.color: root.ink
        implicitWidth: showLabel ? implicitContentWidth + 24 : root.tapTarget
        implicitHeight: showLabel ? Math.max(root.tapTarget, implicitContentHeight + 16) : root.tapTarget
        Layout.minimumWidth: implicitWidth
        Layout.minimumHeight: root.tapTarget
        Layout.maximumHeight: root.compactActions ? root.tapTarget : Infinity
        Accessible.name: text
        QQC2.ToolTip.text: Accessible.name
        QQC2.ToolTip.visible: hovered || visualFocus
        // Kirigami resolves the bundled/theme icon on every style; a bare
        // ToolButton icon renders nothing under some desktop styles.
        contentItem: RowLayout {
            spacing: Kirigami.Units.smallSpacing
            Kirigami.Icon {
                objectName: "mailBarIcon"
                source: barButton.icon.name
                isMask: true
                color: root.ink
                Layout.alignment: Qt.AlignCenter
                Layout.minimumWidth: 24
                Layout.maximumWidth: 24
                Layout.minimumHeight: 24
                Layout.maximumHeight: 24
            }
            QQC2.Label {
                visible: barButton.showLabel
                text: barButton.text
                color: root.ink
                font: Kirigami.Theme.defaultFont
            }
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
        const note = String(c.note || "")
        function paragraphs(value) {
            const linked = escapeHtml(value).replace(/(https?:\/\/[^\s<]+)/g, '<a href="$1">$1</a>')
            return linked.split(/\n{2,}/).map(function(part) { return "<p>" + part.replace(/\n/g, "<br>") + "</p>" }).join("")
        }
        return (text ? paragraphs(text) : "") + (note ? "<section><h2>" + qsTr("Note") + "</h2>" + paragraphs(note) + "</section>" : "")
    }
    readonly property string headerHtml: "<header style='padding:4px 0 16px'><h1 style='font-size:1.4em;line-height:1.3;margin:0 0 8px'>"
        + escapeHtml(root.cardTitle || root.title) + "</h1>"
        + (root.accountName ? "<div style='color:#586d70'>" + escapeHtml(root.accountName) + "</div>" : "")
        + "</header>"
    function threadHtml() {
        if (!messages.length || legacyIncomplete)
            return "<div class='lb-latest'>" + escapeHtml(arrivalLine()) + "</div><section class='lb-original'>" + html + "</section>"
        let result = ""
        for (let i = 0; i < messages.length; ++i) {
            const message = messages[i]
            const senderKnown = String(message.sender_name || message.sender_address || "").trim() !== ""
            const arrival = arrivalLine(message)
            const line = senderKnown ? arrival : qsTr("Earlier message") + (arrival ? " · " + arrival : "")
            const key = message.gmail_message_id || message.id || ("index:" + i)
            const expanded = initialUnread[key] || i === messages.length - 1
            const preview = String(message.preview || "")
            const encodedKey = encodeURIComponent(String(key)).replace(/'/g, "%27")
            result += "<details class='lb-previous' data-lb-key='" + encodedKey + "'" + (expanded ? " open" : "") + "><summary>" + escapeHtml(line)
                + (preview ? " · " + escapeHtml(preview) : "") + "</summary>" + messageMetadata(message) + invitationHtml(message) + "<section class='lb-original'>"
                + String(message.html || "") + "</section></details>"
        }
        return result
    }
    // Native <details> works without scripting in both Chromium renderers.
    // Originals remain intact in one document, with no nested browser/vertical scroll.
    readonly property string documentHtml: !root.hasDocument ? "" : "<main class='lb-reader'>" + headerHtml + threadHtml() + "</main>"
        + "<style>.lb-reader{max-width:880px;margin:0 auto;padding:16px 24px 40px}"
        + ".lb-previous{border-top:1px solid #dce5e3}.lb-previous>summary{cursor:pointer;padding:14px 4px;min-height:48px;color:#586d70;font:inherit;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;box-sizing:border-box}"
        + ".lb-previous>summary:focus-visible{outline:2px solid #397d73;outline-offset:-2px}"
        + ".lb-recipients>summary{min-height:48px;padding:12px 4px;cursor:pointer}.lb-file a{display:inline-block;min-width:48px;min-height:48px;padding:12px 8px;box-sizing:border-box}"
        + ".lb-original{padding:8px 0 16px}.lb-latest{border-top:1px solid #dce5e3;padding-top:16px;color:#586d70}"
        + "@media(max-width:600px){.lb-reader{padding:12px 16px 32px}}</style>"

    padding: 0
    ColumnLayout {
        id: messageColumn
        anchors.fill: parent
        spacing: 0

        Rectangle {
            objectName: "mailAppBar"
            z: 1
            Layout.fillWidth: true
            implicitHeight: root.compactActions ? 56 : Math.max(56, actionRow.implicitHeight + 8)
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
                id: actionRow
                anchors.fill: parent
                anchors.leftMargin: 4
                anchors.rightMargin: 4
                spacing: 0
                BarButton {
                    objectName: "backToInbox"
                    text: qsTr("Back to inbox")
                    icon.name: "go-previous"
                    onClicked: root.closeReader()
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.minimumWidth: 0
                    Layout.leftMargin: 4
                    spacing: 0
                    QQC2.Label {
                        id: barTitle
                        objectName: "mailBarTitle"
                        text: root.cardTitle || root.title
                        textFormat: Text.PlainText
                        color: root.ink
                        font.weight: Font.DemiBold
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                    }
                    QQC2.Label {
                        objectName: "mailArrival"
                        text: root.arrivalLine()
                        textFormat: Text.PlainText
                        color: root.mutedInk
                        font.pointSize: Kirigami.Theme.defaultFont.pointSize - 2
                        // Keep a single slim row even at large platform text scale;
                        // full sender/arrival metadata remains in the document.
                        visible: text !== "" && (!root.compactActions || implicitHeight + barTitle.implicitHeight <= 48)
                        elide: Text.ElideRight
                        maximumLineCount: 1
                        Layout.fillWidth: true
                        Layout.minimumWidth: 0
                    }
                }
                BarButton {
                    id: archiveButton
                    objectName: "archiveMail"
                    showLabel: !root.compactActions
                    text: mailActions.source === "mail" ? qsTr("Archive") : mailActions.source === "journal" ? qsTr("Archive note") : mailActions.source === "manual" ? qsTr("Complete") : qsTr("Dismiss")
                    icon.name: mailActions.source === "mail" || mailActions.source === "journal" ? "mail-archive" : "dialog-ok"
                    Accessible.name: mailActions.primaryName
                    onClicked: mailActions.primaryAction()
                    Shortcut {
                        sequence: "E"
                        enabled: root.visible && (root.embeddedMode || root.QQC2.StackView.status === QQC2.StackView.Active) && !root.actualPopupsVisible
                        onActivated: mailActions.primaryAction()
                    }
                }
                BarButton {
                    objectName: "openInGmail"
                    showLabel: !root.compactActions
                    text: mailActions.source === "mail" ? qsTr("Open in Gmail") : qsTr("Open source")
                    icon.name: "document-open"
                    visible: root.sourceUrl !== ""
                    onClicked: root.openInGmail()
                }
                Litterbox.CardActions {
                    id: mailActions
                    objectName: "mailActions"
                    implicitWidth: root.tapTarget
                    implicitHeight: root.tapTarget
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
                        // Capture callbacks before closing: standalone pop may destroy this page.
                        const undo = root.requestArchiveUndo
                        // Only a local Archive continues the reader. The next presented
                        // card is captured before the action's deferred rebuild drops this
                        // one, then opened after close and Undo; Undo never rewinds the
                        // surviving reader back to the archived card.
                        const next = action === "archive" && root.requestArchiveContinuation
                            ? root.requestArchiveContinuation(root.cardId) : null
                        root.closeReader()
                        if (operation && undo) undo(operation, action)
                        if (next) next()
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

        QQC2.Label {
            visible: root.attachmentError !== ""
            text: root.attachmentError
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        Item {
            objectName: "messageDocument"
            visible: root.hasDocument || root.loading
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
                // Android's native WebView is above the Qt scene. Hide it while
                // a real Qt popup is open, without unloading/resetting its document.
                visible: Qt.platform.os !== "android" || !root.actualPopupsVisible
                onLoaded: item.html = Qt.binding(function() { return root.documentHtml })
                Connections {
                    target: bodyLoader.item
                    function onLinkActivated(link) {
                        if (link.indexOf("litterbox-file:") === 0) {
                            const parts = link.substring(15).split("/")
                            root.store.requestMailFile(root.cardId, decodeURIComponent(parts.slice(1).join("/")), parts[0] === "save")
                        } else if (link.indexOf("data:") === 0) {
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
        Item { Layout.fillHeight: true; visible: !root.hasDocument && !root.loading }
    }
}
