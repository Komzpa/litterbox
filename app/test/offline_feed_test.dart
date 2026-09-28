import 'dart:async';
import 'dart:convert';

import 'package:drift/native.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:intl/date_symbol_data_local.dart';
import 'package:litterbox/db/database.dart';
import 'package:litterbox/main.dart';
import 'package:litterbox/store/local_store.dart';

class OfflineClient extends http.BaseClient {
  final events = StreamController<List<int>>();
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    if (request.url.path == '/v1/cards/events') {
      return http.StreamedResponse(events.stream, 200);
    }
    throw http.ClientException('server down', request.url);
  }
  @override
  void close() => events.close();
}

void main() {
  testWidgets('cached cards render when server is unavailable', (tester) async {
    await Future.wait(['en', 'ru', 'be'].map(initializeDateFormatting));
    final db = AppDatabase(NativeDatabase.memory());
    addTearDown(db.close);
    final store = LocalCardStore(db);
    await store.replaceSnapshot({
      'cursor': 4,
      'cards': [
        {'id': 'c1', 'subject': 'Cached message', 'sender': 'sender@example.test',
          'sort_at': DateTime.utc(2026).toIso8601String(), 'state': 'open', 'note': '',
          'messages': [{'text': 'Saved body', 'timed': false, 'section': 'now'}]},
      ],
    });
    final api = CardsApi('http://offline', client: OfflineClient())..token = 'test-token';
    await tester.pumpWidget(LitterboxApp(api: api, store: store));
    await tester.pumpAndSettle();
    expect(find.text('Cached message'), findsOneWidget);
    expect(find.text('Saved body'), findsOneWidget);
    expect(find.text('Could not load cards'), findsNothing);
  });
}
