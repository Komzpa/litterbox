import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/mail/mail_detail.dart';

void main() {
  testWidgets('renders supplied mail HTML without network access', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: MailDetail(
      html: '<h1>Offline message</h1><p>Body content</p>',
      threadId: 'thread-123', accountId: '0',
    )));
    expect(find.text('Offline message'), findsOneWidget);
    expect(find.text('Body content'), findsOneWidget);
    expect(find.text('Open in Gmail'), findsOneWidget);
  });

  test('builds Gmail thread URL with account and thread identifiers', () {
    expect(gmailThreadUrl('abc/def', '2'),
      'https://mail.google.com/mail/u/2/#all/abc%2Fdef');
  });
}
