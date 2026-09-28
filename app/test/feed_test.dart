import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:intl/intl.dart';
import 'package:litterbox/main.dart';

class LiveClient extends http.BaseClient {
  final http.Client requests;
  final events = StreamController<List<int>>();
  LiveClient(this.requests);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    if (request.url.path == '/v1/cards/events') return Future.value(http.StreamedResponse(events.stream, 200));
    return requests.send(request);
  }
  @override
  void close() { events.close(); requests.close(); }
}

void main() {
  setUpAll(() async {
    await Future.wait(['en', 'ru', 'be'].map(initializeDateFormatting));
  });

  Map<String, Object?> card(String id, String title, String? at, {String note = ''}) => {
        'id': id, 'source': 'todo', 'title': title, 'summary': 'Plain summary',
        'at': at, 'timed': at != null, 'state': 'open', 'note': note,
      };

  testWidgets('reminder creation is reachable and submits title and chosen date-time', (tester) async {
    Map<String, dynamic>? submitted;
    final live = LiveClient(MockClient((request) async {
      if (request.url.path == '/v1/version') return http.Response(jsonEncode({'api': 1, 'min_client_api': 1, 'server_build': 'test'}), 200);
      if (request.url.path == '/v1/reminders') { submitted = jsonDecode(request.body) as Map<String, dynamic>; return http.Response('{}', 200); }
      return http.Response(jsonEncode({'now': [], 'later': [], 'missed': []}), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Add reminder'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).first, 'Call dentist');
    await tester.tap(find.byType(TextButton).first);
    await tester.pumpAndSettle();
    expect(find.byType(DatePickerDialog), findsOneWidget);
    await tester.tap(find.text('Cancel').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Create'));
    await tester.pumpAndSettle();
    expect(submitted?['title'], 'Call dentist');
    expect(DateTime.tryParse(submitted?['due_at'] as String), isNotNull);
  });

  testWidgets('missing version and old cards response identify older server', (tester) async {
    final live = LiveClient(MockClient((request) async {
      if (request.url.path == '/v1/version') return http.Response('', 404);
      return http.Response(jsonEncode({'cards': []}), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pumpAndSettle();
    expect(find.text('Server is older than the app'), findsOneWidget);
  });

  testWidgets('old cards response reports compatibility even when version exists', (tester) async {
    final live = LiveClient(MockClient((request) async {
      if (request.url.path == '/v1/version') {
        return http.Response(jsonEncode({'api': 1, 'min_client_api': 1, 'server_build': 'different-sha'}), 200);
      }
      return http.Response(jsonEncode({'cards': []}), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pumpAndSettle();
    expect(find.text('Server is older than the app'), findsOneWidget);
  });

  testWidgets('higher minimum API shows banner; different sha at same API does not', (tester) async {
    var minimumApi = 2;
    final live = LiveClient(MockClient((request) async {
      expect(request.headers['X-Litterbox-Api'], '1');
      if (request.url.path == '/v1/version') {
        return http.Response(jsonEncode({'api': 1, 'min_client_api': minimumApi,
          'server_build': 'different-sha'}), 200);
      }
      return http.Response(jsonEncode({'now': [], 'later': [], 'missed': []}), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pumpAndSettle();
    expect(find.text('App is outdated (API 1), server needs 2'), findsOneWidget);
    minimumApi = 1;
    await tester.tap(find.byType(PopupMenuButton<String>));
    await tester.pumpAndSettle();
    expect(find.text('About · app $buildSha, server different-sha'), findsOneWidget);
    await tester.tap(find.descendant(of: find.byType(PopupMenuItem<String>).first, matching: find.text('Restart')));
    await tester.pumpAndSettle();
    expect(find.textContaining('App is outdated'), findsNothing);
  });

  testWidgets('lower server API reports older server', (tester) async {
    final live = LiveClient(MockClient((request) async {
      if (request.url.path == '/v1/version') {
        return http.Response(jsonEncode({'api': 0, 'min_client_api': 1, 'server_build': 'old'}), 200);
      }
      return http.Response(jsonEncode({'now': [], 'later': [], 'missed': []}), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pumpAndSettle();
    expect(find.text('Server is older than the app'), findsOneWidget);
  });

  testWidgets('next timed slot refetches without user refresh', (tester) async {
    var fetches = 0;
    final target = DateTime.now().add(const Duration(seconds: 2));
    final live = LiveClient(MockClient((request) async {
      if (request.url.path == '/v1/version') return http.Response(jsonEncode({'api': 1, 'min_client_api': 1, 'server_build': 'different-sha'}), 200);
      fetches++;
      return http.Response(jsonEncode({
        'now': fetches == 1 ? [] : [card('next', 'Next slot', target.toIso8601String())],
        'later': fetches == 1 ? [card('next', 'Next slot', target.toIso8601String())] : [], 'missed': [],
      }), 200);
    }));
    await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
    await tester.pump();
    expect(fetches, 1);
    await tester.pump(const Duration(seconds: 3));
    await tester.pump();
    expect(fetches, 2);
    expect(find.byIcon(Icons.refresh), findsNothing);
  });

  for (final locale in ['en', 'be']) {
    testWidgets('sections and note flow in $locale', (tester) async {
      final calls = <String>[];
      var note = '';
      var done = false;
      final client = MockClient((request) async {
        calls.add('${request.method} ${request.url.path} ${request.body}');
        if (request.url.path == '/v1/version') return http.Response(jsonEncode({'api': 1, 'min_client_api': 1, 'server_build': 'different-sha'}), 200);
        if (request.method == 'GET') {
          return http.Response(jsonEncode({
            'now': done ? [] : [card('one', 'Current work', null, note: note)],
            'later': [card('two', 'Sleep later', '2026-09-29T23:00:00+04:00')],
            'missed': [card('three', 'Earlier work', '2026-09-28T10:00:00+04:00')],
          }), 200);
        }
        if (request.url.path.endsWith('/note')) {
          note = (jsonDecode(request.body) as Map<String, dynamic>)['text'] as String;
        } else if (request.url.path.endsWith('/dismiss')) {
          done = true;
        }
        return http.Response('', 204);
      });
      final live = LiveClient(client);
      tester.binding.platformDispatcher.localeTestValue = Locale(locale);
      tester.binding.platformDispatcher.localesTestValue = [Locale(locale)];
      addTearDown(() {
        tester.binding.platformDispatcher.clearLocaleTestValue();
        tester.binding.platformDispatcher.clearLocalesTestValue();
      });
      await tester.pumpWidget(LitterboxApp(api: CardsApi('http://fake', client: live)));
      await tester.pumpAndSettle();
      expect(find.text(locale == 'be' ? 'Зараз' : 'Now'), findsOneWidget);
      expect(find.text(locale == 'be' ? 'Пазней' : 'Later'), findsOneWidget);
      expect(find.text('Current work'), findsOneWidget);
      expect(find.text('Sleep later'), findsOneWidget);
      final rendered = '${DateFormat.yMMMd(locale).format(DateTime.parse('2026-09-29T23:00:00+04:00').toLocal())} ${DateFormat.jm(locale).format(DateTime.parse('2026-09-29T23:00:00+04:00').toLocal())}';
      expect(find.text(rendered), findsOneWidget);
      expect(find.text('Earlier work'), findsNothing);
      expect(find.byIcon(Icons.refresh), findsNothing);
      final fetched = calls.where((call) => call.startsWith('GET /v1/cards ')).length;
      live.events.add(utf8.encode('event: cards\ndata: {}\n\n'));
      await tester.pump();
      await tester.pump();
      expect(calls.where((call) => call.startsWith('GET /v1/cards ')).length, fetched + 1);
      await tester.tap(find.text(locale == 'be' ? 'Прапушчана (1)' : 'Missed (1)'));
      await tester.pumpAndSettle();
      expect(find.text('Earlier work'), findsOneWidget);
      await tester.tap(find.byTooltip(locale == 'be' ? 'Нататка' : 'Note').first);
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'Wrong suggestion');
      await tester.tap(find.text(locale == 'be' ? 'Захаваць' : 'Save'));
      await tester.pumpAndSettle();
      expect(calls.any((call) => call.contains('POST /v1/cards/one/note') && call.contains('Wrong suggestion')), isTrue);
      expect(find.text('Wrong suggestion'), findsOneWidget);
      await tester.tap(find.byTooltip(locale == 'be' ? 'Нататка' : 'Note').first);
      await tester.pumpAndSettle();
      expect(find.text('Wrong suggestion'), findsWidgets);
      await tester.tap(find.text(locale == 'be' ? 'Гатова з нататкай' : 'Done with note'));
      await tester.pumpAndSettle();
      expect(calls.any((call) => call.contains('POST /v1/cards/one/dismiss') && call.contains('Wrong suggestion')), isTrue);
      expect(find.text('Current work'), findsNothing);
      final beforeRestart = calls.where((call) => call.startsWith('GET /v1/cards ')).length;
      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();
      await tester.tap(find.text(locale == 'be' ? 'Перазапусціць' : 'Restart'));
      await tester.pumpAndSettle();
      expect(calls.where((call) => call.startsWith('GET /v1/cards ')).length, beforeRestart + 1);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyR);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
      await tester.pumpAndSettle();
      expect(calls.where((call) => call.startsWith('GET /v1/cards ')).length, beforeRestart + 2);
    });
  }
}
