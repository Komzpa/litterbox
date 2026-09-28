// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get pinned => 'Закреплённые';

  @override
  String get archiveBundle => 'Архивировать группу';

  @override
  String get pin => 'Закрепить';

  @override
  String get unpin => 'Открепить';

  @override
  String get takeOutOfBundle => 'Убрать из группы';

  @override
  String get snooze => 'Отложить';

  @override
  String get laterToday => 'Позже сегодня';

  @override
  String get tomorrowMorning => 'Завтра утром';

  @override
  String get nextWeek => 'На следующей неделе';

  @override
  String get customDateTime => 'Выбрать дату и время';

  @override
  String bundleCount(Object bundle, int count) {
    return '$bundle ($count)';
  }

  @override
  String get createReminder => 'Добавить напоминание';

  @override
  String get reminderTitle => 'Название';

  @override
  String get reminderWhen => 'Дата и время';

  @override
  String get create => 'Создать';
}
