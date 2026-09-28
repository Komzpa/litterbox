// SPDX-License-Identifier: MIT
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
    property var api
    property var entries: []
    property bool loading: false
    property string errorText: ""
    title: qsTr("Journal")

    function reload() {
        loading = true
        errorText = ""
        api.get("/v1/journal", function (error, response) {
            loading = false
            if (error || !response || !Array.isArray(response.body)) {
                errorText = qsTr("Could not load the journal.")
                return
            }
            entries = response.body
        })
    }

    function compose(body) {
        body = (body || "").trim()
        if (body === "")
            return false
        errorText = ""
        api.post("/v1/journal", { body: body }, function (error, response) {
            if (error || !response || !response.body) {
                errorText = qsTr("Could not save the entry.")
                return
            }
            composer.clear()
            reload()
        })
        return true
    }

    Component.onCompleted: if (api) reload()

    ColumnLayout {
        width: root.width
        RowLayout {
            Layout.fillWidth: true
            QQC2.TextArea {
                id: composer
                objectName: "composer"
                Layout.fillWidth: true
                placeholderText: qsTr("Write an entry…")
                wrapMode: Text.Wrap
                Keys.onReturnPressed: function (event) {
                    if (!(event.modifiers & Qt.ShiftModifier)) {
                        root.compose(composer.text)
                        event.accepted = true
                    }
                }
            }
            QQC2.Button {
                objectName: "addButton"
                text: qsTr("Add")
                onClicked: root.compose(composer.text)
            }
        }
        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        ListView {
            id: list
            objectName: "entriesList"
            Layout.fillWidth: true
            Layout.preferredHeight: Math.max(160, contentHeight)
            clip: true
            keyNavigationEnabled: true
            model: root.entries
            activeFocusOnTab: true
            delegate: QQC2.ItemDelegate {
                width: list.width
                contentItem: Column {
                    spacing: Kirigami.Units.smallSpacing
                    QQC2.Label {
                        text: modelData.body
                        wrapMode: Text.Wrap
                        width: list.width - Kirigami.Units.largeSpacing * 2
                    }
                    QQC2.Label {
                        text: new Date(modelData.created_at).toLocaleString(Qt.locale(), Locale.ShortFormat)
                        color: Kirigami.Theme.disabledTextColor
                        font: Kirigami.Theme.smallFont
                    }
                }
            }
            QQC2.Label {
                anchors.centerIn: parent
                visible: list.count === 0 && !root.loading
                text: qsTr("No entries yet.")
            }
        }
    }
}
