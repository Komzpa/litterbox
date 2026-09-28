import 'dart:convert';
import 'package:http/http.dart' as http;

class MailBody {
  const MailBody({required this.html, required this.threadId, required this.accountId});
  final String html;
  final String threadId;
  final String accountId;
}

abstract interface class BodyRepository {
  Future<MailBody> fetchBody(String cardId);
}

class HttpBodyRepository implements BodyRepository {
  HttpBodyRepository({required this.baseUri, required this.client, this.headers = const {}});
  final Uri baseUri;
  final http.Client client;
  final Map<String, String> headers;

  @override
  Future<MailBody> fetchBody(String cardId) async {
    final uri = baseUri.resolve('/v1/cards/${Uri.encodeComponent(cardId)}/body');
    final response = await client.get(uri, headers: headers);
    if (response.statusCode != 200) throw http.ClientException('Body request failed: ${response.statusCode}', uri);
    final value = jsonDecode(response.body) as Map<String, dynamic>;
    return MailBody(html: value['html'] as String, threadId: value['threadId'] as String, accountId: value['accountId'] as String);
  }
}
