import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:litterbox/main.dart';

void main() {
  testWidgets('Gmail settings connects and disconnects accounts', (tester) async {
    final requests = <String>[];
    var connected = true;
    final client = MockClient((request) async {
      requests.add('${request.method} ${request.url.path}');
      expect(request.headers['X-Litterbox-Api'], '1');
      if (request.method == 'GET') {
        return http.Response(jsonEncode(connected
            ? [{'id': 'account-1', 'address': 'person@example.com'}]
            : []), 200);
      }
      if (request.method == 'POST') {
        return http.Response(jsonEncode({'authorization_url': 'https://accounts.google.com/authorize?state=test'}), 200);
      }
      if (request.method == 'DELETE') {
        expect(request.url.path, '/v1/gmail/accounts/account-1');
        connected = false;
        return http.Response('', 204);
      }
      return http.Response('', 404);
    });

    final api = CardsApi('https://api.example', client: client);
    await tester.pumpWidget(MaterialApp(home: GmailAccountsScreen(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('person@example.com'), findsOneWidget);
    expect(requests, ['GET /v1/gmail/accounts']);

    await tester.tap(find.text('Connect Gmail account'));
    await tester.pumpAndSettle();
    expect(find.text('https://accounts.google.com/authorize?state=test'), findsOneWidget);
    expect(requests, ['GET /v1/gmail/accounts', 'POST /v1/gmail/connect']);

    await tester.tap(find.byTooltip('Disconnect person@example.com'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Disconnect'));
    await tester.pumpAndSettle();
    expect(find.text('No Gmail accounts connected.'), findsOneWidget);
    expect(requests, [
      'GET /v1/gmail/accounts',
      'POST /v1/gmail/connect',
      'DELETE /v1/gmail/accounts/account-1',
      'GET /v1/gmail/accounts',
    ]);
  });
}
