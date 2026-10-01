import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

Controls.ToolButton {
    id: root
    required property var store
    required property string cardKey
    property string source: ""
    property string sourceLabel: source
    property bool hasBody: false
    property string bundleId: ""
    property var pinnedRank: undefined
    property string snoozeError: ""
    property string cardTitle: ""
    property string accountName: ""
    property bool canOpenSource: false
    signal noteRequested()
    signal openRequested()
    signal readCachedRequested()
    readonly property string primaryName: source === "mail" ? qsTr("Archive in Gmail%1").arg(accountName ? " · " + accountName : "") : source === "home_assistant" ? qsTr("Dismiss Home Assistant notification") : qsTr("Done · dismiss in Litterbox only")
    function primaryAction() {
        if (source === "mail") store.enqueueOp(cardKey, "archive", {})
        else store.dismiss(cardKey)
    }
    readonly property bool pinStateKnown: pinnedRank !== undefined
    readonly property bool pinned: pinStateKnown && pinnedRank !== null

    function localDateTime(date) {
        const pad = value => String(value).padStart(2, "0")
        return date.getFullYear() + "-" + pad(date.getMonth() + 1) + "-" + pad(date.getDate()) +
            " " + pad(date.getHours()) + ":" + pad(date.getMinutes())
    }
    function chooseSnoozeDateTime() {
        snoozeError = ""
        snoozeDateTime.text = localDateTime(new Date(Date.now() + 3600000))
        snoozeDialog.open()
    }

    function captureSnooze(localDateTimeValue) {
        chooseSnoozeDateTime()
        snoozeDateTime.text = localDateTimeValue
        snoozeDialog.accept()
        return snoozeError.length === 0
    }
    function movePin(delta) {
        const current = Array.from(store.pinnedCardIds())
        const index = current.indexOf(cardKey)
        const target = index + delta
        if (index < 0 || target < 0 || target >= current.length) return
        const value = current[index]
        current[index] = current[target]
        current[target] = value
        store.enqueueOp(current[0], "reorder_pins", { cards: current })
    }
    function moveCard(delta) { store.moveCard(cardKey, delta) }

    text: "⋮"
    implicitWidth: 44
    implicitHeight: 44
    Accessible.name: qsTr("More actions for %1").arg(cardTitle)
    Controls.ToolTip.text: Accessible.name
    Controls.ToolTip.visible: hovered
    onClicked: actionSheet.open()
    Controls.Dialog {
        id: actionSheet
        parent: Controls.Overlay.overlay
        objectName: "cardActionSheet-" + root.cardKey
        title: root.cardTitle
        modal: true
        width: Math.min(360, root.Window.window ? root.Window.window.width - 32 : 360)
        implicitWidth: width
        background: Rectangle { color: root.palette.base; radius: 8; border.color: root.palette.mid }
        anchors.centerIn: parent
        onClosed: root.forceActiveFocus()
        contentItem: ColumnLayout {
            spacing: 0
            Controls.Label { text: root.accountName || root.sourceLabel; Layout.fillWidth: true; wrapMode: Text.Wrap }
            Controls.Button { text: root.primaryName; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.primaryAction() } }
            Controls.Button { text: root.source === "mail" ? qsTr("Open in Gmail") : qsTr("Open source"); visible: root.canOpenSource; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.openRequested() } }
            Controls.Button { text: qsTr("Read cached · stays in Litterbox"); visible: root.source === "mail" || root.hasBody; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.readCachedRequested() } }
            Controls.Button { objectName: "noteButton-" + root.cardKey; text: qsTr("Note · instruction for this card"); Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.noteRequested() } }
            Controls.Button { text: qsTr("Snooze · choose date and time"); Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.chooseSnoozeDateTime() } }
            Controls.Button { text: root.pinned ? qsTr("Unpin") : qsTr("Pin"); visible: root.pinStateKnown; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.store.enqueueOp(root.cardKey, root.pinned ? "unpin" : "pin", {}) } }
            Controls.Button { text: root.source === "mail" ? qsTr("Archive unpinned bundle members") : qsTr("Complete unpinned bundle members"); visible: root.bundleId.length > 0; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); if (root.source === "mail") archiveDialog.open(); else completeDialog.open() } }
            Controls.Button { text: qsTr("Take out of bundle"); visible: root.bundleId.length > 0; Layout.fillWidth: true; implicitHeight: 44; onClicked: { actionSheet.close(); root.store.enqueueOp(root.cardKey, "take_out", { card: root.cardKey }) } }
            Controls.Button { text: qsTr("Close"); Layout.fillWidth: true; implicitHeight: 44; onClicked: actionSheet.close() }
        }
    }

    Controls.Dialog {
        id: snoozeDialog
        objectName: "snoozeDialog"
        title: qsTr("Snooze until")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        width: Math.min(392, root.Window.window ? root.Window.window.width - 32 : 392)
        implicitWidth: width
        background: Rectangle { color: root.palette.base; radius: 8; border.color: root.palette.mid }
        onOpened: snoozeDateTime.forceActiveFocus()
        contentItem: ColumnLayout {
            Controls.Label { text: qsTr("Local date and time (YYYY-MM-DD HH:MM)"); Layout.fillWidth: true; wrapMode: Text.Wrap }
            Controls.TextField {
                id: snoozeDateTime
                objectName: "snoozeDateTime"
                Layout.fillWidth: true
                placeholderText: "YYYY-MM-DD HH:MM"
                inputMethodHints: Qt.ImhDate | Qt.ImhTime
                Accessible.name: qsTr("Local date and time")
            }
            Controls.Label {
                text: root.snoozeError
                visible: root.snoozeError.length > 0
                color: Kirigami.Theme.negativeTextColor
            }
        }
        onAccepted: {
            const parts = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/.exec(snoozeDateTime.text)
            const until = parts ? new Date(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]), Number(parts[4]), Number(parts[5])) : null
            const valid = until && until.getFullYear() === Number(parts[1]) &&
                until.getMonth() === Number(parts[2]) - 1 && until.getDate() === Number(parts[3]) &&
                until.getHours() === Number(parts[4]) && until.getMinutes() === Number(parts[5]) &&
                until.getTime() > Date.now()
            if (!valid) {
                root.snoozeError = qsTr("Enter a valid future local date and time.")
                Qt.callLater(function() { snoozeDialog.open() })
                return
            }
            root.snoozeError = ""
            root.store.enqueueOp(root.cardKey, "snooze", { until: until.toISOString() })
        }
    }
    Controls.Dialog {
        id: completeDialog
        objectName: "completeDialog"
        title: qsTr("Complete all unpinned cards in this bundle?")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        onAccepted: root.store.enqueueOp(root.cardKey, "bundle_done", { bundle_id: root.bundleId })
    }
    Controls.Dialog {
        id: archiveDialog
        objectName: "archiveDialog"
        title: qsTr("Archive all unpinned cards in this bundle?")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        onAccepted: root.store.enqueueOp(root.cardKey, "bundle_archive", { bundle_id: root.bundleId })
    }
}
