// SPDX-License-Identifier: MIT
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

Kirigami.Page {
    id: root
    objectName: "agentChatPage"
    property var api
    required property var store
    property var thread: ({})
    property string threadId: thread.id || ""
    property var messages: []
    property int cursor: 0
    property bool loading: false
    property string errorText: ""
    property var stream: typeof a2aStream !== "undefined" ? a2aStream : null
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    title: thread.display_name || thread.peer_id || qsTr("Agent chat")
    padding: 18
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: canvas
    Kirigami.Theme.alternateBackgroundColor: surface
    Kirigami.Theme.highlightColor: accent
    Kirigami.Theme.focusColor: accent
    Material.theme: Material.Light
    Material.background: surface
    Material.foreground: ink
    Material.accent: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.text: ink
    palette.button: surface
    palette.buttonText: ink
    palette.highlight: accent
    palette.highlightedText: surface
    background: Rectangle { color: root.canvas }

    function deliveryLabel(message) {
        if (message.delivery_state === "not_delivered")
            return (message.reason_code ? message.reason_code + " / " : "") + qsTr("not delivered")
        return message.delivery_state || ""
    }

    function mergeMessages(rows) {
        var merged = messages.slice()
        for (var i = 0; i < rows.length; ++i) {
            var row = rows[i]
            var found = merged.findIndex(function(existing) {
                return (row.id && existing.id === row.id)
                    || (row.client_msg_id && existing.client_msg_id === row.client_msg_id)
            })
            if (found >= 0) merged[found] = row
            else merged.push(row)
        }
        merged.sort(function(a, b) {
            return (a.seq || Number.MAX_SAFE_INTEGER) - (b.seq || Number.MAX_SAFE_INTEGER)
        })
        messages = merged
    }

    function messagePath() {
        return "/v1/a2a/threads/" + encodeURIComponent(threadId) + "/messages"
    }

    function reload() {
        if (!api || !threadId || loading) return
        loading = true
        errorText = ""
        api.get(messagePath() + "?after=" + cursor + "&limit=200", function(error, response) {
            loading = false
            if (error || !response || !response.body || !Array.isArray(response.body.messages)) {
                errorText = qsTr("Could not load messages. Tap Refresh to try again.")
                return
            }
            mergeMessages(response.body.messages)
            cursor = response.body.next_after
            if (response.body.messages.length === 200) reload()
        })
    }

    function restorePending() {
        mergeMessages(store.pendingAgentMessages(threadId))
    }

    function sendMessage() {
        var text = composer.text.trim()
        if (!text) return
        if (encodeURIComponent(text).replace(/%[0-9A-F]{2}|./g, "x").length > 4096) {
            errorText = qsTr("Message is too long (maximum 4096 bytes).")
            return
        }
        if (!store.sendAgentMessage(threadId, text)) return
        restorePending()
        composer.text = ""
        messageList.positionViewAtEnd()
    }

    function postMessage(message) {
        if (store.sendAgentMessage(threadId, message.text, message.client_msg_id)) restorePending()
    }

    Component.onCompleted: { restorePending(); reload() }
    Connections {
        target: root.store
        function onPendingOpsChanged() { root.restorePending() }
        function onOperationAcknowledged(operation, response) {
            if (operation.type !== "agent_message" || operation.args.thread_id !== root.threadId) return
            if (response.message) {
                var saved = Object.assign({}, response.message, {client_msg_id: operation.args.client_msg_id})
                root.mergeMessages([saved])
            } else root.reload()
        }
    }
    Connections {
        target: root.stream
        function onChanged(seq) { root.reload() }
        function onReconnected() { root.reload() }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 12
        RowLayout {
            Layout.fillWidth: true
            MailActionButton {
                objectName: "backToAgents"
                text: qsTr("Back")
                icon.name: "go-previous"
                onClicked: root.QQC2.StackView.view.pop()
            }
            ColumnLayout {
                Layout.fillWidth: true
                spacing: 2
                Kirigami.Heading {
                    text: root.title
                    level: 2
                    color: root.ink
                    elide: Text.ElideRight
                    Layout.fillWidth: true
                }
                QQC2.Label {
                    text: root.thread.availability || ""
                    visible: text.length > 0
                    color: root.mutedInk
                    Layout.fillWidth: true
                    elide: Text.ElideRight
                }
            }
            MailActionButton {
                objectName: "refreshMessages"
                text: qsTr("Refresh")
                enabled: !root.loading
                onClicked: root.reload()
            }
        }
        QQC2.Label {
            objectName: "chatError"
            text: root.errorText
            visible: text.length > 0
            color: "#a33838"
            wrapMode: Text.Wrap
            Layout.fillWidth: true
        }
        QQC2.BusyIndicator {
            visible: root.loading && root.messages.length === 0
            running: visible
            Layout.alignment: Qt.AlignHCenter
        }
        ListView {
            id: messageList
            objectName: "agentMessages"
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            spacing: 10
            model: root.messages
            QQC2.ScrollBar.vertical: QQC2.ScrollBar {}
            delegate: Item {
                id: messageRow
                required property var modelData
                required property int index
                objectName: "messageRow_" + index
                width: messageList.width
                height: bubble.height
                readonly property bool outgoing: modelData.direction === "out"
                Rectangle {
                    id: bubble
                    width: parent.width * (messageRow.modelData.direction === "observed" ? 1 : 0.94)
                    anchors.right: messageRow.outgoing ? parent.right : undefined
                    height: bubbleContents.implicitHeight + 24
                    radius: 8
                    color: messageRow.outgoing ? "#e2efeb" : root.surface
                    border.color: "#dce5e3"
                    ColumnLayout {
                        id: bubbleContents
                        x: 12
                        y: 12
                        width: parent.width - 24
                        spacing: 6
                        QQC2.Label {
                            text: messageRow.modelData.direction === "observed"
                                ? qsTr("Observed") + " · " + messageRow.modelData.sender + " → " + messageRow.modelData.recipient
                                : messageRow.outgoing ? qsTr("You") : root.title
                            color: root.mutedInk
                            font.pixelSize: 12
                            wrapMode: Text.WrapAnywhere
                            textFormat: Text.PlainText
                            Layout.fillWidth: true
                        }
                        QQC2.Label {
                            objectName: "messageText_" + messageRow.index
                            text: messageRow.modelData.text || ""
                            color: root.ink
                            wrapMode: Text.WrapAnywhere
                            textFormat: Text.PlainText
                            Layout.fillWidth: true
                        }
                        QQC2.Label {
                            objectName: "deliveryState_" + messageRow.index
                            text: root.deliveryLabel(messageRow.modelData)
                            visible: text.length > 0
                            color: messageRow.modelData.delivery_state === "not_delivered" ? "#a33838" : root.mutedInk
                            font.pixelSize: 12
                            wrapMode: Text.WrapAnywhere
                            Layout.fillWidth: true
                        }
                        MailActionButton {
                            objectName: "retryMessage_" + messageRow.index
                            text: qsTr("Retry")
                            visible: messageRow.outgoing && !!messageRow.modelData.client_msg_id
                                && (messageRow.modelData.delivery_state === "not_delivered" || messageRow.modelData.delivery_state === "indeterminate")
                            onClicked: root.postMessage(messageRow.modelData)
                        }
                    }
                }
            }
            QQC2.Label {
                anchors.centerIn: parent
                text: qsTr("No messages yet.")
                visible: root.messages.length === 0 && !root.loading && root.errorText === ""
                color: root.mutedInk
            }
        }
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 6
            QQC2.TextArea {
                id: composer
                objectName: "messageComposer"
                Layout.fillWidth: true
                Layout.preferredHeight: Math.min(120, Math.max(64, implicitHeight))
                placeholderText: qsTr("Message this agent…")
                wrapMode: TextEdit.Wrap
                selectByMouse: true
            }
            MailActionButton {
                objectName: "sendMessage"
                text: qsTr("Send")
                Layout.alignment: Qt.AlignRight
                enabled: composer.text.trim().length > 0 && !!root.api && root.threadId !== ""
                onClicked: root.sendMessage()
            }
        }
    }
}
