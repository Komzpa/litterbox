import 'dart:convert';

import 'package:drift/drift.dart' show Value;

import '../db/database.dart';

/// Offline-first access to the durable Drift mirror and operation outbox.
class LocalCardStore {
  final AppDatabase database;
  LocalCardStore(this.database);

  Stream<List<CardRow>> watchOpenCards() => database.watchOpenCards();

  Future<void> replaceSnapshot(
    Map<String, dynamic> snapshot, {
    Iterable<String> acknowledgedOperationIds = const [],
  }) async {
    final rawCards = snapshot['cards'] as List<dynamic>? ?? const [];
    final cards = rawCards.cast<Map<String, dynamic>>();
    final cursor = _readInt(snapshot['cursor']) ?? 0;
    await database.transaction(() async {
      await database.delete(database.cards).go();
      for (final card in cards) {
        await database.into(database.cards).insert(_cardFromMap(card));
      }
      await database.setCursor(cursor);
      await database.removeOperations(acknowledgedOperationIds);
    });
  }

  Future<void> applyChanges(
    Map<String, dynamic> payload, {
    Iterable<String> acknowledgedOperationIds = const [],
  }) async {
    final changes = payload['changes'] as List<dynamic>? ?? const [];
    await database.transaction(() async {
      var cursor = await database.getCursor();
      for (final rawChange in changes) {
        final change = rawChange as Map<String, dynamic>;
        final nextCursor = _readInt(change['cursor']);
        if (nextCursor != null && nextCursor > cursor) cursor = nextCursor;
        final entity = change['entity'];
        final id = change['id']?.toString();
        final value = change['value'];
        if (entity == 'card' && id != null) {
          if (value == null) {
            await (database.delete(database.cards)
                  ..where((card) => card.id.equals(id)))
                .go();
          } else {
            await _upsertCard(
              Map<String, dynamic>.from(value as Map),
              id,
            );
          }
        } else if (entity == 'message' && id != null) {
          await _applyMessageChange(id, value);
        }
      }
      await database.setCursor(cursor);
      await database.removeOperations(acknowledgedOperationIds);
    });
  }

  Future<void> _upsertCard(Map<String, dynamic> card, String fallbackId) async {
    final id = card['id'] as String? ?? fallbackId;
    final old = await (database.select(database.cards)
          ..where((row) => row.id.equals(id)))
        .getSingleOrNull();
    final companion = _cardFromMap(card, old: old, fallbackId: id);
    await database.into(database.cards).insertOnConflictUpdate(companion);
  }

  Future<void> _applyMessageChange(String id, Object? value) async {
    final message = value == null ? null : Map<String, dynamic>.from(value as Map);
    final cardId = message?['card_id'] as String?;
    if (cardId == null) return;
    final card = await (database.select(database.cards)
          ..where((row) => row.id.equals(cardId)))
        .getSingleOrNull();
    if (card == null) return;
    final messages = (jsonDecode(card.body) as List<dynamic>)
        .map((item) => Map<String, dynamic>.from(item as Map))
        .toList();
    final index = messages.indexWhere((item) => item['id'] == id);
    if (message == null) {
      if (index >= 0) messages.removeAt(index);
    } else if (index >= 0) {
      messages[index] = message;
    } else {
      messages.add(message);
    }
    await (database.update(database.cards)
          ..where((row) => row.id.equals(cardId)))
        .write(CardsCompanion(body: Value(jsonEncode(messages))));
  }

  Future<void> applyDone(String cardId, String opId) =>
      applyAction(opId, cardId, 'done', const {});

  Future<void> applyNote(String cardId, String opId, String note) =>
      applyAction(opId, cardId, 'note', {'note': note});

  /// Applies known local projections and durably queues every operation type.
  Future<void> applyAction(
    String opId,
    String cardId,
    String type,
    Map<String, dynamic> args,
  ) async {
    await database.transaction(() async {
      var update = const CardsCompanion();
      switch (type) {
        case 'done':
          update = update.copyWith(state: const Value('done'));
        case 'archive':
        case 'archive_bundle':
        case 'bundle_archive':
          update = update.copyWith(state: const Value('archived'));
        case 'snooze':
          update = update.copyWith(
            state: const Value('snoozed'),
            snoozeUntil: Value(_parseDate(args['until'])),
          );
        case 'pin':
          update = update.copyWith(
            pinnedRank: Value(args['pinned'] == true ? 1 : null),
          );
        case 'note':
          update = update.copyWith(
            note: Value((args['note'] ?? args['text']) as String? ?? ''),
          );
        case 'bundle':
        case 'set_bundle':
        case 'move_to_bundle':
          update = update.copyWith(
            bundleId: Value(args['bundle_id'] as String?),
          );
        case 'take_out':
        case 'unbundle':
          update = update.copyWith(bundleId: const Value(null));
      }
      if (_hasProjection(type)) {
        await (database.update(database.cards)
              ..where((card) => card.id.equals(cardId)))
            .write(update);
      }
      if (type == 'archive' || type == 'archive_bundle' || type == 'bundle_archive') {
        final affected = args['card_ids'];
        if (affected is List) {
          for (final otherId in affected.whereType<String>()) {
            if (otherId == cardId) continue;
            await (database.update(database.cards)
                  ..where((card) => card.id.equals(otherId)))
                .write(const CardsCompanion(state: Value('archived')));
          }
        }
      }
      if (type == 'reorder_pins' && args['card_ids'] is List) {
        final ids = (args['card_ids'] as List).whereType<String>().toList();
        for (var index = 0; index < ids.length; index++) {
          await (database.update(database.cards)
                ..where((card) => card.id.equals(ids[index])))
              .write(CardsCompanion(pinnedRank: Value(ids.length - index)));
        }
      }
      await database.enqueue(opId, cardId, type, jsonEncode(args));
    });
  }

  bool _hasProjection(String type) => const {
        'done',
        'archive',
        'archive_bundle',
        'bundle_archive',
        'snooze',
        'pin',
        'note',
        'bundle',
        'set_bundle',
        'move_to_bundle',
        'take_out',
        'unbundle',
      }.contains(type);

  CardsCompanion _cardFromMap(
    Map<String, dynamic> card, {
    CardRow? old,
    String? fallbackId,
  }) {
    final messages = card.containsKey('messages')
        ? jsonEncode(card['messages'] ?? const [])
        : old?.body ?? '[]';
    final pinned = card.containsKey('pinned_rank')
        ? _readInt(card['pinned_rank'])
        : old?.pinnedRank;
    final bundleId = card.containsKey('bundle_id')
        ? card['bundle_id'] as String?
        : old?.bundleId;
    final snoozeUntil = card.containsKey('snooze_until')
        ? _parseDate(card['snooze_until'])
        : old?.snoozeUntil;
    return CardsCompanion.insert(
      id: card['id'] as String? ?? fallbackId!,
      subject: card['subject'] as String? ?? old?.subject ?? '',
      sender: card['sender'] as String? ?? old?.sender ?? '',
      sortAt: _parseDate(card['sort_at']) ?? old?.sortAt ?? DateTime.utc(1970),
      state: Value(card['state'] as String? ?? old?.state ?? 'open'),
      body: Value(messages),
      note: Value(card['note'] as String? ?? old?.note ?? ''),
      bundleId: Value(bundleId),
      pinnedRank: Value(pinned),
      snoozeUntil: Value(snoozeUntil),
    );
  }

  Future<List<PendingOperation>> pending() => database.pendingOperations();
  Future<void> removeOperation(String id) => database.removeOperation(id);
  Future<int> cursor() => database.getCursor();
  Future<bool> hasSnapshot() async => await database.readCursor() != null;

  static int? _readInt(Object? value) {
    if (value is int) return value;
    if (value is num) return value.toInt();
    return value == null ? null : int.tryParse(value.toString());
  }

  static DateTime? _parseDate(Object? value) =>
      value == null ? null : DateTime.tryParse(value.toString());
}
