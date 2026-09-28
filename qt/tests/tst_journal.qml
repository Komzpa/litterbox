import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "JournalPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.JournalPage {} }

    function test_listAndCompose() {
        var calls = []
        var entries = [{id: "e1", body: "First", created_at: "2026-09-28T10:00:00Z"}]
        var api = {
            get: function (path, cb) { calls.push(path); cb(null, {status: 200, body: entries}) },
            post: function (path, body, cb) {
                calls.push(path)
                compare(body.body, "Second")
                entries = entries.concat([{id: "e2", body: body.body, created_at: "2026-09-28T11:00:00Z"}])
                cb(null, {status: 201, body: entries[1]})
            }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        verify(page)
        compare(calls[0], "/v1/journal")
        compare(page.entries.length, 1)
        verify(page.compose("  Second  "))
        compare(calls[1], "/v1/journal")
        compare(calls[2], "/v1/journal")
        compare(page.entries.length, 2)
        compare(page.entries[1].body, "Second")
        compare(page.compose("   "), false)
        compare(calls.length, 3)
    }

    function test_failedComposeKeepsDraft() {
        var api = {
            get: function (path, cb) { cb(null, {status: 200, body: []}) },
            post: function (path, body, cb) { cb(new Error("save failed"), {status: 500, body: null}) }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        var draft = findChild(page, "composer")
        draft.text = "Keep me"
        page.compose(draft.text)
        compare(draft.text, "Keep me")
        verify(page.errorText.length > 0)
    }
}
