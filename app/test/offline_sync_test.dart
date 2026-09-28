import 'dart:convert';

import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:litterbox/db/database.dart';
import 'package:litterbox/store/local_store.dart';
import 'package:litterbox/sync/sync_client.dart';

void main() {
  test('queued action flushes before snapshot and is removed only after ack', () async {
    final db = AppDatabase(NativeDatabase.memory());
    addTearDown(db.close);
    final store = LocalCardStore(db);
    await store.replaceSnapshot({'cursor': 0, 'cards': [
      {'id': 'card-1', 'subject': 'Subject', 'sender': 'sender', 'state': 'open', 'sort_at': '2026-09-01T00:00:00Z'},
    ]});
    await store.applyNote('card-1', 'op-1', 'offline note');
    final calls = <String>[];
    final client = MockClient((request) async {
      calls.add('${request.method} ${request.url.path}');
      if (request.url.path == '/v1/ops') {
        final body = jsonDecode(request.body) as Map<String, dynamic>;
        expect(body, {'op_id': 'op-1', 'card_id': 'card-1', 'type': 'note', 'args': {'note': 'offline note'}});
        return http.Response('{"ok":true}', 200);
      }
      expect(request.url.path, '/v1/snapshot');
      return http.Response('{"cursor":1,"cards":[]}', 200);
    });
    await SyncClient(Uri.parse('http://server'), store, client).synchronize();
    expect(calls, ['POST /v1/ops', 'GET /v1/snapshot']);
    expect(await store.pending(), isEmpty);
    expect(await store.cursor(), 1);
  });
}
