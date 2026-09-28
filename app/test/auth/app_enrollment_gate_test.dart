import "package:flutter_test/flutter_test.dart";
import "package:flutter/material.dart";
import "package:litterbox/main.dart" show CardsApi, LitterboxApp;

void main() {
  testWidgets("unenrolled app opens device enrollment", (tester) async {
    final api = CardsApi("https://example.com");
    addTearDown(api.client.close);
    await tester.pumpWidget(LitterboxApp(api: api));
    expect(find.text("Set up device"), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
  });
}
