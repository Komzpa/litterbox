import 'dart:convert';
import 'dart:io';
import 'dart:math';

abstract interface class OpsClient {
  Future<void> sendOp({
    required String cardId,
    required String type,
    Map<String, Object?> args = const {},
  });
}

class HttpOpsClient implements OpsClient {
  HttpOpsClient({required this.baseUri, required this.deviceToken, HttpClient? client})
      : _client = client ?? HttpClient();

  final Uri baseUri;
  final Future<String?> Function() deviceToken;
  final HttpClient _client;

  @override
  Future<void> sendOp({
    required String cardId,
    required String type,
    Map<String, Object?> args = const {},
  }) async {
    final request = await _client.postUrl(baseUri.resolve('/v1/ops'));
    request.headers.contentType = ContentType.json;
    final token = await deviceToken();
    if (token != null) request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
    request.write(jsonEncode({
      'op_id': _newUuid(),
      'card_id': cardId,
      'type': type,
      'args': args,
    }));
    final response = await request.close();
    if (response.statusCode < 200 || response.statusCode >= 300) {
      final body = await utf8.decoder.bind(response).join();
      throw HttpException('Operation failed (${response.statusCode}): $body', uri: request.uri);
    }
    await response.drain<void>();
  }

  static String _newUuid() {
    final random = Random.secure();
    final bytes = List<int>.generate(16, (_) => random.nextInt(256));
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    final hex = bytes.map((byte) => byte.toRadixString(16).padLeft(2, '0')).join();
    return '${hex.substring(0, 8)}-${hex.substring(8, 12)}-${hex.substring(12, 16)}-${hex.substring(16, 20)}-${hex.substring(20)}';
  }
}
