import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    id: testCase
    name: "AgentChatPage"
    width: 412
    height: 915
    visible: true
    when: windowShown
    Component { id: pageComponent; Pages.AgentChatPage {} }
    QtObject {
        id: eventStream
        signal changed(int seq)
        signal reconnected()
    }

    function row(id, seq, direction, text, state, reason) {
        return {id: id, thread_id: "thread-fixture", seq: seq, direction: direction,
            sender: direction === "out" ? "agent:darafei" : "agent:mau",
            recipient: direction === "out" ? "agent:mau" : "agent:korzhik",
            text: text, delivery_state: state, reason_code: reason || null,
            client_msg_id: direction === "out" ? "00000000-0000-4000-8000-" + ("000000000000" + seq).slice(-12) : null}
    }
    function fixtureRows() {
        return [row("observed", 3, "observed", "I asked Korzhik to review the map.", "delivered"),
            row("busy", 4, "out", "Can you check this session?", "not_delivered", "recipient_busy"),
            row("reply", 2, "in", "The map is ready. I checked the labels.", "delivered"),
            row("sent", 1, "out", "Please review the map.", "replied")]
    }
    function pageFor(api, viewWidth) {
        testCase.width = viewWidth || 412
        return createTemporaryObject(pageComponent, testCase, {
            api: api, thread: {id: "thread-fixture", peer_id: "agent:mau", display_name: "Mau", availability: "busy"},
            width: testCase.width, height: 915, stream: eventStream
        })
    }
    function test_orderDeliveryAndCaptures_data() {
        return [{tag: "phone", viewWidth: 412}, {tag: "neighbor", viewWidth: 480}]
    }
    function test_orderDeliveryAndCaptures(data) {
        var getPath = ""
        var page = pageFor({get: function(path, cb) {
            getPath = path
            cb(null, {status: 200, body: {messages: fixtureRows(), next_after: 4}})
        }}, data.viewWidth)
        verify(page)
        compare(getPath, "/v1/a2a/threads/thread-fixture/messages?after=0&limit=200")
        compare(page.messages.map(function(m) { return m.seq }).join(","), "1,2,3,4")
        compare(page.cursor, 4)
        tryVerify(function() { return findChild(page, "deliveryState_3") !== null })
        compare(findChild(page, "deliveryState_3").text, "recipient_busy / not delivered")
        compare(findChild(page, "deliveryState_0").text, "replied")
        compare(findChild(page, "messageText_1").text, "The map is ready. I checked the labels.")
        verify(findChild(page, "retryMessage_3").height >= 48)
        verify(findChild(page, "sendMessage").height >= 48)
        wait(200)
        var busy = findChild(page, "deliveryState_3")
        var position = busy.mapToItem(page, 0, 0)
        verify(position.y >= 0 && position.y + busy.height < page.height)
        var image = grabImage(page)
        compare(image.width, data.viewWidth)
        compare(image.height, 915)
        image.save("/tmp/A2aChatPage-" + data.viewWidth + "x915-busy.png")
    }
    function test_sendPendingThenReplyAndRetryId() {
        var posts = []
        var callbacks = []
        var afterPaths = []
        var updates = []
        var next = 0
        var fake = {
            get: function(path, cb) {
                afterPaths.push(path)
                cb(null, {status: 200, body: {messages: updates, next_after: next}})
            },
            post: function(path, body, cb) { posts.push({path: path, body: body}); callbacks.push(cb) }
        }
        var page = pageFor(fake)
        verify(page)
        var composer = findChild(page, "messageComposer")
        composer.text = "Hello Mau"
        findChild(page, "sendMessage").clicked()
        compare(posts.length, 1)
        compare(posts[0].path, "/v1/a2a/threads/thread-fixture/messages")
        compare(Object.keys(posts[0].body).sort().join(","), "client_msg_id,text")
        compare(posts[0].body.text, "Hello Mau")
        verify(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(posts[0].body.client_msg_id))
        compare(page.messages[0].delivery_state, "pending")
        compare(composer.text, "")
        var sent = row("new-sent", 5, "out", "Hello Mau", "pending")
        sent.client_msg_id = posts[0].body.client_msg_id
        callbacks[0](null, {status: 202, body: {message: sent}})
        compare(page.messages.length, 1)
        updates = [Object.assign({}, sent, {seq: 6, delivery_state: "not_delivered", reason_code: "recipient_busy"})]
        next = 6
        eventStream.changed(6)
        compare(page.messages.length, 1)
        compare(page.deliveryLabel(page.messages[0]), "recipient_busy / not delivered")
        tryVerify(function() { return findChild(page, "retryMessage_0") !== null })
        findChild(page, "retryMessage_0").clicked()
        compare(posts.length, 2)
        compare(posts[1].body.client_msg_id, posts[0].body.client_msg_id)
        compare(page.messages[0].delivery_state, "pending")
        callbacks[1](null, {status: 202, body: {message: sent}})
        updates = [Object.assign({}, sent, {seq: 7, delivery_state: "replied"}), row("new-reply", 8, "in", "Hello Darafei", "delivered")]
        next = 8
        eventStream.reconnected()
        compare(afterPaths[afterPaths.length - 1], "/v1/a2a/threads/thread-fixture/messages?after=6&limit=200")
        compare(page.messages.length, 2)
        compare(page.messages[0].delivery_state, "replied")
        compare(page.messages[1].direction, "in")
        compare(page.messages[1].thread_id, page.messages[0].thread_id)
        wait(200)
        var image = grabImage(page)
        compare(image.width, 412)
        compare(image.height, 915)
        image.save("/tmp/A2aChatPage-412x915-replied.png")
    }
    function test_networkFailureRetriesSameId() {
        var posted = []
        var page = pageFor({
            get: function(path, cb) { cb(null, {status: 200, body: {messages: [], next_after: 0}}) },
            post: function(path, body, cb) { posted.push(body); cb("offline", null) }
        })
        findChild(page, "messageComposer").text = "Retry safely"
        page.sendMessage()
        compare(page.messages[0].delivery_state, "indeterminate")
        page.postMessage(page.messages[0])
        compare(posted[1].client_msg_id, posted[0].client_msg_id)
        compare(page.messages.length, 1)
    }
}
