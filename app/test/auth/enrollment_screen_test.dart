import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/auth/device_auth_client.dart';
import 'package:litterbox/auth/enrollment_screen.dart';

class FakeTokens implements DeviceTokenStore {
  @override
  Future<String?> read() async => null;
  @override
  Future<void> write(String token) async {}
  @override
  Future<void> clear() async {}
}

class FakeEnrollmentClient extends DeviceAuthClient {
  FakeEnrollmentClient()
      : super(server: Uri.parse('https://example.com'), tokens: FakeTokens());

  String? submittedCode;
  int? failWith;

  @override
  Future<DeviceEnrollment> enroll({
    required String inviteCode,
    required String deviceName,
    required String platform,
  }) async {
    submittedCode = inviteCode;
    if (failWith case final status?) throw EnrollmentException(status);
    return const DeviceEnrollment(deviceId: 'device-1', tenantId: 'tenant-1');
  }
}

void main() {
  const labels = EnrollmentLabels(
    title: 'Add a device',
    description: 'Connect this device',
    inviteHint: 'Paste invite',
    oneTimeNote: 'Single use',
    emptyInvite: 'Enter invite',
    inviteCode: 'Invite code',
    enroll: 'Connect',
    scan: 'Scan code',
    scannerTitle: 'Scan invite',
    invalidInvite: 'Invite unavailable',
    networkError: 'Connection failed',
    scanError: 'Camera unavailable',
  );

  for (final size in [const Size(390, 844), const Size(800, 900)]) {
    testWidgets('invite entry and consumed invite at ${size.width}px',
        (tester) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final client = FakeEnrollmentClient()..failWith = 410;
      addTearDown(client.close);
      DeviceEnrollment? completed;
      await tester.pumpWidget(MaterialApp(
        home: EnrollmentScreen(
          client: client,
          labels: labels,
          deviceName: 'Phone',
          platform: 'android',
          onEnrolled: (value) => completed = value,
        ),
      ));
      expect(find.text('Scan code'), findsOneWidget);
      await tester.enterText(find.byType(TextField), 'single-use');
      await tester.tap(find.text('Connect'));
      await tester.pumpAndSettle();
      expect(find.text('Invite unavailable'), findsOneWidget);
      expect(completed, isNull);
      client.failWith = null;
      await tester.tap(find.text('Connect'));
      await tester.pumpAndSettle();
      expect(client.submittedCode, 'single-use');
      expect(completed?.deviceId, 'device-1');
      expect(tester.takeException(), isNull);
    });
  }
}
