// SPDX-License-Identifier: MIT
// Device enrollment: redeem an invite code for a bearer token.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root

    // api.post(path, body, cb) calls cb(error, {status, body}).
    property var api

    property bool busy: false
    property string errorText: ""
    // Set on success; the token is only shown once.
    property string token: ""
    property string deviceId: ""
    property string tenantId: ""

    signal enrolled(var device) // { token, device_id, tenant_id }

    title: qsTr("Connect to Server")

    function enroll() {
        errorText = ""
        const invite = inviteField.text.trim()
        const name = deviceNameField.text.trim()
        if (invite === "" || name === "") {
            errorText = qsTr("Invite code and device name are required.")
            return false
        }
        busy = true
        api.post("/v1/devices/enroll", {
            invite_code: invite,
            device_name: name,
            platform: platformBox.currentValue
        }, function (error, response) {
            busy = false
            if (error || !response || !response.body || !response.body.token) {
                errorText = qsTr("Invite is used, expired, or invalid.")
                return
            }
            token = response.body.token
            deviceId = response.body.device_id
            tenantId = response.body.tenant_id
            api.token = token
            root.enrolled(response.body)
        })
        return true
    }

    Kirigami.FormLayout {
        visible: root.token === ""
        width: parent.width

        QQC2.TextField {
            id: inviteField
            objectName: "inviteField"
            Kirigami.FormData.label: qsTr("Invite code:")
            Layout.fillWidth: true
        }
        QQC2.TextField {
            id: deviceNameField
            objectName: "deviceNameField"
            Kirigami.FormData.label: qsTr("Device name:")
            text: Qt.platform.os === "android" ? qsTr("Phone") : qsTr("Desktop")
            Layout.fillWidth: true
        }
        QQC2.ComboBox {
            id: platformBox
            objectName: "platformBox"
            Kirigami.FormData.label: qsTr("Platform:")
            model: ["linux", "android"]
            Component.onCompleted: currentIndex = Qt.platform.os === "android" ? 1 : 0
        }

        QQC2.Label {
            visible: root.errorText !== ""
            text: root.errorText
            color: Kirigami.Theme.negativeTextColor
            wrapMode: Text.Wrap
            Kirigami.FormData.isSection: true
        }

        QQC2.Button {
            objectName: "enrollButton"
            text: root.busy ? qsTr("Enrolling…") : qsTr("Enroll")
            enabled: !root.busy
            onClicked: root.enroll()
        }
    }

    ColumnLayout {
        visible: root.token !== ""
        width: parent.width
        spacing: Kirigami.Units.smallSpacing

        QQC2.Label {
            text: qsTr("Enrollment complete. Save this token — it will not be shown again.")
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.TextField {
            objectName: "tokenField"
            Layout.fillWidth: true
            readOnly: true
            text: root.token
            selectByMouse: true
        }
        QQC2.Label {
            objectName: "deviceSummary"
            text: qsTr("Device: %1 · Tenant: %2").arg(root.deviceId).arg(root.tenantId)
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
    }
}
