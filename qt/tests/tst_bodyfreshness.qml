import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "BodyFreshness"
    when: windowShown
    width: 598; height: 879
    property var store: resizeStore
    property string cardId: "bbbbbbbb-1111-4111-8111-111111111111"
    Component { id: reader; Pages.MailDetailPage {} }
    SignalSpy { id: requests; target: resizeStore; signalName: "requestMailBodyGet" }

    function snapshot(version) {
        return {now: [{id: cardId, source: "mail", body_version: version}], later: [], missed: []}
    }
    function test_staleCacheRefreshesOpenReader() {
        store.online = false
        verify(store.applyRemoteCards(snapshot("v1")))
        store.applyRemoteMailBody(cardId, {html: "<p>Old copy</p>", shape_version: 2, body_version: "v1",
            messages: [{id: "first", html: "<p>Old copy</p>"}]})
        tryCompare(store, "mailSyncPending", 0)
        verify(store.applyRemoteCards(snapshot("v2")))
        const page = createTemporaryObject(reader, this, {store: store, cardId: cardId,
            card: {source: "mail"}, openLinks: false, width: 598, height: 879})
        verify(page)
        compare(page.html, "<p>Old copy</p>")
        compare(page.loading, false)
        requests.clear()
        store.online = true
        tryCompare(requests, "count", 1)
        store.reportMailBodyFailed(cardId, 503)
        compare(page.html, "<p>Old copy</p>")
        compare(page.errorText, "")
        tryCompare(requests, "count", 2, 3000)
        store.applyRemoteMailBody(cardId, {html: "<p>New copy</p>", shape_version: 2, body_version: "v2",
            messages: [{id: "first", html: "<p>Old copy</p>"}, {id: "reply", html: "<p>New reply</p>"}]})
        tryCompare(page, "html", "<p>New copy</p>")
        compare(page.messages.length, 2)
        verify(page.documentHtml.indexOf("New reply") >= 0)
    }
}
