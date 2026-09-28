import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/bundles/bundle_widgets.dart';
import 'package:litterbox/bundles/ops_client.dart';
import 'package:litterbox/l10n/app_localizations.dart';

class RecordedOp {
  RecordedOp(this.cardId, this.type, this.args);
  final String cardId;
  final String type;
  final Map<String, Object?> args;
}

class RecordingOpsClient implements OpsClient {
  final List<RecordedOp> calls = [];

  @override
  Future<void> sendOp({required String cardId, required String type, Map<String, Object?> args = const {}}) async {
    calls.add(RecordedOp(cardId, type, Map.of(args)));
  }
}

Widget app(List<BundleCard> cards, RecordingOpsClient ops) => MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: BundleInbox(cards: cards, opsClient: ops)),
    );

void main() {
  testWidgets('pin and unpin send distinct operations', (tester) async {
    final ops = RecordingOpsClient();
    await tester.pumpWidget(app(const [BundleCard(id: 'a', subject: 'Alpha', sender: 'A')], ops));
    await tester.tap(find.byKey(const Key('pin-a')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('pin-a')));
    await tester.pumpAndSettle();
    expect(ops.calls.map((call) => call.type), ['pin', 'unpin']);
    expect(ops.calls.every((call) => call.cardId == 'a' && call.args.isEmpty), isTrue);
  });

  testWidgets('snooze picker sends selected future RFC3339 time', (tester) async {
    final ops = RecordingOpsClient();
    await tester.pumpWidget(app(const [BundleCard(id: 'a', subject: 'Alpha', sender: 'A')], ops));
    await tester.tap(find.byKey(const Key('snooze-a')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Tomorrow morning'));
    await tester.pumpAndSettle();
    expect(ops.calls.single.type, 'snooze');
    expect(ops.calls.single.cardId, 'a');
    final until = DateTime.parse(ops.calls.single.args['until']! as String);
    expect(until.isAfter(DateTime.now()), isTrue);
    expect(find.text('Alpha'), findsNothing);
  });

  testWidgets('bundle archive sends bundle id and leaves pinned cards visible', (tester) async {
    final ops = RecordingOpsClient();
    const cards = [
      BundleCard(id: 'p', subject: 'Pinned mail', sender: 'A', bundleId: 'news', pinned: true),
      BundleCard(id: 'u', subject: 'Unpinned mail', sender: 'A', bundleId: 'news'),
    ];
    await tester.pumpWidget(app(cards, ops));
    await tester.tap(find.byKey(const Key('archive-bundle-news')));
    await tester.pumpAndSettle();
    expect(ops.calls.single.type, 'bundle_archive');
    expect(ops.calls.single.args, {'bundle_id': 'news'});
    expect(find.text('Pinned mail'), findsOneWidget);
    expect(find.text('Unpinned mail'), findsNothing);
  });

  testWidgets('bundle done sends bundle id and leaves pinned cards visible', (tester) async {
	final ops = RecordingOpsClient();
	const cards = [
	  BundleCard(id: 'p', subject: 'Pinned mail', sender: 'A', bundleId: 'news', pinned: true),
	  BundleCard(id: 'u', subject: 'Unpinned mail', sender: 'A', bundleId: 'news'),
	];
	await tester.pumpWidget(app(cards, ops));
	await tester.tap(find.byKey(const Key('done-bundle-news')));
	await tester.pumpAndSettle();
	expect(ops.calls.single.type, 'bundle_done');
	expect(ops.calls.single.args, {'bundle_id': 'news'});
	expect(find.text('Pinned mail'), findsOneWidget);
	expect(find.text('Unpinned mail'), findsNothing);
  });

  testWidgets('take out sends card correction and keeps the card visible', (tester) async {
    final ops = RecordingOpsClient();
    await tester.pumpWidget(app(const [
      BundleCard(id: 'a', subject: 'Alpha', sender: 'A', bundleId: 'news'),
    ], ops));
    await tester.tap(find.byKey(const Key('take-out-a')));
    await tester.pumpAndSettle();
    expect(ops.calls.single.type, 'take_out');
    expect(ops.calls.single.cardId, 'a');
    expect(ops.calls.single.args, {'card': 'a'});
    expect(find.text('Alpha'), findsOneWidget);
    expect(find.byKey(const Key('archive-bundle-news')), findsNothing);
  });

  testWidgets('dragging pinned cards sends their complete new order', (tester) async {
    final ops = RecordingOpsClient();
    await tester.pumpWidget(app(const [
      BundleCard(id: 'a', subject: 'Alpha', sender: 'A', pinned: true),
      BundleCard(id: 'b', subject: 'Beta', sender: 'B', pinned: true),
    ], ops));
    await tester.drag(find.byIcon(Icons.drag_handle).last, const Offset(0, -60));
    await tester.pumpAndSettle();
    expect(ops.calls, isNotEmpty);
    expect(ops.calls.last.type, 'reorder_pins');
    expect(ops.calls.last.args['cards'], ['b', 'a']);
  });
}
