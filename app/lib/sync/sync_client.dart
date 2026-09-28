import 'dart:convert';

import 'package:http/http.dart' as http;

import '../store/local_store.dart';

/// Flushes durable user actions before asking the server for incremental state.
class SyncClient {
  final Uri baseUri;
  final LocalCardStore store;
  final http.Client client;
  final Map<String, String> Function() headers;

  SyncClient(this.baseUri, this.store, this.client, {Map<String, String> Function()? headers})
      : headers = headers ?? (() => const {});

  Future<void> synchronize() async {
    for (final op in await store.pending()) {
      final response = await client.post(baseUri.resolve('/v1/ops'),
          headers: {...headers(), 'content-type': 'application/json'},
          body: jsonEncode({
            'op_id': op.opId,
            'card_id': op.cardId,
            'type': op.type,
            'args': jsonDecode(op.argsJson),
          }));
      if (response.statusCode < 200 || response.statusCode >= 300) {
        throw http.ClientException('operation rejected', baseUri);
      }
      await store.removeOperation(op.opId);
    }
    final response = await client.get(baseUri.resolve('/v1/snapshot'), headers: headers());
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw http.ClientException('snapshot failed', baseUri);
    }
    await store.replaceSnapshot(jsonDecode(response.body) as Map<String, dynamic>);
  }
}
