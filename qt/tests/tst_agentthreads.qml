import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    id: tc
    name: "AgentThreadsPage"
    width: 1280
    height: 900
    visible: true
    when: windowShown

    readonly property var fixture: ({threads: [
        {id: "11111111-1111-1111-1111-111111111111", peer_id: "agent:mau", display_name: "Mau", kind: "token_agent", availability: "online", last_text: "Map tiles are rebuilt.", last_at: "2026-10-13T12:40:00Z", last_seq: 9},
        {id: "22222222-2222-2222-2222-222222222222", peer_id: "agent:ugolyok", display_name: "Ugolyok", kind: "token_agent", availability: "online", last_text: "Digest sent to Telegram.", last_at: "2026-10-12T09:10:00Z", last_seq: 7},
        {id: "33333333-3333-3333-3333-333333333333", peer_id: "agent:korzhik", display_name: "Korzhik", kind: "token_agent", availability: "offline", last_text: null, last_at: null, last_seq: 0},
        {id: "44444444-4444-4444-4444-444444444444", peer_id: "agent:claude", display_name: "Claude", kind: "token_agent", availability: "unknown", last_text: "Review finished with two comments.", last_at: "2026-10-01T08:00:00Z", last_seq: 3},
        {id: "55555555-5555-5555-5555-555555555555", peer_id: "omp:tasks-loop", display_name: "omp tasks-loop session with a deliberately long name", kind: "omp_session", availability: "busy", last_text: "recipient_busy / not delivered: please retry once the current turn is over, this text is long enough to elide.", last_at: "2026-10-13T12:41:00Z", last_seq: 12}
    ]})

    function fakeApi(body, requests) {
        return {get: function (path, cb) { requests.push(path); cb(null, {status: 200, body: body}) }}
    }

    function make(api, props) {
        const pushed = []
        const stack = {pushed: pushed, push: function (url, p) { pushed.push({url: url.toString(), props: p}) }}
        const values = Object.assign({api: api, store: {}, width: 412, height: 915}, props || {})
        const page = createTemporaryObject(Qt.createComponent("../pages/AgentThreadsPage.qml"), tc, values)
        verify(page)
        page.stackView = stack
        return {page: page, stack: stack}
    }

    function test_oneRowPerPeerWithAvailabilityAndLastMessage() {
        const requests = []
        const h = make(fakeApi(fixture, requests))
        compare(requests, ["/v1/a2a/threads"])
        for (const t of fixture.threads) {
            const row = findChild(h.page, "threadRow-" + t.peer_id)
            verify(row, t.peer_id)
            const label = findChild(row, "threadAvailability").text
            compare(label, h.page.availabilityLabel(t.availability))
            compare(findChild(row, "threadName").text, t.display_name)
            compare(findChild(row, "threadLastText").text, t.last_text ? t.last_text : "No messages yet")
            verify(row.height >= 48)
        }
        compare(findChild(findChild(h.page, "threadRow-omp:tasks-loop"), "threadAvailability").text, "Busy")
        compare(h.page.threads.length, 5)
    }

    function test_tapPushesChatPage() {
        const h = make(fakeApi(fixture, []))
        const row = findChild(h.page, "threadRow-agent:mau")
        row.clicked()
        compare(h.stack.pushed.length, 1)
        verify(h.stack.pushed[0].url.endsWith("/AgentChatPage.qml"))
        compare(h.stack.pushed[0].props.threadId, "11111111-1111-1111-1111-111111111111")
        compare(h.stack.pushed[0].props.thread.peer_id, "agent:mau")
        verify(h.stack.pushed[0].props.api)
        compare(h.stack.pushed[0].props.store, h.page.store)
    }

    function test_failedLoadShowsErrorNotEmpty() {
        const h = make({get: function (p, cb) { cb(new Error("offline"), null) }})
        compare(h.page.errorText, "Could not load the agent list.")
        verify(!findChild(h.page, "emptyStateLabel").visible)
        verify(findChild(h.page, "threadsError").visible)
    }

    function test_emptyList() {
        const h = make(fakeApi({threads: []}, []))
        verify(findChild(h.page, "emptyStateLabel").visible)
    }

    function test_touchTargetsAndPalette() {
        const h = make(fakeApi(fixture, []))
        const row = findChild(h.page, "threadRow-agent:mau")
        verify(row.height >= 48)
        compare(h.page.surface.toString(), "#ffffff")
        compare(h.page.ink.toString(), "#263b3a")
        compare(h.page.accent.toString(), "#397d73")
        const back = findChild(h.page, "backToInbox")
        verify(back && back.height >= 48)
    }

    function test_reloadsAfterStreamSignal() {
        const requests = []
        const h = make(fakeApi(fixture, requests))
        compare(requests.length, 1)
        h.page.reload()
        compare(requests.length, 2)
    }
}
