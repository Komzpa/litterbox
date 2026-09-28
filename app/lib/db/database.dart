import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';

part 'database.g.dart';

/// Cards shown in the inbox. Scaffold-scope mirror of the server-side
/// `cards` table (server/db/001_mail.sql): one row per card.
@DataClassName('CardRow')
class Cards extends Table {
  TextColumn get id => text()();
  TextColumn get subject => text()();
  TextColumn get sender => text()();
  DateTimeColumn get sortAt => dateTime()();
  TextColumn get state => text().withDefault(const Constant('open'))();

  @override
  Set<Column> get primaryKey => {id};
}

@DriftDatabase(tables: [Cards])
class AppDatabase extends _$AppDatabase {
  AppDatabase([QueryExecutor? executor])
      : super(executor ?? driftDatabase(name: 'litterbox'));

  @override
  int get schemaVersion => 1;

  /// Open inbox cards, most recent first (mirrors cards_open_list_idx).
  Stream<List<CardRow>> watchOpenCards() => (select(cards)
        ..where((c) => c.state.equals('open'))
        ..orderBy([(c) => OrderingTerm.desc(c.sortAt)]))
      .watch();
}
