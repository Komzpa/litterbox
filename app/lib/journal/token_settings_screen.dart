import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

abstract interface class McpTokenClient {
  Future<({String id, String token})> create();
  Future<void> revoke(String id);
}

class TokenSettingsScreen extends StatefulWidget {
  final McpTokenClient client;
  const TokenSettingsScreen({super.key, required this.client});
  @override
  State<TokenSettingsScreen> createState() => _TokenSettingsScreenState();
}
class _TokenSettingsScreenState extends State<TokenSettingsScreen> {
  String? id, shown;
  @override
  Widget build(BuildContext context) => Scaffold(appBar: AppBar(title: const Text('MCP tokens')), body: Column(children: [if (shown != null) SelectableText(shown!, key: const Key('token-once')), ElevatedButton(onPressed: () async { final result = await widget.client.create(); setState(() {id=result.id; shown=result.token;}); }, child: const Text('Create token')), if (id != null) TextButton(onPressed: () async { await widget.client.revoke(id!); setState(() {id=null;shown=null;}); }, child: const Text('Revoke'))]));
}

class HttpMcpTokenClient implements McpTokenClient {
 final String baseUrl; final http.Client client; final Map<String, String> Function() headers;
 HttpMcpTokenClient(this.baseUrl,{http.Client? client, Map<String, String> Function()? headers}):client=client??http.Client(), headers=headers??(()=>const {});
 Future<({String id,String token})> create() async {final r=await client.post(Uri.parse('$baseUrl/v1/mcp-tokens'),headers:headers());if(r.statusCode!=200)throw Exception('Token creation failed');final v=jsonDecode(r.body) as Map<String,dynamic>;return (id:v['id'] as String,token:v['token'] as String);}
 Future<void> revoke(String id) async {final r=await client.delete(Uri.parse('$baseUrl/v1/mcp-tokens/$id'),headers:headers());if(r.statusCode!=204)throw Exception('Token revocation failed');}
}
