// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Belarusian (`be`).
class AppLocalizationsBe extends AppLocalizations {
  AppLocalizationsBe([String locale = 'be']) : super(locale);

  @override
  String get pinned => 'Замацаваныя';

  @override
  String get archiveBundle => 'Архіваваць групу';

  @override
  String get pin => 'Замацаваць';

  @override
  String get unpin => 'Адмацаваць';

  @override
  String get takeOutOfBundle => 'Прыбраць з групы';

  @override
  String get snooze => 'Адкласці';

  @override
  String get laterToday => 'Пазней сёння';

  @override
  String get tomorrowMorning => 'Заўтра раніцай';

  @override
  String get nextWeek => 'На наступным тыдні';

  @override
  String get customDateTime => 'Выбраць дату і час';

  @override
  String bundleCount(Object bundle, int count) {
    return '$bundle ($count)';
  }

  @override
  String get createReminder => 'Дадаць напамін';

  @override
  String get reminderTitle => 'Назва';

  @override
  String get reminderWhen => 'Дата і час';

  @override
  String get create => 'Стварыць';
}
