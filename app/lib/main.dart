import 'package:flutter/material.dart';

import 'db/database.dart';

void main() {
  runApp(LitterboxApp(database: AppDatabase()));
}

class LitterboxApp extends StatelessWidget {
  final AppDatabase database;

  const LitterboxApp({super.key, required this.database});

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'Litterbox',
        home: InboxScreen(database: database),
      );
}

class InboxScreen extends StatelessWidget {
  final AppDatabase database;

  const InboxScreen({super.key, required this.database});

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('Inbox')),
        body: StreamBuilder<List<CardRow>>(
          stream: database.watchOpenCards(),
          builder: (context, snapshot) {
            final cards = snapshot.data ?? const <CardRow>[];
            if (cards.isEmpty) {
              return const Center(child: Text('Inbox is empty'));
            }
            return ListView.builder(
              itemCount: cards.length,
              itemBuilder: (context, index) {
                final card = cards[index];
                return ListTile(
                  title: Text(card.subject),
                  subtitle: Text(card.sender),
                );
              },
            );
          },
        ),
      );
}
