import QtQuick
import QtQml.Models
import QtQuick.Controls
import QtQuick.Controls.Material
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import litterbox 1.0 as Litterbox
import "BundlePresentation.js" as BundlePresentation

ApplicationWindow {
    id: window
    visible: true
    width: Qt.platform.os === "android" ? 520 : 1280
    height: Qt.platform.os === "android" ? 800 : 900
    title: "Litterbox"
    // Sol V5 owns a light content surface, independent of the host's color scheme.
    // Set both Controls and Kirigami colors: neither may inherit dark-theme ink
    // while the other paints the approved white cards and header.
    // Read the platform font once: Kirigami.Theme.defaultFont inside a row
    // creates a Theme attached object per item, ~0.2 ms each, which made
    // every new dense inbox row expensive (scroll collapse regression).
    readonly property font themeFont: Kirigami.Theme.defaultFont
    readonly property color themeHighlight: Kirigami.Theme.highlightColor
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    readonly property bool desktopNavigation: width >= 900
    readonly property bool desktopReader: width >= Math.max(1220, Kirigami.Units.gridUnit * 70)
    readonly property bool compactInbox: inboxList.width < 640
    property string selectedCardId: ""
    property bool readerOpen: false
    property bool relocatingReader: false
    onDesktopReaderChanged: Qt.callLater(placeReader)
    property bool compactDensity: true
    property bool searchShown: false
    property string searchQuery: ""
    readonly property bool searching: searchQuery.trim().length > 0
    property bool wasSearching: false
    property var searchAnchor: null
    property real searchScrollPosition: 0
    property var searchCollapsedBundles: ({})
    property bool inboxReady: false
    property var dateStarts: ({})
    readonly property string activeLocation: {
        const name = stack.currentItem ? stack.currentItem.objectName : ""
        if (name === "gmailAccountsPage") return qsTr("Accounts")
        if (name === "agentThreadsPage" || name === "agentChatPage") return qsTr("Agents")
        if (name === "settingsPage") return qsTr("Settings")
        return stack.depth === 1 || stack.currentItem === mailDetailPage ? qsTr("Inbox") : (stack.currentItem.title || qsTr("Inbox"))
    }
    FontMetrics { id: inboxFontMetrics; font: window.themeFont }
    readonly property bool largeText: inboxFontMetrics.height > 24
    onCompactDensityChanged: {
        if (!inboxReady || !bundleModel) return
        const anchor = bundleModel.captureAnchor()
        Qt.callLater(function() {
            if (bundleModel) bundleModel.restoreAnchor(anchor, bundleModel.rows)
        })
    }
    // Let the derived searching binding settle before filtering or restoring its anchor.
    onSearchQueryChanged: Qt.callLater(applyInboxSearch)
    function applyInboxSearch() {
        if (!inboxReady || !bundleModel) return
        if (searching && !wasSearching) {
            searchAnchor = bundleModel.captureAnchor()
            searchScrollPosition = inboxList.contentY - inboxList.originY
        }
        searchCollapsedBundles = ({})
        bundleModel.rebuildPresentation(false)
        if (!searching && wasSearching) {
            if (!bundleModel.restoreAnchor(searchAnchor, bundleModel.rows))
                inboxList.contentY = inboxList.originY + searchScrollPosition
        } else if (searching) inboxList.positionViewAtBeginning()
        wasSearching = searching
    }
    function matchesSearch(entry) {
        const card = entry.card
        const text = [entry.title, card.snippet, card.summary, card.note, card.sender_name,
            card.sender_address, card.account_name, cardKindLabel(card), card.bundle_id ? bundleTitle(card) : ""].join(" ").toLocaleLowerCase()
        return searchQuery.trim().toLocaleLowerCase().split(/\s+/).every(word => text.indexOf(word) >= 0)
    }
    function dateLandmark(card) {
        const stamp = card.received_at || (card.timed ? card.at : "")
        if (!stamp) return ""
        const date = new Date(stamp)
        if (isNaN(date.getTime())) return ""
        const today = new Date()
        today.setHours(0, 0, 0, 0)
        const yesterday = new Date(today)
        yesterday.setDate(yesterday.getDate() - 1)
        if (date >= today) return date < new Date(today.getFullYear(), today.getMonth(), today.getDate() + 1) ? qsTr("Today") : qsTr("Upcoming")
        return date >= yesterday ? qsTr("Yesterday") : qsTr("Earlier")
    }
    function navigateTo(name) {
        navigationDrawer.close()
        if (stack.depth > 1) {
            if (stack.currentItem === mailDetailPage) readerOpen = false
            stack.pop(null, StackView.Immediate)
        }
        if (name) openPage(name)
        else inboxList.forceActiveFocus()
    }
    component DestinationButton: ToolButton {
        required property string destination
        required property string location
        text: location
        checked: window.activeLocation === location
        checkable: true
        display: AbstractButton.TextBesideIcon
        implicitHeight: Math.max(48, implicitContentHeight + topPadding + bottomPadding)
        Accessible.name: location
        ToolTip.text: location
        ToolTip.visible: hovered || visualFocus
        background: Rectangle {
            radius: 8
            color: parent.checked ? "#dcece7" : parent.hovered ? "#eaf2ef" : "transparent"
            border.width: parent.visualFocus ? 2 : 0
            border.color: window.accent
        }
        onClicked: window.navigateTo(destination)
    }
    component Destinations: ColumnLayout {
        spacing: 4
        DestinationButton { objectName: "navInbox"; destination: ""; location: qsTr("Inbox"); icon.name: "mail-receive"; Layout.fillWidth: true }
        DestinationButton { objectName: "navAccounts"; destination: "GmailAccountsPage"; location: qsTr("Accounts"); icon.name: "mail-receive"; Layout.fillWidth: true }
        DestinationButton { objectName: "navAgents"; destination: "AgentThreadsPage"; location: qsTr("Agents"); icon.name: "user-identity"; Layout.fillWidth: true }
        DestinationButton { objectName: "navSettings"; destination: "SettingsPage"; location: qsTr("Settings"); icon.name: "application-menu"; Layout.fillWidth: true }
    }
    readonly property real edgeSpacing: compactInbox ? 8 : Math.max(18, Kirigami.Units.largeSpacing)
    readonly property int actionIconSize: 20
    // R65: Android "remove animations" / tests can switch motion off.
    property bool reduceMotion: typeof platformMotion !== "undefined" && platformMotion.reduced
    // R63: thin margins and gaps on phones.
    readonly property real cardGap: compactDensity ? 4 : 12
    // R67: stable initials avatar per sender; server images can replace it later.
    readonly property var avatarPalette: ["#397d73", "#5b6abf", "#b0592f", "#7a5aa6", "#2f7fa8", "#9a6a12", "#a3465f", "#4f7d2f"]
    function avatarColor(name) {
        let hash = 0
        const text = String(name || "?").toLowerCase()
        for (let i = 0; i < text.length; ++i) hash = (hash * 31 + text.charCodeAt(i)) >>> 0
        return avatarPalette[hash % avatarPalette.length]
    }
    function avatarInitials(name) {
        const words = String(name || "").replace(/\[bot\]|[<>"@].*$/gi, "").replace(/[^\p{L}\p{N}\s]/gu, " ").trim().split(/\s+/).filter(w => w.length)
        if (!words.length) return "?"
        return (words[0][0] + (words.length > 1 ? words[words.length - 1][0] : "")).toUpperCase()
    }
    // R63 item 13: mail preheaders often start with invisible or unrenderable
    // characters (zero-width, combining grapheme joiner, BOM, private use,
    // variation selectors, pictographs the phone font lacks).
    function cleanPreview(text) {
        // R63: drop leading invisible, private-use, variation-selector and emoji
        // characters. Spelled with surrogate pairs: the QML engine has no \p{}.
        return String(text || "").replace(/^(?:[\s\u00ad\u034f\u061c\u115f\u1160\u17b4\u17b5\u180b-\u180f\u200b-\u200f\u202a-\u202e\u2060-\u206f\u2190-\u21ff\u2300-\u23ff\u2460-\u27bf\u2900-\u297f\u2b00-\u2bff\u3164\ufe00-\ufe0f\ufeff\uffa0\ue000-\uf8ff]|\ud83c[\udc00-\udfff]|\ud83d[\udc00-\udfff]|\ud83e[\udc00-\udfff]|\udb40[\udc00-\udc7f]|[\udb80-\udbff][\udc00-\udfff])+/, "")
            // R66: agent report boilerplate labels are not the preview.
            .replace(/^(?:\**\s*(?:What happened|Summary|Result|Итог|Что сделано)\s*:\s*\**\s*)/i, "")
    }
    component SenderAvatar: Rectangle {
        property string name: ""
        width: 32
        height: 32
        radius: width / 2
        color: window.avatarColor(name)
        Label {
            anchors.centerIn: parent
            text: window.avatarInitials(parent.name)
            color: "#ffffff"
            font.pointSize: Math.max(7, window.themeFont.pointSize - 2)
            font.weight: Font.DemiBold
        }
    }
    component InboxMenuItem: MenuItem {
        id: entry
        implicitWidth: Math.max(200, contentItem.implicitWidth + leftPadding + rightPadding)
        implicitHeight: visible ? Math.max(48, contentItem.implicitHeight + 16) : 0
        height: implicitHeight
        leftPadding: 12
        rightPadding: 12
        topPadding: 8
        bottomPadding: 8
        icon.width: window.actionIconSize
        icon.height: window.actionIconSize
        contentItem: RowLayout {
            spacing: 12
            Kirigami.Icon {
                source: entry.icon.name
                visible: source.length > 0
                color: window.ink
                Layout.preferredWidth: window.actionIconSize
                Layout.preferredHeight: window.actionIconSize
            }
            Label {
                text: entry.text
                color: entry.enabled ? window.ink : window.mutedInk
                font: window.themeFont
                elide: Text.ElideRight
                Layout.fillWidth: true
                verticalAlignment: Text.AlignVCenter
            }
        }
        background: Rectangle {
            radius: 4
            color: entry.highlighted || entry.down ? "#e7f1ee" : "transparent"
            border.width: entry.visualFocus ? 1 : 0
            border.color: window.accent
        }
    }
    color: canvas
    Material.theme: Material.Light
    Material.background: canvas
    Material.foreground: ink
    Material.accent: accent
    Material.primary: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.alternateBase: canvas
    palette.text: ink
    palette.button: surface
    palette.buttonText: ink
    palette.toolTipBase: surface
    palette.toolTipText: ink
    palette.highlight: accent
    palette.highlightedText: surface
    palette.placeholderText: mutedInk
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: surface
    Kirigami.Theme.alternateBackgroundColor: canvas
    Kirigami.Theme.highlightColor: accent
    Kirigami.Theme.highlightedTextColor: surface
    Kirigami.Theme.focusColor: accent
    Kirigami.Theme.hoverColor: accent
    required property var api
    required property var updater
    required property var store
    required property var timeRules
    property var cardStore: store
    property var gmailAccounts: []
    readonly property string reconnectAddress: gmailAccounts.filter(account => account.status === "reauth_required").map(account => account.address).join(", ")
    readonly property string reconnectStatus: reconnectAddress ? qsTr("Gmail access for %1 expired. Reconnect it in Accounts → Connect account…").arg(reconnectAddress) : ""
    function reloadGmailAccounts() {
        if (stack.currentItem && stack.currentItem.objectName === "gmailAccountsPage") {
            stack.currentItem.reload()
            return
        }
        if (!api || typeof api.get !== "function") return
        api.get("/v1/gmail/accounts", function(error, response) {
            if (!error && response && Array.isArray(response.body)) gmailAccounts = response.body
        })
    }
    Component.onCompleted: { inboxReady = true; reloadGmailAccounts() }
    onActiveChanged: if (active) reloadGmailAccounts()
    property var captureActions: ({})
    property var cardHandles: ({})
    property string dragCardId: ""
    property int dragTargetIndex: -1
    property real dragOffset: 0
    property string dragFeedback: ""
    property var expandedBundles: ({})
    property var bundleSections: ({})
    property var sectionStarts: ({})
    property var bundleSummaries: ({})
    property var bundleOpeners: ({})
    function bundleTitle(card) {
        return BundlePresentation.title(card, bundleSummaries[card.bundle_id])
    }
    function mailSender(card) {
        const name = String(card.sender_name || "").trim()
        return name || String(card.sender_address || "").trim()
    }
    function mailArrival(card) {
        if (!card.received_at) return ""
        const arrived = new Date(card.received_at)
        if (isNaN(arrived.getTime())) return ""
        const now = new Date()
        if (arrived.getFullYear() === now.getFullYear() && arrived.getMonth() === now.getMonth() && arrived.getDate() === now.getDate())
            return Qt.formatDateTime(arrived, "HH:mm")
        if (arrived.getFullYear() === now.getFullYear())
            return Qt.formatDateTime(arrived, "d MMM HH:mm")
        return Qt.formatDateTime(arrived, "d MMM yyyy HH:mm")
    }
    function cardKindLabel(card) {
        const kind = String(card.source_kind || card.source || "")
        const labels = {
            manual: qsTr("Task"), journal: qsTr("Journal"), todo: qsTr("To-do"),
            reminder: qsTr("Reminder"), meeting: qsTr("Meeting"), research_result: qsTr("Agent"),
            agent: qsTr("Agent"), home_assistant: qsTr("Home Assistant"), dating_app: qsTr("Dating")
        }
        return labels[kind] || (kind ? kind.charAt(0).toUpperCase() + kind.slice(1) : "")
    }
    function toggleBundle(bundleId) {
        const opener = bundleOpeners[bundleId]
        const headerY = opener ? opener.mapToItem(inboxList, 0, 0).y : 0
        const next = Object.assign({}, searching ? searchCollapsedBundles : expandedBundles)
        next[bundleId] = !next[bundleId]
        if (searching) searchCollapsedBundles = next
        else expandedBundles = next
        bundleModel.rebuildPresentation(false)
        // Layouts polish after callLater; anchor against their rendered geometry.
        window.frameSwapped.connect(function restoreHeader() {
            window.frameSwapped.disconnect(restoreHeader)
            const currentOpener = bundleOpeners[bundleId]
            if (currentOpener) {
                inboxList.contentY += currentOpener.mapToItem(inboxList, 0, 0).y - headerY
                currentOpener.forceActiveFocus()
            }
        })
    }
    function emailCountText(count) {
        const value = Number(count)
        return qsTr("%1 %2").arg(value).arg(value === 1 ? qsTr("email") : qsTr("emails"))
    }
    function archiveBundle(bundleId) {
        const result = store.archiveBundleNow(bundleId)
        if (!result || !result.token || Number(result.count) < 1) return false
        bundleArchiveToken = result.token
        mailArchiveUndoOp = ""
        archiveUndoAvailable = true
        bundleArchiveStatus = Number(result.count) === 1 ? qsTr("Archived") : qsTr("Archived %1").arg(emailCountText(result.count))
        return true
    }
    function showArchiveUndo(opId, action) {
        mailArchiveUndoOp = action === "spam" ? opId : ""
        archiveUndoAvailable = true
        bundleArchiveToken = action === "archive" ? opId : ""
        bundleArchiveStatus = action === "spam" ? qsTr("Marked as spam") : qsTr("Archived")
    }
    function showOperationFailure(opId, status, response) {
        if (opId === mailArchiveUndoOp) {
            mailArchiveUndoOp = ""
            archiveUndoAvailable = false
        }
        // Reconnect state belongs exclusively to the current account list.
        if (response && response.code === "gmail_reconnect_required") {
            if (!archiveUndoAvailable) bundleArchiveStatus = ""
            reloadGmailAccounts()
        } else {
            bundleArchiveStatus = qsTr("Action failed (HTTP %1).").arg(status)
        }
    }
    function undoBundleArchive() {
        if (mailArchiveUndoOp.length > 0) {
            const op = mailArchiveUndoOp
            mailArchiveUndoOp = ""
            store.undoQueuedOp(op)
            archiveUndoAvailable = false
            bundleArchiveStatus = ""
            return true
        }
        const restored = bundleArchiveToken.length > 0 && store.undoBundleArchive(bundleArchiveToken)
        bundleArchiveToken = ""
        archiveUndoAvailable = false
        if (restored) {
            bundleArchiveStatus = ""
        } else {
            bundleArchiveStatus = qsTr("Undo has expired.")
            undoExpiredNoticeTimer.restart()
        }
        return restored
    }
    property string updateStatus: ""
    property string updateVersion: ""
    property string bundleArchiveToken: ""
    property string mailArchiveUndoOp: ""
    property string bundleArchiveStatus: ""
    property bool archiveUndoAvailable: false
    Connections {
        target: store
        ignoreUnknownSignals: true
        function onBundleArchiveExpired(token) {
            if (token !== window.bundleArchiveToken) return
            window.bundleArchiveToken = ""
            window.archiveUndoAvailable = false
            window.bundleArchiveStatus = qsTr("Undo has expired.")
            undoExpiredNoticeTimer.restart()
        }
        function onOperationFailed(opId, status, response) { window.showOperationFailure(opId, status, response) }
        function onRequestCards(path) { window.reloadGmailAccounts() }
        function onFlushFinished() { window.reloadGmailAccounts() }
    }
    Timer {
        id: undoExpiredNoticeTimer
        interval: 2200
        repeat: false
        onTriggered: if (window.bundleArchiveStatus === qsTr("Undo has expired.")) window.bundleArchiveStatus = ""
    }


    // Drop the dragged card at the row the reorder handle was released over.
    function commitCardDrag(cardId, targetIndex) {
        const sourceIndex = store.cardIds().indexOf(cardIdAt(targetIndex))
        return sourceIndex >= 0 && store.moveCardTo(cardId, sourceIndex)
    }
    function cardIdAt(row) {
        return row >= 0 && row < bundleModel.rows.length ? bundleModel.rows[row].cardId : ""
    }

    // The card-controls capture opens the real dialog, records a frame while it is
    // rendered open, and only then accepts it, so the visible dialog itself is
    // evidence rather than just its after-effect.
    function captureOpenCreateDialog(title, summary) {
        createTitle.text = title
        createSummary.text = summary
        createDialog.open()
        return createDialog.visible
    }
    function captureAcceptCreateDialog() {
        const oldCount = store.cardIds().length
        createDialog.accept()
        createDialog.visible = false
        return store.cardIds().length === oldCount + 1
    }
    function captureCloseCreateDialog() { createDialog.close() }
    function captureNoteDialogCardId() { return noteDialog.visible ? noteDialog.cardId : "" }
    function captureCloseNoteDialog() {
        noteDialog.reject()
        noteDialog.visible = false
    }
    function captureCardIds() { return store.cardIds() }
    function captureOpenActions(cardId) {
        const action = captureActions[cardId]
        if (!action) return false
        action.clicked()
        return true
    }
    function captureSnooze(cardId, localDateTimeValue) {
        const action = captureActions[cardId]
        if (!action) return "missing-card-action"
        return action.captureSnooze(localDateTimeValue) ? "accepted" : "rejected"
    }
    function capturePinMoveUp(cardId) {
        const action = captureActions[cardId]
        if (!action) return "missing-card-action"
        if (!action.pinned) return "not-pinned"
        action.movePin(-1)
        return store.pinnedCardIds()[0] === cardId ? "moved" : "unchanged"
    }
    onClosing: function(close) {
        close.accepted = !(Qt.platform.os === "android" && handleBack())
    }
    // Real popup state: only these named dialogs/menu count. The Overlay item
    // itself exists and stays visible with no popup open, so its visible flag
    // must never proxy for "a popup is open" (it hid the Android native body).
    readonly property bool actualPopupsVisible: !!((navigationDrawer && navigationDrawer.visible) || (headerMenu && headerMenu.visible)
        || (noteDialog && noteDialog.visible) || (createDialog && createDialog.visible))
    function handleBack() {
        if (navigationDrawer.visible) { navigationDrawer.close(); return true }
        if (headerMenu.visible) { headerMenu.close(); return true }
        if (noteDialog.visible) { noteDialog.reject(); return true }
        if (createDialog.visible) { createDialog.reject(); return true }
        // The overlay item also exists without a popup; it must not swallow Back.
        if (stack.depth > 1) {
            if (stack.currentItem === mailDetailPage) closeReader()
            else stack.pop()
            return true
        }
        if (readerOpen) {
            closeReader()
            return true
        }
        if (searching || (searchShown && !desktopNavigation)) {
            searchQuery = ""
            inboxSearch.clear()
            searchShown = false
            inboxList.forceActiveFocus()
            return true
        }
        return false
    }
    function closeReader() {
        if (!readerOpen && (!mailDetailPage || !mailDetailPage.visible)) return
        saveInboxScroll(selectedCardId)
        readerOpen = false
        if (stack.currentItem === mailDetailPage && stack.depth > 1) stack.pop()
        else if (mailDetailPage) mailDetailPage.visible = false
        inboxList.forceActiveFocus()
    }
    // Move the same reader, not a second document, across the breakpoint.
    // StackView never owns this explicitly created item, so pop cannot destroy it.
    function placeReader() {
        if (!mailDetailPage || !readerOpen || relocatingReader) return
        relocatingReader = true
        if (desktopReader) {
            if (stack.currentItem === mailDetailPage) stack.pop(StackView.Immediate)
            mailDetailPage.parent = readerHost
            mailDetailPage.embeddedMode = true
            mailDetailPage.anchors.fill = readerHost
            mailDetailPage.visible = true
        } else if (stack.currentItem !== mailDetailPage) {
            saveInboxScroll(selectedCardId)
            mailDetailPage.anchors.fill = null
            mailDetailPage.parent = stack
            mailDetailPage.embeddedMode = false
            stack.push(mailDetailPage, StackView.Immediate)
        }
        relocatingReader = false
    }
    property real inboxScrollPosition: 0
    // Raw contentY drifts a few px when Back re-runs layout (rows re-estimated
    // after delegates were culled). Anchor on the opened card's measured
    // viewport y instead and correct contentY to restore that measurement.
    property var inboxScrollAnchorId: ""
    property real inboxScrollAnchorY: 0
    function saveInboxScroll(cardId) {
        inboxScrollPosition = inboxList.contentY - inboxList.originY
        inboxScrollAnchorId = ""
        for (let row = 0; row < inboxList.count; ++row) {
            if (cardIdAt(row) !== cardId) continue
            const item = inboxList.itemAtIndex(row)
            if (!item) break
            inboxScrollAnchorId = cardId
            inboxScrollAnchorY = item.mapToItem(inboxList, 0, 0).y
            break
        }
        // Desktop readers do not lock the list to the selected card. When it
        // has scrolled away, remember the current visible anchor for phone Back.
        if (!inboxScrollAnchorId) {
            const anchor = bundleModel.captureAnchor()
            if (anchor) {
                inboxScrollAnchorId = anchor.ids[0]
                inboxScrollAnchorY = anchor.offset
            }
        }
    }
    function restoreInboxScroll() {
        inboxList.forceLayout()
        inboxList.contentY = inboxList.originY + inboxScrollPosition
        if (!inboxScrollAnchorId) return
        for (let row = 0; row < inboxList.count; ++row) {
            if (cardIdAt(row) !== inboxScrollAnchorId) continue
            const item = inboxList.itemAtIndex(row)
            if (!item) break
            for (let pass = 0; pass < 3; ++pass) {
                const delta = item.mapToItem(inboxList, 0, 0).y - inboxScrollAnchorY
                if (delta === 0) break
                inboxList.contentY += delta
            }
            break
        }
    }
    property var mailDetailPage: null
    function openPage(name, properties) {
        if (stack.depth === 1) saveInboxScroll((properties || {}).cardId || "")
        const page = pagesDir.toString() + name + ".qml"
        if (name === "MailDetailPage") {
            const values = Object.assign({store: window.cardStore, cardTitle: "", accountName: "", openLinks: true}, properties || {}, {
                embeddedMode: desktopReader,
                requestClose: function() { window.closeReader() },
                // The retained reader follows the window's real popup state so
                // its Android WebView hides only while a real dialog/menu is open.
                ownerPopupVisible: Qt.binding(function() { return window.actualPopupsVisible })
            })
            selectedCardId = values.cardId || ""
            if (!mailDetailPage) {
                // An existing item is not owned/destroyed by StackView on pop.
                const component = Qt.createComponent(page)
                mailDetailPage = component.createObject(stack, values)
            } else {
                for (const key of Object.keys(values)) mailDetailPage[key] = values[key]
                mailDetailPage.reload()
            }
            if (!mailDetailPage) return
            readerOpen = true
            placeReader()
        } else {
            if (readerOpen) closeReader()
            const values = Object.assign({api: api}, properties || {})
            if (name === "GmailAccountsPage") values.accountsLoaded = function(accounts) { window.gmailAccounts = accounts }
            if (name === "SettingsPage") values.updater = window.updater
            stack.push(page, values)
        }
    }
    function clock(card) { return card.timed ? timeRules.display(card.at || "") : "" }

    // R55: online is silent; offline is one actionable line.
    property double lastSyncAt: store.online ? Date.now() : 0
    property bool offlineDetailsShown: false
    Connections {
        target: store
        ignoreUnknownSignals: true
        function onOnlineChanged() {
            if (store.online) { window.lastSyncAt = Date.now(); window.offlineDetailsShown = false }
        }
    }
    function offlineSummary() {
        let text = qsTr("Can't reach the server")
        if (store.pendingOps > 0)
            text += " · " + (store.pendingOps === 1 ? qsTr("1 change waiting") : qsTr("%1 changes waiting").arg(store.pendingOps))
        if (window.lastSyncAt > 0)
            text += " · " + qsTr("synced %1").arg(Qt.formatTime(new Date(window.lastSyncAt), "HH:mm"))
        return text
    }

    // R63: one slim app bar; the reader (R59) brings its own bar instead.
    header: ToolBar {
        visible: !(stack.depth > 1 && stack.currentItem === window.mailDetailPage)
        padding: 0
        leftPadding: window.edgeSpacing
        rightPadding: window.edgeSpacing
        background: Rectangle {
            color: window.surface
            Rectangle { anchors.bottom: parent.bottom; width: parent.width; height: 1; color: "#e0e9e6" }
        }
        ColumnLayout {
            width: parent.width
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 0
            RowLayout {
                spacing: 0
                Layout.fillWidth: true
                Layout.preferredHeight: 52
                ToolButton {
                    objectName: "openNavigation"
                    visible: !window.desktopNavigation
                    text: qsTr("Navigation")
                    icon.name: "application-menu"
                    display: AbstractButton.IconOnly
                    implicitWidth: 48
                    implicitHeight: 48
                    Accessible.name: text
                    ToolTip.text: text
                    ToolTip.visible: hovered || visualFocus
                    onClicked: navigationDrawer.open()
                }
                Kirigami.Heading { text: window.activeLocation; color: window.ink; level: 3; Layout.fillWidth: true; elide: Text.ElideRight }
                ToolButton {
                    objectName: "toggleSearch"
                    visible: !window.desktopNavigation && stack.depth === 1
                    text: qsTr("Search inbox")
                    icon.name: "edit-find"
                    display: AbstractButton.IconOnly
                    implicitWidth: 48
                    implicitHeight: 48
                    Accessible.name: text
                    ToolTip.text: text
                    ToolTip.visible: hovered || visualFocus
                    onClicked: { window.searchShown = !window.searchShown; if (window.searchShown) inboxSearch.forceActiveFocus() }
                }
                ToolButton {
                    id: addCardButton
                    objectName: "addCardButton"
                    text: qsTr("Add card")
                    icon.name: "list-add"
                    icon.width: 24
                    icon.height: 24
                    icon.color: window.accent
                    display: window.compactInbox ? AbstractButton.IconOnly : AbstractButton.TextBesideIcon
                    implicitHeight: 48
                    implicitWidth: window.compactInbox ? 48 : implicitContentWidth + leftPadding + rightPadding
                    Material.foreground: window.accent
                    palette.buttonText: window.accent
                    Accessible.name: text
                    ToolTip.text: text
                    ToolTip.visible: hovered
                    background: Rectangle {
                        radius: 24
                        color: addCardButton.down ? "#e7f1ee" : addCardButton.hovered ? "#f3f7f6" : "transparent"
                        border.width: addCardButton.visualFocus ? 2 : 0
                        border.color: window.ink
                    }
                    onClicked: createDialog.open()
                    TapHandler { onTapped: createDialog.open() }
                }
                ToolButton {
                    objectName: "inboxCommands"
                    icon.name: "application-menu"
                    icon.width: 24
                    icon.height: 24
                    text: qsTr("Inbox commands")
                    display: AbstractButton.IconOnly
                    implicitWidth: 48
                    implicitHeight: 48
                    Accessible.name: qsTr("Inbox commands")
                    ToolTip.text: Accessible.name
                    onClicked: headerMenu.open()
                    Menu {
                        id: headerMenu
                        objectName: "headerMenu"
                        x: parent.width - width
                        y: parent.height + 8
                        padding: 6
                        Material.theme: Material.Light
                        palette.window: window.surface
                        palette.text: window.ink
                        palette.buttonText: window.ink
                        background: Rectangle {
                            implicitWidth: 220
                            color: window.surface
                            radius: Kirigami.Units.cornerRadius
                            border.color: "#dce5e3"
                        }
                        InboxMenuItem { objectName: "headerMenuAccounts"; text: qsTr("Accounts"); icon.name: "mail-receive"; onTriggered: openPage("GmailAccountsPage") }
                        InboxMenuItem { objectName: "headerMenuAgents"; text: qsTr("Agents"); icon.name: "user-identity"; onTriggered: openPage("AgentThreadsPage") }
                        InboxMenuItem { objectName: "headerMenuSettings"; text: qsTr("Settings"); icon.name: "application-menu"; onTriggered: openPage("SettingsPage") }
                        InboxMenuItem { objectName: "headerMenuEnroll"; text: qsTr("Enroll device"); icon.name: "user-identity"; visible: !api.token; onTriggered: openPage("EnrollmentPage") }
                        InboxMenuItem { objectName: "headerMenuRefresh"; text: qsTr("Refresh"); icon.name: "view-refresh"; onTriggered: store.refresh() }
                        InboxMenuItem { objectName: "headerMenuUpdates"; text: qsTr("Check updates"); icon.name: "system-software-update"; visible: updater.supported; enabled: !updater.busy; onTriggered: updater.checkForUpdates() }
                        InboxMenuItem { text: qsTr("Comfortable density"); checkable: true; checked: !window.compactDensity; onTriggered: window.compactDensity = false }
                        InboxMenuItem { text: qsTr("Compact density"); checkable: true; checked: window.compactDensity; onTriggered: window.compactDensity = true }
                    }
                }
            }
            RowLayout {
                objectName: "inboxSearchRow"
                visible: stack.depth === 1 && (window.desktopNavigation || window.searchShown || window.searching)
                Layout.fillWidth: true
                spacing: 8
                Label { text: qsTr("Search inbox"); visible: window.desktopNavigation; color: window.mutedInk }
                TextField {
                    id: inboxSearch
                    objectName: "inboxSearch"
                    Layout.minimumHeight: 48
                    Layout.fillWidth: true
                    implicitHeight: Math.max(48, contentHeight + topPadding + bottomPadding)
                    placeholderText: qsTr("Search inbox")
                    Accessible.name: qsTr("Search inbox")
                    text: window.searchQuery
                    onTextEdited: window.searchQuery = text
                    selectByMouse: true
                }
                ToolButton {
                    objectName: "clearInboxSearch"
                    text: qsTr("Clear search")
                    icon.name: "edit-clear"
                    display: AbstractButton.IconOnly
                    implicitWidth: 48
                    implicitHeight: 48
                    Layout.minimumHeight: 48
                    enabled: window.searching
                    Accessible.name: text
                    ToolTip.text: text
                    ToolTip.visible: hovered || visualFocus
                    onClicked: { window.searchQuery = ""; inboxSearch.clear(); inboxList.forceActiveFocus() }
                }
                ComboBox {
                    objectName: "inboxDensity"
                    visible: window.desktopNavigation
                    model: [qsTr("Comfortable"), qsTr("Compact")]
                    currentIndex: window.compactDensity ? 1 : 0
                    Layout.minimumHeight: 48
                    implicitHeight: 48
                    Accessible.name: qsTr("Inbox density")
                    onActivated: window.compactDensity = currentIndex === 1
                }
            }
            RowLayout {
                objectName: "connectionStatusRow"
                visible: !store.online
                Layout.fillWidth: true
                spacing: 4
                Label {
                    objectName: "connectionStatus"
                    text: store.online ? "" : window.offlineSummary()
                    color: window.mutedInk
                    font.family: window.themeFont.family
                    font.pointSize: window.themeFont.pointSize - 1
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                    TapHandler { onTapped: window.offlineDetailsShown = !window.offlineDetailsShown }
                }
                ToolButton {
                    objectName: "offlineRetry"
                    text: qsTr("Retry")
                    implicitHeight: 40
                    Material.foreground: window.accent
                    palette.buttonText: window.accent
                    onClicked: store.refresh()
                }
            }
            Label {
                objectName: "offlineDetails"
                visible: !store.online && window.offlineDetailsShown && store.offlineReason.length > 0
                text: qsTr("Details: %1").arg(store.offlineReason)
                color: window.mutedInk
                font.pointSize: window.themeFont.pointSize - 2
                wrapMode: Text.Wrap
                Layout.fillWidth: true
                Layout.bottomMargin: 4
            }
            Label { objectName: "updateStatus"; text: window.updateStatus; visible: text.length > 0; color: window.mutedInk; elide: Text.ElideRight; Layout.fillWidth: true }
            RowLayout {
                objectName: "updateReadyBar"
                visible: window.updateVersion.length > 0
                Layout.fillWidth: true
                spacing: 8
                Label {
                    text: qsTr("Update ready")
                    color: window.ink
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
                Button {
                    objectName: "updateInstall"
                    text: qsTr("Install")
                    Layout.minimumHeight: 48
                    onClicked: updater.installDownloadedUpdate()
                }
            }
            // Reconnect stays a persistent actionable banner (R51, R52), never
            // part of the dismissible Undo bar (R57).
            Rectangle {
                objectName: "reconnectBanner"
                visible: window.reconnectStatus.length > 0
                Layout.fillWidth: true
                Layout.bottomMargin: 6
                implicitHeight: Math.max(48, reconnectRow.implicitHeight) + 8
                radius: Kirigami.Units.cornerRadius
                color: "#fff4df"
                border.color: "#e2c993"
                RowLayout {
                    id: reconnectRow
                    anchors.fill: parent
                    anchors.leftMargin: 12
                    anchors.rightMargin: 6
                    spacing: 8
                    Label {
                        objectName: "reconnectBannerMessage"
                        text: window.reconnectStatus
                        color: window.ink
                        wrapMode: Text.Wrap
                        maximumLineCount: 2
                        elide: Text.ElideRight
                        Layout.fillWidth: true
                    }
                    Button {
                        objectName: "reconnectBannerButton"
                        text: qsTr("Reconnect")
                        implicitHeight: 48
                        contentItem: Label { text: parent.text; color: window.ink; verticalAlignment: Text.AlignVCenter; horizontalAlignment: Text.AlignHCenter }
                        background: Rectangle { radius: 4; color: "#ffffff"; border.color: "#e2c993" }
                        onClicked: window.openPage("GmailAccountsPage")
                    }
                }
            }
            Label { text: window.dragFeedback; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true; Accessible.role: Accessible.AlertMessage }
        }
    }
    // R57: one-line Undo snackbar over the bottom of the list; it never
    // reflows the inbox. The store owns how long Undo stays valid.
    Rectangle {
        id: bundleArchiveUndoBar
        objectName: "bundleArchiveUndoBar"
        parent: window.contentItem
        z: 50
        visible: (window.archiveUndoAvailable || window.bundleArchiveStatus.length > 0) && stack.depth === 1
        opacity: visible ? 1 : 0
        Behavior on opacity { enabled: !window.reduceMotion; NumberAnimation { duration: 180 } }
        width: Math.min(parent.width - 2 * 12, 560)
        height: Math.max(48, undoBarRow.implicitHeight + 8)
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 16
        radius: 8
        color: "#263b3a"
        RowLayout {
            id: undoBarRow
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 16
            anchors.rightMargin: 4
            spacing: 8
            Label {
                objectName: "bundleArchiveUndoMessage"
                text: window.bundleArchiveStatus
                color: "#ffffff"
                font: window.themeFont
                elide: Text.ElideRight
                maximumLineCount: 1
                Layout.fillWidth: true
            }
            Button {
                id: bundleArchiveUndoButton
                objectName: "bundleArchiveUndoButton"
                visible: window.archiveUndoAvailable
                text: qsTr("Undo")
                implicitWidth: Math.max(64, implicitContentWidth + leftPadding + rightPadding)
                implicitHeight: 48
                flat: true
                contentItem: Label { text: bundleArchiveUndoButton.text; color: "#9fe0d3"; font.weight: Font.DemiBold; horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter }
                background: Rectangle {
                    radius: 4
                    color: bundleArchiveUndoButton.down ? "#3a5553" : "transparent"
                    border.width: bundleArchiveUndoButton.visualFocus ? 2 : 0
                    border.color: "#9fe0d3"
                }
                onClicked: window.undoBundleArchive()
            }
        }
    }
    Connections {
        target: updater
        // R64: automatic checks are silent; only a manual check shows progress.
        function onBusyChanged() { window.updateStatus = updater.busy && !updater.automatic ? qsTr("Checking for updates…") : "" }
        function onNoUpdateAvailable() { window.updateStatus = qsTr("No update available") }
        // R64: a verified predownload shows one non-blocking Install line;
        // tapping it opens Android's installer directly.
        function onUpdateReady(version, sha256) {
            window.updateVersion = version
            window.updateStatus = ""
        }
        function onInstallConsentRequired() {
            window.updateStatus = qsTr("Allow installs from Litterbox in Android settings, then tap Install again")
        }
        function onErrorOccurred(message) { window.updateStatus = message }
    }
    Drawer {
        id: navigationDrawer
        objectName: "navigationDrawer"
        width: Math.min(window.width - 48, Math.max(260, Kirigami.Units.gridUnit * 16))
        height: window.height
        edge: Qt.LeftEdge
        background: Rectangle { color: window.surface }
        contentItem: ScrollView {
            clip: true
            ColumnLayout {
                width: navigationDrawer.availableWidth
                spacing: 12
                Label { text: qsTr("Litterbox"); font.family: window.themeFont.family; font.pointSize: window.themeFont.pointSize; font.bold: true; color: window.ink; padding: 16; Layout.fillWidth: true }
                Destinations { Layout.fillWidth: true; Layout.margins: 8 }
                ToolButton {
                    text: qsTr("Search inbox")
                    icon.name: "edit-find"
                    implicitHeight: 48
                    Layout.fillWidth: true
                    onClicked: { window.navigateTo(""); window.searchShown = true; inboxSearch.forceActiveFocus() }
                }
                ToolButton { text: qsTr("Close navigation"); icon.name: "go-previous"; implicitHeight: 48; Layout.fillWidth: true; onClicked: navigationDrawer.close() }
            }
        }
    }
    Rectangle {
        id: desktopNav
        objectName: "desktopNavigation"
        visible: window.desktopNavigation
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        width: Math.max(180, Kirigami.Units.gridUnit * 10)
        color: "#edf4f1"
        ScrollView {
            anchors.fill: parent
            anchors.margins: 12
            clip: true
            Destinations { width: desktopNav.width - 24 }
        }
    }
    Shortcut {
        sequence: "Alt+Left"
        enabled: !window.actualPopupsVisible
        onActivated: window.handleBack()
    }
    SplitView {
        id: inboxSplit
        objectName: "inboxSplit"
        anchors.left: desktopNav.visible ? desktopNav.right : parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        orientation: Qt.Horizontal
        handle: Rectangle {
            implicitWidth: 12
            activeFocusOnTab: true
            Accessible.role: Accessible.Slider
            Accessible.name: qsTr("Inbox pane width")
            Keys.onLeftPressed: stack.SplitView.preferredWidth = Math.max(400, stack.width - 32)
            Keys.onRightPressed: stack.SplitView.preferredWidth = Math.min(inboxSplit.width - 532, stack.width + 32)
            color: SplitHandle.pressed ? "#cbded8" : SplitHandle.hovered ? "#e0e9e6" : window.canvas
            Rectangle {
                anchors.centerIn: parent
                width: 2
                height: 40
                radius: 1
                color: window.mutedInk
            }
        }
    StackView {
        id: stack
        objectName: "pageStack"
        focus: true
        SplitView.minimumWidth: window.desktopReader ? 400 : 0
        SplitView.preferredWidth: window.width * 0.43
        SplitView.fillWidth: !readerHost.visible
        // Sliding the inbox moves its ListView viewport out of the window,
        // causing whole rows of Controls to be destroyed and recreated.
        pushExit: null
        popEnter: null
        Keys.onBackPressed: function(event) {
            if (window.handleBack()) event.accepted = true
        }
        // Esc and Alt+Left leave any pushed page; open menus and dialogs
        // keep their own Escape handling before keys bubble up to here.
        Keys.onEscapePressed: function(event) {
            if (window.handleBack()) event.accepted = true
        }
        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Left && (event.modifiers & Qt.AltModifier) && window.handleBack())
                event.accepted = true
        }
        onCurrentItemChanged: {
            if (!window.relocatingReader && depth === 1 && window.mailDetailPage && window.mailDetailPage.parent === stack)
                window.readerOpen = false
            if (depth > 1 && currentItem && currentItem !== window.mailDetailPage)
                currentItem.forceActiveFocus()
        }
        initialItem: Item {
            StackView.visible: true
            enabled: StackView.status === StackView.Active
            StackView.onActivated: {
                window.restoreInboxScroll()
                inboxList.forceActiveFocus()
            }
            ListView {
                id: inboxList
                objectName: "inboxList"
                anchors.fill: parent
                clip: true
                ScrollBar.vertical: ScrollBar { }
                // Width changes reflow delegates; preserve the visible card and
                // its offset rather than jumping to the selected (possibly offscreen) card.
                onWidthChanged: {
                    if (!bundleModel) return
                    const anchor = bundleModel.captureAnchor()
                    Qt.callLater(function() {
                        if (!bundleModel) return
                        bundleModel.restoreAnchor(anchor, bundleModel.rows)
                    })
                }
                topMargin: window.edgeSpacing
                bottomMargin: window.edgeSpacing
                Component.onCompleted: forceActiveFocus()
                function captureLayoutMetrics() {
                    function find(item, name) {
                        if (item.objectName === name) return item
                        for (const child of item.children) {
                            const found = find(child, name)
                            if (found) return found
                        }
                        return null
                    }
                    const card = find(contentItem, "inboxCard")
                    const actions = card ? find(card, "inboxActions") : null
                    return ({
                        itemCount: count,
                        cardWidth: card ? card.width : -1,
                        cardX: card ? card.mapToItem(null, 0, 0).x : -1,
                        hasActions: actions !== null,
                        actionsX: actions ? actions.mapToItem(null, 0, 0).x : -1,
                        actionsWidth: actions ? actions.width : -1
                        , moreTargetHeight: actions ? actions.height : -1
                        , moreTargetWidth: actions ? actions.width : -1
                        , dragActive: window.dragCardId.length > 0
                        , dragOffset: window.dragOffset
                        , dropTarget: window.dragTargetIndex
                    })
                }
                function captureVisibleRange() {
                    // indexAt takes content coordinates and returns -1 over
                    // section headings, so scan inward from both viewport
                    // edges; a blank region yields -1 at that edge, which
                    // numeric contentY assertions cannot see.
                    let first = -1
                    for (let y = contentY + 8; y < contentY + height; y += 16) {
                        first = indexAt(24, y)
                        if (first >= 0) break
                    }
                    let last = -1
                    for (let y = contentY + height - 8; y > contentY; y -= 16) {
                        last = indexAt(24, y)
                        if (last >= 0) break
                    }
                    return ({ contentY: contentY, firstVisible: first, lastVisible: last })
                }
                Keys.onPressed: function(event) {
                    const page = Math.max(1, height * 0.85)
                    switch (event.key) {
                    case Qt.Key_Home: positionViewAtBeginning(); break
                    case Qt.Key_End: positionViewAtEnd(); break
                    case Qt.Key_PageUp: contentY = Math.max(0, contentY - page); break
                    case Qt.Key_PageDown: contentY = Math.max(0, Math.min(contentHeight - height, contentY + page)); break
                    case Qt.Key_Up: contentY = Math.max(0, contentY - 48); break
                    case Qt.Key_Down: contentY = Math.max(0, Math.min(contentHeight - height, contentY + 48)); break
                    default: return
                    }
                    event.accepted = true
                }
                WheelHandler {
                    target: null
                    acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
                    onWheel: function(event) {
                        const delta = event.pixelDelta.y !== 0 ? event.pixelDelta.y * 3 : event.angleDelta.y / 120 * 48
                        const top = inboxList.originY - inboxList.topMargin
                        const bottom = Math.max(top, inboxList.originY + inboxList.contentHeight - inboxList.height + inboxList.bottomMargin)
                        inboxList.contentY = Math.max(top, Math.min(bottom, inboxList.contentY - delta))
                        event.accepted = true
                    }
                }
                // Stable slots preserve viewport delegates on both reordering
                // and source removal; a numeric/array model resets them all.
                model: presentationModel
                ListModel { id: presentationModel }
                // Hidden bundle members must contribute neither height nor spacing.
                spacing: 0
                DelegateModel {
                    id: bundleModel
                    objectName: "bundlePresentation"
                    model: store
                    property var rows: []
                    property var sourceEntries: []
                    // Read source roles without ever mutating DelegateModel's groups.
                    // Publish one linear presentation snapshot; only source signals
                    // and explicit expansion can rebuild it, never its own reset.
                    function rebuildPresentation(refreshSource) {
                        const refresh = refreshSource !== false
                        const members = {}
                        const sections = refresh ? {} : window.bundleSections
                        const summaries = refresh ? {} : window.bundleSummaries
                        const entries = refresh ? [] : sourceEntries
                        const groups = []
                        for (let i = 0; i < (refresh ? items.count : entries.length); ++i) {
                            const source = refresh ? items.get(i).model : null
                            const entry = refresh ? {card: source.card, cardId: source.cardId, title: source.title} : entries[i]
                            if (refresh) entries.push(entry)
                            const card = entry.card
                            if (refresh && card.bundle_id && card.pinned_rank == null && card.section !== "pinned") {
                                const summary = summaries[card.bundle_id] || (summaries[card.bundle_id] = {accounts: [], senders: [], important: 0, unpinned: 0, lastId: ""})
                                if (card.account_name && summary.accounts.indexOf(card.account_name) < 0) summary.accounts.push(card.account_name)
                                if (card.pinned_rank == null && card.section !== "pinned") {
                                    ++summary.unpinned
                                    if (card.important) ++summary.important
                                }
                                if (!summary.latestAt || (card.received_at && card.received_at > summary.latestAt)) {
                                    summary.latestAt = card.received_at || ""
                                    summary.latestTitle = entry.title || ""
                                }
                                const sender = typeof card.sender_name === "string" ? card.sender_name.trim() : ""
                                if (sender && summary.senders.indexOf(sender) < 0) summary.senders.push(sender)
                            }
                            if (card.bundle_leader === true) {
                                sections[card.bundle_id] = card.section
                                groups.push([entry])
                                if (window.expandedBundles[card.bundle_id] || window.searching) {
                                    const bucket = members[card.bundle_id] || (members[card.bundle_id] = [])
                                    groups.push(bucket)
                                }
                            }
                            else if (card.bundle_leader === false && (window.expandedBundles[card.bundle_id] || window.searching)) {
                                const bucket = members[card.bundle_id] || (members[card.bundle_id] = [])
                                bucket.push(entry)
                            } else groups.push([entry])
                        }
                        if (refresh) {
                            for (const bundle in summaries) {
                                const summary = summaries[bundle]
                                summary.senderLabel = summary.senders.slice(0, 2).join(", ") + (summary.senders.length > 2 ? " +" + (summary.senders.length - 2) : "")
                            }
                            sourceEntries = entries
                            window.bundleSections = sections
                        }
                        let ordered = [].concat(...groups)
                        if (window.searching) {
                            const matches = {}
                            const matchingBundles = {}
                            for (const entry of ordered) {
                                if (!window.matchesSearch(entry)) continue
                                matches[entry.cardId] = true
                                if (entry.card.bundle_id) matchingBundles[entry.card.bundle_id] = true
                            }
                            ordered = ordered.filter(entry => matches[entry.cardId] || (entry.card.bundle_leader === true && matchingBundles[entry.card.bundle_id]))
                        }
                        for (const bundle in summaries) summaries[bundle].lastId = ""
                        for (let i = 0; i < ordered.length; ++i) {
                            const entry = ordered[i]
                            if (entry.card.bundle_leader !== undefined && (i + 1 === ordered.length
                                    || ordered[i + 1].card.bundle_leader === undefined
                                    || ordered[i + 1].card.bundle_id !== entry.card.bundle_id))
                                summaries[entry.card.bundle_id].lastId = entry.cardId
                        }
                        // Publish only after lastId is ready: nested object writes
                        // do not notify the delegates' bundleSummary bindings.
                        window.bundleSummaries = refresh ? summaries : Object.assign({}, summaries)
                        const headings = {}
                        const dates = {}
                        let previousDate = ""
                        let previousSection = ""
                        for (const entry of ordered) {
                            const card = entry.card
                            if (card.bundle_leader === false && (window.searching ? window.searchCollapsedBundles[card.bundle_id] : !window.expandedBundles[card.bundle_id])) continue
                            const section = card.bundle_leader !== undefined ? sections[card.bundle_id] : card.section
                            headings[entry.cardId] = section !== previousSection
                            const summary = summaries[card.bundle_id]
                            const date = window.dateLandmark(card.bundle_leader === true && summary && summary.latestAt ? {received_at: summary.latestAt} : card)
                            if (date && (date !== previousDate || section !== previousSection)) dates[entry.cardId] = date
                            previousDate = date
                            previousSection = section
                        }
                        window.sectionStarts = headings
                        window.dateStarts = dates
                        const position = inboxList.contentY - inboxList.originY
                        const anchor = captureAnchor()
                        if (presentationModel.count > ordered.length)
                            presentationModel.remove(ordered.length, presentationModel.count - ordered.length)
                        rows = ordered
                        while (presentationModel.count < ordered.length)
                            presentationModel.append({slot: presentationModel.count})
                        inboxList.forceLayout()
                        if (!restoreAnchor(anchor, ordered))
                            inboxList.contentY = inboxList.originY + position
                        if (refresh && window.readerOpen && !entries.some(entry => entry.cardId === window.selectedCardId))
                            Qt.callLater(function() {
                                if (window.readerOpen && store.cardIds().indexOf(window.selectedCardId) < 0) window.closeReader()
                            })
                    }
                    // R68: keep the visual anchor across archive, Undo and snapshots.
                    // The first visible card stays at the same offset; if it is gone,
                    // the next surviving card from the old order takes its place.
                    function captureAnchor() {
                        if (!rows.length || inboxList.atYBeginning) return null
                        const top = inboxList.indexAt(inboxList.width / 2, inboxList.contentY + 1)
                        if (top < 0) return null
                        const item = inboxList.itemAtIndex(top)
                        if (!item) return null
                        const ids = []
                        for (let i = top; i < rows.length && ids.length < 64; ++i)
                            if (rows[i] && rows[i].cardId) ids.push(rows[i].cardId)
                        return {ids: ids, offset: item.y - inboxList.contentY}
                    }
                    function restoreAnchor(anchor, ordered) {
                        if (!anchor) return false
                        const index = {}
                        for (let i = 0; i < ordered.length; ++i) index[ordered[i].cardId] = i
                        for (const id of anchor.ids) {
                            const at = index[id]
                            if (at === undefined) continue
                            const card = ordered[at].card
                            if (card.bundle_leader === false && !window.expandedBundles[card.bundle_id] && !window.searching) continue
                            inboxList.positionViewAtIndex(at, ListView.Beginning)
                            const item = inboxList.itemAtIndex(at)
                            if (!item) return false
                            const maxY = inboxList.originY + Math.max(0, inboxList.contentHeight - inboxList.height)
                            inboxList.contentY = Math.max(inboxList.originY, Math.min(maxY, item.y - anchor.offset))
                            return true
                        }
                        return false
                    }
                    Component.onCompleted: rebuildPresentation()
                    onCountChanged: Qt.callLater(rebuildPresentation)
                    Connections {
                        target: store
                        ignoreUnknownSignals: true
                        function onDataChanged() { Qt.callLater(bundleModel.rebuildPresentation) }
                        function onRowsMoved() { Qt.callLater(bundleModel.rebuildPresentation) }
                        function onModelReset() { Qt.callLater(bundleModel.rebuildPresentation) }
                    }
                }
                delegate: Item {
                    required property int slot
                    readonly property var entry: bundleModel.rows[slot] || {card: {}, cardId: "", title: ""}
                    readonly property var card: entry.card
                    readonly property string cardId: entry.cardId
                    readonly property string title: entry.title
                    id: cardRow
                    readonly property bool selected: cardId === window.selectedCardId
                    readonly property string displaySection: card.bundle_leader !== undefined
                        ? (window.bundleSections[card.bundle_id] || card.section) : card.section
                    HoverHandler { id: rowHover }
                    readonly property bool lifted: window.dragCardId === cardId
                    readonly property bool bundled: !!card.bundle_id && card.section !== "pinned" && card.pinned_rank == null && !card.important
                    readonly property bool bundleExpanded: window.searching ? !window.searchCollapsedBundles[card.bundle_id] : !!window.expandedBundles[card.bundle_id]
                    readonly property bool bundleLeader: bundled && card.bundle_leader === true
                    readonly property var bundleSummary: window.bundleSummaries[card.bundle_id] || {accounts: [], senders: [], important: 0, unpinned: 0, lastId: cardId}
                    readonly property bool showMemberSender: !bundled || bundleSummary.senders.length !== 1 || bundleSummary.senders[0] !== window.mailSender(card)
                    readonly property bool showMemberAccount: !bundled || bundleSummary.accounts.length !== 1 || bundleSummary.accounts[0] !== card.account_name
                    readonly property bool lastMember: bundleSummary.lastId === cardId
                    readonly property var latestCard: bundleSummary.latestAt && (!card.received_at || bundleSummary.latestAt > card.received_at) ? Object.assign({}, card, {received_at: bundleSummary.latestAt}) : card
                    readonly property real bundleTextInset: window.compactInbox ? 56 : Math.max(48, Kirigami.Units.gridUnit * 3) + Kirigami.Units.smallSpacing
                    property string registeredCardId: ""
                    property string registeredBundleId: ""
                    function unregisterControls() {
                        if (window.cardHandles[registeredCardId] === dragHandle) delete window.cardHandles[registeredCardId]
                        if (window.captureActions[registeredCardId] === cardActions) delete window.captureActions[registeredCardId]
                        if (window.bundleOpeners[registeredBundleId] === bundleToggle) delete window.bundleOpeners[registeredBundleId]
                    }
                    function registerControls() {
                        unregisterControls()
                        if (!cardId) return
                        registeredCardId = cardId
                        registeredBundleId = bundleLeader ? card.bundle_id : ""
                        window.cardHandles[cardId] = dragHandle
                        window.captureActions[cardId] = cardActions
                        if (bundleLeader) window.bundleOpeners[card.bundle_id] = bundleToggle
                    }
                    onCardIdChanged: if (dragHandle && cardActions && bundleToggle) registerControls()
                    onBundleLeaderChanged: if (dragHandle && cardActions && bundleToggle) registerControls()
                    Component.onCompleted: registerControls()
                    Component.onDestruction: unregisterControls()
                    Keys.onLeftPressed: function(event) {
                        if (bundled && bundleExpanded) window.toggleBundle(card.bundle_id)
                        else event.accepted = false
                    }
                    Keys.onEscapePressed: function(event) {
                        if (bundled && bundleExpanded) window.toggleBundle(card.bundle_id)
                        else event.accepted = false
                    }
                    visible: !bundled || card.bundle_leader !== false || bundleExpanded
                    width: ListView.view.width
                    height: (!bundled || card.bundle_leader !== false || bundleExpanded) ? sectionHeader.height + cardFrame.implicitHeight + (bundled && bundleExpanded && !lastMember ? 0 : window.cardGap) : 0
                    z: lifted ? 10 : 0
                    Rectangle { anchors.fill: parent; color: "#eef3f2"; visible: cardRow.lifted; radius: 8 }
                    Item {
                        id: sectionHeader
                        width: parent.width
                        height: window.sectionStarts[cardId] || window.dateStarts[cardId] ? sectionHeading.implicitHeight + (window.compactInbox ? 12 : 2 * window.edgeSpacing) : 0
                        visible: height > 0
                        Label {
                            id: sectionHeading
                            objectName: "dateLandmark-" + cardId
                            width: Math.min(parent.width - 2 * window.edgeSpacing, 1200)
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.top: parent.top
                            anchors.topMargin: window.compactInbox ? 6 : window.edgeSpacing
                            font.pointSize: window.compactInbox ? window.themeFont.pointSize - 2 : window.themeFont.pointSize
                            text: (window.sectionStarts[cardId] ? (cardRow.displaySection === "pinned" ? qsTr("Pinned") : cardRow.displaySection === "now" ? qsTr("Now") : cardRow.displaySection === "later" ? qsTr("Later") : qsTr("Missed")) : "")
                                + (window.sectionStarts[cardId] && window.dateStarts[cardId] ? " · " : "") + (window.dateStarts[cardId] || "")
                            font.family: window.themeFont.family
                            color: window.mutedInk
                            padding: 0
                        }
                    }
                    Frame {
                        id: cardFrame
                        objectName: "inboxCard"
                        width: Math.min(parent.width - 2 * window.edgeSpacing, 1200)
                        x: (parent.width - width) / 2
                        y: sectionHeader.height + (cardRow.lifted ? window.dragOffset : 0)
                        padding: window.edgeSpacing
                        topPadding: window.compactDensity ? 6 : 12
                        bottomPadding: window.compactDensity ? 6 : 12
                        rightPadding: window.compactInbox ? 0 : window.edgeSpacing
                        background: Rectangle {
                            color: cardRow.lifted ? "#e4efed" : cardRow.selected ? "#e7f1ee" : window.surface
                            border.color: cardRow.selected || cardText.activeFocus || bundleToggle.activeFocus || archiveBundle.activeFocus || bundleOptions.activeFocus || cardRow.lifted ? window.accent : cardRow.bundled ? "#cbded8" : "#edf0ef"
                            border.width: cardRow.bundled && cardRow.bundleExpanded ? 0 : cardRow.selected || cardText.activeFocus || bundleToggle.activeFocus || archiveBundle.activeFocus || bundleOptions.activeFocus ? 2 : 1
                            radius: cardRow.bundled && cardRow.bundleExpanded ? 0 : Kirigami.Units.cornerRadius
                            // Adjacent delegates paint the sides of one group, not
                            // individual cards. Only its first/last row closes it.
                            Rectangle { x: 0; width: 1; height: parent.height; color: "#cbded8"; visible: cardRow.bundled && cardRow.bundleExpanded }
                            Rectangle { x: parent.width - 1; width: 1; height: parent.height; color: "#cbded8"; visible: cardRow.bundled && cardRow.bundleExpanded }
                            Rectangle { width: parent.width; height: 1; color: "#cbded8"; visible: cardRow.bundled && cardRow.bundleExpanded && cardRow.bundleLeader }
                            Rectangle { y: parent.height - 1; width: parent.width; height: 1; color: "#cbded8"; visible: cardRow.bundled && cardRow.bundleExpanded && cardRow.lastMember }
                            Rectangle {
                                x: window.edgeSpacing
                                width: parent.width - 2 * window.edgeSpacing
                                height: 1
                                color: "#e0e9e6"
                                visible: cardRow.bundled && cardRow.bundleExpanded && !cardRow.bundleLeader
                            }
                            Rectangle {
                                x: window.edgeSpacing
                                y: cardRow.bundleLeader ? cardFrame.topPadding + bundleDivider.y : 0
                                width: 3
                                height: parent.height - y - (cardRow.lastMember ? window.edgeSpacing : 0)
                                color: "#cbded8"
                                visible: cardRow.bundled && cardRow.bundleExpanded
                            }
                        }
                        contentItem: ColumnLayout {
                            spacing: 0
                            GridLayout {
                                visible: cardRow.bundleLeader
                                Layout.fillWidth: true
                                Layout.minimumHeight: window.compactDensity ? 0 : 84
                                columnSpacing: Kirigami.Units.smallSpacing
                                rowSpacing: Kirigami.Units.smallSpacing
                                columns: window.compactInbox ? 1 : 2
                                Button {
                                    id: bundleToggle
                                    objectName: "bundleToggle-" + cardId
                                    Layout.row: 0
                                    Layout.column: 0
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 48
                                    implicitHeight: Math.max(48, contentItem.implicitHeight)
                                    padding: 0
                                    leftPadding: 0
                                    rightPadding: 0
                                    topPadding: 0
                                    bottomPadding: 0
                                    focusPolicy: Qt.StrongFocus
                                    text: window.bundleTitle(card)
                                    icon.name: cardRow.bundleExpanded ? "arrow-down" : "arrow-right"
                                    Accessible.name: (cardRow.bundleExpanded ? qsTr("Collapse %1, %2 emails") : qsTr("Expand %1, %2 emails")).arg(text).arg(card.bundle_member_count || 0)
                                    background: Rectangle {
                                        color: bundleToggle.down ? "#e7f1ee" : bundleToggle.hovered ? "#f3f7f6" : "transparent"
                                        radius: Kirigami.Units.cornerRadius
                                        border.width: bundleToggle.visualFocus ? 2 : 0
                                        border.color: window.accent
                                    }
                                    contentItem: ColumnLayout {
                                        spacing: window.compactInbox ? 2 : Kirigami.Units.smallSpacing
                                        GridLayout {
                                            Layout.fillWidth: true
                                            columnSpacing: Kirigami.Units.smallSpacing
                                            rowSpacing: Kirigami.Units.smallSpacing
                                            columns: window.compactInbox ? 1 : 4
                                            Item {
                                                visible: !window.compactInbox
                                                implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
                                                implicitHeight: Kirigami.Units.iconSizes.smallMedium
                                                Kirigami.Icon { anchors.centerIn: parent; source: bundleToggle.icon.name; color: window.ink; isMask: true; width: Kirigami.Units.iconSizes.smallMedium; height: width }
                                            }
                                            Label { objectName: "bundleTitleLabel-" + cardId; Layout.leftMargin: window.compactInbox ? cardRow.bundleTextInset : 0; text: bundleToggle.text; color: window.ink; font: Qt.font({family: window.themeFont.family, pointSize: window.themeFont.pointSize, bold: true}); wrapMode: window.compactInbox ? Text.WordWrap : Text.Wrap; Layout.fillWidth: true; Layout.maximumWidth: Infinity }
                                            Rectangle {
                                                Layout.row: window.compactInbox ? 1 : 0
                                                Layout.column: window.compactInbox ? 0 : 2
                                                Layout.leftMargin: window.compactInbox ? cardRow.bundleTextInset : 0
                                                Layout.alignment: Qt.AlignLeft | Qt.AlignTop
                                                implicitWidth: countLabel.implicitWidth + (window.compactInbox ? 10 : 16)
                                                implicitHeight: countLabel.implicitHeight + (window.compactInbox ? 4 : 8)
                                                radius: Kirigami.Units.cornerRadius
                                                color: "#e7f1ee"
                                                Label { id: countLabel; objectName: "bundleMemberCount-" + cardId; anchors.centerIn: parent; text: window.emailCountText(card.bundle_member_count || 0); color: window.ink; font.family: window.themeFont.family; font.pointSize: window.compactInbox ? window.themeFont.pointSize - 2 : window.themeFont.pointSize }
                                            }
                                            Item { visible: !window.compactInbox; Layout.fillWidth: true }
                                        }
                                        Label {
                                            objectName: "bundleAccountLabel-" + cardId
                                            text: (cardRow.bundleSummary.senderLabel || bundleToggle.text) + (cardRow.bundleSummary.accounts.length > 1 ? " · " + qsTr("%1 accounts").arg(cardRow.bundleSummary.accounts.length) : card.account_name ? " · " + card.account_name : "")
                                            color: window.mutedInk
                                            font.family: window.themeFont.family
                                            font.pointSize: window.compactInbox ? window.themeFont.pointSize - 2 : window.themeFont.pointSize
                                            Layout.fillWidth: true
                                            leftPadding: cardRow.bundleTextInset
                                            wrapMode: window.compactInbox ? Text.NoWrap : Text.Wrap
                                            elide: Text.ElideRight
                                        }
                                        Label {
                                            objectName: "bundleLatestLabel-" + cardId
                                            text: window.cleanPreview(card.snippet) || (cardRow.bundleSummary.latestTitle !== bundleToggle.text ? (cardRow.bundleSummary.latestTitle || title) : "")
                                            visible: !cardRow.bundleExpanded && text.length > 0
                                            color: window.mutedInk
                                            font.family: window.themeFont.family
                                            Layout.fillWidth: true
                                            leftPadding: cardRow.bundleTextInset
                                            wrapMode: window.compactInbox ? Text.NoWrap : Text.Wrap
                                            font.pointSize: window.compactInbox ? window.themeFont.pointSize - 1 : window.themeFont.pointSize
                                            maximumLineCount: window.compactInbox ? 1 : Infinity
                                            elide: Text.ElideRight
                                        }
                                        Label {
                                            visible: cardRow.bundleSummary.important > 0
                                            text: cardRow.bundleSummary.important === 1
                                                ? qsTr("1 important email stays separate; archive includes all %1 unpinned emails").arg(cardRow.bundleSummary.unpinned)
                                                : qsTr("%1 important emails stay separate; archive includes all %2 unpinned emails").arg(cardRow.bundleSummary.important).arg(cardRow.bundleSummary.unpinned)
                                            objectName: "importantDisclosure-" + cardId
                                            color: window.mutedInk
                                            font: window.themeFont
                                            Layout.fillWidth: true
                                            leftPadding: cardRow.bundleTextInset
                                            wrapMode: Text.Wrap
                                        }
                                    }
                                    SenderAvatar {
                                        visible: window.compactInbox
                                        x: 0
                                        y: 2
                                        name: (cardRow.bundleSummary.senders || [])[0] || bundleToggle.text
                                    }
                                    onClicked: window.toggleBundle(card.bundle_id)
                                    Keys.onReturnPressed: clicked()
                                    Keys.onEnterPressed: clicked()
                                    Keys.onRightPressed: { if (!cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                    Keys.onLeftPressed: { if (cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                    Keys.onEscapePressed: { if (cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                }
                                ColumnLayout {
                                    Layout.row: window.compactInbox ? 1 : 0
                                    Layout.column: window.compactInbox ? 0 : 1
                                    Layout.alignment: Qt.AlignTop
                                    Layout.fillWidth: window.compactInbox
                                    Layout.preferredWidth: window.compactInbox ? -1 : 120
                                    Layout.minimumWidth: window.compactInbox ? 0 : 120
                                    Layout.maximumWidth: window.compactInbox ? Infinity : 120
                                    spacing: window.compactInbox ? 0 : Kirigami.Units.smallSpacing
                                    Label {
                                        objectName: "bundleDate-" + cardId
                                        text: window.mailArrival(cardRow.latestCard)
                                        visible: !window.compactInbox && text.length > 0
                                        color: window.mutedInk
                                        font: window.themeFont
                                        Layout.fillWidth: true
                                        horizontalAlignment: Text.AlignRight
                                    }
                                    GridLayout {
                                        // R63: phones stack the newest time over the archive so the title keeps the width.
                                        Layout.alignment: Qt.AlignRight
                                        columns: window.compactInbox ? 2 : 3
                                        rowSpacing: 0
                                        columnSpacing: window.compactInbox ? 0 : Kirigami.Units.smallSpacing
                                        Label {
                                            objectName: "phoneBundleDate-" + cardId
                                            Layout.alignment: Qt.AlignRight
                                            Layout.columnSpan: window.compactInbox ? 2 : 1
                                            visible: window.compactInbox
                                            text: window.mailArrival(cardRow.latestCard)
                                            color: window.mutedInk
                                            font.pointSize: window.themeFont.pointSize - 2
                                        }
                                Button {
                                    id: archiveBundle
                                    objectName: "archiveBundle-" + cardId
                                    text: qsTr("Archive bundle")
                                    icon.name: "mail-mark-read-symbolic"
                                    icon.color: window.ink
                                    icon.width: window.actionIconSize
                                    icon.height: window.actionIconSize
                                    display: AbstractButton.IconOnly
                                    Layout.minimumWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    Layout.preferredWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    Layout.maximumWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    Accessible.name: qsTr("Archive bundle")
                                    Accessible.description: qsTr("Archives all %1 unpinned %2; pinned cards stay open").arg(cardRow.bundleSummary.unpinned).arg(cardRow.bundleSummary.unpinned === 1 ? qsTr("email") : qsTr("emails"))
                                    ToolTip.text: Accessible.name
                                    ToolTip.visible: hovered
                                    background: Rectangle {
                                        radius: Kirigami.Units.cornerRadius
                                        color: parent.down ? "#e7f1ee" : parent.hovered ? "#f3f7f6" : "transparent"
                                        border.width: parent.visualFocus ? 2 : 0
                                        border.color: window.accent
                                    }
                                    onClicked: window.archiveBundle(card.bundle_id)
                                }
                                ToolButton {
                                    id: bundleOptions
                                    objectName: "bundleOptions-" + cardId
                                    text: qsTr("Bundle details")
                                    icon.name: "overflow-menu"
                                    icon.color: window.ink
                                    icon.width: window.actionIconSize
                                    icon.height: window.actionIconSize
                                    display: AbstractButton.IconOnly
                                    implicitWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    Accessible.name: text
                                    ToolTip.text: text
                                    ToolTip.visible: hovered
                                    onClicked: { bundleDetailsLoader.active = true; bundleDetailsLoader.item.open() }
                                    // Built on first open: a Menu per row slowed creating every inbox row.
                                    Loader {
                                        id: bundleDetailsLoader
                                        active: false
                                        sourceComponent: Component {
                                            Menu {
                                                parent: bundleOptions
                                                id: bundleDetails
                                                width: 320
                                                x: window.width - width - 3 * window.edgeSpacing
                                                background: Rectangle { color: window.surface; border.color: "#dce5e3"; radius: Kirigami.Units.cornerRadius }
                                                MenuItem { text: card.bundle_title || window.bundleTitle(card); enabled: false; width: bundleDetails.width; contentItem: Label { text: parent.text; color: window.ink; wrapMode: Text.Wrap; maximumLineCount: 2; elide: Text.ElideRight; verticalAlignment: Text.AlignVCenter } }
                                                MenuItem { text: cardRow.bundleSummary.accounts.join(", "); enabled: false; width: bundleDetails.width; contentItem: Label { text: parent.text; color: window.mutedInk; wrapMode: Text.Wrap; maximumLineCount: 2; elide: Text.ElideRight; verticalAlignment: Text.AlignVCenter } }
                                                MenuItem {
                                                    objectName: "bundleSpam-" + cardId
                                                    text: qsTr("Mark bundle as spam · %1").arg(cardRow.bundleSummary.accounts.join(", "))
                                                    icon.name: "mail-mark-junk"
                                                    Accessible.name: text
                                                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                                    onTriggered: {
                                                        const operation = window.cardStore.enqueueOp(cardId, "bundle_spam", {bundle_id: card.bundle_id})
                                                        if (operation) window.showArchiveUndo(operation, "spam")
                                                    }
                                                }
                                            }
                                        }
                                    }
                                }
                                    }
                                }
                            }
                            Rectangle { id: bundleDivider; Layout.fillWidth: true; implicitHeight: 1; color: "#e0e9e6"; visible: cardRow.bundleLeader && cardRow.bundleExpanded }
                            GridLayout {
                                columns: 3
                                Layout.leftMargin: cardRow.bundled ? cardRow.bundleTextInset : 0
                                visible: !cardRow.bundleLeader || cardRow.bundleExpanded
                                Layout.fillWidth: true
                                Layout.minimumHeight: window.compactDensity ? 0 : 84
                                Layout.topMargin: cardRow.bundleLeader ? Kirigami.Units.smallSpacing : 0
                                columnSpacing: window.compactInbox ? 8 : Kirigami.Units.smallSpacing
                                rowSpacing: 0
                                ToolButton {
                                    id: dragHandle
                                    objectName: "reorderHandle-" + cardId
                                    Layout.row: 0
                                    Layout.column: 0
                                    // R63: on phones the sender avatar is the reorder handle;
                                    // a touch reorder starts only after a long press (R56).
                                    Layout.alignment: Qt.AlignTop
                                    Layout.minimumWidth: window.compactInbox ? 48 : -1
                                    Layout.preferredWidth: window.compactInbox ? 48 : -1
                                    Layout.maximumWidth: window.compactInbox ? 48 : Infinity
                                    visible: !cardRow.bundled
                                    implicitWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    // Loaded only where shown: collapsed bundle members stay cheap (scrollresilience).
                                    Loader {
                                        anchors.centerIn: parent
                                        active: window.compactInbox && cardRow.visible && !cardRow.bundled
                                        sourceComponent: SenderAvatar {
                                            name: card.source === "mail" ? window.mailSender(card) : window.cardKindLabel(card)
                                        }
                                    }
                                    readonly property bool pinned: card.pinned_rank != null || card.section === "pinned"
                                    readonly property bool reorderable: pinned || !card.timed
                                    // Timed rows stay visible but cannot be dragged;
                                    // their position is fixed by R30, not manual order.
                                    opacity: 1.0

                                    // The list-level surface keeps the pointer throughout
                                    // the drag, beyond this handle's hit target.
                                    display: AbstractButton.IconOnly
                                    icon.name: window.compactInbox ? "" : "transform-move"
                                    icon.width: Kirigami.Units.iconSizes.smallMedium
                                    icon.height: Kirigami.Units.iconSizes.smallMedium
                                    icon.color: window.ink

                                    Accessible.name: qsTr("Drag to reorder")
                                    Accessible.description: reorderable ? qsTr("Hold and drag to a new position") : qsTr("Position is fixed by pin or time")
                                    Accessible.role: Accessible.Button
                                    // A popup tooltip would cancel the pressed drag surface;
                                    // the named icon and cursor carry the affordance instead.
                                    HoverHandler { cursorShape: dragHandle.reorderable ? Qt.OpenHandCursor : Qt.ArrowCursor }
                                }
                                ColumnLayout {
                                    id: cardText
                                    objectName: "openCard-" + cardId
                                    Layout.row: 0
                                    Layout.column: 1
                                    Layout.columnSpan: 1
                                    function openCard() {
                                        if (card.source === "mail" || card.has_body) cardActions.readCachedRequested()
                                        else cardActions.openRequested()
                                    }
                                    activeFocusOnTab: card.source === "mail" || !!card.has_body || !!card.source_url
                                    Accessible.role: Accessible.Button
                                    Accessible.name: qsTr("Open %1").arg(title)
                                    Keys.onReturnPressed: openCard()
                                    Keys.onSpacePressed: openCard()
                                    // R63: long press opens the card's action sheet on touch.
                                    TapHandler { onTapped: cardText.openCard(); onLongPressed: cardActions.clicked() }
                                    HoverHandler { cursorShape: cardText.activeFocusOnTab ? Qt.PointingHandCursor : Qt.ArrowCursor }
                                    Keys.onPressed: function(event) {
                                        if (event.key === Qt.Key_Delete) {
                                            cardActions.primaryAction()
                                            event.accepted = true
                                        }
                                    }
                                    Shortcut {
                                        sequence: StandardKey.Delete
                                        enabled: cardText.activeFocus
                                        onActivated: cardActions.primaryAction()
                                    }
                                    Layout.minimumHeight: 48
                                    Layout.fillWidth: true
                                    spacing: window.compactInbox ? 2 : Kirigami.Units.smallSpacing
                                    Kirigami.Heading {
                                        objectName: "mailTitle-" + cardId
                                        text: title
                                        color: window.ink
                                        level: 4
                                        // Same as Kirigami's level-4 size without a per-row Theme object.
                                        font.pointSize: window.themeFont.pointSize * 1.10
                                        font.weight: Font.DemiBold
                                        wrapMode: !window.compactDensity || window.largeText ? Text.Wrap : Text.NoWrap
                                        maximumLineCount: !window.compactDensity || window.largeText ? 2 : 1
                                        elide: Text.ElideRight
                                        Layout.fillWidth: true
                                    }
                                    RowLayout {
                                        objectName: "mailContext-" + cardId
                                        visible: card.source === "mail" && (window.compactInbox || (cardRow.showMemberSender && mailSenderLabel.text.length > 0) || (cardRow.showMemberAccount && mailAccountLabel.text.length > 0 && (!window.compactInbox || cardRow.bundled)))
                                        Layout.fillWidth: true
                                        spacing: Kirigami.Units.smallSpacing
                                        Label {
                                            id: mailSenderLabel
                                            objectName: "mailSender-" + cardId
                                            text: window.mailSender(card)
                                            visible: text.length > 0 && cardRow.showMemberSender
                                            color: window.mutedInk
                                            font.family: window.themeFont.family
                                            font.pointSize: window.compactInbox ? window.themeFont.pointSize - 2 : window.themeFont.pointSize
                                            elide: Text.ElideRight
                                            Layout.minimumWidth: 0
                                            Layout.fillWidth: window.compactInbox
                                        }
                                        Label {
                                            id: mailAccountLabel
                                            objectName: "mailAccount-" + cardId
                                            text: card.account_name || ""
                                            visible: text.length > 0 && cardRow.showMemberAccount && (!window.compactInbox || cardRow.bundled)
                                            color: window.mutedInk
                                            font: window.themeFont
                                            elide: Text.ElideLeft
                                            Layout.maximumWidth: Math.min(180, cardText.width / 2)
                                        }
                                        Item { Layout.fillWidth: true; visible: window.compactInbox && !mailSenderLabel.visible }
                                        // R63: the arrival time sits on the header line, right-aligned.
                                        Label {
                                            objectName: "phoneMailDate-" + cardId
                                            visible: window.compactInbox && card.source === "mail" && text.length > 0
                                            text: card.source === "mail" ? window.mailArrival(card) : ""
                                            color: window.mutedInk
                                            font.pointSize: window.themeFont.pointSize - 2
                                        }
                                    }
                                    Label {
                                        objectName: "sourceLabel-" + cardId
                                        text: window.cardKindLabel(card) + (card.account_name ? " · " + card.account_name : "")
                                        visible: card.source !== "mail"
                                        color: window.mutedInk
                                        font.family: window.themeFont.family
                                        font.pointSize: window.compactInbox ? window.themeFont.pointSize - 2 : window.themeFont.pointSize
                                        wrapMode: window.compactInbox ? Text.NoWrap : Text.Wrap
                                        elide: Text.ElideRight
                                        Layout.fillWidth: true
                                    }
                                    Label {
                                        objectName: "mailSnippet-" + cardId
                                        text: card.source === "mail" ? window.cleanPreview(card.snippet) : (card.summary || "")
                                        visible: text.length > 0
                                        color: window.mutedInk
                                        font.family: window.themeFont.family
                                        font.pointSize: window.compactInbox ? window.themeFont.pointSize - 1 : window.themeFont.pointSize
                                        textFormat: Text.PlainText
                                        wrapMode: window.largeText || card.source !== "mail" ? Text.Wrap : Text.NoWrap
                                        // Large platform text reflows instead of clipping the content.
                                        maximumLineCount: window.largeText || card.source !== "mail" ? 2 : 1
                                        elide: Text.ElideRight
                                        Layout.fillWidth: true
                                    }
                                    Label { text: card.note || ""; color: window.mutedInk; visible: text.length > 0; font.italic: true; wrapMode: Text.Wrap; maximumLineCount: window.compactInbox ? 2 : Infinity; elide: Text.ElideRight; Layout.fillWidth: true }
                                    Label { text: window.clock(card); visible: text.length > 0; color: window.mutedInk; font: window.themeFont }
                                }
                                ColumnLayout {
                                    Layout.row: 0
                                    Layout.column: 2
                                    Layout.alignment: Qt.AlignTop
                                    Layout.preferredWidth: window.compactInbox ? 96 : Math.max(120, Kirigami.Units.gridUnit * 6)
                                    Layout.minimumWidth: window.compactInbox ? 96 : Math.max(120, Kirigami.Units.gridUnit * 6)
                                    Layout.maximumWidth: window.compactInbox ? 96 : Math.max(120, Kirigami.Units.gridUnit * 6)
                                    spacing: window.compactInbox ? 0 : Kirigami.Units.smallSpacing
                                    Label {
                                        objectName: "mailDate-" + cardId
                                        text: window.mailArrival(card)
                                        visible: !window.compactInbox && card.source === "mail" && text.length > 0
                                        color: window.mutedInk
                                        font: window.themeFont
                                        Layout.fillWidth: true
                                        horizontalAlignment: Text.AlignRight
                                    }
                                    GridLayout {
                                        // R63: phones keep one narrow overflow beside the archive; no resting icon row.
                                        Layout.alignment: Qt.AlignRight
                                        columns: 2
                                        layoutDirection: window.compactInbox ? Qt.RightToLeft : Qt.LeftToRight
                                        rowSpacing: 0
                                        columnSpacing: window.compactInbox ? 0 : Kirigami.Units.smallSpacing
                                ToolButton {
                                    objectName: "doneButton-" + cardId
                                    text: card.source === "mail" || card.source === "journal" ? qsTr("Archive") : qsTr("Done")
                                    icon.name: card.source === "mail" ? "mail-mark-read-symbolic" : "dialog-ok"
                                    icon.width: window.actionIconSize
                                    icon.height: window.actionIconSize
                                    icon.color: window.ink
                                    display: AbstractButton.IconOnly
                                    implicitWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    opacity: 1
                                    Accessible.name: cardActions.primaryName
                                    ToolTip.text: Accessible.name
                                    ToolTip.visible: hovered || visualFocus
                                    onClicked: cardActions.primaryAction()
                                }
                                Litterbox.CardActions {
                                    id: cardActions
                                    objectName: "inboxActions"
                                    implicitWidth: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: window.compactInbox ? 48 : Math.max(48, Kirigami.Units.gridUnit * 3)
                                    icon.width: window.compactInbox ? 16 : 20
                                    icon.height: window.compactInbox ? 16 : 20
                                    store: window.cardStore
                                    cardKey: cardId
                                    source: card.source || ""
                                    sourceLabel: window.cardStore.sourceLabel(card)
                                    hasBody: !!card.has_body
                                    bundleId: card.bundle_id || ""
                                    pinnedRank: card.pinned_rank
                                    cardTitle: title
                                    accountName: card.account_name || ""
                                    canOpenSource: !!card.source_url
                                    onCardHandled: function(operation, action) { if (operation) window.showArchiveUndo(operation, action) }
                                    onNoteRequested: { noteDialog.cardId = cardId; noteField.text = card.note || ""; noteDialog.open() }
                                    onOpenRequested: {
                                        if (card.source_url) Qt.openUrlExternally(card.source_url)
                                    }
                                    onReadCachedRequested: window.openPage("MailDetailPage", {
                                        store: window.cardStore, cardId: cardId, cardTitle: title,
                                        accountName: card.account_name || "", card: card,
                                        requestNote: function(detailCardId, note) {
                                            noteDialog.cardId = detailCardId
                                            noteField.text = note
                                            noteDialog.open()
                                        },
                                        requestArchiveUndo: function(opId, action) { window.showArchiveUndo(opId, action) }
                                    })
                                }
                                    }
                                }
                            }
                        }
                    }
                    // Keep overlays outside the Frame so its height follows only its content.
                    Rectangle {
                        objectName: "dropIndicator"
                        visible: window.dragCardId.length > 0 && window.dragCardId !== cardId && window.cardIdAt(window.dragTargetIndex) === cardId
                        anchors.top: cardFrame.top
                        anchors.left: cardFrame.left
                        anchors.right: cardFrame.right
                        height: 2
                        color: window.themeHighlight
                        z: 1
                    }
                }
            }

            Item {
                objectName: "inboxEmptyState"
                anchors.fill: parent
                visible: inboxList.count === 0
                z: 2
                ColumnLayout {
                    anchors.centerIn: parent
                    width: Math.min(parent.width - 2 * window.edgeSpacing, 420)
                    spacing: Kirigami.Units.mediumSpacing
                    Kirigami.Icon {
                        source: "mail-mark-read"
                        color: window.accent
                        implicitWidth: Kirigami.Units.iconSizes.large
                        implicitHeight: implicitWidth
                        Layout.alignment: Qt.AlignHCenter
                    }
                    Kirigami.Heading {
                        text: window.searching ? qsTr("No matching cards") : qsTr("You're all caught up")
                        color: window.ink
                        level: 3
                        horizontalAlignment: Text.AlignHCenter
                        Layout.fillWidth: true
                    }
                    Label {
                        text: window.searching ? qsTr("Try another subject, sender, account or source, or clear the search to return to your inbox.") : qsTr("New cards will appear here. Add a private note or task whenever you need one.")
                        color: window.mutedInk
                        wrapMode: Text.Wrap
                        horizontalAlignment: Text.AlignHCenter
                        Layout.fillWidth: true
                    }
                    Button {
                        text: window.searching ? qsTr("Clear search") : qsTr("Add card")
                        icon.name: window.searching ? "edit-clear" : "list-add"
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        Layout.alignment: Qt.AlignHCenter
                        onClicked: {
                            if (window.searching) { window.searchQuery = ""; inboxSearch.clear(); inboxList.forceActiveFocus() }
                            else createDialog.open()
                        }
                    }
                }
            }
            // The accepting surface for the reorder drag. It must be as tall as the drag
            // travel: under delivered pointer events a pressed item stops receiving the
            // pointer as soon as it leaves its own 32px bounds, so a handle-sized press
            // target can never drive a reorder (probe15). It is confined to the handle
            // column instead of covering the list, so it covers no button, no card
            // action and no card body and those keep working unchanged (a full-area
            // surface was measured to swallow a Controls button click, probe17); a
            // press that is not on a reorderable handle is not accepted.
            MouseArea {
                id: reorderSurface
                objectName: "reorderSurface"
                property string draggingId: ""
                property real pressY: 0
                property real pointerY: 0
                property bool moved: false
                // R56/R63: a touch press on the avatar handle only scrolls until it is
                // held; the reorder arms on press-and-hold. Mouse presses drag at once.
                property bool touchPending: false
                property real touchLastY: 0
                pressAndHoldInterval: 400
                function updateTarget() {
                    const point = mapToItem(inboxList, 24, pointerY)
                    const target = inboxList.indexAt(point.x, point.y + inboxList.contentY)
                    const ids = []
                    for (let i = 0; i < inboxList.count; ++i) ids.push(window.cardIdAt(i))
                    const handle = window.cardHandles[draggingId]
                    if (!handle || target < 0) { window.dragTargetIndex = -1; return }
                    const from = ids.indexOf(draggingId)
                    if (handle.pinned) window.dragTargetIndex = Math.min(target, store.pinnedCardIds().length - 1)
                    else {
                        const targetHandle = window.cardHandles[ids[target]]
                        window.dragTargetIndex = targetHandle && targetHandle.reorderable && !targetHandle.pinned ? target : from
                    }
                }
                Timer {
                    interval: 40
                    repeat: true
                    running: reorderSurface.moved && reorderSurface.draggingId.length > 0
                    onTriggered: {
                        const delta = reorderSurface.pointerY < 48 ? -12 : reorderSurface.pointerY > reorderSurface.height - 48 ? 12 : 0
                        if (!delta) return
                        const before = inboxList.contentY
                        inboxList.contentY = Math.max(0, Math.min(inboxList.contentHeight - inboxList.height, before + delta))
                        window.dragOffset += inboxList.contentY - before
                        reorderSurface.updateTarget()
                    }
                }
                // Same column as the `=` handle inside the centred card frame.
                x: (parent.width - Math.min(parent.width - 32, 1200)) / 2
                y: 0
                width: 48
                height: parent.height
                // A press on a handle keeps the pointer for the whole travel: the list
                // is a Flickable and would otherwise take the grab mid-drag, which
                // cancels the drag before the drop can be committed. Flicking is
                // unaffected elsewhere: a press that is not on a reorderable handle is
                // not accepted by this surface, so the list still gets it.
                preventStealing: true
                function startDragAt(mouse) {
                    const ids = Object.keys(window.cardHandles)
                    for (let i = 0; i < ids.length; ++i) {
                        const handle = window.cardHandles[ids[i]]
                        if (!handle || !handle.visible) continue
                        const p = handle.mapToItem(reorderSurface, 0, 0)
                        if (mouse.x >= p.x && mouse.x <= p.x + handle.width
                                && mouse.y >= p.y && mouse.y <= p.y + handle.height) {
                            if (!handle.reorderable) {
                                window.dragFeedback = qsTr("This card is fixed to its scheduled time")
                                mouse.accepted = false
                                return
                            }
                            // Take the card id from the rendered handle: a recycled
                            // delegate keeps its original registry key, so the map
                            // entry can be stale.
                            const cardKey = String(handle.objectName).replace("reorderHandle-", "")
                            if (cardKey !== ids[i]) {
                                delete window.cardHandles[ids[i]]
                                window.cardHandles[cardKey] = handle
                            }
                            reorderSurface.draggingId = cardKey
                            reorderSurface.pressY = mouse.y
                            reorderSurface.pointerY = mouse.y
                            reorderSurface.moved = false
                            window.dragOffset = 0
                            window.dragFeedback = ""
                            window.dragTargetIndex = -1
                            mouse.accepted = true
                            return
                        }
                    }
                    mouse.accepted = false
                }
                onPressed: function (mouse) {
                    reorderSurface.startDragAt(mouse)
                    reorderSurface.touchPending = mouse.accepted && mouse.source !== Qt.MouseEventNotSynthesized
                    reorderSurface.touchLastY = mouse.y
                }
                onPressAndHold: function (mouse) {
                    if (reorderSurface.touchPending && reorderSurface.draggingId.length > 0) {
                        reorderSurface.touchPending = false
                        reorderSurface.pressY = mouse.y
                        reorderSurface.pointerY = mouse.y
                        window.dragCardId = reorderSurface.draggingId
                    }
                }
                // A press that lands on the same point as the previous release within
                // the double-click interval arrives as a double click, not as a press;
                // grabbing the handle twice in a row must still start a drag.
                onDoubleClicked: function (mouse) {
                    reorderSurface.startDragAt(mouse)
                }
                onPositionChanged: function (mouse) {
                    if (reorderSurface.draggingId.length === 0) {
                        mouse.accepted = false
                        return
                    }
                    if (touchPending) {
                        // Not held yet: pass the finger travel to the list as a scroll.
                        const maxY = Math.max(0, inboxList.contentHeight - inboxList.height)
                        inboxList.contentY = Math.max(0, Math.min(maxY, inboxList.contentY + touchLastY - mouse.y))
                        touchLastY = mouse.y
                        return
                    }
                    pointerY = mouse.y
                    if (!moved && Math.abs(mouse.y - pressY) < 8) return
                    moved = true
                    window.dragCardId = draggingId
                    window.dragOffset = mouse.y - pressY
                    updateTarget()
                }
                onReleased: function (mouse) {
                    if (reorderSurface.draggingId.length === 0) {
                        mouse.accepted = false
                        return
                    }
                    if (touchPending) {
                        touchPending = false
                        reorderSurface.draggingId = ""
                        window.dragCardId = ""
                        return
                    }
                    const cardId = reorderSurface.draggingId
                    pointerY = mouse.y
                    updateTarget()
                    const target = window.dragTargetIndex
                    const shouldCommit = moved
                    reorderSurface.draggingId = ""
                    window.dragCardId = ""
                    window.dragTargetIndex = -1
                    window.dragOffset = 0
                    moved = false
                    if (shouldCommit && target >= 0)
                        window.commitCardDrag(cardId, target)
                }
                // canceled() carries no event argument; a drag that was taken away is
                // never committed from here.
                onCanceled: function () {
                    touchPending = false
                    reorderSurface.draggingId = ""
                    window.dragCardId = ""
                    window.dragTargetIndex = -1
                    window.dragOffset = 0
                    moved = false
                }
            }
        }
    }
        Item {
            id: readerHost
            objectName: "desktopReaderHost"
            visible: window.desktopReader && stack.depth === 1
            SplitView.minimumWidth: 520
            SplitView.fillWidth: true
            clip: true
            Rectangle { anchors.fill: parent; color: window.surface; z: -1 }
            ColumnLayout {
                anchors.centerIn: parent
                width: Math.min(parent.width - 48, 400)
                visible: !window.readerOpen
                spacing: Kirigami.Units.largeSpacing
                Kirigami.Icon {
                    source: "mail-message"
                    color: window.mutedInk
                    Layout.preferredWidth: 48
                    Layout.preferredHeight: 48
                    Layout.alignment: Qt.AlignHCenter
                }
                Kirigami.Heading {
                    text: qsTr("Select a card to read")
                    color: window.ink
                    level: 2
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                Label {
                    text: qsTr("Your inbox stays here while you read. Drag the divider to adjust the panes.")
                    color: window.mutedInk
                    wrapMode: Text.Wrap
                    horizontalAlignment: Text.AlignHCenter
                    Layout.fillWidth: true
                }
            }
        }
    }
    // Extra-button mouse events are not reliably delivered to TapHandler by
    // all Qt platform plugins, so use a MouseArea restricted to Back.
    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.BackButton
        onClicked: window.handleBack()
    }
    Dialog {
        id: noteDialog
        Material.theme: Material.Light
        Material.background: window.surface
        Material.foreground: window.ink
        Material.accent: window.accent
        Material.primary: window.accent
        property string cardId
        title: "Card note"
        modal: true
        width: Math.min(392, window.width - 32)
        implicitWidth: width
        background: Rectangle { color: window.surface; radius: 8; border.color: "#dce5e3" }
        anchors.centerIn: parent
        // Explicit English buttons: StandardButton labels follow the system
        // locale (be_BY shows Belarusian) while the app is English, and the
        // style paints them from the host color scheme. Explicit light
        // backgrounds keep them legible in every theme.
        footer: DialogButtonBox {
            background: Rectangle { color: window.surface }
            Button {
                id: noteSaveButton
                objectName: "noteSaveButton"
                text: qsTr("Save")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: window.surface
                palette.buttonText: window.ink
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: noteSaveButton.down ? "#e8eeed" : noteSaveButton.hovered ? "#eef3f2" : window.surface
                    border.width: noteSaveButton.visualFocus ? 2 : 1
                    border.color: noteSaveButton.visualFocus ? window.ink : "#dce5e3"
                }
                onClicked: noteDialog.accept()
            }
            Button {
                id: noteCancelButton
                objectName: "noteCancelButton"
                text: qsTr("Cancel")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: window.surface
                palette.buttonText: window.ink
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: noteCancelButton.down ? "#e8eeed" : noteCancelButton.hovered ? "#eef3f2" : window.surface
                    border.width: noteCancelButton.visualFocus ? 2 : 1
                    border.color: noteCancelButton.visualFocus ? window.ink : "#dce5e3"
                }
                onClicked: noteDialog.reject()
            }
        }
        onOpened: noteField.forceActiveFocus()
        // KDE Breeze TextArea assigns its TextArea target to a TextInput-only
        // mobile toolbar and aborts desktop root creation. Keep multiline edit
        // semantics using QtQuick.TextEdit inside a styled, scrollable frame.
        contentItem: Frame {
            implicitWidth: 0
            implicitHeight: 120
            padding: Kirigami.Units.smallSpacing
            background: Rectangle { color: window.surface; border.color: "#dce5e3"; radius: 4 }
            Flickable {
                id: noteScroll
                anchors.fill: parent
                clip: true
                contentWidth: width
                contentHeight: Math.max(height, noteField.contentHeight)
                boundsBehavior: Flickable.StopAtBounds
                TextEdit {
                    id: noteField
                    width: noteScroll.width
                    height: Math.max(noteScroll.height, contentHeight)
                    wrapMode: TextEdit.Wrap
                    selectByMouse: true
                    color: Kirigami.Theme.textColor
                    selectionColor: window.themeHighlight
                    selectedTextColor: Kirigami.Theme.highlightedTextColor
                    Accessible.role: Accessible.EditableText
                }
            }
        }
        onAccepted: store.saveNote(cardId, noteField.text)
    }
    Dialog {
        id: createDialog
        objectName: "createDialog"
        readonly property bool journal: createKind.currentIndex === 1
        Material.theme: Material.Light
        Material.background: window.surface
        Material.foreground: window.ink
        Material.accent: window.accent
        Material.primary: window.accent
        title: qsTr("Create card")
        modal: true
        width: Math.min(392, window.width - 32)
        implicitWidth: width
        background: Rectangle { color: window.surface; radius: 8; border.color: "#dce5e3" }
        anchors.centerIn: parent
        // Explicit English buttons (see noteDialog): no locale-dependent
        // StandardButton labels, explicit light background in any style.
        footer: DialogButtonBox {
            background: Rectangle { color: window.surface }
            Button {
                id: createSaveButton
                objectName: "createSaveButton"
                text: qsTr("Create")
                enabled: (createDialog.journal ? createJournal.text : createTitle.text).trim().length > 0
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: window.surface
                palette.buttonText: window.ink
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: createSaveButton.down ? "#e8eeed" : createSaveButton.hovered ? "#eef3f2" : window.surface
                    border.width: createSaveButton.visualFocus ? 2 : 1
                    border.color: createSaveButton.visualFocus ? window.ink : "#dce5e3"
                }
                onClicked: {
                    if (store.createCard(createDialog.journal ? createJournal.text : createTitle.text,
                                         createDialog.journal ? "" : createSummary.text,
                                         createDialog.journal ? "journal" : "manual")) {
                        createTitle.clear()
                        createSummary.clear()
                        createJournal.clear()
                        createDialog.accept()
                    }
                }
            }
            Button {
                id: createCancelButton
                objectName: "createCancelButton"
                text: qsTr("Cancel")
                implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                palette.button: window.surface
                palette.buttonText: window.ink
                background: Rectangle {
                    radius: Kirigami.Units.cornerRadius
                    color: createCancelButton.down ? "#e8eeed" : createCancelButton.hovered ? "#eef3f2" : window.surface
                    border.width: createCancelButton.visualFocus ? 2 : 1
                    border.color: createCancelButton.visualFocus ? window.ink : "#dce5e3"
                }
                onClicked: createDialog.reject()
            }
        }
        onOpened: (journal ? createJournal : createTitle).forceActiveFocus()
        contentItem: ColumnLayout {
            ComboBox {
                id: createKind
                objectName: "createKind"
                model: [qsTr("Task"), qsTr("Private journal note")]
                Layout.fillWidth: true
                Accessible.name: qsTr("Card kind")
                onActivated: (createDialog.journal ? createJournal : createTitle).forceActiveFocus()
            }
            TextField { id: createTitle; objectName: "createTitle"; visible: !createDialog.journal; placeholderText: qsTr("Title"); Layout.fillWidth: true }
            TextField { id: createSummary; objectName: "createSummary"; visible: !createDialog.journal; placeholderText: qsTr("Details (optional)"); Layout.fillWidth: true }
            ScrollView {
                visible: createDialog.journal
                Layout.fillWidth: true
                Layout.preferredHeight: 180
                TextArea {
                    id: createJournal
                    objectName: "createJournal"
                    placeholderText: qsTr("Write a private note…")
                    textFormat: TextEdit.PlainText
                    wrapMode: TextEdit.Wrap
                }
            }
            Label {
                visible: createDialog.journal
                text: qsTr("Private · saved offline and synced to your inbox. Your assistant can read it, not write it.")
                color: window.mutedInk
                wrapMode: Text.Wrap
                Layout.fillWidth: true
            }
        }
    }
    Dialog {
        id: updateDialog
        title: qsTr("Install Android update")
        modal: true
        width: Math.min(392, window.width - 32)
        implicitWidth: width
        background: Rectangle { color: window.surface; radius: 8; border.color: "#dce5e3" }
        anchors.centerIn: parent
        contentItem: ColumnLayout {
            Label {
                Layout.fillWidth: true
                wrapMode: Text.Wrap
                text: qsTr("Version %1 is downloaded and checksum-verified. Android will ask you to confirm installation.").arg(window.updateVersion)
            }
            RowLayout {
                Layout.alignment: Qt.AlignRight
                Button { text: qsTr("Later"); onClicked: updateDialog.close() }
                Button {
                    text: qsTr("Install")
                    onClicked: updater.installDownloadedUpdate()
                }
            }
        }
    }
}
