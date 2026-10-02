import QtQuick
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
    color: canvas
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
    function toggleBundle(bundleId) {
        const next = Object.assign({}, expandedBundles)
        next[bundleId] = !next[bundleId]
        expandedBundles = next
    }
    property string updateStatus: ""
    property string updateVersion: ""


    // Drop the dragged card at the row the `=` handle was released over.
    function commitCardDrag(cardId, targetIndex) {
        return targetIndex >= 0 && store.moveCardTo(cardId, targetIndex)
    }
    function cardIdAt(row) {
        const ids = store.cardIds()
        return row >= 0 && row < ids.length ? ids[row] : ""
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
    function openPage(name, properties) {
        const page = pagesDir.toString() + name + ".qml"
        stack.push(page, properties || { api: api })
    }
    function clock(card) { return card.timed ? timeRules.display(card.at || "") : "" }

    header: ToolBar {
        padding: 16
        background: Rectangle { color: window.surface }
        ColumnLayout {
            width: parent.width
            spacing: 4
            RowLayout {
                Layout.fillWidth: true
                Kirigami.Heading { text: qsTr("Inbox"); color: window.ink; level: 2; Layout.fillWidth: true }
                Button {
                    id: addCardButton
                    objectName: "addCardButton"
                    text: qsTr("+ Add card")
                    implicitHeight: 44
                    leftPadding: 16
                    rightPadding: 16
                    background: Rectangle {
                        radius: 22
                        color: addCardButton.down ? "#286358" : addCardButton.hovered ? "#326f65" : window.accent
                        border.width: addCardButton.visualFocus ? 2 : 0
                        border.color: window.ink
                    }
                    contentItem: Label {
                        text: addCardButton.text
                        color: window.surface
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                        font: addCardButton.font
                    }
                    onClicked: createDialog.open()
                    TapHandler { onTapped: createDialog.open() }
                }
                ToolButton {
                    text: "⋮"
                    implicitWidth: 44
                    implicitHeight: 44
                    Accessible.name: qsTr("Inbox commands")
                    onClicked: headerMenu.open()
                    Menu {
                        id: headerMenu
                        MenuItem { text: qsTr("Accounts"); onTriggered: openPage("GmailAccountsPage") }
                        MenuItem { text: qsTr("Enroll device"); onTriggered: openPage("EnrollmentPage") }
                        MenuItem { text: qsTr("Refresh"); onTriggered: store.refresh() }
                        MenuItem { text: qsTr("Private journal"); onTriggered: openPage("JournalPage") }
                        MenuItem { text: qsTr("Check updates"); visible: updater.supported; enabled: !updater.busy; onTriggered: updater.checkForUpdates() }
                    }
                }
            }
            Label { text: store.online ? qsTr("Online · changes sync across devices") : qsTr("Offline · changes saved on this device"); color: window.mutedInk; font.pointSize: 9; Layout.fillWidth: true; wrapMode: Text.Wrap }
            Label { text: window.updateStatus; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true }
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
        Keys.onBackPressed: function(event) {
            if (window.handleBack()) event.accepted = true
        }
        initialItem: Item {
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
                model: store
                section.property: "section"
                spacing: 0
                section.delegate: Item {
                    width: ListView.view.width
                    height: sectionHeading.implicitHeight + 16
                    Label {
                        id: sectionHeading
                        width: Math.min(parent.width - 32, 1200)
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: (section === "pinned" ? qsTr("Pinned") : section === "now" ? qsTr("Now") : section === "later" ? qsTr("Later") : qsTr("Missed")).toLocaleUpperCase()
                        font.pointSize: 9
                        font.bold: true
                        font.letterSpacing: 1
                        color: window.mutedInk
                        padding: 12
                    }
                }
                delegate: Item {
                    required property var card
                    required property string cardId
                    required property string title
                    id: cardRow
                    HoverHandler { id: rowHover }
                    readonly property bool lifted: window.dragCardId === cardId
                    readonly property bool bundled: !!card.bundle_id && card.section !== "pinned" && card.pinned_rank == null && !card.important
                    readonly property bool bundleExpanded: !!window.expandedBundles[card.bundle_id]
                    visible: !bundled || card.bundle_leader !== false || bundleExpanded
                    width: ListView.view.width
                    height: visible ? cardFrame.implicitHeight : 0
                    z: lifted ? 10 : 0
                    Rectangle { anchors.fill: parent; color: "#eef3f2"; visible: cardRow.lifted; radius: 8 }
                    Frame {
                        id: cardFrame
                        objectName: "inboxCard"
                        width: Math.min(parent.width - 32, 1200)
                        x: (parent.width - width) / 2
                        y: cardRow.lifted ? window.dragOffset : 0
                        padding: 10
                        background: Rectangle { color: cardRow.lifted ? "#e4efed" : "#ffffff"; border.color: cardRow.lifted ? "#397d73" : "#edf0ef"; radius: cardRow.lifted ? 8 : 0 }
                        RowLayout {
                            width: parent.width
                            spacing: 4
                                Item {
                                    id: dragHandle
                                    objectName: "reorderHandle-" + cardId
                                    implicitWidth: 44
                                    implicitHeight: 44
                                    readonly property bool pinned: card.pinned_rank != null || card.section === "pinned"
                                    readonly property bool reorderable: pinned || !card.timed
                                    opacity: reorderable ? 1.0 : 0.4

                                    // The handle is the visible affordance and the hit-test
                                    // target; the drag itself is driven by the list-level
                                    // surface below, because under delivered pointer events a
                                    // pressed item only receives the pointer while it stays
                                    // inside its own 32px bounds.
                                    Component.onCompleted: window.cardHandles[cardId] = dragHandle
                                    Component.onDestruction: delete window.cardHandles[cardId]

                                    Label {
                                        anchors.centerIn: parent
                                        text: "="
                                        font.bold: true
                                        color: window.mutedInk
                                    }
                                    Accessible.name: qsTr("Drag to reorder")
                                    Accessible.description: reorderable ? qsTr("Hold and drag to a new position") : qsTr("Position is fixed by pin or time")
                                    Accessible.role: Accessible.Button
                                    // No attached ToolTip on this handle: its popup closes on
                                    // mouse press, and that close cancels the press on the
                                    // drag surface a millisecond in, so grabbing `=` while
                                    // hovering it never started a drag. The cursor carries
                                    // the affordance instead.
                                    HoverHandler { cursorShape: dragHandle.reorderable ? Qt.OpenHandCursor : Qt.ArrowCursor }
                                }
                                ColumnLayout {
                                    Layout.fillWidth: true
                                    spacing: 4
                                    ToolButton {
                                        visible: cardRow.bundled && card.bundle_leader === true
                                        text: (cardRow.bundleExpanded ? "⌄ " : "› ") + (card.bundle_title || qsTr("Bundle")) + " · " + (card.bundle_member_count || "")
                                        Layout.fillWidth: true
                                        implicitHeight: 44
                                        Accessible.name: (cardRow.bundleExpanded ? qsTr("Collapse %1") : qsTr("Expand %1")).arg(card.bundle_title || qsTr("bundle"))
                                        onClicked: window.toggleBundle(card.bundle_id)
                                    }
                                    Label { text: window.cardStore.sourceLabel(card) + (card.account_name ? " · " + card.account_name : ""); color: window.mutedInk; font.pointSize: 9; wrapMode: Text.Wrap; Layout.fillWidth: true }
                                    Label {
                                        text: title
                                        color: window.ink
                                        font.bold: true
                                        wrapMode: Text.Wrap
                                        Layout.fillWidth: true
                                        TapHandler { onTapped: cardActions.openRequested() }
                                    }
                                    Label { text: card.summary || ""; color: window.mutedInk; visible: text.length > 0; wrapMode: Text.Wrap; Layout.fillWidth: true }
                                    Label { text: card.note || ""; color: window.mutedInk; visible: text.length > 0; font.italic: true; wrapMode: Text.Wrap; Layout.fillWidth: true }
                                    Label { text: window.clock(card); visible: text.length > 0; color: window.mutedInk; font.pointSize: 9 }
                                }
                                ToolButton {
                                    objectName: "doneButton-" + cardId
                                    text: card.source === "mail" ? "⇣" : "✓"
                                    implicitWidth: 44
                                    implicitHeight: 44
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
                                    onReadCachedRequested: window.openPage("MailDetailPage", { store: window.cardStore, cardId: cardId })
                                    Component.onCompleted: window.captureActions[cardId] = cardActions
                                    Component.onDestruction: delete window.captureActions[cardId]
                                }
                            }
                    }
                    // Overlay, not a Frame child: a second declared child collapses the
                    // Frame's implicit height (probe: 87px with one child, 18px with two).
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

            // The accepting surface for the `=` drag. It must be as tall as the drag
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
                    const ids = store.cardIds()
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
                        if (!handle) continue
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
                // grabbing `=` twice in a row must still start a drag.
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
        standardButtons: Dialog.Save | Dialog.Cancel
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
        standardButtons: Dialog.Save | Dialog.Cancel
        onOpened: createTitle.forceActiveFocus()
        contentItem: ColumnLayout {
            TextField { id: createTitle; placeholderText: qsTr("Title"); Layout.fillWidth: true }
            TextField { id: createSummary; placeholderText: qsTr("Details (optional)"); Layout.fillWidth: true }
        }
        onAccepted: if (store.createCard(createTitle.text, createSummary.text)) { createTitle.clear(); createSummary.clear() }
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
