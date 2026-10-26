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
        var store = {createReminder: function(title, dueAt, recurrence) {
            call = {title: title, due_at: dueAt, recurrence: recurrence}; return "rem-1"
        }}
        var dialog = createTemporaryObject(dialogComponent, this, {store: store})
        verify(dialog)
        findChild(dialog, "titleField").text = "  Feed cat  "
        var localDate = new Date(2026, 8, 29, 14, 30)
        findChild(dialog, "whenField").text = localDate.toLocaleString(Qt.locale(), dialog.localeDateTimeFormat)
        findChild(dialog, "recurrenceBox").currentIndex = 2
        verify(dialog.submit())
        compare(call.title, "Feed cat")
        compare(call.recurrence, "weekly")
        compare(new Date(call.due_at).getTime(), localDate.getTime())
        compare(dialog.reminderId, "rem-1")
    }

    function test_invalidDateNeverPosts() {
        var called = false
        var store = {createReminder: function() { called = true; return "rem-1" }}
        var dialog = createTemporaryObject(dialogComponent, this, {store: store})
        findChild(dialog, "titleField").text = "Feed cat"
        findChild(dialog, "whenField").text = "not a date"
        compare(dialog.submit(), false)
        verify(!called)
        verify(dialog.errorText.length > 0)
    }
    function test_invalidSaveButtonKeepsDialogOpen() {
        var called = false
        var store = {createReminder: function() { called = true; return "rem-1" }}
        var dialog = createTemporaryObject(dialogComponent, this, {store: store})
        dialog.open()
        verify(dialog.visible)
        findChild(dialog, "saveButton").clicked()
        verify(dialog.visible)
        verify(dialog.errorText.length > 0)
        verify(!called)
        dialog.close()
    }

}
