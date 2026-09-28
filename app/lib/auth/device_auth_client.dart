import 'dart:convert';
import 'dart:io';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Store the bearer token only in platform-protected storage, never in Drift.
abstract class DeviceTokenStore {
  Future<String?> read();
  Future<void> write(String token);
  Future<void> clear();
}

class SecureDeviceTokenStore implements DeviceTokenStore {
  SecureDeviceTokenStore([FlutterSecureStorage? storage])
      : _storage = storage ?? const FlutterSecureStorage();

  static const _key = 'litterbox_device_token';
  final FlutterSecureStorage _storage;

  @override
  Future<String?> read() => _storage.read(key: _key);

  @override
  Future<void> write(String token) => _storage.write(key: _key, value: token);

  @override
  Future<void> clear() => _storage.delete(key: _key);
}

class DeviceEnrollment {
  const DeviceEnrollment({
    required this.deviceId,
    required this.tenantId,
  });

  final String deviceId;
  final String tenantId;
}

class EnrollmentException implements Exception {
  const EnrollmentException(this.statusCode);

  final int statusCode;
}

/// Construct with the HTTPS server origin; the -dev server may use localhost HTTP.
class DeviceAuthClient {
  DeviceAuthClient({
    required this.server,
    required this.tokens,
    HttpClient? httpClient,
  }) : _http = httpClient ?? HttpClient() {
    if (!server.hasAuthority ||
        (server.scheme != 'https' &&
            !(server.scheme == 'http' &&
                (server.host == 'localhost' ||
                    server.host == '127.0.0.1' ||
                    server.host == '::1')))) {
      throw ArgumentError.value(server, 'server', 'Use HTTPS or local HTTP');
    }
  }

  final Uri server;
  final DeviceTokenStore tokens;
  final HttpClient _http;

  Uri _url(String path) => server.replace(path: path);

  Future<DeviceEnrollment> enroll({
    required String inviteCode,
    required String deviceName,
    required String platform,
  }) async {
    final code = inviteCode.trim();
    if (code.isEmpty || deviceName.trim().isEmpty || platform.trim().isEmpty) {
      throw ArgumentError('Invite code, device name and platform are required');
    }
    final request = await _http.postUrl(_url('/v1/devices/enroll'));
    request.headers.contentType = ContentType.json;
    request.write(jsonEncode({
      'invite_code': code,
      'device_name': deviceName.trim(),
      'platform': platform.trim(),
    }));
    final response = await request.close();
    final body = await utf8.decoder.bind(response).join();
    if (response.statusCode != HttpStatus.created) {
      throw EnrollmentException(response.statusCode);
    }
    final data = jsonDecode(body) as Map<String, dynamic>;
    final token = data['token'];
    final deviceId = data['device_id'];
    final tenantId = data['tenant_id'];
    if (token is! String || token.isEmpty ||
        deviceId is! String || deviceId.isEmpty ||
        tenantId is! String || tenantId.isEmpty) {
      throw const FormatException('Invalid enrollment response');
    }
    await tokens.write(token);
    return DeviceEnrollment(deviceId: deviceId, tenantId: tenantId);
  }

  /// Sends an authenticated JSON request. Caller owns the response status/body.
  Future<HttpClientResponse> request(String method, String path,
      {Object? jsonBody}) async {
    if (!path.startsWith('/v1/')) {
      throw ArgumentError.value(path, 'path', 'Expected a /v1/ API path');
    }
    final token = await tokens.read();
    if (token == null || token.isEmpty) {
      throw StateError('Device enrollment is required');
    }
    final request = await _http.openUrl(method, _url(path));
    request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
    if (jsonBody != null) {
      request.headers.contentType = ContentType.json;
      request.write(jsonEncode(jsonBody));
    }
    return request.close();
  }

  void close() => _http.close();
}
