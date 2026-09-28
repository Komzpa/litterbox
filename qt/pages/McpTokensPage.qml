// SPDX-License-Identifier: MIT
// MCP tokens: create (token shown once), mask, revoke.
// Server has no list endpoint, so this session's created tokens are listed.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root

    // Contract: callbacks receive (error, {status, body}).
    property var api

    property var tokens: [] // { id, token, shown }
    property bool loading: false
    property string errorText: ""

    title: qsTr("MCP Tokens")

    function createToken() {
        loading = true
        errorText = ""
        api.post("/v1/mcp-tokens", {}, function (error, response) {
            loading = false
            if (error || !response || !response.body || !response.body.token) {
                errorText = qsTr("Could not create a token.")
                return
            }
            const copy = tokens.slice()
            copy.unshift({ id: response.body.id, token: response.body.token, shown: true })
            tokens = copy
        })
    }

    // A token is only visible right after creation; once hidden it stays hidden.
    function hideToken(id) {
        tokens = tokens.map(function (t) {
            return t.id === id ? { id: t.id, token: "", shown: false } : t
        })
    }

    function revokeToken(id) {
        api.del("/v1/mcp-tokens/" + id, function (error) {
            if (error) {
                errorText = qsTr("Could not revoke the token.")
                return
            }
            tokens = tokens.filter(function (t) { return t.id !== id })
        })
    }

    ColumnLayout {
        width: root.width

        QQC2.Label {
            text: qsTr("MCP tokens let external tools read your journal and create cards.")
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }

        QQC2.Button {
            objectName: "createButton"
            text: qsTr("Create token")
            onClicked: root.createToken()
        }

        QQC2.BusyIndicator {
            visible: root.loading
            Layout.alignment: Qt.AlignHCenter
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
            objectName: "tokensList"
            Layout.fillWidth: true
            Layout.preferredHeight: Math.max(160, contentHeight)
            clip: true
            keyNavigationEnabled: true
            model: root.tokens
            activeFocusOnTab: true

            delegate: ColumnLayout {
                width: list.width
                spacing: Kirigami.Units.smallSpacing

                RowLayout {
                    Layout.fillWidth: true
                    QQC2.Label {
                        objectName: "tokenValue"
                        Layout.fillWidth: true
                        text: modelData.shown ? modelData.token : "••••••••••••"
                        wrapMode: Text.Wrap
                        font.family: "monospace"
                    }
                    QQC2.Button {
                        objectName: "hideButton"
                        text: qsTr("Hide")
                        visible: modelData.shown
                        onClicked: root.hideToken(modelData.id)
                    }
                }

                QQC2.Button {
                    objectName: "revokeButton"
                    text: qsTr("Revoke")
                    onClicked: root.revokeToken(modelData.id)
                }
            }

            QQC2.Label {
                anchors.centerIn: parent
                visible: list.count === 0 && !root.loading
                text: qsTr("No tokens created in this session.")
            }
        }
    }
}
