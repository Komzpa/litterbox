// SPDX-License-Identifier: MIT
// Reminder composer: title, date-time in the system locale, recurrence.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

QQC2.Dialog {
    id: root

    // api.post(path, body, cb) calls cb(error, {status, body}).
    property var api

    property string errorText: ""
    property string reminderId: ""
    property string lastDueAt: ""
    readonly property var recurrenceValues: ["", "daily", "weekly"]
    // Keep locale ordering and separators, but avoid ambiguous two-digit years.
    readonly property string localeDateTimeFormat: Qt.locale().dateTimeFormat(Locale.ShortFormat).replace(/y+/g, "yyyy")

    title: qsTr("New Reminder")
    modal: true

    function parseWhen() {
        const d = Date.fromLocaleString(Qt.locale(), whenField.text, localeDateTimeFormat)
        return isNaN(d.getTime()) ? null : d
    }

    function submit() {
        errorText = ""
        const title = titleField.text.trim()
        if (title === "") {
            errorText = qsTr("Title is required.")
            return false
        }
        const when = parseWhen()
        if (!when) {
            errorText = qsTr("Enter the date and time in your locale format (%1).").arg(Qt.locale().name)
            return false
        }
        api.post("/v1/reminders", {
            title: title,
            due_at: when.toISOString(),
            recurrence: recurrenceValues[recurrenceBox.currentIndex]
        }, function (error, response) {
            if (error || !response || !response.body || !response.body.id) {
                errorText = qsTr("Could not save the reminder.")
                return
            }
            reminderId = response.body.id
            lastDueAt = when.toISOString()
            root.close()
        })
        return true
    }

    contentItem: ColumnLayout {
        spacing: Kirigami.Units.smallSpacing

        QQC2.Label { text: qsTr("Title:") }
        QQC2.TextField {
            id: titleField
            objectName: "titleField"
            Layout.fillWidth: true
        }

        QQC2.Label {
            text: qsTr("When (%1):").arg(Qt.locale().name)
        }
        QQC2.TextField {
            id: whenField
            objectName: "whenField"
            Layout.fillWidth: true
            placeholderText: new Date().toLocaleString(Qt.locale(), root.localeDateTimeFormat)
        }
        QQC2.Label {
            objectName: "whenPreview"
            visible: text !== ""
            color: Kirigami.Theme.disabledTextColor
            text: {
                const d = root.parseWhen()
                return d ? qsTr("Will remind: %1").arg(d.toLocaleString(Qt.locale(), root.localeDateTimeFormat)) : ""
            }
        }

        QQC2.Label { text: qsTr("Repeats:") }
        QQC2.ComboBox {
            id: recurrenceBox
            objectName: "recurrenceBox"
            model: [qsTr("Never"), qsTr("Daily"), qsTr("Weekly")]
        }

        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }

    footer: QQC2.DialogButtonBox {
        QQC2.Button {
            objectName: "saveButton"
            text: qsTr("Save")
            onClicked: root.submit()
        }
        QQC2.Button {
            text: qsTr("Cancel")
            onClicked: root.close()
        }
    }
}
