// SPDX-License-Identifier: MIT
// Gmail accounts: connect via OAuth URL, list, disconnect.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root

    // Contract: { get(path, cb), post(path, body, cb), del(path, cb) }.
    // Set openLinks to false in tests to avoid spawning a browser.
    property var api
    property bool openLinks: true

    property var accounts: [] // { id, address }
    property bool loading: false
    property string errorText: ""
    property string lastConnectUrl: ""

    title: qsTr("Gmail Accounts")

    function reload() {
        loading = true
        errorText = ""
        api.get("/v1/gmail/accounts", function (error, response) {
            loading = false
            if (error || !response || !Array.isArray(response.body)) {
                errorText = qsTr("Could not load the account list.")
                return
            }
            accounts = response.body
        })
    }

    function connectAccount() {
        errorText = ""
        api.post("/v1/gmail/connect", {}, function (error, response) {
            if (error || !response || !response.body || !response.body.url) {
                errorText = qsTr("Could not start the Gmail connection.")
                return
            }
            lastConnectUrl = response.body.url
            if (root.openLinks)
                Qt.openUrlExternally(response.body.url)
        })
    }

    function disconnect(id) {
        api.del("/v1/gmail/accounts/" + id, function (error) {
            if (error) {
                errorText = qsTr("Could not disconnect the account.")
                return
            }
            reload()
        })
    }

    Component.onCompleted: {
        if (api)
            reload()
    }

    ColumnLayout {
        width: root.width

        QQC2.Button {
            objectName: "connectButton"
            text: qsTr("Connect account…")
            onClicked: root.connectAccount()
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
            objectName: "accountsList"
            Layout.fillWidth: true
            Layout.preferredHeight: Math.max(160, contentHeight)
            clip: true
            keyNavigationEnabled: true
            model: root.accounts
            activeFocusOnTab: true

            delegate: QQC2.ItemDelegate {
                width: list.width
                contentItem: RowLayout {
                    spacing: Kirigami.Units.smallSpacing
                    QQC2.Label {
                        text: modelData.address
                        Layout.fillWidth: true
                        elide: Text.ElideRight
                    }
                    QQC2.Button {
                        objectName: "disconnectButton"
                        text: qsTr("Disconnect")
                        onClicked: root.disconnect(modelData.id)
                    }
                }
            }

            QQC2.Label {
                anchors.centerIn: parent
                visible: list.count === 0 && !root.loading
                text: qsTr("No accounts connected.")
            }
        }
    }
}
