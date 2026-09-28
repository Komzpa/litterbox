import 'package:flutter/material.dart';

import '../l10n/app_localizations.dart';
import 'ops_client.dart';

class BundleCard {
  const BundleCard({
    required this.id,
    required this.subject,
    required this.sender,
    this.bundleId,
    this.pinned = false,
  });

  final String id;
  final String subject;
  final String sender;
  final String? bundleId;
  final bool pinned;

  BundleCard copyWith({
    String? bundleId,
    bool clearBundle = false,
    bool? pinned,
  }) => BundleCard(
    id: id,
    subject: subject,
    sender: sender,
    bundleId: clearBundle ? null : (bundleId ?? this.bundleId),
    pinned: pinned ?? this.pinned,
  );
}

class BundleInbox extends StatefulWidget {
  const BundleInbox({super.key, required this.cards, required this.opsClient});

  final List<BundleCard> cards;
  final OpsClient opsClient;

  @override
  State<BundleInbox> createState() => _BundleInboxState();
}

class _BundleInboxState extends State<BundleInbox> {
  late List<BundleCard> _cards = List.of(widget.cards);
  final Set<String> _hidden = {};

  @override
  void didUpdateWidget(covariant BundleInbox oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.cards != widget.cards) _cards = List.of(widget.cards);
  }

  void _hideCard(BundleCard card) => setState(() => _hidden.add(card.id));

  Future<void> _setPinned(BundleCard card, bool pinned) async {
    await widget.opsClient.sendOp(
      cardId: card.id,
      type: pinned ? 'pin' : 'unpin',
      args: const {},
    );
    setState(() {
      final index = _cards.indexWhere((item) => item.id == card.id);
      if (index >= 0) _cards[index] = _cards[index].copyWith(pinned: pinned);
    });
  }

  Future<void> _reorderPinned(int oldIndex, int newIndex) async {
    final pinned = _cards.where((card) => card.pinned).toList()
      ..sort((a, b) => _cards.indexOf(a).compareTo(_cards.indexOf(b)));
    if (newIndex > oldIndex) newIndex--;
    final moved = pinned.removeAt(oldIndex);
    pinned.insert(newIndex, moved);
    final unpinned = _cards.where((card) => !card.pinned).toList();
    setState(() => _cards = [...pinned, ...unpinned]);
    await widget.opsClient.sendOp(
      cardId: moved.id,
      type: 'reorder_pins',
      args: {'cards': pinned.map((card) => card.id).toList()},
    );
  }

  Future<void> _archiveBundle(String bundleId, List<BundleCard> cards) async {
    final first = cards.first;
    await widget.opsClient.sendOp(
      cardId: first.id,
      type: 'bundle_archive',
      args: {'bundle_id': bundleId},
    );
    setState(
      () => _hidden.addAll(
        cards.where((card) => !card.pinned).map((card) => card.id),
      ),
    );
  }

  Future<void> _completeBundle(String bundleId, List<BundleCard> cards) async {
    final first = cards.first;
    await widget.opsClient.sendOp(
      cardId: first.id,
      type: 'bundle_done',
      args: {'bundle_id': bundleId},
    );
    setState(
      () => _hidden.addAll(
        cards.where((card) => !card.pinned).map((card) => card.id),
      ),
    );
  }

  Future<void> _takeOut(BundleCard card) async {
    await widget.opsClient.sendOp(
      cardId: card.id,
      type: 'take_out',
      args: {'card': card.id},
    );
    setState(() {
      final index = _cards.indexWhere((item) => item.id == card.id);
      if (index >= 0) _cards[index] = _cards[index].copyWith(clearBundle: true);
    });
  }

  @override
  Widget build(BuildContext context) {
    final visible = _cards.where((card) => !_hidden.contains(card.id)).toList();
    final pinned = visible.where((card) => card.pinned).toList();
    final bundles = <String, List<BundleCard>>{};
    for (final card in visible.where((card) => card.bundleId != null)) {
      bundles.putIfAbsent(card.bundleId!, () => []).add(card);
    }
    final bundledIds = bundles.values
        .expand((cards) => cards)
        .map((card) => card.id)
        .toSet();
    final ordinary = visible.where(
      (card) => !card.pinned && !bundledIds.contains(card.id),
    );
    final rest = <Widget>[
      for (final entry in bundles.entries)
        _BundleSection(
          key: ValueKey('bundle-${entry.key}'),
          id: entry.key,
          cards: entry.value,
          opsClient: widget.opsClient,
          onArchive: () => _archiveBundle(entry.key, entry.value),
          onDone: () => _completeBundle(entry.key, entry.value),
          onPin: _setPinned,
          onSnoozed: _hideCard,
          onTakeOut: _takeOut,
        ),
      for (final card in ordinary)
        _CardTile(
          key: ValueKey(card.id),
          card: card,
          opsClient: widget.opsClient,
          onPin: (value) => _setPinned(card, value),
          onSnoozed: () => _hideCard(card),
        ),
    ];
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (pinned.isNotEmpty) ...[
          ListTile(title: Text(AppLocalizations.of(context)!.pinned)),
          SizedBox(
            height: (pinned.length * 76.0).clamp(76.0, 228.0),
            child: ReorderableListView.builder(
              buildDefaultDragHandles: false,
              itemCount: pinned.length,
              onReorder: _reorderPinned,
              itemBuilder: (context, index) => KeyedSubtree(
                key: ValueKey(pinned[index].id),
                child: _CardTile(
                  card: pinned[index],
                  opsClient: widget.opsClient,
                  onPin: (value) => _setPinned(pinned[index], value),
                  onSnoozed: () => _hideCard(pinned[index]),
                  onTakeOut: pinned[index].bundleId == null
                      ? null
                      : () => _takeOut(pinned[index]),
                  dragIndex: index,
                ),
              ),
            ),
          ),
        ],
        ...rest,
      ],
    );
  }
}

class _BundleSection extends StatelessWidget {
  const _BundleSection({
    super.key,
    required this.id,
    required this.cards,
    required this.opsClient,
    required this.onArchive,
    required this.onDone,
    required this.onPin,
    required this.onSnoozed,
    required this.onTakeOut,
  });

  final String id;
  final List<BundleCard> cards;
  final OpsClient opsClient;
  final VoidCallback onArchive;
  final VoidCallback onDone;
  final Future<void> Function(BundleCard, bool) onPin;
  final ValueChanged<BundleCard> onSnoozed;
  final Future<void> Function(BundleCard) onTakeOut;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      ListTile(
        title: Text(
          AppLocalizations.of(context)!.bundleCount(id, cards.length),
        ),
        trailing: Wrap(
          children: [
            IconButton(
              key: Key('archive-bundle-$id'),
              tooltip: AppLocalizations.of(context)!.archiveBundle,
              icon: const Icon(Icons.archive_outlined),
              onPressed: onArchive,
            ),
            IconButton(
              key: Key('done-bundle-$id'),
              tooltip: 'Done',
              icon: const Icon(Icons.check),
              onPressed: onDone,
            ),
          ],
        ),
      ),
      for (final card in cards.where((card) => !card.pinned))
        _CardTile(
          key: ValueKey(card.id),
          card: card,
          opsClient: opsClient,
          onPin: (value) => onPin(card, value),
          onSnoozed: () => onSnoozed(card),
          onTakeOut: () => onTakeOut(card),
        ),
    ],
  );
}

class _CardTile extends StatelessWidget {
  const _CardTile({
    super.key,
    required this.card,
    required this.opsClient,
    required this.onPin,
    required this.onSnoozed,
    this.onTakeOut,
    this.dragIndex,
  });

  final BundleCard card;
  final OpsClient opsClient;
  final ValueChanged<bool> onPin;
  final VoidCallback onSnoozed;
  final VoidCallback? onTakeOut;
  final int? dragIndex;

  @override
  Widget build(BuildContext context) => ListTile(
    title: Text(card.subject),
    subtitle: Text(card.sender),
    trailing: Wrap(
      children: [
        IconButton(
          key: Key('pin-${card.id}'),
          tooltip: card.pinned
              ? AppLocalizations.of(context)!.unpin
              : AppLocalizations.of(context)!.pin,
          icon: Icon(card.pinned ? Icons.push_pin : Icons.push_pin_outlined),
          onPressed: () => onPin(!card.pinned),
        ),
        SnoozePicker(
          cardId: card.id,
          opsClient: opsClient,
          onSnoozed: onSnoozed,
        ),
        if (onTakeOut != null)
          IconButton(
            key: Key('take-out-${card.id}'),
            tooltip: AppLocalizations.of(context)!.takeOutOfBundle,
            icon: const Icon(Icons.remove_circle_outline),
            onPressed: onTakeOut,
          ),
        if (dragIndex != null)
          ReorderableDragStartListener(
            index: dragIndex!,
            child: const Padding(
              padding: EdgeInsets.all(12),
              child: Icon(Icons.drag_handle),
            ),
          ),
      ],
    ),
  );
}

class SnoozePicker extends StatelessWidget {
  const SnoozePicker({
    super.key,
    required this.cardId,
    required this.opsClient,
    required this.onSnoozed,
  });

  final String cardId;
  final OpsClient opsClient;
  final VoidCallback onSnoozed;

  DateTime _tomorrowMorning(DateTime now) =>
      DateTime(now.year, now.month, now.day + 1, 9);

  DateTime _nextWeek(DateTime now) =>
      DateTime(now.year, now.month, now.day + 7, 9);

  Future<void> _send(DateTime until) async {
    await opsClient.sendOp(
      cardId: cardId,
      type: 'snooze',
      args: {'until': until.toUtc().toIso8601String()},
    );
    onSnoozed();
  }

  Future<void> _custom(BuildContext context) async {
    final now = DateTime.now();
    final date = await showDatePicker(
      context: context,
      initialDate: now.add(const Duration(days: 1)),
      firstDate: now,
      lastDate: now.add(const Duration(days: 3650)),
    );
    if (date == null || !context.mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: const TimeOfDay(hour: 9, minute: 0),
    );
    if (time != null)
      await _send(
        DateTime(date.year, date.month, date.day, time.hour, time.minute),
      );
  }

  @override
  Widget build(BuildContext context) => PopupMenuButton<String>(
    key: Key('snooze-$cardId'),
    tooltip: AppLocalizations.of(context)!.snooze,
    icon: const Icon(Icons.snooze),
    onSelected: (choice) {
      final now = DateTime.now();
      switch (choice) {
        case 'today':
          final later = DateTime(now.year, now.month, now.day, 17);
          _send(
            later.isAfter(now)
                ? later
                : DateTime(now.year, now.month, now.day + 1, 17),
          );
        case 'tomorrow':
          _send(_tomorrowMorning(now));
        case 'week':
          _send(_nextWeek(now));
        case 'custom':
          _custom(context);
      }
    },
    itemBuilder: (context) => [
      PopupMenuItem(
        value: 'today',
        child: Text(AppLocalizations.of(context)!.laterToday),
      ),
      PopupMenuItem(
        value: 'tomorrow',
        child: Text(AppLocalizations.of(context)!.tomorrowMorning),
      ),
      PopupMenuItem(
        value: 'week',
        child: Text(AppLocalizations.of(context)!.nextWeek),
      ),
      PopupMenuItem(
        value: 'custom',
        child: Text(AppLocalizations.of(context)!.customDateTime),
      ),
    ],
  );
}
