// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'database.dart';

// ignore_for_file: type=lint
class $CardsTable extends Cards with TableInfo<$CardsTable, CardRow> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $CardsTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _idMeta = const VerificationMeta('id');
  @override
  late final GeneratedColumn<String> id = GeneratedColumn<String>(
    'id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _subjectMeta = const VerificationMeta(
    'subject',
  );
  @override
  late final GeneratedColumn<String> subject = GeneratedColumn<String>(
    'subject',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _senderMeta = const VerificationMeta('sender');
  @override
  late final GeneratedColumn<String> sender = GeneratedColumn<String>(
    'sender',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _sortAtMeta = const VerificationMeta('sortAt');
  @override
  late final GeneratedColumn<DateTime> sortAt = GeneratedColumn<DateTime>(
    'sort_at',
    aliasedName,
    false,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _stateMeta = const VerificationMeta('state');
  @override
  late final GeneratedColumn<String> state = GeneratedColumn<String>(
    'state',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant('open'),
  );
  static const VerificationMeta _bodyMeta = const VerificationMeta('body');
  @override
  late final GeneratedColumn<String> body = GeneratedColumn<String>(
    'body',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant('[]'),
  );
  static const VerificationMeta _noteMeta = const VerificationMeta('note');
  @override
  late final GeneratedColumn<String> note = GeneratedColumn<String>(
    'note',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
    defaultValue: const Constant(''),
  );
  static const VerificationMeta _bundleIdMeta = const VerificationMeta(
    'bundleId',
  );
  @override
  late final GeneratedColumn<String> bundleId = GeneratedColumn<String>(
    'bundle_id',
    aliasedName,
    true,
    type: DriftSqlType.string,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _pinnedRankMeta = const VerificationMeta(
    'pinnedRank',
  );
  @override
  late final GeneratedColumn<int> pinnedRank = GeneratedColumn<int>(
    'pinned_rank',
    aliasedName,
    true,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
  );
  static const VerificationMeta _snoozeUntilMeta = const VerificationMeta(
    'snoozeUntil',
  );
  @override
  late final GeneratedColumn<DateTime> snoozeUntil = GeneratedColumn<DateTime>(
    'snooze_until',
    aliasedName,
    true,
    type: DriftSqlType.dateTime,
    requiredDuringInsert: false,
  );
  @override
  List<GeneratedColumn> get $columns => [
    id,
    subject,
    sender,
    sortAt,
    state,
    body,
    note,
    bundleId,
    pinnedRank,
    snoozeUntil,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'cards';
  @override
  VerificationContext validateIntegrity(
    Insertable<CardRow> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('id')) {
      context.handle(_idMeta, id.isAcceptableOrUnknown(data['id']!, _idMeta));
    } else if (isInserting) {
      context.missing(_idMeta);
    }
    if (data.containsKey('subject')) {
      context.handle(
        _subjectMeta,
        subject.isAcceptableOrUnknown(data['subject']!, _subjectMeta),
      );
    } else if (isInserting) {
      context.missing(_subjectMeta);
    }
    if (data.containsKey('sender')) {
      context.handle(
        _senderMeta,
        sender.isAcceptableOrUnknown(data['sender']!, _senderMeta),
      );
    } else if (isInserting) {
      context.missing(_senderMeta);
    }
    if (data.containsKey('sort_at')) {
      context.handle(
        _sortAtMeta,
        sortAt.isAcceptableOrUnknown(data['sort_at']!, _sortAtMeta),
      );
    } else if (isInserting) {
      context.missing(_sortAtMeta);
    }
    if (data.containsKey('state')) {
      context.handle(
        _stateMeta,
        state.isAcceptableOrUnknown(data['state']!, _stateMeta),
      );
    }
    if (data.containsKey('body')) {
      context.handle(
        _bodyMeta,
        body.isAcceptableOrUnknown(data['body']!, _bodyMeta),
      );
    }
    if (data.containsKey('note')) {
      context.handle(
        _noteMeta,
        note.isAcceptableOrUnknown(data['note']!, _noteMeta),
      );
    }
    if (data.containsKey('bundle_id')) {
      context.handle(
        _bundleIdMeta,
        bundleId.isAcceptableOrUnknown(data['bundle_id']!, _bundleIdMeta),
      );
    }
    if (data.containsKey('pinned_rank')) {
      context.handle(
        _pinnedRankMeta,
        pinnedRank.isAcceptableOrUnknown(data['pinned_rank']!, _pinnedRankMeta),
      );
    }
    if (data.containsKey('snooze_until')) {
      context.handle(
        _snoozeUntilMeta,
        snoozeUntil.isAcceptableOrUnknown(
          data['snooze_until']!,
          _snoozeUntilMeta,
        ),
      );
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {id};
  @override
  CardRow map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return CardRow(
      id: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}id'],
      )!,
      subject: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}subject'],
      )!,
      sender: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}sender'],
      )!,
      sortAt: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}sort_at'],
      )!,
      state: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}state'],
      )!,
      body: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}body'],
      )!,
      note: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}note'],
      )!,
      bundleId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}bundle_id'],
      ),
      pinnedRank: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}pinned_rank'],
      ),
      snoozeUntil: attachedDatabase.typeMapping.read(
        DriftSqlType.dateTime,
        data['${effectivePrefix}snooze_until'],
      ),
    );
  }

  @override
  $CardsTable createAlias(String alias) {
    return $CardsTable(attachedDatabase, alias);
  }
}

class CardRow extends DataClass implements Insertable<CardRow> {
  final String id;
  final String subject;
  final String sender;
  final DateTime sortAt;
  final String state;
  final String body;
  final String note;
  final String? bundleId;
  final int? pinnedRank;
  final DateTime? snoozeUntil;
  const CardRow({
    required this.id,
    required this.subject,
    required this.sender,
    required this.sortAt,
    required this.state,
    required this.body,
    required this.note,
    this.bundleId,
    this.pinnedRank,
    this.snoozeUntil,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['id'] = Variable<String>(id);
    map['subject'] = Variable<String>(subject);
    map['sender'] = Variable<String>(sender);
    map['sort_at'] = Variable<DateTime>(sortAt);
    map['state'] = Variable<String>(state);
    map['body'] = Variable<String>(body);
    map['note'] = Variable<String>(note);
    if (!nullToAbsent || bundleId != null) {
      map['bundle_id'] = Variable<String>(bundleId);
    }
    if (!nullToAbsent || pinnedRank != null) {
      map['pinned_rank'] = Variable<int>(pinnedRank);
    }
    if (!nullToAbsent || snoozeUntil != null) {
      map['snooze_until'] = Variable<DateTime>(snoozeUntil);
    }
    return map;
  }

  CardsCompanion toCompanion(bool nullToAbsent) {
    return CardsCompanion(
      id: Value(id),
      subject: Value(subject),
      sender: Value(sender),
      sortAt: Value(sortAt),
      state: Value(state),
      body: Value(body),
      note: Value(note),
      bundleId: bundleId == null && nullToAbsent
          ? const Value.absent()
          : Value(bundleId),
      pinnedRank: pinnedRank == null && nullToAbsent
          ? const Value.absent()
          : Value(pinnedRank),
      snoozeUntil: snoozeUntil == null && nullToAbsent
          ? const Value.absent()
          : Value(snoozeUntil),
    );
  }

  factory CardRow.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return CardRow(
      id: serializer.fromJson<String>(json['id']),
      subject: serializer.fromJson<String>(json['subject']),
      sender: serializer.fromJson<String>(json['sender']),
      sortAt: serializer.fromJson<DateTime>(json['sortAt']),
      state: serializer.fromJson<String>(json['state']),
      body: serializer.fromJson<String>(json['body']),
      note: serializer.fromJson<String>(json['note']),
      bundleId: serializer.fromJson<String?>(json['bundleId']),
      pinnedRank: serializer.fromJson<int?>(json['pinnedRank']),
      snoozeUntil: serializer.fromJson<DateTime?>(json['snoozeUntil']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'id': serializer.toJson<String>(id),
      'subject': serializer.toJson<String>(subject),
      'sender': serializer.toJson<String>(sender),
      'sortAt': serializer.toJson<DateTime>(sortAt),
      'state': serializer.toJson<String>(state),
      'body': serializer.toJson<String>(body),
      'note': serializer.toJson<String>(note),
      'bundleId': serializer.toJson<String?>(bundleId),
      'pinnedRank': serializer.toJson<int?>(pinnedRank),
      'snoozeUntil': serializer.toJson<DateTime?>(snoozeUntil),
    };
  }

  CardRow copyWith({
    String? id,
    String? subject,
    String? sender,
    DateTime? sortAt,
    String? state,
    String? body,
    String? note,
    Value<String?> bundleId = const Value.absent(),
    Value<int?> pinnedRank = const Value.absent(),
    Value<DateTime?> snoozeUntil = const Value.absent(),
  }) => CardRow(
    id: id ?? this.id,
    subject: subject ?? this.subject,
    sender: sender ?? this.sender,
    sortAt: sortAt ?? this.sortAt,
    state: state ?? this.state,
    body: body ?? this.body,
    note: note ?? this.note,
    bundleId: bundleId.present ? bundleId.value : this.bundleId,
    pinnedRank: pinnedRank.present ? pinnedRank.value : this.pinnedRank,
    snoozeUntil: snoozeUntil.present ? snoozeUntil.value : this.snoozeUntil,
  );
  CardRow copyWithCompanion(CardsCompanion data) {
    return CardRow(
      id: data.id.present ? data.id.value : this.id,
      subject: data.subject.present ? data.subject.value : this.subject,
      sender: data.sender.present ? data.sender.value : this.sender,
      sortAt: data.sortAt.present ? data.sortAt.value : this.sortAt,
      state: data.state.present ? data.state.value : this.state,
      body: data.body.present ? data.body.value : this.body,
      note: data.note.present ? data.note.value : this.note,
      bundleId: data.bundleId.present ? data.bundleId.value : this.bundleId,
      pinnedRank: data.pinnedRank.present
          ? data.pinnedRank.value
          : this.pinnedRank,
      snoozeUntil: data.snoozeUntil.present
          ? data.snoozeUntil.value
          : this.snoozeUntil,
    );
  }

  @override
  String toString() {
    return (StringBuffer('CardRow(')
          ..write('id: $id, ')
          ..write('subject: $subject, ')
          ..write('sender: $sender, ')
          ..write('sortAt: $sortAt, ')
          ..write('state: $state, ')
          ..write('body: $body, ')
          ..write('note: $note, ')
          ..write('bundleId: $bundleId, ')
          ..write('pinnedRank: $pinnedRank, ')
          ..write('snoozeUntil: $snoozeUntil')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(
    id,
    subject,
    sender,
    sortAt,
    state,
    body,
    note,
    bundleId,
    pinnedRank,
    snoozeUntil,
  );
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is CardRow &&
          other.id == this.id &&
          other.subject == this.subject &&
          other.sender == this.sender &&
          other.sortAt == this.sortAt &&
          other.state == this.state &&
          other.body == this.body &&
          other.note == this.note &&
          other.bundleId == this.bundleId &&
          other.pinnedRank == this.pinnedRank &&
          other.snoozeUntil == this.snoozeUntil);
}

class CardsCompanion extends UpdateCompanion<CardRow> {
  final Value<String> id;
  final Value<String> subject;
  final Value<String> sender;
  final Value<DateTime> sortAt;
  final Value<String> state;
  final Value<String> body;
  final Value<String> note;
  final Value<String?> bundleId;
  final Value<int?> pinnedRank;
  final Value<DateTime?> snoozeUntil;
  final Value<int> rowid;
  const CardsCompanion({
    this.id = const Value.absent(),
    this.subject = const Value.absent(),
    this.sender = const Value.absent(),
    this.sortAt = const Value.absent(),
    this.state = const Value.absent(),
    this.body = const Value.absent(),
    this.note = const Value.absent(),
    this.bundleId = const Value.absent(),
    this.pinnedRank = const Value.absent(),
    this.snoozeUntil = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  CardsCompanion.insert({
    required String id,
    required String subject,
    required String sender,
    required DateTime sortAt,
    this.state = const Value.absent(),
    this.body = const Value.absent(),
    this.note = const Value.absent(),
    this.bundleId = const Value.absent(),
    this.pinnedRank = const Value.absent(),
    this.snoozeUntil = const Value.absent(),
    this.rowid = const Value.absent(),
  }) : id = Value(id),
       subject = Value(subject),
       sender = Value(sender),
       sortAt = Value(sortAt);
  static Insertable<CardRow> custom({
    Expression<String>? id,
    Expression<String>? subject,
    Expression<String>? sender,
    Expression<DateTime>? sortAt,
    Expression<String>? state,
    Expression<String>? body,
    Expression<String>? note,
    Expression<String>? bundleId,
    Expression<int>? pinnedRank,
    Expression<DateTime>? snoozeUntil,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (id != null) 'id': id,
      if (subject != null) 'subject': subject,
      if (sender != null) 'sender': sender,
      if (sortAt != null) 'sort_at': sortAt,
      if (state != null) 'state': state,
      if (body != null) 'body': body,
      if (note != null) 'note': note,
      if (bundleId != null) 'bundle_id': bundleId,
      if (pinnedRank != null) 'pinned_rank': pinnedRank,
      if (snoozeUntil != null) 'snooze_until': snoozeUntil,
      if (rowid != null) 'rowid': rowid,
    });
  }

  CardsCompanion copyWith({
    Value<String>? id,
    Value<String>? subject,
    Value<String>? sender,
    Value<DateTime>? sortAt,
    Value<String>? state,
    Value<String>? body,
    Value<String>? note,
    Value<String?>? bundleId,
    Value<int?>? pinnedRank,
    Value<DateTime?>? snoozeUntil,
    Value<int>? rowid,
  }) {
    return CardsCompanion(
      id: id ?? this.id,
      subject: subject ?? this.subject,
      sender: sender ?? this.sender,
      sortAt: sortAt ?? this.sortAt,
      state: state ?? this.state,
      body: body ?? this.body,
      note: note ?? this.note,
      bundleId: bundleId ?? this.bundleId,
      pinnedRank: pinnedRank ?? this.pinnedRank,
      snoozeUntil: snoozeUntil ?? this.snoozeUntil,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (id.present) {
      map['id'] = Variable<String>(id.value);
    }
    if (subject.present) {
      map['subject'] = Variable<String>(subject.value);
    }
    if (sender.present) {
      map['sender'] = Variable<String>(sender.value);
    }
    if (sortAt.present) {
      map['sort_at'] = Variable<DateTime>(sortAt.value);
    }
    if (state.present) {
      map['state'] = Variable<String>(state.value);
    }
    if (body.present) {
      map['body'] = Variable<String>(body.value);
    }
    if (note.present) {
      map['note'] = Variable<String>(note.value);
    }
    if (bundleId.present) {
      map['bundle_id'] = Variable<String>(bundleId.value);
    }
    if (pinnedRank.present) {
      map['pinned_rank'] = Variable<int>(pinnedRank.value);
    }
    if (snoozeUntil.present) {
      map['snooze_until'] = Variable<DateTime>(snoozeUntil.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('CardsCompanion(')
          ..write('id: $id, ')
          ..write('subject: $subject, ')
          ..write('sender: $sender, ')
          ..write('sortAt: $sortAt, ')
          ..write('state: $state, ')
          ..write('body: $body, ')
          ..write('note: $note, ')
          ..write('bundleId: $bundleId, ')
          ..write('pinnedRank: $pinnedRank, ')
          ..write('snoozeUntil: $snoozeUntil, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

class $OutboxTable extends Outbox
    with TableInfo<$OutboxTable, PendingOperation> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $OutboxTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _sequenceMeta = const VerificationMeta(
    'sequence',
  );
  @override
  late final GeneratedColumn<int> sequence = GeneratedColumn<int>(
    'sequence',
    aliasedName,
    false,
    hasAutoIncrement: true,
    type: DriftSqlType.int,
    requiredDuringInsert: false,
    defaultConstraints: GeneratedColumn.constraintIsAlways(
      'PRIMARY KEY AUTOINCREMENT',
    ),
  );
  static const VerificationMeta _opIdMeta = const VerificationMeta('opId');
  @override
  late final GeneratedColumn<String> opId = GeneratedColumn<String>(
    'op_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
    defaultConstraints: GeneratedColumn.constraintIsAlways('UNIQUE'),
  );
  static const VerificationMeta _cardIdMeta = const VerificationMeta('cardId');
  @override
  late final GeneratedColumn<String> cardId = GeneratedColumn<String>(
    'card_id',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _typeMeta = const VerificationMeta('type');
  @override
  late final GeneratedColumn<String> type = GeneratedColumn<String>(
    'type',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _argsJsonMeta = const VerificationMeta(
    'argsJson',
  );
  @override
  late final GeneratedColumn<String> argsJson = GeneratedColumn<String>(
    'args_json',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [
    sequence,
    opId,
    cardId,
    type,
    argsJson,
  ];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'outbox';
  @override
  VerificationContext validateIntegrity(
    Insertable<PendingOperation> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('sequence')) {
      context.handle(
        _sequenceMeta,
        sequence.isAcceptableOrUnknown(data['sequence']!, _sequenceMeta),
      );
    }
    if (data.containsKey('op_id')) {
      context.handle(
        _opIdMeta,
        opId.isAcceptableOrUnknown(data['op_id']!, _opIdMeta),
      );
    } else if (isInserting) {
      context.missing(_opIdMeta);
    }
    if (data.containsKey('card_id')) {
      context.handle(
        _cardIdMeta,
        cardId.isAcceptableOrUnknown(data['card_id']!, _cardIdMeta),
      );
    } else if (isInserting) {
      context.missing(_cardIdMeta);
    }
    if (data.containsKey('type')) {
      context.handle(
        _typeMeta,
        type.isAcceptableOrUnknown(data['type']!, _typeMeta),
      );
    } else if (isInserting) {
      context.missing(_typeMeta);
    }
    if (data.containsKey('args_json')) {
      context.handle(
        _argsJsonMeta,
        argsJson.isAcceptableOrUnknown(data['args_json']!, _argsJsonMeta),
      );
    } else if (isInserting) {
      context.missing(_argsJsonMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {sequence};
  @override
  PendingOperation map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return PendingOperation(
      sequence: attachedDatabase.typeMapping.read(
        DriftSqlType.int,
        data['${effectivePrefix}sequence'],
      )!,
      opId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}op_id'],
      )!,
      cardId: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}card_id'],
      )!,
      type: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}type'],
      )!,
      argsJson: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}args_json'],
      )!,
    );
  }

  @override
  $OutboxTable createAlias(String alias) {
    return $OutboxTable(attachedDatabase, alias);
  }
}

class PendingOperation extends DataClass
    implements Insertable<PendingOperation> {
  final int sequence;
  final String opId;
  final String cardId;
  final String type;
  final String argsJson;
  const PendingOperation({
    required this.sequence,
    required this.opId,
    required this.cardId,
    required this.type,
    required this.argsJson,
  });
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['sequence'] = Variable<int>(sequence);
    map['op_id'] = Variable<String>(opId);
    map['card_id'] = Variable<String>(cardId);
    map['type'] = Variable<String>(type);
    map['args_json'] = Variable<String>(argsJson);
    return map;
  }

  OutboxCompanion toCompanion(bool nullToAbsent) {
    return OutboxCompanion(
      sequence: Value(sequence),
      opId: Value(opId),
      cardId: Value(cardId),
      type: Value(type),
      argsJson: Value(argsJson),
    );
  }

  factory PendingOperation.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return PendingOperation(
      sequence: serializer.fromJson<int>(json['sequence']),
      opId: serializer.fromJson<String>(json['opId']),
      cardId: serializer.fromJson<String>(json['cardId']),
      type: serializer.fromJson<String>(json['type']),
      argsJson: serializer.fromJson<String>(json['argsJson']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'sequence': serializer.toJson<int>(sequence),
      'opId': serializer.toJson<String>(opId),
      'cardId': serializer.toJson<String>(cardId),
      'type': serializer.toJson<String>(type),
      'argsJson': serializer.toJson<String>(argsJson),
    };
  }

  PendingOperation copyWith({
    int? sequence,
    String? opId,
    String? cardId,
    String? type,
    String? argsJson,
  }) => PendingOperation(
    sequence: sequence ?? this.sequence,
    opId: opId ?? this.opId,
    cardId: cardId ?? this.cardId,
    type: type ?? this.type,
    argsJson: argsJson ?? this.argsJson,
  );
  PendingOperation copyWithCompanion(OutboxCompanion data) {
    return PendingOperation(
      sequence: data.sequence.present ? data.sequence.value : this.sequence,
      opId: data.opId.present ? data.opId.value : this.opId,
      cardId: data.cardId.present ? data.cardId.value : this.cardId,
      type: data.type.present ? data.type.value : this.type,
      argsJson: data.argsJson.present ? data.argsJson.value : this.argsJson,
    );
  }

  @override
  String toString() {
    return (StringBuffer('PendingOperation(')
          ..write('sequence: $sequence, ')
          ..write('opId: $opId, ')
          ..write('cardId: $cardId, ')
          ..write('type: $type, ')
          ..write('argsJson: $argsJson')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(sequence, opId, cardId, type, argsJson);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is PendingOperation &&
          other.sequence == this.sequence &&
          other.opId == this.opId &&
          other.cardId == this.cardId &&
          other.type == this.type &&
          other.argsJson == this.argsJson);
}

class OutboxCompanion extends UpdateCompanion<PendingOperation> {
  final Value<int> sequence;
  final Value<String> opId;
  final Value<String> cardId;
  final Value<String> type;
  final Value<String> argsJson;
  const OutboxCompanion({
    this.sequence = const Value.absent(),
    this.opId = const Value.absent(),
    this.cardId = const Value.absent(),
    this.type = const Value.absent(),
    this.argsJson = const Value.absent(),
  });
  OutboxCompanion.insert({
    this.sequence = const Value.absent(),
    required String opId,
    required String cardId,
    required String type,
    required String argsJson,
  }) : opId = Value(opId),
       cardId = Value(cardId),
       type = Value(type),
       argsJson = Value(argsJson);
  static Insertable<PendingOperation> custom({
    Expression<int>? sequence,
    Expression<String>? opId,
    Expression<String>? cardId,
    Expression<String>? type,
    Expression<String>? argsJson,
  }) {
    return RawValuesInsertable({
      if (sequence != null) 'sequence': sequence,
      if (opId != null) 'op_id': opId,
      if (cardId != null) 'card_id': cardId,
      if (type != null) 'type': type,
      if (argsJson != null) 'args_json': argsJson,
    });
  }

  OutboxCompanion copyWith({
    Value<int>? sequence,
    Value<String>? opId,
    Value<String>? cardId,
    Value<String>? type,
    Value<String>? argsJson,
  }) {
    return OutboxCompanion(
      sequence: sequence ?? this.sequence,
      opId: opId ?? this.opId,
      cardId: cardId ?? this.cardId,
      type: type ?? this.type,
      argsJson: argsJson ?? this.argsJson,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (sequence.present) {
      map['sequence'] = Variable<int>(sequence.value);
    }
    if (opId.present) {
      map['op_id'] = Variable<String>(opId.value);
    }
    if (cardId.present) {
      map['card_id'] = Variable<String>(cardId.value);
    }
    if (type.present) {
      map['type'] = Variable<String>(type.value);
    }
    if (argsJson.present) {
      map['args_json'] = Variable<String>(argsJson.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('OutboxCompanion(')
          ..write('sequence: $sequence, ')
          ..write('opId: $opId, ')
          ..write('cardId: $cardId, ')
          ..write('type: $type, ')
          ..write('argsJson: $argsJson')
          ..write(')'))
        .toString();
  }
}

class $LocalStateTable extends LocalState
    with TableInfo<$LocalStateTable, LocalStateEntry> {
  @override
  final GeneratedDatabase attachedDatabase;
  final String? _alias;
  $LocalStateTable(this.attachedDatabase, [this._alias]);
  static const VerificationMeta _keyMeta = const VerificationMeta('key');
  @override
  late final GeneratedColumn<String> key = GeneratedColumn<String>(
    'key',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  static const VerificationMeta _valueMeta = const VerificationMeta('value');
  @override
  late final GeneratedColumn<String> value = GeneratedColumn<String>(
    'value',
    aliasedName,
    false,
    type: DriftSqlType.string,
    requiredDuringInsert: true,
  );
  @override
  List<GeneratedColumn> get $columns => [key, value];
  @override
  String get aliasedName => _alias ?? actualTableName;
  @override
  String get actualTableName => $name;
  static const String $name = 'local_state';
  @override
  VerificationContext validateIntegrity(
    Insertable<LocalStateEntry> instance, {
    bool isInserting = false,
  }) {
    final context = VerificationContext();
    final data = instance.toColumns(true);
    if (data.containsKey('key')) {
      context.handle(
        _keyMeta,
        key.isAcceptableOrUnknown(data['key']!, _keyMeta),
      );
    } else if (isInserting) {
      context.missing(_keyMeta);
    }
    if (data.containsKey('value')) {
      context.handle(
        _valueMeta,
        value.isAcceptableOrUnknown(data['value']!, _valueMeta),
      );
    } else if (isInserting) {
      context.missing(_valueMeta);
    }
    return context;
  }

  @override
  Set<GeneratedColumn> get $primaryKey => {key};
  @override
  LocalStateEntry map(Map<String, dynamic> data, {String? tablePrefix}) {
    final effectivePrefix = tablePrefix != null ? '$tablePrefix.' : '';
    return LocalStateEntry(
      key: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}key'],
      )!,
      value: attachedDatabase.typeMapping.read(
        DriftSqlType.string,
        data['${effectivePrefix}value'],
      )!,
    );
  }

  @override
  $LocalStateTable createAlias(String alias) {
    return $LocalStateTable(attachedDatabase, alias);
  }
}

class LocalStateEntry extends DataClass implements Insertable<LocalStateEntry> {
  final String key;
  final String value;
  const LocalStateEntry({required this.key, required this.value});
  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    map['key'] = Variable<String>(key);
    map['value'] = Variable<String>(value);
    return map;
  }

  LocalStateCompanion toCompanion(bool nullToAbsent) {
    return LocalStateCompanion(key: Value(key), value: Value(value));
  }

  factory LocalStateEntry.fromJson(
    Map<String, dynamic> json, {
    ValueSerializer? serializer,
  }) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return LocalStateEntry(
      key: serializer.fromJson<String>(json['key']),
      value: serializer.fromJson<String>(json['value']),
    );
  }
  @override
  Map<String, dynamic> toJson({ValueSerializer? serializer}) {
    serializer ??= driftRuntimeOptions.defaultSerializer;
    return <String, dynamic>{
      'key': serializer.toJson<String>(key),
      'value': serializer.toJson<String>(value),
    };
  }

  LocalStateEntry copyWith({String? key, String? value}) =>
      LocalStateEntry(key: key ?? this.key, value: value ?? this.value);
  LocalStateEntry copyWithCompanion(LocalStateCompanion data) {
    return LocalStateEntry(
      key: data.key.present ? data.key.value : this.key,
      value: data.value.present ? data.value.value : this.value,
    );
  }

  @override
  String toString() {
    return (StringBuffer('LocalStateEntry(')
          ..write('key: $key, ')
          ..write('value: $value')
          ..write(')'))
        .toString();
  }

  @override
  int get hashCode => Object.hash(key, value);
  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      (other is LocalStateEntry &&
          other.key == this.key &&
          other.value == this.value);
}

class LocalStateCompanion extends UpdateCompanion<LocalStateEntry> {
  final Value<String> key;
  final Value<String> value;
  final Value<int> rowid;
  const LocalStateCompanion({
    this.key = const Value.absent(),
    this.value = const Value.absent(),
    this.rowid = const Value.absent(),
  });
  LocalStateCompanion.insert({
    required String key,
    required String value,
    this.rowid = const Value.absent(),
  }) : key = Value(key),
       value = Value(value);
  static Insertable<LocalStateEntry> custom({
    Expression<String>? key,
    Expression<String>? value,
    Expression<int>? rowid,
  }) {
    return RawValuesInsertable({
      if (key != null) 'key': key,
      if (value != null) 'value': value,
      if (rowid != null) 'rowid': rowid,
    });
  }

  LocalStateCompanion copyWith({
    Value<String>? key,
    Value<String>? value,
    Value<int>? rowid,
  }) {
    return LocalStateCompanion(
      key: key ?? this.key,
      value: value ?? this.value,
      rowid: rowid ?? this.rowid,
    );
  }

  @override
  Map<String, Expression> toColumns(bool nullToAbsent) {
    final map = <String, Expression>{};
    if (key.present) {
      map['key'] = Variable<String>(key.value);
    }
    if (value.present) {
      map['value'] = Variable<String>(value.value);
    }
    if (rowid.present) {
      map['rowid'] = Variable<int>(rowid.value);
    }
    return map;
  }

  @override
  String toString() {
    return (StringBuffer('LocalStateCompanion(')
          ..write('key: $key, ')
          ..write('value: $value, ')
          ..write('rowid: $rowid')
          ..write(')'))
        .toString();
  }
}

abstract class _$AppDatabase extends GeneratedDatabase {
  _$AppDatabase(QueryExecutor e) : super(e);
  $AppDatabaseManager get managers => $AppDatabaseManager(this);
  late final $CardsTable cards = $CardsTable(this);
  late final $OutboxTable outbox = $OutboxTable(this);
  late final $LocalStateTable localState = $LocalStateTable(this);
  @override
  Iterable<TableInfo<Table, Object?>> get allTables =>
      allSchemaEntities.whereType<TableInfo<Table, Object?>>();
  @override
  List<DatabaseSchemaEntity> get allSchemaEntities => [
    cards,
    outbox,
    localState,
  ];
}

typedef $$CardsTableCreateCompanionBuilder =
    CardsCompanion Function({
      required String id,
      required String subject,
      required String sender,
      required DateTime sortAt,
      Value<String> state,
      Value<String> body,
      Value<String> note,
      Value<String?> bundleId,
      Value<int?> pinnedRank,
      Value<DateTime?> snoozeUntil,
      Value<int> rowid,
    });
typedef $$CardsTableUpdateCompanionBuilder =
    CardsCompanion Function({
      Value<String> id,
      Value<String> subject,
      Value<String> sender,
      Value<DateTime> sortAt,
      Value<String> state,
      Value<String> body,
      Value<String> note,
      Value<String?> bundleId,
      Value<int?> pinnedRank,
      Value<DateTime?> snoozeUntil,
      Value<int> rowid,
    });

class $$CardsTableFilterComposer extends Composer<_$AppDatabase, $CardsTable> {
  $$CardsTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get subject => $composableBuilder(
    column: $table.subject,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get sender => $composableBuilder(
    column: $table.sender,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get sortAt => $composableBuilder(
    column: $table.sortAt,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get state => $composableBuilder(
    column: $table.state,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get note => $composableBuilder(
    column: $table.note,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get bundleId => $composableBuilder(
    column: $table.bundleId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<int> get pinnedRank => $composableBuilder(
    column: $table.pinnedRank,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<DateTime> get snoozeUntil => $composableBuilder(
    column: $table.snoozeUntil,
    builder: (column) => ColumnFilters(column),
  );
}

class $$CardsTableOrderingComposer
    extends Composer<_$AppDatabase, $CardsTable> {
  $$CardsTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get id => $composableBuilder(
    column: $table.id,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get subject => $composableBuilder(
    column: $table.subject,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get sender => $composableBuilder(
    column: $table.sender,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get sortAt => $composableBuilder(
    column: $table.sortAt,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get state => $composableBuilder(
    column: $table.state,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get body => $composableBuilder(
    column: $table.body,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get note => $composableBuilder(
    column: $table.note,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get bundleId => $composableBuilder(
    column: $table.bundleId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<int> get pinnedRank => $composableBuilder(
    column: $table.pinnedRank,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<DateTime> get snoozeUntil => $composableBuilder(
    column: $table.snoozeUntil,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$CardsTableAnnotationComposer
    extends Composer<_$AppDatabase, $CardsTable> {
  $$CardsTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get id =>
      $composableBuilder(column: $table.id, builder: (column) => column);

  GeneratedColumn<String> get subject =>
      $composableBuilder(column: $table.subject, builder: (column) => column);

  GeneratedColumn<String> get sender =>
      $composableBuilder(column: $table.sender, builder: (column) => column);

  GeneratedColumn<DateTime> get sortAt =>
      $composableBuilder(column: $table.sortAt, builder: (column) => column);

  GeneratedColumn<String> get state =>
      $composableBuilder(column: $table.state, builder: (column) => column);

  GeneratedColumn<String> get body =>
      $composableBuilder(column: $table.body, builder: (column) => column);

  GeneratedColumn<String> get note =>
      $composableBuilder(column: $table.note, builder: (column) => column);

  GeneratedColumn<String> get bundleId =>
      $composableBuilder(column: $table.bundleId, builder: (column) => column);

  GeneratedColumn<int> get pinnedRank => $composableBuilder(
    column: $table.pinnedRank,
    builder: (column) => column,
  );

  GeneratedColumn<DateTime> get snoozeUntil => $composableBuilder(
    column: $table.snoozeUntil,
    builder: (column) => column,
  );
}

class $$CardsTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $CardsTable,
          CardRow,
          $$CardsTableFilterComposer,
          $$CardsTableOrderingComposer,
          $$CardsTableAnnotationComposer,
          $$CardsTableCreateCompanionBuilder,
          $$CardsTableUpdateCompanionBuilder,
          (CardRow, BaseReferences<_$AppDatabase, $CardsTable, CardRow>),
          CardRow,
          PrefetchHooks Function()
        > {
  $$CardsTableTableManager(_$AppDatabase db, $CardsTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$CardsTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$CardsTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$CardsTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> id = const Value.absent(),
                Value<String> subject = const Value.absent(),
                Value<String> sender = const Value.absent(),
                Value<DateTime> sortAt = const Value.absent(),
                Value<String> state = const Value.absent(),
                Value<String> body = const Value.absent(),
                Value<String> note = const Value.absent(),
                Value<String?> bundleId = const Value.absent(),
                Value<int?> pinnedRank = const Value.absent(),
                Value<DateTime?> snoozeUntil = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CardsCompanion(
                id: id,
                subject: subject,
                sender: sender,
                sortAt: sortAt,
                state: state,
                body: body,
                note: note,
                bundleId: bundleId,
                pinnedRank: pinnedRank,
                snoozeUntil: snoozeUntil,
                rowid: rowid,
              ),
          createCompanionCallback:
              ({
                required String id,
                required String subject,
                required String sender,
                required DateTime sortAt,
                Value<String> state = const Value.absent(),
                Value<String> body = const Value.absent(),
                Value<String> note = const Value.absent(),
                Value<String?> bundleId = const Value.absent(),
                Value<int?> pinnedRank = const Value.absent(),
                Value<DateTime?> snoozeUntil = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => CardsCompanion.insert(
                id: id,
                subject: subject,
                sender: sender,
                sortAt: sortAt,
                state: state,
                body: body,
                note: note,
                bundleId: bundleId,
                pinnedRank: pinnedRank,
                snoozeUntil: snoozeUntil,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$CardsTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $CardsTable,
      CardRow,
      $$CardsTableFilterComposer,
      $$CardsTableOrderingComposer,
      $$CardsTableAnnotationComposer,
      $$CardsTableCreateCompanionBuilder,
      $$CardsTableUpdateCompanionBuilder,
      (CardRow, BaseReferences<_$AppDatabase, $CardsTable, CardRow>),
      CardRow,
      PrefetchHooks Function()
    >;
typedef $$OutboxTableCreateCompanionBuilder =
    OutboxCompanion Function({
      Value<int> sequence,
      required String opId,
      required String cardId,
      required String type,
      required String argsJson,
    });
typedef $$OutboxTableUpdateCompanionBuilder =
    OutboxCompanion Function({
      Value<int> sequence,
      Value<String> opId,
      Value<String> cardId,
      Value<String> type,
      Value<String> argsJson,
    });

class $$OutboxTableFilterComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<int> get sequence => $composableBuilder(
    column: $table.sequence,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get opId => $composableBuilder(
    column: $table.opId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get cardId => $composableBuilder(
    column: $table.cardId,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get type => $composableBuilder(
    column: $table.type,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get argsJson => $composableBuilder(
    column: $table.argsJson,
    builder: (column) => ColumnFilters(column),
  );
}

class $$OutboxTableOrderingComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<int> get sequence => $composableBuilder(
    column: $table.sequence,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get opId => $composableBuilder(
    column: $table.opId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get cardId => $composableBuilder(
    column: $table.cardId,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get type => $composableBuilder(
    column: $table.type,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get argsJson => $composableBuilder(
    column: $table.argsJson,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$OutboxTableAnnotationComposer
    extends Composer<_$AppDatabase, $OutboxTable> {
  $$OutboxTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<int> get sequence =>
      $composableBuilder(column: $table.sequence, builder: (column) => column);

  GeneratedColumn<String> get opId =>
      $composableBuilder(column: $table.opId, builder: (column) => column);

  GeneratedColumn<String> get cardId =>
      $composableBuilder(column: $table.cardId, builder: (column) => column);

  GeneratedColumn<String> get type =>
      $composableBuilder(column: $table.type, builder: (column) => column);

  GeneratedColumn<String> get argsJson =>
      $composableBuilder(column: $table.argsJson, builder: (column) => column);
}

class $$OutboxTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $OutboxTable,
          PendingOperation,
          $$OutboxTableFilterComposer,
          $$OutboxTableOrderingComposer,
          $$OutboxTableAnnotationComposer,
          $$OutboxTableCreateCompanionBuilder,
          $$OutboxTableUpdateCompanionBuilder,
          (
            PendingOperation,
            BaseReferences<_$AppDatabase, $OutboxTable, PendingOperation>,
          ),
          PendingOperation,
          PrefetchHooks Function()
        > {
  $$OutboxTableTableManager(_$AppDatabase db, $OutboxTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$OutboxTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$OutboxTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$OutboxTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<int> sequence = const Value.absent(),
                Value<String> opId = const Value.absent(),
                Value<String> cardId = const Value.absent(),
                Value<String> type = const Value.absent(),
                Value<String> argsJson = const Value.absent(),
              }) => OutboxCompanion(
                sequence: sequence,
                opId: opId,
                cardId: cardId,
                type: type,
                argsJson: argsJson,
              ),
          createCompanionCallback:
              ({
                Value<int> sequence = const Value.absent(),
                required String opId,
                required String cardId,
                required String type,
                required String argsJson,
              }) => OutboxCompanion.insert(
                sequence: sequence,
                opId: opId,
                cardId: cardId,
                type: type,
                argsJson: argsJson,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$OutboxTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $OutboxTable,
      PendingOperation,
      $$OutboxTableFilterComposer,
      $$OutboxTableOrderingComposer,
      $$OutboxTableAnnotationComposer,
      $$OutboxTableCreateCompanionBuilder,
      $$OutboxTableUpdateCompanionBuilder,
      (
        PendingOperation,
        BaseReferences<_$AppDatabase, $OutboxTable, PendingOperation>,
      ),
      PendingOperation,
      PrefetchHooks Function()
    >;
typedef $$LocalStateTableCreateCompanionBuilder =
    LocalStateCompanion Function({
      required String key,
      required String value,
      Value<int> rowid,
    });
typedef $$LocalStateTableUpdateCompanionBuilder =
    LocalStateCompanion Function({
      Value<String> key,
      Value<String> value,
      Value<int> rowid,
    });

class $$LocalStateTableFilterComposer
    extends Composer<_$AppDatabase, $LocalStateTable> {
  $$LocalStateTableFilterComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnFilters<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnFilters(column),
  );

  ColumnFilters<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnFilters(column),
  );
}

class $$LocalStateTableOrderingComposer
    extends Composer<_$AppDatabase, $LocalStateTable> {
  $$LocalStateTableOrderingComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  ColumnOrderings<String> get key => $composableBuilder(
    column: $table.key,
    builder: (column) => ColumnOrderings(column),
  );

  ColumnOrderings<String> get value => $composableBuilder(
    column: $table.value,
    builder: (column) => ColumnOrderings(column),
  );
}

class $$LocalStateTableAnnotationComposer
    extends Composer<_$AppDatabase, $LocalStateTable> {
  $$LocalStateTableAnnotationComposer({
    required super.$db,
    required super.$table,
    super.joinBuilder,
    super.$addJoinBuilderToRootComposer,
    super.$removeJoinBuilderFromRootComposer,
  });
  GeneratedColumn<String> get key =>
      $composableBuilder(column: $table.key, builder: (column) => column);

  GeneratedColumn<String> get value =>
      $composableBuilder(column: $table.value, builder: (column) => column);
}

class $$LocalStateTableTableManager
    extends
        RootTableManager<
          _$AppDatabase,
          $LocalStateTable,
          LocalStateEntry,
          $$LocalStateTableFilterComposer,
          $$LocalStateTableOrderingComposer,
          $$LocalStateTableAnnotationComposer,
          $$LocalStateTableCreateCompanionBuilder,
          $$LocalStateTableUpdateCompanionBuilder,
          (
            LocalStateEntry,
            BaseReferences<_$AppDatabase, $LocalStateTable, LocalStateEntry>,
          ),
          LocalStateEntry,
          PrefetchHooks Function()
        > {
  $$LocalStateTableTableManager(_$AppDatabase db, $LocalStateTable table)
    : super(
        TableManagerState(
          db: db,
          table: table,
          createFilteringComposer: () =>
              $$LocalStateTableFilterComposer($db: db, $table: table),
          createOrderingComposer: () =>
              $$LocalStateTableOrderingComposer($db: db, $table: table),
          createComputedFieldComposer: () =>
              $$LocalStateTableAnnotationComposer($db: db, $table: table),
          updateCompanionCallback:
              ({
                Value<String> key = const Value.absent(),
                Value<String> value = const Value.absent(),
                Value<int> rowid = const Value.absent(),
              }) => LocalStateCompanion(key: key, value: value, rowid: rowid),
          createCompanionCallback:
              ({
                required String key,
                required String value,
                Value<int> rowid = const Value.absent(),
              }) => LocalStateCompanion.insert(
                key: key,
                value: value,
                rowid: rowid,
              ),
          withReferenceMapper: (p0) => p0
              .map((e) => (e.readTable(table), BaseReferences(db, table, e)))
              .toList(),
          prefetchHooksCallback: null,
        ),
      );
}

typedef $$LocalStateTableProcessedTableManager =
    ProcessedTableManager<
      _$AppDatabase,
      $LocalStateTable,
      LocalStateEntry,
      $$LocalStateTableFilterComposer,
      $$LocalStateTableOrderingComposer,
      $$LocalStateTableAnnotationComposer,
      $$LocalStateTableCreateCompanionBuilder,
      $$LocalStateTableUpdateCompanionBuilder,
      (
        LocalStateEntry,
        BaseReferences<_$AppDatabase, $LocalStateTable, LocalStateEntry>,
      ),
      LocalStateEntry,
      PrefetchHooks Function()
    >;

class $AppDatabaseManager {
  final _$AppDatabase _db;
  $AppDatabaseManager(this._db);
  $$CardsTableTableManager get cards =>
      $$CardsTableTableManager(_db, _db.cards);
  $$OutboxTableTableManager get outbox =>
      $$OutboxTableTableManager(_db, _db.outbox);
  $$LocalStateTableTableManager get localState =>
      $$LocalStateTableTableManager(_db, _db.localState);
}
