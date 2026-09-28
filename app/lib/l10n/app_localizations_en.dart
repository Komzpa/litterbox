// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

  @override
  String get nowSection => 'Now';

  @override
  String get laterSection => 'Later';

  @override
  String missedCount(int count) {
    return 'Missed ($count)';
  }

  @override
  String get emptyCards => 'No cards';

  @override
  String get retry => 'Retry';

  @override
  String get oldServer => 'Server is older than the app';

  @override
  String get restart => 'Restart';

  @override
  String get done => 'Done';

  @override
  String get note => 'Note';

  @override
  String get noteHint => 'What should the task generator know?';

  @override
  String get cancel => 'Cancel';

  @override
  String get save => 'Save';

  @override
  String get doneWithNote => 'Done with note';

  @override
  String get loadError => 'Could not load cards';

  @override
  String actionError(Object error) {
    return 'Could not complete action: $error';
  }

  @override
  String noteSaveError(Object error) {
    return 'Could not save note: $error';
  }

  @override
  String outdatedApi(int client, int server) {
    return 'App is outdated (API $client), server needs $server';
  }

  @override
  String aboutBuild(Object clientBuild, Object serverBuild) {
    return 'About · app $clientBuild, server $serverBuild';
  }

  @override
  String get createReminder => 'Add reminder';

  @override
  String get reminderTitle => 'Title';

  @override
  String get reminderWhen => 'Date and time';

  @override
  String get create => 'Create';

  @override
  String get pinned => 'Pinned';

  @override
  String get archiveBundle => 'Archive bundle';

  @override
  String get pin => 'Pin';

  @override
  String get unpin => 'Unpin';

  @override
  String get takeOutOfBundle => 'Take out of bundle';

  @override
  String get snooze => 'Snooze';

  @override
  String get laterToday => 'Later today';

  @override
  String get tomorrowMorning => 'Tomorrow morning';

  @override
  String get nextWeek => 'Next week';

  @override
  String get customDateTime => 'Custom date/time';

  @override
  String bundleCount(Object bundle, int count) {
    return '$bundle ($count)';
  }

  @override
  String get journal => 'Journal';

  @override
  String get journalEntryHint => 'Write a journal entry';

  @override
  String get mcpSettings => 'MCP tokens';

  @override
  String get createToken => 'Create token';

  @override
  String get revokeToken => 'Revoke';

  @override
  String get tokenShownOnce =>
      'Copy this token now. It will not be shown again.';
}
