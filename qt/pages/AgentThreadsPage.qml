// SPDX-License-Identifier: MIT
// Agents: one row per a2a peer with availability and the last message.
import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2
import QtQuick.Controls.Material
import org.kde.kirigami as Kirigami

Kirigami.ScrollablePage {
    id: root
    objectName: "agentThreadsPage"
    readonly property color ink: "#263b3a"
    readonly property color mutedInk: "#586d70"
    readonly property color surface: "#ffffff"
    readonly property color canvas: "#f3f7f6"
    readonly property color accent: "#397d73"
    Kirigami.Theme.inherit: false
    Kirigami.Theme.textColor: ink
    Kirigami.Theme.disabledTextColor: mutedInk
    Kirigami.Theme.backgroundColor: canvas
    Kirigami.Theme.alternateBackgroundColor: surface
    Kirigami.Theme.highlightColor: accent
    Kirigami.Theme.focusColor: accent
    Kirigami.Theme.hoverColor: accent
    Material.theme: Material.Light
    Material.background: surface
    Material.foreground: ink
    Material.accent: accent
    Material.primary: accent
    palette.window: canvas
    palette.windowText: ink
    palette.base: surface
    palette.text: ink
    palette.button: surface
    palette.buttonText: ink
    palette.highlight: accent
    palette.highlightedText: surface
    padding: Math.max(18, Kirigami.Units.largeSpacing)
    background: Rectangle { color: canvas }
    header: BackToInboxBar { stackPage: root }

    // Contract: { get(path, cb) }; GET /v1/a2a/threads answers
    // {threads: [{id, peer_id, display_name, kind, availability, last_text, last_at, last_seq}]}.
    property var api
    required property var store
    // Where AgentChatPage is pushed; the inbox pageStack when shown there.
    property var stackView: QQC2.StackView.view
    property var threads: []
    property bool loading: false
    property string errorText: ""

    title: qsTr("Agents")

    function reload() {
        if (!api)
            return
        loading = true
        errorText = ""
        api.get("/v1/a2a/threads", function (error, response) {
            loading = false
            if (error || !response || !response.body || !Array.isArray(response.body.threads)) {
                errorText = qsTr("Could not load the agent list.")
                return
            }
            threads = response.body.threads
        })
    }

    function openThread(thread) {
        if (!stackView)
            return
        stackView.push(Qt.resolvedUrl("AgentChatPage.qml"), {api: api, store: store, thread: thread, threadId: thread.id})
    }

    function availabilityLabel(value) {
        switch (value) {
        case "online": return qsTr("Online")
        case "busy": return qsTr("Busy")
        case "offline": return qsTr("Offline")
        default: return qsTr("Unknown")
        }
    }

    function availabilityColor(value) {
        switch (value) {
        case "online": return accent
        case "busy": return "#a8741a"
        default: return mutedInk
        }
    }

    function lastTime(iso) {
        if (!iso)
            return ""
        const when = new Date(iso)
        if (isNaN(when.getTime()))
            return ""
        const now = new Date()
        if (when.toDateString() === now.toDateString())
            return Qt.formatTime(when, "hh:mm")
        return Qt.formatDate(when, when.getFullYear() === now.getFullYear() ? "d MMM" : "d MMM yyyy")
    }

    Component.onCompleted: reload()

    Connections {
        target: typeof a2aStream !== "undefined" ? a2aStream : null
        ignoreUnknownSignals: true
        function onChanged(seq) { root.reload() }
        function onReconnected() { root.reload() }
    }

    Item {
        implicitHeight: threadColumn.implicitHeight
        ColumnLayout {
            id: threadColumn
            objectName: "threadsColumn"
            width: Math.min(parent.width, 720)
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: 12

            Kirigami.Heading { text: root.title; level: 2; color: root.ink; Layout.fillWidth: true }

            QQC2.BusyIndicator {
                visible: root.loading && root.threads.length === 0
                Layout.alignment: Qt.AlignHCenter
            }

            QQC2.Label {
                objectName: "threadsError"
                visible: root.errorText !== ""
                text: root.errorText
                color: Kirigami.Theme.negativeTextColor
                wrapMode: Text.Wrap
                Layout.fillWidth: true
            }

            QQC2.Label {
                objectName: "emptyStateLabel"
                visible: root.threads.length === 0 && !root.loading && root.errorText === ""
                text: qsTr("No agents yet.")
                color: root.mutedInk
                Layout.fillWidth: true
            }

            Repeater {
                id: list
                objectName: "threadsList"
                model: root.threads

                delegate: QQC2.ItemDelegate {
                    id: row
                    objectName: "threadRow-" + modelData.peer_id
                    Layout.fillWidth: true
                    implicitHeight: Math.max(72, contentItem.implicitHeight + topPadding + bottomPadding)
                    padding: 12
                    activeFocusOnTab: true
                    Accessible.name: qsTr("%1, %2").arg(modelData.display_name).arg(root.availabilityLabel(modelData.availability))
                    background: Rectangle {
                        radius: 6
                        color: row.down ? "#e8eeed" : row.hovered ? "#eef3f2" : root.surface
                        border.width: row.visualFocus ? 2 : 1
                        border.color: row.visualFocus ? root.ink : "#dce5e3"
                    }
                    contentItem: ColumnLayout {
                        spacing: 4
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8
                            QQC2.Label {
                                objectName: "threadName"
                                text: modelData.display_name
                                color: root.ink
                                font.bold: true
                                elide: Text.ElideRight
                                Layout.fillWidth: true
                            }
                            Rectangle {
                                width: 10; height: 10; radius: 5
                                color: root.availabilityColor(modelData.availability)
                            }
                            QQC2.Label {
                                objectName: "threadAvailability"
                                text: root.availabilityLabel(modelData.availability)
                                color: root.availabilityColor(modelData.availability)
                                font: Kirigami.Theme.smallFont
                            }
                        }
                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8
                            QQC2.Label {
                                objectName: "threadLastText"
                                text: modelData.last_text ? modelData.last_text : qsTr("No messages yet")
                                color: root.mutedInk
                                elide: Text.ElideRight
                                maximumLineCount: 1
                                Layout.fillWidth: true
                            }
                            QQC2.Label {
                                text: root.lastTime(modelData.last_at)
                                visible: text.length > 0
                                color: root.mutedInk
                                font: Kirigami.Theme.smallFont
                            }
                        }
                    }
                    onClicked: root.openThread(modelData)
                }
            }
        }
    }
}
