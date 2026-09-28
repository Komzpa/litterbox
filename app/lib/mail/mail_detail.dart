import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'dart:convert';
import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' as html_parser;
import 'package:url_launcher/url_launcher.dart';

class MailDetail extends StatelessWidget {
  const MailDetail({super.key, required this.html, required this.threadId, required this.accountId});
  final String html;
  final String threadId;
  final String accountId;

  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: AppBar(title: const Text('Mail')),
    body: Column(children: [
      Expanded(child: SingleChildScrollView(child: _MailHtml(html: html))),
      Padding(padding: const EdgeInsets.all(16), child: FilledButton.icon(
        onPressed: () => openInGmail(threadId: threadId, accountId: accountId),
        icon: const Icon(Icons.open_in_new), label: const Text('Open in Gmail'),
      )),
    ]),
  );
}

String gmailThreadUrl(String threadId, String accountId) =>
  'https://mail.google.com/mail/u/${Uri.encodeComponent(accountId)}/#all/${Uri.encodeComponent(threadId)}';

Future<void> openInGmail({required String threadId, required String accountId}) async {
  const channel = MethodChannel('io.github.komzpa.litterbox/gmail');
  final url = gmailThreadUrl(threadId, accountId);
  try {
    await channel.invokeMethod<void>('openThread', {'threadId': threadId, 'url': url});
  } on MissingPluginException {
    await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
  }
}



class _MailHtml extends StatelessWidget {
  const _MailHtml({required this.html});
  final String html;

  @override
  Widget build(BuildContext context) {
    final fragment = html_parser.parseFragment(html);
    return Column(crossAxisAlignment: CrossAxisAlignment.stretch,
      children: fragment.nodes.expand(_renderNode).toList());
  }

  List<Widget> _renderNode(dom.Node node, [TextStyle style = const TextStyle()]) {
    if (node is dom.Text) {
      final text = node.text;
      return text.trim().isEmpty ? const [] : [Text(text, style: style)];
    }
    if (node is! dom.Element) return const [];
    final tag = node.localName?.toLowerCase() ?? '';
    if (tag == 'img') {
      final src = node.attributes['src'] ?? '';
      final match = RegExp(r'^data:image/[^;]+;base64,(.*)$', caseSensitive: false).firstMatch(src);
      if (match == null) return const [];
      try { return [Padding(padding: const EdgeInsets.symmetric(vertical: 8), child: Image.memory(base64Decode(match.group(1)!), fit: BoxFit.contain))]; } catch (_) { return const []; }
    }
    if (tag == 'br') return const [SizedBox(height: 8)];
    var childStyle = style;
    if (tag == 'b' || tag == 'strong') childStyle = style.merge(const TextStyle(fontWeight: FontWeight.bold));
    if (tag == 'i' || tag == 'em') childStyle = style.merge(const TextStyle(fontStyle: FontStyle.italic));
    if (tag == 'h1' || tag == 'h2' || tag == 'h3') childStyle = style.merge(TextStyle(fontSize: tag == 'h1' ? 24 : tag == 'h2' ? 20 : 18, fontWeight: FontWeight.bold));
    final children = node.nodes.expand((child) => _renderNode(child, childStyle)).toList();
    if (children.isEmpty) return const [];
    if (tag == 'p' || tag == 'div' || tag == 'section' || tag.startsWith('h')) {
      return [Padding(padding: const EdgeInsets.symmetric(vertical: 4), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: children))];
    }
    return children;
  }
}
