import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

const _serverUrl = String.fromEnvironment('LITTERBOX_URL');

void main() {
  runApp(LitterboxApp(api: CardsApi(_serverUrl)));
}

class LitterboxApp extends StatelessWidget {
  final CardsApi api;

  const LitterboxApp({super.key, required this.api});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Litterbox',
        theme: ThemeData(colorSchemeSeed: Colors.deepPurple),
        home: InboxScreen(api: api),
      );
}

class InboxCard {
  final String id;
  final String source;
  final String title;
  final String summary;
  final DateTime sortAt;
  final String state;

  const InboxCard({
    required this.id,
    required this.source,
    required this.title,
    required this.summary,
    required this.sortAt,
    required this.state,
  });

  factory InboxCard.fromJson(Map<String, dynamic> json) => InboxCard(
        id: json['id'] as String,
        source: json['source'] as String,
        title: json['title'] as String,
        summary: json['summary'] as String? ?? '',
        sortAt: DateTime.parse(json['sort_at'] as String).toLocal(),
        state: json['state'] as String,
      );
}

class CardsApi {
  final String baseUrl;
  final http.Client client;

  CardsApi(this.baseUrl, {http.Client? client}) : client = client ?? http.Client();

  Future<List<InboxCard>> fetchCards() async {
    final response = await client.get(Uri.parse('$baseUrl/v1/cards'));
    if (response.statusCode != 200) {
      throw Exception('Server returned ${response.statusCode}');
    }
    final json = jsonDecode(response.body) as Map<String, dynamic>;
    return (json['cards'] as List<dynamic>)
        .map((card) => InboxCard.fromJson(card as Map<String, dynamic>))
        .where((card) => card.state != 'done')
        .toList();
  }

  Future<void> dismiss(String id) async {
    final response = await client.post(
      Uri.parse('$baseUrl/v1/cards/$id/dismiss'),
    );
    if (response.statusCode != 204) {
      throw Exception('Dismiss failed (${response.statusCode})');
    }
  }
}

class InboxScreen extends StatefulWidget {
  final CardsApi api;

  const InboxScreen({super.key, required this.api});

  @override
  State<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends State<InboxScreen> {
  late Future<List<InboxCard>> _cards;

  @override
  void initState() {
    super.initState();
    _cards = widget.api.fetchCards();
  }

  Future<void> _refresh() async {
    final cards = widget.api.fetchCards();
    setState(() {
      _cards = cards;
    });
    await cards;
  }

  Future<void> _dismiss(InboxCard card) async {
    try {
      await widget.api.dismiss(card.id);
      await _refresh();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Could not dismiss card: $error')),
      );
      await _refresh();
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(
          title: const Text('Inbox'),
          actions: [
            IconButton(
              tooltip: 'Refresh',
              onPressed: _refresh,
              icon: const Icon(Icons.refresh),
            ),
          ],
        ),
        body: FutureBuilder<List<InboxCard>>(
          future: _cards,
          builder: (context, snapshot) {
            if (snapshot.hasError) {
              return Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text('Could not load cards'),
                    const SizedBox(height: 8),
                    Text(snapshot.error.toString(), textAlign: TextAlign.center),
                    TextButton(onPressed: _refresh, child: const Text('Retry')),
                  ],
                ),
              );
            }
            if (!snapshot.hasData) {
              return const Center(child: CircularProgressIndicator());
            }
            final cards = snapshot.data!;
            if (cards.isEmpty) {
              return RefreshIndicator(
                onRefresh: _refresh,
                child: ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  children: const [
                    SizedBox(height: 240),
                    Center(child: Text('Inbox is empty')),
                  ],
                ),
              );
            }
            return RefreshIndicator(
              onRefresh: _refresh,
              child: ListView.builder(
                physics: const AlwaysScrollableScrollPhysics(),
                itemCount: cards.length,
                itemBuilder: (context, index) {
                  final card = cards[index];
                  return Dismissible(
                    key: ValueKey(card.id),
                    direction: DismissDirection.endToStart,
                    background: Container(
                      alignment: Alignment.centerRight,
                      padding: const EdgeInsets.symmetric(horizontal: 20),
                      color: Theme.of(context).colorScheme.errorContainer,
                      child: const Icon(Icons.done),
                    ),
                    confirmDismiss: (_) async {
                      await _dismiss(card);
                      return false;
                    },
                    child: Card(
                      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      child: ListTile(
                        title: Text(card.title),
                        subtitle: Padding(
                          padding: const EdgeInsets.only(top: 6),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(card.summary),
                              const SizedBox(height: 6),
                              Text(
                                '${card.source} · ${MaterialLocalizations.of(context).formatShortDate(card.sortAt)} ${MaterialLocalizations.of(context).formatTimeOfDay(TimeOfDay.fromDateTime(card.sortAt))}',
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            ],
                          ),
                        ),
                        trailing: IconButton(
                          tooltip: 'Dismiss',
                          icon: const Icon(Icons.done),
                          onPressed: () => _dismiss(card),
                        ),
                        isThreeLine: true,
                      ),
                    ),
                  );
                },
              ),
            );
          },
        ),
      );
}
