import "package:flutter_test/flutter_test.dart";
import "package:flutter/material.dart";
import "package:litterbox/auth/device_auth_client.dart";
import "package:litterbox/main.dart" show CardsApi, LitterboxApp;

class FakeTokens implements DeviceTokenStore {
  @override
  Future<String?> read() async => null;
  @override
  Future<void> write(String token) async {}
  @override
  Future<void> clear() async {}
}

void main() {
  testWidgets("unenrolled app opens device enrollment", (tester) async {
    final api = CardsApi("http://fake-host");
    final enrollment = DeviceAuthClient(
      server: Uri.parse('https://fake-host'),
      tokens: FakeTokens(),
    );
    addTearDown(api.client.close);
    addTearDown(enrollment.close);
    await tester.pumpWidget(
      LitterboxApp(api: api, enrollmentClient: enrollment),
    );
    expect(find.text("Set up device"), findsOneWidget);
    expect(find.byType(TextField), findsOneWidget);
  });
}
