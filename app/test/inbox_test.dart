import 'package:drift/drift.dart' show Value;
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/db/database.dart';

void main() {
  test('open cards are ordered newest first and non-open cards are excluded',
      () async {
    final database = AppDatabase(NativeDatabase.memory());
    addTearDown(database.close);

    await database.into(database.cards).insert(CardsCompanion(
          id: const Value('card-1'),
          subject: const Value('Older mail'),
          sender: const Value('Alice'),
          sortAt: Value(DateTime(2026, 9, 27)),
        ));
    await database.into(database.cards).insert(CardsCompanion(
          id: const Value('card-2'),
          subject: const Value('Newer mail'),
          sender: const Value('Bob'),
          sortAt: Value(DateTime(2026, 9, 28)),
        ));
    await database.into(database.cards).insert(CardsCompanion(
          id: const Value('card-3'),
          subject: const Value('Archived mail'),
          sender: const Value('Alice'),
          sortAt: Value(DateTime(2026, 9, 29)),
          state: const Value('archived'),
        ));

    final cards = await database.watchOpenCards().first;

    expect(cards.map((card) => card.subject), ['Newer mail', 'Older mail']);
  });
}
