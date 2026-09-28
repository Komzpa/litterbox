import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

Controls.ToolButton {
    id: root
    required property var store
    required property string cardKey
    property string source: ""
    property string bundleId: ""
    property var pinnedRank: undefined
    property string snoozeError: ""
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

    text: "⋮"
    Accessible.name: qsTr("Card actions")
    Controls.ToolTip.text: Accessible.name
    onClicked: actionsMenu.open()

    Controls.Menu {
        id: actionsMenu
        Controls.MenuItem {
            text: qsTr("Archive")
            visible: root.source === "mail"
            onTriggered: root.store.enqueueOp(root.cardKey, "archive", {})
        }
        Controls.MenuItem {
            text: root.pinned ? qsTr("Unpin") : qsTr("Pin")
            visible: root.pinStateKnown
            onTriggered: root.store.enqueueOp(root.cardKey, root.pinned ? "unpin" : "pin", {})
        }
        Controls.MenuItem {
            text: qsTr("Move pin up")
            visible: root.pinned
            enabled: root.store.pinnedCardIds().indexOf(root.cardKey) > 0
            onTriggered: root.movePin(-1)
        }
        Controls.MenuItem {
            text: qsTr("Move pin down")
            visible: root.pinned
            enabled: root.store.pinnedCardIds().indexOf(root.cardKey) >= 0 &&
                root.store.pinnedCardIds().indexOf(root.cardKey) < root.store.pinnedCardIds().length - 1
            onTriggered: root.movePin(1)
        }
        Controls.Menu {
            title: qsTr("Snooze")
            Controls.MenuItem {
                text: qsTr("For one hour")
                onTriggered: root.store.enqueueOp(root.cardKey, "snooze", {
                    until: new Date(Date.now() + 3600000).toISOString()
                })
            }
            Controls.MenuItem {
                text: qsTr("Until tomorrow")
                onTriggered: {
                    const tomorrow = new Date()
                    tomorrow.setDate(tomorrow.getDate() + 1)
                    tomorrow.setHours(9, 0, 0, 0)
                    root.store.enqueueOp(root.cardKey, "snooze", { until: tomorrow.toISOString() })
                }
            }
            Controls.MenuItem {
                text: qsTr("Choose date and time")
                onTriggered: root.chooseSnoozeDateTime()
            }
        }
        Controls.MenuSeparator { visible: root.bundleId.length > 0 }
        Controls.MenuItem {
            text: qsTr("Complete bundle")
            visible: root.bundleId.length > 0
            onTriggered: completeDialog.open()
        }
        Controls.MenuItem {
            text: qsTr("Archive bundle")
            visible: root.bundleId.length > 0
            onTriggered: archiveDialog.open()
        }
        Controls.MenuItem {
            text: qsTr("Take out of bundle")
            visible: root.bundleId.length > 0
            onTriggered: root.store.enqueueOp(root.cardKey, "take_out", { card: root.cardKey })
        }
    }
    Controls.Dialog {
        id: snoozeDialog
        objectName: "snoozeDialog"
        title: qsTr("Snooze until")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        onOpened: snoozeDateTime.forceActiveFocus()
        contentItem: ColumnLayout {
            Controls.Label { text: qsTr("Local date and time (YYYY-MM-DD HH:MM)") }
            Controls.TextField {
                id: snoozeDateTime
                objectName: "snoozeDateTime"
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
