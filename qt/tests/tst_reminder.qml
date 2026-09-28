import QtQuick
import QtTest
import "../pages" as Pages

TestCase {
    name: "ReminderDialog"
    width: 500
    height: 500
    Component { id: dialogComponent; Pages.ReminderDialog {} }

    function test_localDateAndRecurrence() {
        var call = null
        var api = { post: function (path, body, cb) { call = {path: path, body: body}; cb(null, {status: 201, body: {id: "rem-1"}}) } }
        var dialog = createTemporaryObject(dialogComponent, this, {api: api})
        verify(dialog)
        findChild(dialog, "titleField").text = "  Feed cat  "
        var localDate = new Date(2026, 8, 29, 14, 30)
        findChild(dialog, "whenField").text = localDate.toLocaleString(Qt.locale(), dialog.localeDateTimeFormat)
        findChild(dialog, "recurrenceBox").currentIndex = 2
        verify(dialog.submit())
        compare(call.path, "/v1/reminders")
        compare(call.body.title, "Feed cat")
        compare(call.body.recurrence, "weekly")
        compare(new Date(call.body.due_at).getTime(), localDate.getTime())
        compare(dialog.reminderId, "rem-1")
    }

    function test_invalidDateNeverPosts() {
        var called = false
        var api = { post: function (path, body, cb) { called = true } }
        var dialog = createTemporaryObject(dialogComponent, this, {api: api})
        findChild(dialog, "titleField").text = "Feed cat"
        findChild(dialog, "whenField").text = "not a date"
        compare(dialog.submit(), false)
        verify(!called)
        verify(dialog.errorText.length > 0)
    }
    function test_invalidSaveButtonKeepsDialogOpen() {
        var called = false
        var api = { post: function (path, body, cb) { called = true } }
        var dialog = createTemporaryObject(dialogComponent, this, {api: api})
        dialog.open()
        verify(dialog.visible)
        findChild(dialog, "saveButton").clicked()
        verify(dialog.visible)
        verify(dialog.errorText.length > 0)
        verify(!called)
        dialog.close()
    }

}
