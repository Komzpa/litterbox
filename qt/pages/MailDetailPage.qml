// SPDX-License-Identifier: MIT
// Mail message detail: sanitized HTML body plus an "Open in Gmail" link.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root

    // api.get(path, cb) calls cb(error, {status, body}).
    property var api
    property string cardId: ""
    // Set to false in tests to avoid spawning an external browser.
    property bool openLinks: true

    property bool loading: false
    property string errorText: ""
    property string html: ""
    property string threadId: ""
    readonly property string gmailUrl: threadId ? "https://mail.google.com/mail/u/0/#all/" + threadId : ""

    title: qsTr("Message")

    function reload() {
        loading = true
        errorText = ""
        html = ""
        threadId = ""
        api.get("/v1/cards/" + cardId + "/body", function (error, response) {
            loading = false
            if (error || !response || !response.body) {
                errorText = qsTr("Could not load the message.")
                return
            }
            const data = response.body
            html = data.html || ""
            threadId = data.threadId || ""
        })
    }

    function openInGmail() {
        if (gmailUrl && openLinks)
            Qt.openUrlExternally(gmailUrl)
    }

    Component.onCompleted: {
        if (api && cardId !== "")
            reload()
    }

    ColumnLayout {
        width: root.width

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

        Kirigami.LinkButton {
            objectName: "openInGmail"
            text: qsTr("Open in Gmail")
            visible: root.gmailUrl !== ""
            onClicked: root.openInGmail()
        }

        QQC2.Label {
            id: bodyLabel
            objectName: "body"
            text: root.html
            textFormat: Text.RichText
            wrapMode: Text.Wrap
            visible: !root.loading && root.errorText === ""
            Layout.fillWidth: true
        }
    }
}
