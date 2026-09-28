import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

const _serverUrl = String.fromEnvironment('LITTERBOX_URL');
const buildSha = String.fromEnvironment('BUILD_SHA', defaultValue: 'unknown');
const kClientApi = 1;
const _apiHeaders = {'X-Litterbox-Api': '$kClientApi'};

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

typedef ServerVersion = ({int api, int minClientApi, String build});

class CardsApi {
  final String baseUrl;
  final http.Client client;

  CardsApi(this.baseUrl, {http.Client? client}) : client = client ?? http.Client();

  Future<ServerVersion> version() async {
    final response = await client.get(Uri.parse('$baseUrl/v1/version'), headers: _apiHeaders);
    if (response.statusCode == 404) throw OldServerException();
    if (response.statusCode != 200) throw StateError('Version request failed (${response.statusCode})');
    try {
      final json = jsonDecode(response.body) as Map<String, dynamic>;
      return (api: json['api'] as int, minClientApi: json['min_client_api'] as int,
          build: json['server_build'] as String);
    } on FormatException { throw OldServerException(); }
      on TypeError { throw OldServerException(); }
  }

  Future<http.StreamedResponse> events() {
    final request = http.Request('GET', Uri.parse('$baseUrl/v1/cards/events'));
    request.headers.addAll(_apiHeaders);
    return client.send(request);
  }

  Future<CardSections> fetchCards() async {
    final response = await client.get(Uri.parse('$baseUrl/v1/cards'), headers: _apiHeaders);
    if (response.statusCode != 200) throw Exception('Server returned ${response.statusCode}');
    return CardSections.fromJson(jsonDecode(response.body) as Map<String, dynamic>);
  }

  Future<void> dismiss(String id, {String? note}) async {
    final response = await client.post(Uri.parse('$baseUrl/v1/cards/$id/dismiss'),
        headers: {..._apiHeaders, 'content-type': 'application/json'},
        body: jsonEncode({if (note != null && note.isNotEmpty) 'note': note}));
    if (response.statusCode < 200 || response.statusCode >= 300) throw Exception('Dismiss failed (${response.statusCode})');
  }

  Future<void> saveNote(String id, String note) async {
    final response = await client.post(Uri.parse('$baseUrl/v1/cards/$id/note'),
        headers: {..._apiHeaders, 'content-type': 'application/json'}, body: jsonEncode({'text': note}));
    if (response.statusCode < 200 || response.statusCode >= 300) throw Exception('Saving note failed (${response.statusCode})');
  }
}

class InboxScreen extends StatefulWidget {
  final CardsApi api;

  const InboxScreen({super.key, required this.api});

  @override
  State<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends State<InboxScreen> {
  late Future<CardSections> _cards;
  StreamSubscription<String>? _events;
  Timer? _reconnect, _nextSlot;
  int _retrySeconds = 1;
  int? _requiredApi;
  String? _serverBuild;
  bool _serverOld = false;

  @override
  void initState() {
    super.initState();
    _cards = _fetchCards();
    _cards.then(_scheduleNextSlot, onError: (Object _) {});
    _connectEvents();
    _checkVersion();
  }

  @override
  void dispose() {
    _events?.cancel();
    _reconnect?.cancel();
    _nextSlot?.cancel();
    super.dispose();
  }

  Future<CardSections> _fetchCards() async {
    try { return await widget.api.fetchCards(); }
    on FormatException { if (mounted) setState(() { _serverOld = true; }); rethrow; }
    on TypeError { if (mounted) setState(() { _serverOld = true; }); rethrow; }
  }

  Future<void> _checkVersion() async {
    try {
      final version = await widget.api.version();
      if (mounted) {
        setState(() {
          _serverBuild = version.build;
          if (version.api < kClientApi) _serverOld = true;
          _requiredApi = version.minClientApi > kClientApi ? version.minClientApi : null;
        });
      }
    } on OldServerException {
      if (mounted) setState(() { _serverOld = true; });
    } catch (_) {
      // Keep the last known warning when the server cannot be reached.
    }
  }

  Future<void> _connectEvents() async {
    try {
      final response = await widget.api.events();
      if (!mounted) return;
      if (response.statusCode != 200) throw StateError('Event stream returned ${response.statusCode}');
      _retrySeconds = 1;
      _checkVersion();
      var cardEvent = false;
      _events = response.stream.transform(utf8.decoder).transform(const LineSplitter()).listen((line) {
        if (line.startsWith('event:')) { cardEvent = line.substring(6).trim() == 'cards'; }
        if (line.isEmpty) {
          if (cardEvent && mounted) unawaited(_refresh().catchError((Object _) {}));
          cardEvent = false;
        }
      }, onError: (Object _) => _scheduleReconnect(), onDone: _scheduleReconnect, cancelOnError: true);
    } catch (_) { _scheduleReconnect(); }
  }

  void _scheduleReconnect() {
    if (!mounted) return;
    _events?.cancel();
    _reconnect?.cancel();
    _reconnect = Timer(Duration(seconds: _retrySeconds), _connectEvents);
    _retrySeconds = (_retrySeconds * 2).clamp(1, 10);
  }

  void _scheduleNextSlot(CardSections sections) {
    if (!mounted) return;
    _nextSlot?.cancel();
    final current = DateTime.now();
    final futureSlots = sections.later.map((card) => card.at).whereType<DateTime>()
        .where((at) => at.isAfter(current)).toList()..sort();
    if (futureSlots.isNotEmpty) {
      _nextSlot = Timer(futureSlots.first.difference(current) + const Duration(milliseconds: 100),
          () => unawaited(_refresh().catchError((Object _) {})));
    }
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
    appBar: AppBar(title: const Text('Litterbox'), actions: [
      PopupMenuButton<String>(onSelected: (value) { if (value == 'restart') widget.onRestart(); }, itemBuilder: (context) => [
        PopupMenuItem(value: 'restart', child: Text(AppLocalizations.of(context)!.restart)),
        PopupMenuItem(enabled: false, child: Text(AppLocalizations.of(context)!.aboutBuild(buildSha, _serverBuild ?? '?'))),
      ]),
    ]),
    body: _serverOld ? Center(child: Text(AppLocalizations.of(context)!.oldServer)) : Column(children: [
      if (_requiredApi != null) MaterialBanner(
        content: Text(AppLocalizations.of(context)!.outdatedApi(kClientApi, _requiredApi!)),
        actions: [TextButton(onPressed: widget.onRestart, child: Text(AppLocalizations.of(context)!.restart))],
      ),
      Expanded(child: FutureBuilder<CardSections>(future: _cards, builder: (context, snapshot) {
      if (snapshot.hasError) { return Center(child: Column(mainAxisSize: MainAxisSize.min, children: [
        Text(AppLocalizations.of(context)!.loadError), TextButton(onPressed: _refresh, child: Text(AppLocalizations.of(context)!.retry)),
      ])); }
      if (!snapshot.hasData) return const Center(child: CircularProgressIndicator());
      final sections = snapshot.data!;
      return ListView(physics: const AlwaysScrollableScrollPhysics(), children: [
        _section(AppLocalizations.of(context)!.nowSection, sections.now),
        _section(AppLocalizations.of(context)!.laterSection, sections.later),
        ExpansionTile(title: Text(AppLocalizations.of(context)!.missedCount(sections.missed.length)), children: sections.missed.map(_card).toList()),
      ]);
    })),
    ]),
  );
}
