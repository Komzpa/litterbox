import QtQuick
import QtQuick.Controls as Controls
import QtQuick.Layouts
import org.kde.kirigami as Kirigami

RowLayout {
    id: root
    required property var store
    required property string cardId
    property string bundleId: ""
    property bool pinned: false
    spacing: Kirigami.Units.smallSpacing

    Controls.Button {
        text: root.pinned ? qsTr("Unpin") : qsTr("Pin")
        icon.name: root.pinned ? "window-unpin" : "window-pin"
        Accessible.name: text
        onClicked: root.store.enqueueOp(root.cardId, root.pinned ? "unpin" : "pin", {})
    }
    Controls.Button {
        id: snoozeButton
        text: qsTr("Snooze")
        icon.name: "alarm-symbolic"
        Accessible.name: text
        onClicked: snoozeMenu.open()
        Controls.Menu {
            id: snoozeMenu
            objectName: "snoozeMenu"
            y: snoozeButton.height
            Controls.MenuItem {
                text: qsTr("For one hour")
                onTriggered: root.store.enqueueOp(root.cardId, "snooze", {
                    until: new Date(Date.now() + 3600000).toISOString()
                })
            }
            Controls.MenuItem {
                text: qsTr("Until tomorrow")
                onTriggered: {
                    const tomorrow = new Date()
                    tomorrow.setDate(tomorrow.getDate() + 1)
                    tomorrow.setHours(9, 0, 0, 0)
                    root.store.enqueueOp(root.cardId, "snooze", { until: tomorrow.toISOString() })
                }
            }
        }
    }
    Controls.Button {
        visible: root.bundleId !== ""
        text: qsTr("Complete bundle")
        icon.name: "task-complete"
        Accessible.name: text
        onClicked: completeDialog.open()
    }
    Controls.Button {
        visible: root.bundleId !== ""
        text: qsTr("Archive bundle")
        icon.name: "archive-insert"
        Accessible.name: text
        onClicked: archiveDialog.open()
    }
    Controls.Button {
        visible: root.bundleId !== ""
        text: qsTr("Take out of bundle")
        icon.name: "list-remove"
        Accessible.name: text
        onClicked: root.store.enqueueOp(root.cardId, "take_out", { card: root.cardId })
    }
    Controls.Dialog {
        id: completeDialog
        objectName: "completeDialog"
        title: qsTr("Complete all unpinned cards in this bundle?")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        onAccepted: root.store.enqueueOp(root.cardId, "bundle_done", { bundle_id: root.bundleId })
    }
    Controls.Dialog {
        id: archiveDialog
        objectName: "archiveDialog"
        title: qsTr("Archive all unpinned cards in this bundle?")
        modal: true
        standardButtons: Controls.Dialog.Ok | Controls.Dialog.Cancel
        onAccepted: root.store.enqueueOp(root.cardId, "bundle_archive", { bundle_id: root.bundleId })
    }
}
