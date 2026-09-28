// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for English (`en`).
class AppLocalizationsEn extends AppLocalizations {
  AppLocalizationsEn([String locale = 'en']) : super(locale);

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
}
