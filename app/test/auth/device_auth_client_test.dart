import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:litterbox/auth/device_auth_client.dart';

class MemoryTokens implements DeviceTokenStore {
  String? value;

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String token) async => value = token;

  @override
  Future<void> clear() async => value = null;
}

void main() {
  late HttpServer server;
  late MemoryTokens tokens;
  late DeviceAuthClient client;

  setUp(() async {
    server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    tokens = MemoryTokens();
    client = DeviceAuthClient(
      server: Uri.parse('http://127.0.0.1:${server.port}'),
      tokens: tokens,
    );
  });

  tearDown(() async {
    client.close();
    await server.close(force: true);
  });

  test('one-use invite enrolls and stores token for authenticated API requests',
      () async {
    var used = false;
    server.listen((request) async {
      if (request.uri.path == '/v1/devices/enroll') {
        final body = jsonDecode(await utf8.decoder.bind(request).join())
            as Map<String, dynamic>;
        expect(request.method, 'POST');
        expect(body, {
          'invite_code': 'one-use',
          'device_name': 'Phone',
          'platform': 'android',
        });
        expect(request.headers.value(HttpHeaders.authorizationHeader), isNull);
        if (used) {
          request.response.statusCode = 410;
        } else {
          used = true;
          request.response.statusCode = 201;
          request.response.headers.contentType = ContentType.json;
          request.response.write(jsonEncode({
            'device_id': 'device-1',
            'tenant_id': 'tenant-1',
            'token': 'only-on-this-device',
          }));
        }
      } else {
        expect(request.uri.path, '/v1/cards');
        expect(request.headers.value(HttpHeaders.authorizationHeader),
            'Bearer only-on-this-device');
        request.response.write('{}');
      }
      await request.response.close();
    });

    final enrolled = await client.enroll(
        inviteCode: ' one-use ', deviceName: 'Phone', platform: 'android');
    expect(enrolled.deviceId, 'device-1');
    expect(enrolled.tenantId, 'tenant-1');
    expect(tokens.value, 'only-on-this-device');
    final response = await client.request('GET', '/v1/cards');
    expect(response.statusCode, 200);
    await response.drain<void>();
    await expectLater(
      client.enroll(
          inviteCode: 'one-use', deviceName: 'Phone', platform: 'android'),
      throwsA(isA<EnrollmentException>()
          .having((error) => error.statusCode, 'statusCode', 410)),
    );
    expect(tokens.value, 'only-on-this-device');
  });

  test('missing token refuses protected requests before opening network', () async {
    await expectLater(client.request('GET', '/v1/cards'), throwsStateError);
  });

  test('plaintext HTTP to remote server is rejected', () {
    expect(
      () => DeviceAuthClient(
        server: Uri.parse('http://example.com'),
        tokens: tokens,
      ),
      throwsArgumentError,
    );
  });
}
