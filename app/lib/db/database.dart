import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';

part 'database.g.dart';

/// Cached card metadata plus complete server message objects in [body].
@DataClassName('CardRow')
class Cards extends Table {
  TextColumn get id => text()();
  TextColumn get subject => text()();
  TextColumn get sender => text()();
  DateTimeColumn get sortAt => dateTime()();
  TextColumn get state => text().withDefault(const Constant('open'))();
  TextColumn get body => text().withDefault(const Constant('[]'))();
  TextColumn get note => text().withDefault(const Constant(''))();
  TextColumn get bundleId => text().nullable()();
  IntColumn get pinnedRank => integer().nullable()();
  DateTimeColumn get snoozeUntil => dateTime().nullable()();

  @override
  Set<Column> get primaryKey => {id};
}

@DataClassName('PendingOperation')
class Outbox extends Table {
  IntColumn get sequence => integer().autoIncrement()();
  TextColumn get opId => text().unique()();
  TextColumn get cardId => text()();
  TextColumn get type => text()();
  TextColumn get argsJson => text()();
}

@DataClassName('LocalStateEntry')
class LocalState extends Table {
  TextColumn get key => text()();
  TextColumn get value => text()();

  @override
  Set<Column> get primaryKey => {key};
}

@DriftDatabase(tables: [Cards, Outbox, LocalState])
class AppDatabase extends _$AppDatabase {
  AppDatabase([QueryExecutor? executor])
      : super(executor ?? driftDatabase(name: 'litterbox'));

  @override
  int get schemaVersion => 2;

  @override
  MigrationStrategy get migration => MigrationStrategy(
        onCreate: (m) async {
          await m.createAll();
          await _createOpenCardsIndex();
        },
        onUpgrade: (m, from, to) async {
          if (from < 2) {
            await customStatement(
                "ALTER TABLE cards ADD COLUMN body TEXT NOT NULL DEFAULT '[]'");
            await customStatement(
                "ALTER TABLE cards ADD COLUMN note TEXT NOT NULL DEFAULT ''");
            await customStatement(
                'ALTER TABLE cards ADD COLUMN bundle_id TEXT');
            await customStatement(
                'ALTER TABLE cards ADD COLUMN pinned_rank INTEGER');
            await customStatement(
                'ALTER TABLE cards ADD COLUMN snooze_until INTEGER');
            await m.createTable(outbox);
            await m.createTable(localState);
            await _createOpenCardsIndex();
          }
        },
      );

  Future<void> _createOpenCardsIndex() => customStatement(
      "CREATE INDEX IF NOT EXISTS cards_open_list_idx "
      "ON cards(pinned_rank DESC, sort_at DESC) WHERE state = 'open'");

  /// Open inbox cards, with pinned mail first and newest mail next.
  Stream<List<CardRow>> watchOpenCards() => (select(cards)
        ..where((c) => c.state.equals('open'))
        ..orderBy([
          (c) => OrderingTerm.desc(c.pinnedRank),
          (c) => OrderingTerm.desc(c.sortAt),
        ]))
      .watch();

  Future<void> setCardState(String id, String state) async {
    await (update(cards)..where((c) => c.id.equals(id)))
        .write(CardsCompanion(state: Value(state)));
  }

  Future<int?> readCursor() async {
    final row = await (select(localState)
          ..where((entry) => entry.key.equals('cursor')))
        .getSingleOrNull();
    return row == null ? null : int.parse(row.value);
  }

  Future<int> getCursor() async => await readCursor() ?? 0;

  Future<void> setCursor(int cursor) async {
    await into(localState).insertOnConflictUpdate(
      LocalStateCompanion.insert(key: 'cursor', value: cursor.toString()),
    );
  }

  Future<void> enqueue(
      String opId, String cardId, String type, String argsJson) async {
    await into(outbox).insert(
      OutboxCompanion.insert(
        opId: opId,
        cardId: cardId,
        type: type,
        argsJson: argsJson,
      ),
      mode: InsertMode.insertOrIgnore,
    );
  }

  Future<List<PendingOperation>> pendingOperations() =>
      (select(outbox)..orderBy([(op) => OrderingTerm.asc(op.sequence)])).get();

  Future<void> removeOperation(String opId) async {
    await (delete(outbox)..where((op) => op.opId.equals(opId))).go();
  }

  Future<void> removeOperations(Iterable<String> opIds) async {
    final ids = opIds.toList(growable: false);
    if (ids.isEmpty) return;
    await (delete(outbox)..where((op) => op.opId.isIn(ids))).go();
  }
}
