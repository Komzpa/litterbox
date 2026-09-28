import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'l10n/app_localizations.dart';
import 'package:http/http.dart' as http;
import 'package:intl/date_symbol_data_local.dart';
import 'package:intl/intl.dart';

const _serverUrl = String.fromEnvironment('LITTERBOX_URL');
const buildSha = String.fromEnvironment('BUILD_SHA', defaultValue: 'unknown');
const kClientApi = 1;
const _apiHeaders = {'X-Litterbox-Api': '$kClientApi'};

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Future.wait(['en', 'ru', 'be'].map(initializeDateFormatting));
  runApp(LitterboxApp(api: CardsApi(_serverUrl)));
}

class LitterboxApp extends StatefulWidget {
  final CardsApi api;
  const LitterboxApp({super.key, required this.api});
  @override
  State<LitterboxApp> createState() => _LitterboxAppState();
}

class _LitterboxAppState extends State<LitterboxApp> {
  int _generation = 0;
  late final DateTime? _binaryModified = Platform.isLinux
      ? File(Platform.resolvedExecutable).lastModifiedSync() : null;

  Future<void> _restart() async {
    if (Platform.isLinux && _binaryModified != null) {
      try {
        if (File(Platform.resolvedExecutable).lastModifiedSync().isAfter(_binaryModified)) {
          await Process.start(Platform.resolvedExecutable, [], mode: ProcessStartMode.detached);
          exit(0);
        }
      } on FileSystemException {
        // A changed executable may no longer be available; restart the widget tree.
      }
    }
    if (mounted) setState(() { _generation++; });
  }

  @override
  Widget build(BuildContext context) => MaterialApp(
        key: ValueKey(_generation),
        title: 'Litterbox',
        supportedLocales: AppLocalizations.supportedLocales,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        theme: ThemeData(colorSchemeSeed: Colors.deepPurple),
        builder: (context, child) => CallbackShortcuts(
          bindings: {const SingleActivator(LogicalKeyboardKey.keyR, control: true, shift: true): _restart},
          child: Focus(autofocus: true, child: child!),
        ),
        home: InboxScreen(api: widget.api, onRestart: _restart),
      );
}

class InboxCard {
  final String id, source, title, summary, state, note;
  final DateTime? at;
  final bool timed;
  const InboxCard({required this.id, required this.source, required this.title, required this.summary, required this.at, required this.timed, required this.state, required this.note});

  factory InboxCard.fromJson(Map<String, dynamic> json) => InboxCard(
        id: json['id'] as String,
        source: json['source'] as String,
        title: json['title'] as String,
        summary: json['summary'] as String? ?? '',
        at: json['at'] == null ? null : DateTime.parse(json['at'] as String),
        timed: json['timed'] as bool? ?? false,
        state: json['state'] as String? ?? 'open',
        note: json['note'] as String? ?? '',
      );

  DateTime? get localTime => at?.toLocal();
}

class OldServerException implements Exception {}

class CardSections {
  final List<InboxCard> now, later, missed;
  const CardSections({required this.now, required this.later, required this.missed});

  factory CardSections.fromJson(Map<String, dynamic> json) {
    if (json['now'] is! List || json['later'] is! List || json['missed'] is! List) {
      throw const FormatException('Old cards response');
    }
    return CardSections(now: _cards(json['now']), later: _cards(json['later']), missed: _cards(json['missed']));
  }

  static List<InboxCard> _cards(dynamic value) => (value as List<dynamic>? ?? [])
      .map((item) => InboxCard.fromJson(item as Map<String, dynamic>))
      .where((card) => card.state != 'done').toList();
}

typedef ServerVersion = ({int api, int minClientApi, String build});

class CardsApi {
  final String baseUrl;
  final http.Client client;
  CardsApi(this.baseUrl, {http.Client? client}) : client = client ?? http.Client();
  String? token;
  Map<String, String> get headers => {..._apiHeaders, if (token != null) 'Authorization': 'Bearer $token'};

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
extension GmailAccountsApi on CardsApi {
  Future<List<GmailAccount>> gmailAccounts() async {
    final response = await client.get(Uri.parse('$baseUrl/v1/gmail/accounts'), headers: headers);
    if (response.statusCode != 200) throw StateError('Could not load Gmail accounts (${response.statusCode})');
    return (jsonDecode(response.body) as List<dynamic>)
        .map((account) => GmailAccount.fromJson(account as Map<String, dynamic>)).toList();
  }

  Future<Uri> beginGmailConnect() async {
    final response = await client.post(Uri.parse('$baseUrl/v1/gmail/connect'), headers: headers);
    if (response.statusCode != 200) throw StateError('Could not start Gmail connection (${response.statusCode})');
    final url = (jsonDecode(response.body) as Map<String, dynamic>)['authorization_url'] as String;
    return Uri.parse(url);
  }

  Future<void> disconnectGmailAccount(String id) async {
    final response = await client.delete(Uri.parse('$baseUrl/v1/gmail/accounts/$id'), headers: headers);
    if (response.statusCode < 200 || response.statusCode >= 300) {
      throw StateError('Could not disconnect Gmail account (${response.statusCode})');
    }
  }
}

class GmailAccount {
  final String id;
  final String address;
  const GmailAccount({required this.id, required this.address});

  factory GmailAccount.fromJson(Map<String, dynamic> json) => GmailAccount(
        id: json['id'] as String, address: json['address'] as String);
}

class GmailAccountsScreen extends StatefulWidget {
  final CardsApi api;
  const GmailAccountsScreen({super.key, required this.api});
  @override
  State<GmailAccountsScreen> createState() => _GmailAccountsScreenState();
}

class _GmailAccountsScreenState extends State<GmailAccountsScreen> {
  late Future<List<GmailAccount>> _accounts;
  bool _busy = false;
  String? _error;
  Uri? _authorizationUrl;

  @override
  void initState() {
    super.initState();
    _accounts = widget.api.gmailAccounts();
  }

  void _refresh() {
    setState(() {
      _error = null;
      _accounts = widget.api.gmailAccounts();
    });
  }

  Future<void> _connect() async {
    setState(() { _busy = true; _error = null; _authorizationUrl = null; });
    try {
      final url = await widget.api.beginGmailConnect();
      if (mounted) setState(() { _authorizationUrl = url; });
    } catch (error) {
      if (mounted) setState(() { _error = error.toString(); });
    } finally {
      if (mounted) setState(() { _busy = false; });
    }
  }

  Future<void> _disconnect(GmailAccount account) async {
    final confirmed = await showDialog<bool>(context: context, builder: (context) => AlertDialog(
      title: const Text('Disconnect Gmail account?'),
      content: Text('${account.address} will no longer sync with Litterbox.'),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('Cancel')),
        FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Disconnect')),
      ],
    ));
    if (confirmed != true) return;
    setState(() { _busy = true; _error = null; });
    try {
      await widget.api.disconnectGmailAccount(account.id);
      if (mounted) _refresh();
    } catch (error) {
      if (mounted) setState(() { _error = error.toString(); });
    } finally {
      if (mounted) setState(() { _busy = false; });
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Gmail accounts'), actions: [
      IconButton(tooltip: 'Refresh accounts', onPressed: _busy ? null : _refresh, icon: const Icon(Icons.refresh)),
    ]),
    body: ListView(padding: const EdgeInsets.all(16), children: [
      FilledButton.icon(
        onPressed: _busy ? null : _connect,
        icon: _busy ? const SizedBox.square(dimension: 18, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.add),
        label: const Text('Connect Gmail account'),
      ),
      if (_error != null) Padding(
        padding: const EdgeInsets.only(top: 12),
        child: Semantics(liveRegion: true, child: Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
      ),
      if (_authorizationUrl case final url?) Card(
        margin: const EdgeInsets.only(top: 16),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            Text('Open this link in your browser to authorize Gmail, then return and refresh.',
                style: Theme.of(context).textTheme.bodyLarge),
            const SizedBox(height: 8),
            SelectableText(url.toString(), semanticsLabel: 'Gmail authorization link'),
            Align(alignment: Alignment.centerRight, child: TextButton.icon(
              onPressed: () => Clipboard.setData(ClipboardData(text: url.toString())),
              icon: const Icon(Icons.copy), label: const Text('Copy link'),
            )),
          ]),
        ),
      ),
      const Padding(
        padding: EdgeInsets.only(top: 24, bottom: 8),
        child: Text('Connected accounts', style: TextStyle(fontSize: 18, fontWeight: FontWeight.w600)),
      ),
      FutureBuilder<List<GmailAccount>>(future: _accounts, builder: (context, snapshot) {
        if (snapshot.hasError) return const Text('Could not load Gmail accounts. Use Refresh accounts to try again.');
        if (!snapshot.hasData) return const Center(child: CircularProgressIndicator(semanticsLabel: 'Loading Gmail accounts'));
        if (snapshot.data!.isEmpty) return const Text('No Gmail accounts connected.');
        return Column(children: snapshot.data!.map((account) => Card(
          child: ListTile(
            leading: const Icon(Icons.mail_outline),
            title: Text(account.address),
            trailing: IconButton(
              tooltip: 'Disconnect ${account.address}',
              onPressed: _busy ? null : () => _disconnect(account),
              icon: const Icon(Icons.link_off),
            ),
          ),
        )).toList());
      }),
    ]),
  );
}

class InboxScreen extends StatefulWidget {
  final CardsApi api;
  final VoidCallback onRestart;
  const InboxScreen({super.key, required this.api, required this.onRestart});
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
    final cards = _fetchCards();
    if (!mounted) return;
    setState(() { _cards = cards; });
    final sections = await cards;
    _scheduleNextSlot(sections);
  }

  Future<void> _dismiss(InboxCard card) async {
    try { await widget.api.dismiss(card.id); await _refresh(); }
    catch (error) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(AppLocalizations.of(context)!.actionError(error.toString())))); }
  }

  Future<void> _editNote(InboxCard card) async {
    var draft = card.note;
    final action = await showDialog<(String, bool)>(context: context, builder: (context) => AlertDialog(
      title: Text(AppLocalizations.of(context)!.note),
      content: TextFormField(initialValue: draft, onChanged: (value) => draft = value, autofocus: true, maxLines: 5,
          decoration: InputDecoration(hintText: AppLocalizations.of(context)!.noteHint)),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text(AppLocalizations.of(context)!.cancel)),
        TextButton(onPressed: () => Navigator.pop(context, (draft, true)), child: Text(AppLocalizations.of(context)!.doneWithNote)),
        FilledButton(onPressed: () => Navigator.pop(context, (draft, false)), child: Text(AppLocalizations.of(context)!.save)),
      ],
    ));
    if (action == null) return;
    try {
      if (action.$2) { await widget.api.dismiss(card.id, note: action.$1); }
      else { await widget.api.saveNote(card.id, action.$1); }
      await _refresh();
    }
    catch (error) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(AppLocalizations.of(context)!.noteSaveError(error.toString())))); }
  }

  String _time(InboxCard card) {
    final time = card.localTime;
    if (time == null) return '';
    final locale = Localizations.localeOf(context).toString();
    final pattern = MediaQuery.alwaysUse24HourFormatOf(context) ? DateFormat.Hm(locale) : DateFormat.jm(locale);
    final today = DateTime.now();
    final sameDay = time.year == today.year && time.month == today.month && time.day == today.day;
    return sameDay ? pattern.format(time) : '${DateFormat.yMMMd(locale).format(time)} ${pattern.format(time)}';
  }

  Widget _card(InboxCard card) => Card(
    margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 5),
    child: Padding(padding: const EdgeInsets.fromLTRB(16, 12, 8, 8), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Expanded(child: Text(card.title, style: Theme.of(context).textTheme.titleMedium)),
        if (card.timed && card.at != null) Padding(padding: const EdgeInsets.only(left: 8), child: Text(_time(card), style: Theme.of(context).textTheme.titleSmall)),
      ]),
      if (card.summary.isNotEmpty) ...[const SizedBox(height: 6), Text(card.summary, maxLines: 3, overflow: TextOverflow.ellipsis)],
      if (card.note.isNotEmpty) ...[const SizedBox(height: 6), Text(card.note, style: Theme.of(context).textTheme.bodySmall)],
      const SizedBox(height: 4),
      Row(children: [
        Chip(label: Text(card.source), visualDensity: VisualDensity.compact), const Spacer(),
        IconButton(tooltip: AppLocalizations.of(context)!.note, onPressed: () => _editNote(card), icon: const Icon(Icons.note_add_outlined)),
        IconButton(tooltip: AppLocalizations.of(context)!.done, onPressed: () => _dismiss(card), icon: const Icon(Icons.check)),
      ]),
    ])),
  );

  Widget _section(String title, List<InboxCard> cards) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
    Padding(padding: const EdgeInsets.fromLTRB(16, 18, 16, 4), child: Text(title, style: Theme.of(context).textTheme.titleLarge)),
    if (cards.isEmpty) Padding(padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8), child: Text(AppLocalizations.of(context)!.emptyCards)) else ...cards.map(_card),
  ]);

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Litterbox'), actions: [
      IconButton(tooltip: 'Gmail accounts', onPressed: () => Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => GmailAccountsScreen(api: widget.api))), icon: const Icon(Icons.email_outlined)),
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
