import QtQuick

// Optional bridge for callback-only APIs; the shell may wire these signals
// directly in C++. API callback contract: cb(error, {status, body}).
QtObject {
    id: root
    required property var store
    required property var api
    property bool online: false
    property Connections requests: Connections {
        target: root.store
        function onRequestPost(opId, path, body) {
            root.api.post(path, body, function(error, response) {
                root.store.reportPostResult(opId, response ? response.status : 0,
                                            !error && response ? response.body : {})
            })
        }
        function onRequestCards(path) {
            root.api.get(path, function(error, response) {
                if (!error && response)
                    root.store.applyRemoteCards(response.body)
            })
        }
    }
    onOnlineChanged: store.online = online
    Component.onCompleted: store.online = online
}
