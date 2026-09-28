import 'package:flutter/material.dart';
import 'dart:convert';
import 'package:http/http.dart' as http;

class JournalEntry {
  final String id;
  final String body;
  const JournalEntry(this.id, this.body);
}

abstract interface class JournalClient {
  Future<List<JournalEntry>> list();
  Future<void> append(String body);
  Future<void> update(String id, String body);
}

class JournalScreen extends StatefulWidget {
  final JournalClient client;
  const JournalScreen({super.key, required this.client});
  @override
  State<JournalScreen> createState() => _JournalScreenState();
}

class _JournalScreenState extends State<JournalScreen> {
  final controller = TextEditingController();
  late Future<List<JournalEntry>> entries = widget.client.list();
  @override
  void dispose() { controller.dispose(); super.dispose(); }
  Future<void> save() async {
    final text = controller.text.trim();
    if (text.isEmpty) return;
    await widget.client.append(text);
    controller.clear();
    setState(() { entries = widget.client.list(); });
  }
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Journal')),
    body: Column(children: [
      Expanded(child: FutureBuilder<List<JournalEntry>>(future: entries, builder: (context, snapshot) {
        final rows = snapshot.data ?? const <JournalEntry>[];
        return ListView(children: [for (final row in rows) ListTile(title: Text(row.body), onTap: () async {
          final edit = TextEditingController(text: row.body);
          final value = await showDialog<String>(context: context, builder: (context) => AlertDialog(content: TextField(controller: edit), actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('Cancel')), TextButton(onPressed: () => Navigator.pop(context, edit.text), child: const Text('Save'))]));
          edit.dispose(); if (value != null) { await widget.client.update(row.id, value); setState(() { entries = widget.client.list(); }); }
        })]);
      })),
      Padding(padding: const EdgeInsets.all(16), child: Row(children: [Expanded(child: TextField(key: const Key('journal-compose'), controller: controller, decoration: const InputDecoration(hintText: 'Write a journal entry'))), IconButton(key: const Key('journal-save'), onPressed: save, icon: const Icon(Icons.send))]))
    ]),
  );
}

class HttpJournalClient implements JournalClient {
  final String baseUrl; final http.Client client;
  HttpJournalClient(this.baseUrl, {http.Client? client}) : client=client??http.Client();
  Future<List<JournalEntry>> list() async { final r=await client.get(Uri.parse('$baseUrl/v1/journal')); if(r.statusCode!=200) throw Exception('Journal load failed'); return (jsonDecode(r.body) as List).map((v)=>JournalEntry(v['id'] as String,v['body'] as String)).toList(); }
  Future<void> append(String body) async { final r=await client.post(Uri.parse('$baseUrl/v1/journal'),headers:{'content-type':'application/json'},body:jsonEncode({'body':body})); if(r.statusCode!=201) throw Exception('Journal append failed'); }
  Future<void> update(String id,String body) async { final r=await client.put(Uri.parse('$baseUrl/v1/journal/$id'),headers:{'content-type':'application/json'},body:jsonEncode({'body':body})); if(r.statusCode!=200) throw Exception('Journal update failed'); }
}
