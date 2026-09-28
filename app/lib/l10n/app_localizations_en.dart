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
}
