import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/journal/journal_screen.dart';

class FakeJournal implements JournalClient {
  final rows = <JournalEntry>[];
  @override Future<List<JournalEntry>> list() async => List.of(rows);
  @override Future<void> append(String body) async { rows.add(JournalEntry('${rows.length}', body)); }
  @override Future<void> update(String id, String body) async { final i=rows.indexWhere((e)=>e.id==id); rows[i]=JournalEntry(id,body); }
}
void main() {
  testWidgets('journal lists and composes an entry', (tester) async {
    final client=FakeJournal();
    await tester.pumpWidget(MaterialApp(home: JournalScreen(client: client)));
    await tester.enterText(find.byKey(const Key('journal-compose')), 'Private thought');
    await tester.tap(find.byKey(const Key('journal-save')));
    await tester.pumpAndSettle();
    expect(find.text('Private thought'), findsOneWidget);
  });
}
