import 'package:flutter/foundation.dart' show TargetPlatform, defaultTargetPlatform, kIsWeb;
import 'package:flutter/material.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import 'device_auth_client.dart';

/// Supply these from the app's active localization bundle when wiring the route.
class EnrollmentLabels {
  const EnrollmentLabels({
    required this.title,
    required this.description,
    required this.inviteCode,
    required this.inviteHint,
    required this.oneTimeNote,
    required this.emptyInvite,
    required this.enroll,
    required this.scan,
    required this.scannerTitle,
    required this.invalidInvite,
    required this.networkError,
    required this.scanError,
  });

  final String title;
  final String description;
  final String inviteCode;
  final String inviteHint;
  final String oneTimeNote;
  final String emptyInvite;
  final String enroll;
  final String scan;
  final String scannerTitle;
  final String invalidInvite;
  final String networkError;
  final String scanError;
}

/// Mount as a route when no device token exists. [onEnrolled] opens the app.
class EnrollmentScreen extends StatefulWidget {
  const EnrollmentScreen({
    super.key,
    required this.client,
    required this.labels,
    required this.deviceName,
    required this.platform,
    required this.onEnrolled,
  });

  final DeviceAuthClient client;
  final EnrollmentLabels labels;
  final String deviceName;
  final String platform;
  final ValueChanged<DeviceEnrollment> onEnrolled;

  @override
  State<EnrollmentScreen> createState() => _EnrollmentScreenState();
}

class _EnrollmentScreenState extends State<EnrollmentScreen> {
  final _code = TextEditingController();
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _enroll() async {
    if (_submitting) return;
    if (_code.text.trim().isEmpty) {
      setState(() => _error = widget.labels.emptyInvite);
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final enrolled = await widget.client.enroll(
        inviteCode: _code.text,
        deviceName: widget.deviceName,
        platform: widget.platform,
      );
      if (mounted) widget.onEnrolled(enrolled);
    } on EnrollmentException catch (error) {
      if (mounted) {
        setState(() => _error = error.statusCode == 410
            ? widget.labels.invalidInvite
            : widget.labels.networkError);
      }
    } catch (_) {
      if (mounted) setState(() => _error = widget.labels.networkError);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _scan() async {
    final invite = await Navigator.of(context).push<String>(
      MaterialPageRoute(
        builder: (_) => _InviteScanner(labels: widget.labels),
      ),
    );
    if (invite == null || !mounted) return;
    _code.text = invite;
    await _enroll();
  }

  bool get _scannerAvailable => kIsWeb ||
      const {
        TargetPlatform.android,
        TargetPlatform.iOS,
        TargetPlatform.macOS,
      }.contains(defaultTargetPlatform);

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: Text(widget.labels.title)),
        body: SafeArea(
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: ListView(
                padding: const EdgeInsets.all(24),
                children: [
                  const SizedBox(height: 24),
                  Center(
                    child: Icon(
                      Icons.phonelink_setup,
                      size: 72,
                      color: Theme.of(context).colorScheme.primary,
                    ),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    widget.labels.description,
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.headlineSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    widget.labels.oneTimeNote,
                    textAlign: TextAlign.center,
                    style: Theme.of(context).textTheme.bodyMedium,
                  ),
                  const SizedBox(height: 28),
                  TextField(
                    controller: _code,
                    decoration: InputDecoration(
                      labelText: widget.labels.inviteCode,
                      hintText: widget.labels.inviteHint,
                      errorText: _error,
                      border: const OutlineInputBorder(),
                    ),
                    onChanged: (_) {
                      if (_error != null) setState(() => _error = null);
                    },
                    onSubmitted: (_) => _enroll(),
                  ),
                  const SizedBox(height: 16),
                  FilledButton(
                    onPressed: _submitting ? null : _enroll,
                    child: _submitting
                        ? const SizedBox(
                            width: 20,
                            height: 20,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Text(widget.labels.enroll),
                  ),
                  if (_scannerAvailable) ...[
                    const SizedBox(height: 8),
                    OutlinedButton.icon(
                      onPressed: _submitting ? null : _scan,
                      icon: const Icon(Icons.qr_code_scanner),
                      label: Text(widget.labels.scan),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      );
}

class _InviteScanner extends StatefulWidget {
  const _InviteScanner({required this.labels});

  final EnrollmentLabels labels;

  @override
  State<_InviteScanner> createState() => _InviteScannerState();
}

class _InviteScannerState extends State<_InviteScanner> {
  bool _found = false;

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: Text(widget.labels.scannerTitle)),
        body: MobileScanner(
          onDetect: (capture) {
            if (_found) return;
            for (final barcode in capture.barcodes) {
              final code = barcode.rawValue?.trim();
              if (code == null || code.isEmpty) continue;
              _found = true;
              Navigator.of(context).pop(code);
              break;
            }
          },
          errorBuilder: (context, error) => Center(
            child: Text(widget.labels.scanError),
          ),
        ),
      );
}
