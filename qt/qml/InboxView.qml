import QtQuick
import QtQml.Models
import QtQuick.Controls
import QtQuick.Controls.Material
import QtQuick.Layouts
import org.kde.kirigami as Kirigami
import litterbox 1.0 as Litterbox

ApplicationWindow {
    id: window
    visible: true
    width: 520
    height: 800
    title: "Litterbox"
    // Sol V5 owns a light content surface, independent of the host's color scheme.
    // Set both Controls and Kirigami colors: neither may inherit dark-theme ink
    // while the other paints the approved white cards and header.
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    readonly property real edgeSpacing: Math.max(18, Kirigami.Units.largeSpacing)
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
        const title = card.bundle_title || ""
        if (!title.startsWith("sender:")) return title || qsTr("Bundle")
        const summary = bundleSummaries[card.bundle_id]
        if (summary && summary.senderLabel) return summary.senderLabel
        const sender = title.slice(7)
        const domain = sender.slice(sender.lastIndexOf("@") + 1).replace(/[<>]/g, "")
        return domain || qsTr("Bundle")
    }
    function toggleBundle(bundleId) {
        const next = Object.assign({}, expandedBundles)
        next[bundleId] = !next[bundleId]
        expandedBundles = next
        bundleModel.rebuildPresentation(false)
        Qt.callLater(function() {
            const opener = bundleOpeners[bundleId]
            if (opener) opener.forceActiveFocus()
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
        bundleArchiveStatus = qsTr("Archived %1").arg(emailCountText(result.count))
        return true
    }
    function undoBundleArchive() {
        const restored = bundleArchiveToken.length > 0 && store.undoBundleArchive(bundleArchiveToken)
        bundleArchiveToken = ""
        if (restored) {
            bundleArchiveStatus = ""
        } else {
            bundleArchiveStatus = qsTr("Undo expired")
            undoExpiredNoticeTimer.restart()
        }
        return restored
    }
    property string updateStatus: ""
    property string updateVersion: ""
    property string bundleArchiveToken: ""
    property string bundleArchiveStatus: ""
    Connections {
        target: store
        ignoreUnknownSignals: true
        function onBundleArchiveExpired(token) {
            if (token !== window.bundleArchiveToken) return
            window.bundleArchiveToken = ""
            window.bundleArchiveStatus = qsTr("Undo expired")
            undoExpiredNoticeTimer.restart()
        }
    }
    Timer {
        id: undoExpiredNoticeTimer
        interval: 2200
        repeat: false
        onTriggered: if (window.bundleArchiveStatus === qsTr("Undo expired")) window.bundleArchiveStatus = ""
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
        if (Qt.platform.os === "android" && stack.depth > 1) {
            stack.pop()
            close.accepted = false
        } else {
            close.accepted = true
        }
    }
    function handleBack() {
        if (stack.depth <= 1) return false
        stack.pop()
        return true
    }
    property real inboxScrollPosition: 0
    property var mailDetailPage: null
    function openPage(name, properties) {
        if (stack.depth === 1) inboxScrollPosition = inboxList.contentY - inboxList.originY
        const page = pagesDir.toString() + name + ".qml"
        if (name === "MailDetailPage") {
            const values = Object.assign({store: window.cardStore, cardTitle: "", accountName: "", openLinks: true}, properties || {})
            if (!mailDetailPage) {
                // An existing item is not owned/destroyed by StackView on pop.
                const component = Qt.createComponent(page)
                mailDetailPage = component.createObject(stack, values)
            } else {
                for (const key of Object.keys(values)) mailDetailPage[key] = values[key]
                mailDetailPage.reload()
            }
            stack.push(mailDetailPage)
        } else {
            stack.push(page, properties || { api: api })
        }
    }
    function clock(card) { return card.timed ? timeRules.display(card.at || "") : "" }

    header: ToolBar {
        padding: window.edgeSpacing
        background: Rectangle { color: window.surface }
        ColumnLayout {
            width: Math.min(parent.width - 2 * window.edgeSpacing, 1200)
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: Kirigami.Units.smallSpacing
            RowLayout {
                spacing: Kirigami.Units.mediumSpacing
                Layout.fillWidth: true
                Kirigami.Heading { text: qsTr("Inbox"); color: window.ink; level: 2; Layout.fillWidth: true }
                Button {
                    id: addCardButton
                    objectName: "addCardButton"
                    text: qsTr("Add card")
                    icon.name: "list-add"
                    icon.width: Kirigami.Units.iconSizes.small
                    icon.height: Kirigami.Units.iconSizes.small
                    icon.color: window.surface
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    leftPadding: Kirigami.Units.largeSpacing
                    rightPadding: Kirigami.Units.largeSpacing
                    Material.foreground: window.surface
                    palette.buttonText: window.surface
                    background: Rectangle {
                        radius: Kirigami.Units.cornerRadius
                        color: addCardButton.down ? "#286358" : addCardButton.hovered ? "#326f65" : window.accent
                        border.width: addCardButton.visualFocus ? 2 : 0
                        border.color: window.ink
                    }
                    onClicked: createDialog.open()
                    TapHandler { onTapped: createDialog.open() }
                }
                ToolButton {
                    icon.name: "application-menu"
                    icon.width: Kirigami.Units.iconSizes.smallMedium
                    icon.height: Kirigami.Units.iconSizes.smallMedium
                    text: qsTr("Inbox commands")
                    display: AbstractButton.IconOnly
                    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                    Accessible.name: qsTr("Inbox commands")
                    ToolTip.text: Accessible.name
                    onClicked: headerMenu.open()
                    Menu {
                        id: headerMenu
                        MenuItem { text: qsTr("Accounts"); icon.name: "mail-receive"; implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3); onTriggered: openPage("GmailAccountsPage") }
                        MenuItem { text: qsTr("Enroll device"); icon.name: "user-identity"; implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3); onTriggered: openPage("EnrollmentPage") }
                        MenuItem { text: qsTr("Refresh"); icon.name: "view-refresh"; implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3); onTriggered: store.refresh() }
                        MenuItem { text: qsTr("Check updates"); icon.name: "system-software-update"; implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3); visible: updater.supported; enabled: !updater.busy; onTriggered: updater.checkForUpdates() }
                    }
                }
            }
            Label { text: store.online ? qsTr("Online · changes sync across devices") : qsTr("Offline · changes saved on this device"); color: window.mutedInk; font: Kirigami.Theme.defaultFont; Layout.fillWidth: true; wrapMode: Text.Wrap }
            Label { text: window.updateStatus; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true }
            Rectangle {
                id: bundleArchiveUndoBar
                objectName: "bundleArchiveUndoBar"
                visible: window.bundleArchiveStatus.length > 0
                Layout.fillWidth: true
                implicitHeight: undoBarRow.implicitHeight + Kirigami.Units.smallSpacing * 2
                radius: Kirigami.Units.cornerRadius
                color: window.surface
                border.color: "#dce5e3"
                RowLayout {
                    id: undoBarRow
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Kirigami.Units.smallSpacing
                    anchors.rightMargin: Kirigami.Units.smallSpacing
                    spacing: 0
                    Label {
                        objectName: "bundleArchiveUndoMessage"
                        text: window.bundleArchiveStatus === qsTr("Undo expired")
                            ? window.bundleArchiveStatus
                            : window.bundleArchiveStatus + " ·"
                        color: window.ink
                        font: Kirigami.Theme.defaultFont
                        Layout.fillWidth: true
                        wrapMode: Text.Wrap
                    }
                    Button {
                        id: bundleArchiveUndoButton
                        objectName: "bundleArchiveUndoButton"
                        visible: window.bundleArchiveStatus !== qsTr("Undo expired")
                        text: qsTr("Undo")
                        implicitWidth: Math.max(48, implicitContentWidth + leftPadding + rightPadding)
                        implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                        leftPadding: Kirigami.Units.smallSpacing
                        rightPadding: Kirigami.Units.smallSpacing
                        Material.theme: Material.Light
                        Material.foreground: window.accent
                        palette.buttonText: window.accent
                        background: Rectangle {
                            radius: Kirigami.Units.cornerRadius
                            color: bundleArchiveUndoButton.down ? "#e7f1ee" : bundleArchiveUndoButton.hovered ? "#f3f7f6" : window.surface
                            border.width: bundleArchiveUndoButton.visualFocus ? 2 : 0
                            border.color: window.accent
                        }
                        onClicked: window.undoBundleArchive()
                    }
                }
            }
             Label { text: window.dragFeedback; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true; Accessible.role: Accessible.AlertMessage }
        }
    }
    Connections {
        target: updater
        function onBusyChanged() { window.updateStatus = updater.busy ? qsTr("Checking for updates…") : "" }
        function onNoUpdateAvailable() { window.updateStatus = qsTr("No update available") }
        function onUpdateReady(version, sha256) {
            window.updateVersion = version
            window.updateStatus = qsTr("Update %1 downloaded and verified").arg(version)
            updateDialog.open()
        }
        function onInstallConsentRequired() {
            window.updateStatus = qsTr("Allow installs from Litterbox in Android settings, then tap Install again")
        }
        function onErrorOccurred(message) { window.updateStatus = message }
    }
    StackView {
        id: stack
        objectName: "pageStack"
        anchors.fill: parent
        focus: true
        // Sliding the inbox moves its ListView viewport out of the window,
        // causing whole rows of Controls to be destroyed and recreated.
        pushExit: null
        popEnter: null
        Keys.onBackPressed: function(event) {
            if (window.handleBack()) event.accepted = true
        }
        initialItem: Item {
            StackView.visible: true
            enabled: StackView.status === StackView.Active
            StackView.onActivated: {
                inboxList.forceLayout()
                inboxList.contentY = inboxList.originY + window.inboxScrollPosition
                inboxList.forceActiveFocus()
            }
            ListView {
                id: inboxList
                objectName: "inboxList"
                anchors.fill: parent
                clip: true
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
                        inboxList.contentY = Math.max(0, Math.min(inboxList.contentHeight - inboxList.height, inboxList.contentY - delta))
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
                            if (refresh && card.bundle_id) {
                                const summary = summaries[card.bundle_id] || (summaries[card.bundle_id] = {accounts: [], senders: [], important: 0, unpinned: 0, lastId: ""})
                                if (card.account_name && summary.accounts.indexOf(card.account_name) < 0) summary.accounts.push(card.account_name)
                                if (card.pinned_rank == null && card.section !== "pinned") {
                                    ++summary.unpinned
                                    if (card.important) ++summary.important
                                }
                                if (card.bundle_leader !== undefined) {
                                    summary.lastId = entry.cardId
                                    const sender = typeof card.sender_name === "string" ? card.sender_name.trim() : ""
                                    if (sender && summary.senders.indexOf(sender) < 0) summary.senders.push(sender)
                                }
                            }
                            if (card.bundle_leader === true) {
                                sections[card.bundle_id] = card.section
                                groups.push([entry])
                                if (window.expandedBundles[card.bundle_id]) {
                                    const bucket = members[card.bundle_id] || (members[card.bundle_id] = [])
                                    groups.push(bucket)
                                }
                            }
                            else if (card.bundle_leader === false && window.expandedBundles[card.bundle_id]) {
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
                            window.bundleSummaries = summaries
                        }
                        const ordered = [].concat(...groups)
                        const headings = {}
                        let previousSection = ""
                        for (const entry of ordered) {
                            const card = entry.card
                            if (card.bundle_leader === false && !window.expandedBundles[card.bundle_id]) continue
                            const section = card.bundle_leader !== undefined ? sections[card.bundle_id] : card.section
                            headings[entry.cardId] = section !== previousSection
                            previousSection = section
                        }
                        window.sectionStarts = headings
                        const position = inboxList.contentY - inboxList.originY
                        if (presentationModel.count > ordered.length)
                            presentationModel.remove(ordered.length, presentationModel.count - ordered.length)
                        rows = ordered
                        while (presentationModel.count < ordered.length)
                            presentationModel.append({slot: presentationModel.count})
                        inboxList.forceLayout()
                        inboxList.contentY = inboxList.originY + position
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
                    readonly property string displaySection: card.bundle_leader !== undefined
                        ? (window.bundleSections[card.bundle_id] || card.section) : card.section
                    HoverHandler { id: rowHover }
                    readonly property bool lifted: window.dragCardId === cardId
                    readonly property bool bundled: !!card.bundle_id && card.section !== "pinned" && card.pinned_rank == null && !card.important
                    readonly property bool bundleExpanded: !!window.expandedBundles[card.bundle_id]
                    readonly property bool bundleLeader: bundled && card.bundle_leader === true
                    readonly property var bundleSummary: window.bundleSummaries[card.bundle_id] || {accounts: [], important: 0, unpinned: 0, lastId: cardId}
                    readonly property bool lastMember: bundleSummary.lastId === cardId
                    readonly property real bundleTextInset: bundleToggle.leftPadding + Kirigami.Units.iconSizes.smallMedium + Kirigami.Units.smallSpacing
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
                    height: (!bundled || card.bundle_leader !== false || bundleExpanded) ? sectionHeader.height + cardFrame.implicitHeight + (bundled && bundleExpanded && !lastMember ? 0 : window.edgeSpacing) : 0
                    z: lifted ? 10 : 0
                    Rectangle { anchors.fill: parent; color: "#eef3f2"; visible: cardRow.lifted; radius: 8 }
                    Item {
                        id: sectionHeader
                        width: parent.width
                        height: window.sectionStarts[cardId] ? sectionHeading.implicitHeight + 2 * window.edgeSpacing : 0
                        visible: height > 0
                        Label {
                            id: sectionHeading
                            width: Math.min(parent.width - 2 * window.edgeSpacing, 1200)
                            anchors.horizontalCenter: parent.horizontalCenter
                            anchors.top: parent.top
                            anchors.topMargin: window.edgeSpacing
                            text: (cardRow.displaySection === "pinned" ? qsTr("Pinned") : cardRow.displaySection === "now" ? qsTr("Now") : cardRow.displaySection === "later" ? qsTr("Later") : qsTr("Missed")).toLocaleUpperCase()
                            font: Kirigami.Theme.defaultFont
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
                        background: Rectangle {
                            color: cardRow.lifted ? "#e4efed" : window.surface
                            border.color: cardRow.lifted ? window.accent : cardRow.bundled ? "#cbded8" : "#edf0ef"
                            border.width: cardRow.bundled && cardRow.bundleExpanded ? 0 : 1
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
                            RowLayout {
                                visible: cardRow.bundleLeader
                                Layout.fillWidth: true
                                spacing: Kirigami.Units.smallSpacing
                                Button {
                                    id: bundleToggle
                                    objectName: "bundleToggle-" + cardId
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 48
                                    implicitHeight: Math.max(48, contentItem.implicitHeight + 12)
                                    padding: 6
                                    focusPolicy: Qt.StrongFocus
                                    text: window.bundleTitle(card)
                                    icon.name: cardRow.bundleExpanded ? "arrow-down" : "arrow-right"
                                    Accessible.name: (cardRow.bundleExpanded ? qsTr("Collapse %1, %2 emails") : qsTr("Expand %1, %2 emails")).arg(text).arg(card.bundle_member_count || 0)
                                    background: Rectangle {
                                        color: bundleToggle.down ? "#e7f1ee" : "transparent"
                                        radius: Kirigami.Units.cornerRadius
                                        border.width: bundleToggle.visualFocus ? 2 : 0
                                        border.color: window.accent
                                    }
                                    contentItem: ColumnLayout {
                                        spacing: Kirigami.Units.smallSpacing
                                        RowLayout {
                                            Layout.fillWidth: true
                                            spacing: Kirigami.Units.smallSpacing
                                            Kirigami.Icon { source: bundleToggle.icon.name; color: window.ink; isMask: true; implicitWidth: Kirigami.Units.iconSizes.smallMedium; implicitHeight: implicitWidth }
                                            Label { objectName: "bundleTitleLabel-" + cardId; text: bundleToggle.text; color: window.ink; font: Qt.font({family: Kirigami.Theme.defaultFont.family, pointSize: Kirigami.Theme.defaultFont.pointSize, bold: true}); wrapMode: Text.Wrap; Layout.fillWidth: true; Layout.maximumWidth: implicitWidth }
                                            Rectangle {
                                                implicitWidth: countLabel.implicitWidth + 16
                                                implicitHeight: countLabel.implicitHeight + 8
                                                radius: Kirigami.Units.cornerRadius
                                                color: "#e7f1ee"
                                                Label { id: countLabel; anchors.centerIn: parent; text: window.emailCountText(card.bundle_member_count || 0); color: window.ink; font: Kirigami.Theme.defaultFont }
                                            }
                                            Item { Layout.fillWidth: true }
                                        }
                                        Label {
                                            objectName: "bundleAccountLabel-" + cardId
                                            text: (cardRow.bundleSummary.senderLabel || bundleToggle.text) + (cardRow.bundleSummary.accounts.length > 1 ? " · " + qsTr("%1 accounts").arg(cardRow.bundleSummary.accounts.length) : card.account_name ? " · " + card.account_name : "")
                                            color: window.mutedInk
                                            font: Kirigami.Theme.defaultFont
                                            Layout.fillWidth: true
                                            leftPadding: Kirigami.Units.iconSizes.smallMedium + Kirigami.Units.smallSpacing
                                            wrapMode: Text.Wrap
                                        }
                                        Label {
                                            objectName: "bundleLatestLabel-" + cardId
                                            text: qsTr("Latest: %1").arg(title)
                                            visible: !cardRow.bundleExpanded
                                            color: window.mutedInk
                                            font: Kirigami.Theme.defaultFont
                                            Layout.fillWidth: true
                                            leftPadding: Kirigami.Units.iconSizes.smallMedium + Kirigami.Units.smallSpacing
                                            wrapMode: Text.Wrap
                                        }
                                        Label {
                                            text: qsTr("%1 important stay separate; archive includes all %2 unpinned emails").arg(cardRow.bundleSummary.important).arg(cardRow.bundleSummary.unpinned)
                                            visible: cardRow.bundleSummary.important > 0
                                            color: window.mutedInk
                                            font: Kirigami.Theme.defaultFont
                                            Layout.fillWidth: true
                                            leftPadding: Kirigami.Units.iconSizes.smallMedium + Kirigami.Units.smallSpacing
                                            wrapMode: Text.Wrap
                                        }
                                    }
                                    onClicked: window.toggleBundle(card.bundle_id)
                                    Keys.onReturnPressed: clicked()
                                    Keys.onEnterPressed: clicked()
                                    Keys.onRightPressed: { if (!cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                    Keys.onLeftPressed: { if (cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                    Keys.onEscapePressed: { if (cardRow.bundleExpanded) window.toggleBundle(card.bundle_id) }
                                }
                                Button {
                                    objectName: "archiveBundle-" + cardId
                                    text: qsTr("Archive bundle")
                                    icon.name: "mail-mark-read-symbolic"
                                    icon.color: window.ink
                                    display: window.width < 640 ? AbstractButton.IconOnly : AbstractButton.TextBesideIcon
                                    Layout.minimumWidth: 48
                                    Layout.preferredWidth: window.width < 640 ? 48 : -1
                                    Layout.maximumWidth: window.width < 640 ? 48 : -1
                                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
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
                                    objectName: "bundleOptions-" + cardId
                                    text: qsTr("Bundle details")
                                    icon.name: "overflow-menu"
                                    icon.color: window.ink
                                    display: AbstractButton.IconOnly
                                    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    Accessible.name: text
                                    ToolTip.text: text
                                    ToolTip.visible: hovered
                                    onClicked: bundleDetails.open()
                                    Menu {
                                        id: bundleDetails
                                        MenuItem { text: card.bundle_title || window.bundleTitle(card); enabled: false }
                                        MenuItem { text: cardRow.bundleSummary.accounts.join(", "); enabled: false }
                                    }
                                }
                            }
                            Rectangle { id: bundleDivider; Layout.fillWidth: true; implicitHeight: 1; color: "#e0e9e6"; visible: cardRow.bundleLeader && cardRow.bundleExpanded }
                            RowLayout {
                                Layout.leftMargin: cardRow.bundled ? cardRow.bundleTextInset : 0
                                visible: !cardRow.bundleLeader || cardRow.bundleExpanded
                                Layout.fillWidth: true
                                Layout.topMargin: cardRow.bundleLeader ? Kirigami.Units.smallSpacing : 0
                                spacing: Kirigami.Units.smallSpacing
                                ToolButton {
                                    id: dragHandle
                                    objectName: "reorderHandle-" + cardId
                                    visible: !cardRow.bundled
                                    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    readonly property bool pinned: card.pinned_rank != null || card.section === "pinned"
                                    readonly property bool reorderable: pinned || !card.timed
                                    opacity: reorderable ? 1.0 : 0.4

                                    // The list-level surface keeps the pointer throughout
                                    // the drag, beyond this handle's hit target.
                                    display: AbstractButton.IconOnly
                                    icon.name: "transform-move"
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
                                    function openCard() {
                                        if (card.source === "mail" || card.has_body) cardActions.readCachedRequested()
                                        else cardActions.openRequested()
                                    }
                                    activeFocusOnTab: card.source === "mail" || !!card.has_body || !!card.source_url
                                    Accessible.role: Accessible.Button
                                    Accessible.name: qsTr("Open %1").arg(title)
                                    Keys.onReturnPressed: openCard()
                                    Keys.onSpacePressed: openCard()
                                    TapHandler { onTapped: cardText.openCard() }
                                    HoverHandler { cursorShape: cardText.activeFocusOnTab ? Qt.PointingHandCursor : Qt.ArrowCursor }
                                    Layout.minimumHeight: 48
                                    Layout.fillWidth: true
                                    spacing: Kirigami.Units.smallSpacing
                                    Label {
                                        text: cardRow.bundled ? (card.account_name || "") : window.cardStore.sourceLabel(card) + (card.account_name ? " · " + card.account_name : "")
                                        visible: !cardRow.bundled || (!!card.account_name && (cardRow.bundleSummary.accounts.length > 1 || card.account_name !== cardRow.bundleSummary.accounts[0]))
                                        color: window.mutedInk
                                        font: Kirigami.Theme.defaultFont
                                        wrapMode: Text.Wrap
                                        Layout.fillWidth: true
                                    }
                                    Kirigami.Heading {
                                        text: title
                                        color: window.ink
                                        level: 4
                                        wrapMode: Text.Wrap
                                        Layout.fillWidth: true
                                    }
                                    Label { text: card.summary || ""; color: window.mutedInk; visible: text.length > 0; font: Kirigami.Theme.defaultFont; wrapMode: Text.Wrap; Layout.fillWidth: true }
                                    Label { text: card.note || ""; color: window.mutedInk; visible: text.length > 0; font.italic: true; wrapMode: Text.Wrap; Layout.fillWidth: true }
                                    Label { text: window.clock(card); visible: text.length > 0; color: window.mutedInk; font: Kirigami.Theme.defaultFont }
                                }
                                ToolButton {
                                    objectName: "doneButton-" + cardId
                                    text: card.source === "mail" || card.source === "journal" ? qsTr("Archive") : qsTr("Done")
                                    icon.name: card.source === "mail" ? "mail-mark-read-symbolic" : "dialog-ok"
                                    icon.width: Kirigami.Units.iconSizes.smallMedium
                                    icon.height: Kirigami.Units.iconSizes.smallMedium
                                    icon.color: window.ink
                                    display: AbstractButton.IconOnly
                                    implicitWidth: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    implicitHeight: Math.max(48, Kirigami.Units.gridUnit * 3)
                                    opacity: 1
                                    Accessible.name: cardActions.primaryName
                                    ToolTip.text: Accessible.name
                                    ToolTip.visible: hovered
                                    onClicked: cardActions.primaryAction()
                                }
                                Litterbox.CardActions {
                                    id: cardActions
                                    objectName: "inboxActions"
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
                                        }
                                    })
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
                        color: Kirigami.Theme.highlightColor
                        z: 1
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
                onPressed: function (mouse) { reorderSurface.startDragAt(mouse) }
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
                    reorderSurface.draggingId = ""
                    window.dragCardId = ""
                    window.dragTargetIndex = -1
                    window.dragOffset = 0
                    moved = false
                }
            }
        }
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
                    selectionColor: Kirigami.Theme.highlightColor
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
