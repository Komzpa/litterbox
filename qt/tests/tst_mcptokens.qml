import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "McpTokensPage"
    width: 500
    height: 500
    Component { id: pageComponent; Pages.McpTokensPage {} }

    function test_createHideAndRevoke() {
        var calls = []
        var api = {
            post: function (path, body, cb) { calls.push(path); cb(null, {status: 200, body: {id: "key-1", token: "secret-once"}}) },
            del: function (path, cb) { calls.push(path); cb(null, {status: 204, body: null}) }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        verify(page)
        page.createToken()
        compare(calls[0], "/v1/mcp-tokens")
        compare(page.tokens[0].token, "secret-once")
        verify(page.tokens[0].shown)
        page.hideToken("key-1")
        verify(!page.tokens[0].shown)
        compare(page.tokens[0].token, "")
        page.revokeToken("key-1")
        compare(calls[1], "/v1/mcp-tokens/key-1")
        compare(page.tokens.length, 0)
    }

    function test_revokeFailureRetainsToken() {
        var api = {
            post: function (path, body, cb) { cb(null, {status: 200, body: {id: "key-2", token: "secret"}}) },
            del: function (path, cb) { cb(new Error("revoke failed"), {status: 500, body: null}) }
        }
        var page = createTemporaryObject(pageComponent, this, {api: api})
        page.createToken()
        page.revokeToken("key-2")
        compare(page.tokens.length, 1)
        verify(page.errorText.length > 0)
    }
}
